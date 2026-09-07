package tui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const sc009Runs = 20

type sc009ProgramModel struct {
	model *Model
	init  tea.Cmd
}

func (m *sc009ProgramModel) Init() tea.Cmd { return m.init }

func (m *sc009ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, command := m.model.Update(msg)
	return m, command
}

func (m *sc009ProgramModel) View() tea.View { return m.model.View() }

func sc009Update(t *testing.T, model *Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	updated, command := model.Update(msg)
	if updated != model {
		t.Fatal("Update replaced the root model")
	}
	assertSC009NoColorOwner(t, model)
	return command
}

func sc009RunCommand(t *testing.T, model *Model, command tea.Cmd) tea.Cmd {
	t.Helper()
	if command == nil {
		t.Fatal("expected command")
	}
	return sc009Update(t, model, command())
}

func sc009RunExec(t *testing.T, model *Model, command tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatal("expected terminal command")
	}
	program := tea.NewProgram(
		&sc009ProgramModel{model: model, init: command},
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
	)
	if _, err := program.Run(); err != nil {
		t.Fatalf("terminal command: %v", err)
	}
	assertSC009NoColorOwner(t, model)
}

func assertSC009NoColorOwner(t *testing.T, model *Model) {
	t.Helper()
	view := model.View().Content
	assertSCNoANSI(t, view)
	if calculateLayout(model.width, model.height, model.focusedLayoutRegion()).mode == layoutUndersized {
		return
	}
	want := "[*] Tree"
	switch model.focusOwner {
	case focusOwnerDetail, focusOwnerConnectionForm:
		want = "[*] Details"
	case focusOwnerModal:
		want = "[*] " + modalTitle(model.modal.kind)
	}
	if !strings.Contains(view, want) {
		t.Fatalf("no-color frame omitted focus owner %q:\n%s", want, view)
	}
}

func sc009Snapshot(model *Model, root app.Folder, folders []app.Folder, connections []app.Connection, selected app.NodeID) {
	snapshot := newCatalogSnapshot(root, 1)
	byParentFolders := make(map[app.NodeID][]app.Folder)
	byParentConnections := make(map[app.NodeID][]app.Connection)
	for _, folder := range folders {
		byParentFolders[folder.ParentID] = append(byParentFolders[folder.ParentID], folder)
	}
	for _, connection := range connections {
		byParentConnections[connection.ParentID] = append(byParentConnections[connection.ParentID], connection)
	}
	pending := []app.NodeID{root.ID}
	for len(pending) != 0 {
		parent := pending[0]
		pending = pending[1:]
		children := app.ListChildrenResult{Folders: byParentFolders[parent], Connections: byParentConnections[parent]}
		_ = snapshot.addChildren(parent, children)
		for _, folder := range children.Folders {
			pending = append(pending, folder.ID)
		}
	}
	model.browser.setSnapshot(snapshot, selected)
	model.browser.selectedID = selected
	model.browser.expandAncestors(selected)
	model.browser.rebuildRows()
	model.ownedSelectionID = selected
	model.syncDetail()
}

func sc009FolderReader(root app.Folder, contents func(app.NodeID) app.ListChildrenResult) (func(context.Context, app.ItemSelector) (app.FolderResult, error), func(context.Context, app.ListChildrenRequest) (app.ListChildrenResult, error)) {
	get := func(_ context.Context, selector app.ItemSelector) (app.FolderResult, error) {
		if selector.Path == "/" || selector.ID == root.ID {
			return app.FolderResult{Folder: root, CatalogRevision: 2}, nil
		}
		for _, folder := range contents(root.ID).Folders {
			if folder.ID == selector.ID {
				return app.FolderResult{Folder: folder, CatalogRevision: 2}, nil
			}
		}
		return app.FolderResult{}, app.ErrNotFound
	}
	list := func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
		result := contents(request.Folder.ID)
		result.CatalogRevision = 2
		return result, nil
	}
	return get, list
}

func TestSC009ClosedModalInvariantMatrix(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 3)
	destination := testFolder("destination", root.ID, "/destination", 2)
	connection := testConnection("connection", folder.ID, "/folder/connection", 4)
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()

	tests := []struct {
		name    string
		kind    modalKind
		payload func() any
		helpKey tea.KeyPressMsg
		want    []string
	}{
		{"folder create", modalKindFolderCreate, func() any {
			form := newFolderForm(nil, app.ItemSelector{ID: folder.ID})
			form.setDestination(folder)
			return folderCreatePayload{form: form}
		}, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}), []string{"Create folder", "Target: /folder", "Destination ID: folder", "Name:", "Save", "Cancel"}},
		{"folder edit", modalKindFolderEdit, func() any {
			return folderEditPayload{form: newFolderForm(&folder, app.ItemSelector{})}
		}, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}), []string{"Edit folder", "Target: /folder", "ID/revision: folder/3", "Name:", "Save", "Cancel"}},
		{"move", modalKindMovePicker, func() any {
			return movePickerPayload{picker: newMovePicker(connection.Node, []app.Folder{root, destination})}
		}, keyPress("?"), []string{"Move destination", "Source: /folder/connection", "ID/revision: connection/4", "/destination", "Enter Move", "Esc Cancel"}},
		{"delete connection", modalKindDeleteConnection, func() any {
			return deleteConnectionPayload{confirmation: newDeleteConfirmation(app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision, HasRememberedPassword: true})}
		}, keyPress("?"), []string{"Delete connection?", "Path: /folder/connection", "Target: (default)@host.test", "ID/revision: connection/4", "saved password", "y Confirm", "Enter/Esc Cancel"}},
		{"delete folder", modalKindDeleteFolder, func() any {
			return deleteFolderPayload{confirmation: newFolderDeleteConfirmation(app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 2, Connections: 3, RememberedCredentials: 1})}
		}, keyPress("?"), []string{"Delete folder", "Path: /folder", "ID/revision: folder/3", "Scope: 2 folders, 3 connections, 1 remembered credential", "y Confirm", "Enter/Esc Cancel"}},
		{"connect confirmation", modalKindConnectConfirmation, func() any {
			return connectConfirmationPayload{confirmation: newConnectConfirmation(connection)}
		}, keyPress("?"), []string{"Connect confirmation", "Path: /folder/connection", "Endpoint: host.test:22", "ID: connection", "Revision: 4", "y Confirm", "Enter/Esc Cancel"}},
		{"unsaved changes", modalKindUnsavedChanges, func() any {
			return unsavedChangesPayload{intent: unsavedIntentCancel, target: connection.Path}
		}, keyPress("?"), []string{"Unsaved changes", "Target: /folder/connection", "s Save", "d Discard", "Esc Cancel"}},
		{"help", modalKindHelp, func() any {
			return helpPayload{lines: []string{"Up Move up", "Down Move down", "Esc Close"}}
		}, keyPress("j"), []string{"Help", "Up Move up", "Down Move down", "?/Esc Close"}},
		{"operation error", modalKindOperationError, func() any {
			return operationErrorPayload{modal: newErrorModal("move catalog item", connection.Path, app.ErrInvalidRequest)}
		}, keyPress("?"), []string{"Recoverable operation error", "Operation: move catalog item", "Target: /folder/connection", "Cause:", "b/Esc Back", "q Quit"}},
		{"SSH failure", modalKindSSHFailure, func() any {
			attempt := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
			return sshFailurePayload{modal: newSSHFailureModal(attempt, failure)}
		}, keyPress("?"), []string{"SSH startup failed", "Category: timeout", "Path: /folder/connection", "Endpoint: host.test:22", "ID/revision: connection/4", "Stage: network_connection", "Recommendation:", "d Detail", "r Retry", "e Edit", "b/Esc Back", "q Quit"}},
	}

	if len(tests) != 10 || len(newModalRegistry().payloadTypes) != len(tests) {
		t.Fatalf("closed inventory = %d cases/%d registrations, want 10/10", len(tests), len(newModalRegistry().payloadTypes))
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for run := range sc009Runs {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				sc009Snapshot(model, root, []app.Folder{folder, destination}, []app.Connection{connection}, connection.ID)
				model.focusOwner = focusOwnerDetail
				if !model.openGenericModal(test.kind, nil, test.payload()) {
					t.Fatalf("run %d: open failed", run)
				}
				if model.focusOwner != focusOwnerModal || model.modal.kind != test.kind || !model.modal.blocksBackgroundInput() {
					t.Fatalf("run %d: modal ownership = %v/%q", run, model.focusOwner, model.modal.kind)
				}
				if model.openGenericModal(modalKindHelp, nil, helpPayload{lines: []string{"nested"}}) || model.modal.kind != test.kind {
					t.Fatalf("run %d: second modal occupied the single slot", run)
				}

				layout := calculateLayout(model.width, model.height, model.focusedLayoutRegion())
				rect := layout.modalOverlay()
				if rect.x != (model.width-rect.width)/2 || rect.y != (model.height-rect.height)/2 || rect.x < 0 || rect.y < 0 || rect.right() > model.width || rect.bottom() > model.height {
					t.Fatalf("run %d: overlay is not centered and bounded: %#v", run, rect)
				}
				view := model.View().Content
				lines := strings.Split(view, "\n")
				if len(lines) > model.height || rect.y >= len(lines) || !strings.Contains(lines[rect.y], modalTitle(test.kind)) {
					t.Fatalf("run %d: centered panel title missing at row %d", run, rect.y)
				}
				for _, want := range test.want {
					if !strings.Contains(view, want) {
						t.Fatalf("run %d: payload omitted %q", run, want)
					}
				}
				if test.kind == modalKindOperationError && strings.Contains(view, "r Retry") {
					t.Fatalf("run %d: non-retryable operation error displayed Retry", run)
				}
				for _, line := range lines {
					if width := ansi.StringWidth(line); width > model.width {
						t.Fatalf("run %d: rendered width %d > %d", run, width, model.width)
					}
				}

				selected := model.browser.selectedID
				sc009Update(t, model, keyPress("tab"))
				if model.browser.selectedID != selected || model.focusOwner != focusOwnerModal || model.modal.kind != test.kind {
					t.Fatalf("run %d: background input escaped modal ownership", run)
				}

				if test.kind != modalKindHelp {
					sc009Update(t, model, test.helpKey)
					if !model.modal.helpVisible || model.modal.kind != test.kind || model.focusOwner != focusOwnerModal {
						t.Fatalf("run %d: inline Help replaced its owner", run)
					}
					sc009Update(t, model, keyPress("esc"))
					if model.modal.helpVisible || model.modal.kind != test.kind {
						t.Fatalf("run %d: Esc did not restore the payload owner", run)
					}
				}
				sc009Update(t, model, keyPress("esc"))
				if model.modal.isOpen() || model.focusOwner != focusOwnerDetail || model.browser.selectedID != selected {
					t.Fatalf("run %d: cancel did not restore opener context", run)
				}
			}
		})
	}
}

func TestSC009FolderCreateEditAndMoveEffects(t *testing.T) {
	t.Run("folder create", func(t *testing.T) {
		for run := range sc009Runs {
			root := testFolder("root", "", "/", 1)
			created := testFolder("created", root.ID, "/created", 1)
			createCalls := 0
			contents := func(parent app.NodeID) app.ListChildrenResult {
				if parent == root.ID && createCalls != 0 {
					return app.ListChildrenResult{Folders: []app.Folder{created}}
				}
				return app.ListChildrenResult{}
			}
			get, list := sc009FolderReader(root, contents)
			model := New(Config{Width: 80, Height: 24, NoColor: true, Folders: FolderFuncs{
				GetFunc: get, ListFunc: list,
				CreateFunc: func(_ context.Context, request app.CreateFolderRequest) (app.FolderResult, error) {
					createCalls++
					if request.Name != created.Name || request.Parent.ID != root.ID {
						t.Fatalf("run %d: create request = %#v", run, request)
					}
					return app.FolderResult{Folder: created, CatalogRevision: 2}, nil
				},
			}})
			sc009Snapshot(model, root, nil, nil, root.ID)
			sc009Update(t, model, keyPress("f"))
			payload := model.modal.payload.(folderCreatePayload)
			payload.form.input.SetValue("invalid/name")
			if command := sc009Update(t, model, keyPress("enter")); command != nil || createCalls != 0 || payload.form.err == "" || !model.modal.isOpen() {
				t.Fatalf("run %d: create validation did not retain the modal and block persistence", run)
			}
			payload.form.input.SetValue(created.Name)
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("enter")))
			sc009RunCommand(t, model, reload)
			if createCalls != 1 || model.modal.isOpen() || model.browser.selectedID != created.ID || model.focusOwner != focusOwnerTree {
				t.Fatalf("run %d: create result calls=%d selection=%q focus=%v", run, createCalls, model.browser.selectedID, model.focusOwner)
			}
		}
	})

	t.Run("folder edit", func(t *testing.T) {
		for run := range sc009Runs {
			root := testFolder("root", "", "/", 1)
			folder := testFolder("folder", root.ID, "/old", 3)
			renamed := folder
			renamed.Name, renamed.Path, renamed.Revision = "new", "/new", 4
			renameCalls := 0
			contents := func(parent app.NodeID) app.ListChildrenResult {
				if parent != root.ID {
					return app.ListChildrenResult{}
				}
				if renameCalls == 0 {
					return app.ListChildrenResult{Folders: []app.Folder{folder}}
				}
				return app.ListChildrenResult{Folders: []app.Folder{renamed}}
			}
			get, list := sc009FolderReader(root, contents)
			model := New(Config{Width: 80, Height: 24, NoColor: true, Folders: FolderFuncs{
				GetFunc: get, ListFunc: list,
				RenameFunc: func(_ context.Context, request app.RenameFolderRequest) (app.FolderResult, error) {
					renameCalls++
					if request.Folder.ID != folder.ID || request.Expected == nil || *request.Expected != folder.Revision || request.Name != renamed.Name {
						t.Fatalf("run %d: rename request = %#v", run, request)
					}
					return app.FolderResult{Folder: renamed, CatalogRevision: 2}, nil
				},
			}})
			sc009Snapshot(model, root, []app.Folder{folder}, nil, folder.ID)
			sc009Update(t, model, keyPress("e"))
			payload := model.modal.payload.(folderEditPayload)
			payload.form.input.SetValue("invalid/name")
			if command := sc009Update(t, model, keyPress("enter")); command != nil || renameCalls != 0 || payload.form.err == "" || !model.modal.isOpen() {
				t.Fatalf("run %d: edit validation did not retain the modal and block persistence", run)
			}
			payload.form.input.SetValue(renamed.Name)
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("enter")))
			sc009RunCommand(t, model, reload)
			if renameCalls != 1 || model.modal.isOpen() || model.browser.selectedID != folder.ID {
				t.Fatalf("run %d: edit result calls=%d selection=%q", run, renameCalls, model.browser.selectedID)
			}
		}
	})

	t.Run("move", func(t *testing.T) {
		for run := range sc009Runs {
			root := testFolder("root", "", "/", 1)
			destination := testFolder("destination", root.ID, "/destination", 2)
			connection := testConnection("connection", root.ID, "/connection", 3)
			moved := connection
			moved.ParentID, moved.Path = destination.ID, "/destination/connection"
			moveCalls := 0
			contents := func(parent app.NodeID) app.ListChildrenResult {
				switch parent {
				case root.ID:
					result := app.ListChildrenResult{Folders: []app.Folder{destination}}
					if moveCalls == 0 {
						result.Connections = []app.Connection{connection}
					}
					return result
				case destination.ID:
					if moveCalls != 0 {
						return app.ListChildrenResult{Connections: []app.Connection{moved}}
					}
				}
				return app.ListChildrenResult{}
			}
			get, list := sc009FolderReader(root, contents)
			model := New(Config{Width: 80, Height: 24, NoColor: true,
				Folders: FolderFuncs{GetFunc: get, ListFunc: list},
				Connections: ConnectionFuncs{MoveFunc: func(_ context.Context, request app.MoveConnectionRequest) (app.ConnectionResult, error) {
					moveCalls++
					if request.Connection.ID != connection.ID || request.Destination.ID != destination.ID {
						t.Fatalf("run %d: move request = %#v", run, request)
					}
					return app.ConnectionResult{Connection: moved, CatalogRevision: 2}, nil
				}},
			})
			sc009Snapshot(model, root, []app.Folder{destination}, []app.Connection{connection}, connection.ID)
			sc009Update(t, model, keyPress("m"))
			sc009Update(t, model, keyPress("G"))
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("enter")))
			sc009RunCommand(t, model, reload)
			if moveCalls != 1 || model.modal.isOpen() || model.browser.selectedID != connection.ID {
				t.Fatalf("run %d: move result calls=%d selection=%q", run, moveCalls, model.browser.selectedID)
			}
		}
	})

	t.Run("folder source move", func(t *testing.T) {
		for run := range sc009Runs {
			root := testFolder("root", "", "/", 1)
			source := testFolder("source", root.ID, "/source", 3)
			destination := testFolder("destination", root.ID, "/destination", 2)
			moved := source
			moved.ParentID, moved.Path, moved.Revision = destination.ID, "/destination/source", 4
			moveCalls := 0
			contents := func(parent app.NodeID) app.ListChildrenResult {
				switch parent {
				case root.ID:
					result := app.ListChildrenResult{Folders: []app.Folder{destination}}
					if moveCalls == 0 {
						result.Folders = append(result.Folders, source)
					}
					return result
				case destination.ID:
					if moveCalls != 0 {
						return app.ListChildrenResult{Folders: []app.Folder{moved}}
					}
				}
				return app.ListChildrenResult{}
			}
			get, list := sc009FolderReader(root, contents)
			model := New(Config{Width: 80, Height: 24, NoColor: true, Folders: FolderFuncs{
				GetFunc: get, ListFunc: list,
				MoveFunc: func(_ context.Context, request app.MoveFolderRequest) (app.FolderResult, error) {
					moveCalls++
					if request.Folder.ID != source.ID || request.Destination.ID != destination.ID || request.Expected == nil || *request.Expected != source.Revision {
						t.Fatalf("run %d: folder move request = %#v", run, request)
					}
					return app.FolderResult{Folder: moved, CatalogRevision: 2}, nil
				},
			}})
			sc009Snapshot(model, root, []app.Folder{source, destination}, nil, source.ID)
			sc009Update(t, model, keyPress("m"))
			payload := model.modal.payload.(movePickerPayload)
			for payload.picker.destination() == nil || payload.picker.destination().ID != destination.ID {
				sc009Update(t, model, keyPress("j"))
			}
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("enter")))
			sc009RunCommand(t, model, reload)
			if moveCalls != 1 || model.modal.isOpen() || model.browser.selectedID != source.ID {
				t.Fatalf("run %d: folder move result calls=%d selection=%q", run, moveCalls, model.browser.selectedID)
			}
		}
	})
}

func TestSC009DeleteAndConnectEffects(t *testing.T) {
	t.Run("delete connection", func(t *testing.T) {
		for run := range sc009Runs {
			root := testFolder("root", "", "/", 1)
			folder := testFolder("folder", root.ID, "/folder", 2)
			connection := testConnection("connection", folder.ID, "/folder/connection", 3)
			scopeCalls, deleteCalls := 0, 0
			contents := func(parent app.NodeID) app.ListChildrenResult {
				if parent == root.ID {
					return app.ListChildrenResult{Folders: []app.Folder{folder}}
				}
				if parent == folder.ID && deleteCalls == 0 {
					return app.ListChildrenResult{Connections: []app.Connection{connection}}
				}
				return app.ListChildrenResult{}
			}
			get, list := sc009FolderReader(root, contents)
			model := New(Config{Width: 80, Height: 24, NoColor: true,
				Folders: FolderFuncs{GetFunc: get, ListFunc: list},
				Connections: ConnectionFuncs{
					DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
						scopeCalls++
						return app.ConnectionDeleteScope{ID: selector.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision}, nil
					},
					DeleteFunc: func(_ context.Context, request app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
						deleteCalls++
						if request.Connection.ID != connection.ID || request.Expected == nil || *request.Expected != connection.Revision {
							t.Fatalf("run %d: delete request = %#v", run, request)
						}
						return app.DeleteConnectionResult{Deleted: connection, CatalogRevision: 2}, nil
					},
				},
			})
			sc009Snapshot(model, root, []app.Folder{folder}, []app.Connection{connection}, connection.ID)
			sc009RunCommand(t, model, sc009Update(t, model, keyPress("d")))
			if model.modal.kind != modalKindDeleteConnection || scopeCalls != 1 || deleteCalls != 0 {
				t.Fatalf("run %d: scope phase = %q/%d/%d", run, model.modal.kind, scopeCalls, deleteCalls)
			}
			payload := model.modal.payload.(deleteConnectionPayload)
			target := *model.modal.target
			sc009Update(t, model, keyPress("enter"))
			if deleteCalls != 0 || model.modal.isOpen() || !model.openGenericModal(modalKindDeleteConnection, &target, payload) {
				t.Fatalf("run %d: Enter accepted deletion or confirmation could not reopen", run)
			}
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("y")))
			sc009RunCommand(t, model, reload)
			if deleteCalls != 1 || model.modal.isOpen() || model.browser.selectedID != folder.ID {
				t.Fatalf("run %d: delete result calls=%d selection=%q", run, deleteCalls, model.browser.selectedID)
			}
		}
	})

	t.Run("delete folder", func(t *testing.T) {
		for run := range sc009Runs {
			root := testFolder("root", "", "/", 1)
			folder := testFolder("folder", root.ID, "/folder", 2)
			scopeCalls, deleteCalls := 0, 0
			contents := func(parent app.NodeID) app.ListChildrenResult {
				if parent == root.ID && deleteCalls == 0 {
					return app.ListChildrenResult{Folders: []app.Folder{folder}}
				}
				return app.ListChildrenResult{}
			}
			get, list := sc009FolderReader(root, contents)
			model := New(Config{Width: 80, Height: 24, NoColor: true, Folders: FolderFuncs{
				GetFunc: get, ListFunc: list,
				DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.FolderDeleteScope, error) {
					scopeCalls++
					return app.FolderDeleteScope{ID: selector.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1, Snapshot: []app.NodeRevision{{ID: folder.ID, Revision: folder.Revision}}}, nil
				},
				DeleteFunc: func(_ context.Context, request app.DeleteFolderRequest) (app.DeleteFolderResult, error) {
					deleteCalls++
					if request.Folder.ID != folder.ID || !request.Recursive {
						t.Fatalf("run %d: folder delete request = %#v", run, request)
					}
					return app.DeleteFolderResult{FoldersDeleted: 1, CatalogRevision: 2}, nil
				},
			}})
			sc009Snapshot(model, root, []app.Folder{folder}, nil, folder.ID)
			sc009RunCommand(t, model, sc009Update(t, model, keyPress("d")))
			if model.modal.kind != modalKindDeleteFolder || scopeCalls != 1 || deleteCalls != 0 {
				t.Fatalf("run %d: scope phase = %q/%d/%d", run, model.modal.kind, scopeCalls, deleteCalls)
			}
			payload := model.modal.payload.(deleteFolderPayload)
			target := *model.modal.target
			sc009Update(t, model, keyPress("enter"))
			if deleteCalls != 0 || model.modal.isOpen() || !model.openGenericModal(modalKindDeleteFolder, &target, payload) {
				t.Fatalf("run %d: Enter accepted folder deletion or confirmation could not reopen", run)
			}
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("y")))
			sc009RunCommand(t, model, reload)
			if deleteCalls != 1 || model.modal.isOpen() || model.browser.selectedID != root.ID {
				t.Fatalf("run %d: folder delete result calls=%d selection=%q", run, deleteCalls, model.browser.selectedID)
			}
		}
	})

	t.Run("connect confirm and cancel", func(t *testing.T) {
		for run := range sc009Runs {
			connection := testConnection("connection", syntheticRootID, "/connection", 3)
			connectCalls := 0
			model := loadedModel(t, []app.Connection{connection}, true)
			model.connect = ConnectFunc(func(_ context.Context, request app.ConnectRequest) (app.ConnectResult, error) {
				connectCalls++
				if request.Connection.ID != connection.ID || request.Expected == nil || *request.Expected != connection.Revision {
					t.Fatalf("run %d: connect request = %#v", run, request)
				}
				return app.ConnectResult{Connection: connection, Attempt: app.SSHAttemptTarget{ID: connection.ID}, Session: app.SSHSessionResult{StartedAt: time.Unix(1, 0)}}, nil
			})
			model.browser.selectedID = connection.ID
			model.ownedSelectionID = connection.ID
			sc009Update(t, model, keyPress("c"))
			command := sc009Update(t, model, keyPress("y"))
			if connectCalls != 0 || model.modal.kind != modalKindConnectConfirmation || model.operation == nil || model.operation.kind != asyncOperationSSHStart {
				t.Fatalf("run %d: confirmation did not defer one SSH operation", run)
			}
			sc009RunExec(t, model, command)
			if connectCalls != 1 || model.operation != nil || model.sessionResult.Connection.ID != connection.ID {
				t.Fatalf("run %d: connect result calls=%d operation=%#v", run, connectCalls, model.operation)
			}

			model = loadedModel(t, []app.Connection{connection}, true)
			model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
				connectCalls++
				return app.ConnectResult{}, nil
			})
			model.browser.selectedID = connection.ID
			model.ownedSelectionID = connection.ID
			sc009Update(t, model, keyPress("c"))
			if command := sc009Update(t, model, keyPress("enter")); command != nil || connectCalls != 1 || model.modal.isOpen() {
				t.Fatalf("run %d: cancel started network or retained modal", run)
			}
		}
	})
}

func TestSC009UnsavedHelpAndOperationErrorPaths(t *testing.T) {
	t.Run("unsaved discard and save", func(t *testing.T) {
		for run := range sc009Runs {
			connection := testConnection("connection", syntheticRootID, "/connection", 2)
			updates := 0
			service := ConnectionFuncs{
				ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
					return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: app.CatalogRevision(2 + updates)}, nil
				},
				UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
					updates++
					connection.Host = *request.Host
					connection.Revision++
					return app.ConnectionResult{Connection: connection, CatalogRevision: 3}, nil
				},
			}
			model := New(Config{Width: 80, Height: 24, NoColor: true, Connections: service})
			sc009RunCommand(t, model, model.Init())
			model.browser.selectedID = connection.ID
			model.ownedSelectionID = connection.ID
			sc009Update(t, model, keyPress("e"))
			model.form.inputs[fieldHost].SetValue("changed.test")
			sc009Update(t, model, keyPress("esc"))
			form := model.form
			sc009Update(t, model, keyPress("esc"))
			if model.modal.isOpen() || model.form != form || model.focusOwner != focusOwnerConnectionForm {
				t.Fatalf("run %d: cancel did not restore dirty form", run)
			}
			sc009Update(t, model, keyPress("esc"))
			sc009Update(t, model, keyPress("d"))
			if model.form != nil || model.screen != screenBrowser || updates != 0 {
				t.Fatalf("run %d: discard retained form or persisted", run)
			}

			model.browser.selectedID = connection.ID
			model.ownedSelectionID = connection.ID
			sc009Update(t, model, keyPress("e"))
			model.form.inputs[fieldHost].SetValue("saved.test")
			sc009Update(t, model, keyPress("esc"))
			reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("s")))
			sc009RunCommand(t, model, reload)
			if updates != 1 || model.form != nil || model.modal.isOpen() || model.browser.selectedID != connection.ID {
				t.Fatalf("run %d: save result updates=%d selection=%q", run, updates, model.browser.selectedID)
			}

			model = New(Config{Width: 80, Height: 24, NoColor: true, Connections: service})
			sc009RunCommand(t, model, model.Init())
			model.browser.selectedID = connection.ID
			model.ownedSelectionID = connection.ID
			sc009Update(t, model, keyPress("e"))
			model.form.inputs[fieldHost].SetValue("discarded.test")
			sc009Update(t, model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
			if command := sc009Update(t, model, keyPress("d")); command == nil || model.form != nil || model.modal.isOpen() || updates != 1 {
				t.Fatalf("run %d: quit-discard did not exit without persistence", run)
			}

			model = New(Config{Width: 80, Height: 24, NoColor: true, Connections: service})
			sc009RunCommand(t, model, model.Init())
			model.browser.selectedID = connection.ID
			model.ownedSelectionID = connection.ID
			sc009Update(t, model, keyPress("e"))
			model.form.inputs[fieldHost].SetValue("quit-after-save.test")
			sc009Update(t, model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
			reload = sc009RunCommand(t, model, sc009Update(t, model, keyPress("s")))
			if quit := sc009RunCommand(t, model, reload); quit == nil || updates != 2 || model.operation != nil || model.form != nil || model.modal.isOpen() {
				t.Fatalf("run %d: quit-save did not wait for commit and cleanup", run)
			}
		}
	})

	t.Run("help scroll and close", func(t *testing.T) {
		for run := range sc009Runs {
			model := New(Config{Width: 40, Height: 12, NoColor: true})
			lines := make([]string, 30)
			for index := range lines {
				lines[index] = "Help action " + strings.Repeat("x", index+1)
			}
			if !model.openGenericModal(modalKindHelp, nil, helpPayload{lines: lines}) {
				t.Fatalf("run %d: help open failed", run)
			}
			sc009Update(t, model, keyPress("G"))
			if model.modal.viewport.logicalOffset == 0 {
				t.Fatalf("run %d: Help did not scroll", run)
			}
			sc009Update(t, model, keyPress("?"))
			if model.modal.isOpen() || model.focusOwner != focusOwnerTree {
				t.Fatalf("run %d: Help close did not restore opener", run)
			}
		}
	})

	t.Run("operation error reload back and quit", func(t *testing.T) {
		for run := range sc009Runs {
			for _, errorKind := range []error{app.ErrConflict, app.ErrNotFound} {
				reloads := 0
				model := New(Config{Width: 80, Height: 24, NoColor: true, Connections: ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
					reloads++
					return app.ListConnectionsResult{CatalogRevision: 2}, nil
				}}})
				modal := newErrorModal("reload catalog", "/captured", errorKind)
				model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: modal})
				sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
				if reloads != 1 || model.modal.isOpen() {
					t.Fatalf("run %d: %v recovery calls=%d modal=%q", run, errorKind, reloads, model.modal.kind)
				}

				model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: modal})
				sc009Update(t, model, keyPress("b"))
				if model.modal.isOpen() || model.focusOwner != focusOwnerTree {
					t.Fatalf("run %d: Back did not close operation error", run)
				}

				model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: modal})
				if command := sc009Update(t, model, keyPress("q")); command == nil || !model.modal.isOpen() {
					t.Fatalf("run %d: Quit path unavailable or destroyed owner data", run)
				}
			}
		}
	})
}

func TestSC009SSHFailurePaths(t *testing.T) {
	for run := range sc009Runs {
		failed := app.SSHAttemptTarget{ID: "connection", Revision: 2, Path: "/old", Host: "old.test", Port: 22}
		current := testConnection("connection", syntheticRootID, "/current", 3)
		failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()
		getCalls, connectCalls := 0, 0
		model := loadedModel(t, []app.Connection{current}, true)
		model.connections = ConnectionFuncs{
			GetFunc: func(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
				getCalls++
				return app.ConnectionResult{Connection: current}, nil
			},
			ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				return app.ListConnectionsResult{Connections: []app.Connection{current}, CatalogRevision: 8}, nil
			},
		}
		model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
			connectCalls++
			return app.ConnectResult{Connection: current, Session: app.SSHSessionResult{StartedAt: time.Unix(1, 0)}}, nil
		})
		model.installSSHFailure(newSSHFailureModal(failed, failure))
		sc009Update(t, model, keyPress("d"))
		if !model.activeSSHFailure().detailVisible {
			t.Fatalf("run %d: Detail path unavailable", run)
		}
		resolve := sc009Update(t, model, keyPress("r"))
		if resolve == nil || getCalls != 0 || connectCalls != 0 || model.activeSSHFailure().recovery != recoveryResolving {
			t.Fatalf("run %d: Retry was not deferred", run)
		}
		sc009RunCommand(t, model, resolve)
		payload := model.modal.payload.(sshFailurePayload)
		if getCalls != 1 || payload.confirmation == nil || payload.modal.recovery != recoveryConfirming {
			t.Fatalf("run %d: Retry did not enter inline confirmation", run)
		}
		sc009Update(t, model, keyPress("enter"))
		payload = model.modal.payload.(sshFailurePayload)
		if payload.confirmation != nil || payload.modal.recovery != recoveryIdle || connectCalls != 0 {
			t.Fatalf("run %d: inline cancel changed diagnostic or network", run)
		}
		resolve = sc009Update(t, model, keyPress("r"))
		sc009RunCommand(t, model, resolve)
		sc009Update(t, model, keyPress("esc"))
		payload = model.modal.payload.(sshFailurePayload)
		if payload.confirmation != nil || payload.modal.recovery != recoveryIdle || connectCalls != 0 {
			t.Fatalf("run %d: Esc from inline confirmation changed diagnostic or network", run)
		}
		resolve = sc009Update(t, model, keyPress("r"))
		sc009RunCommand(t, model, resolve)
		connect := sc009Update(t, model, keyPress("y"))
		if connect == nil || connectCalls != 0 || model.modal.isOpen() {
			t.Fatalf("run %d: explicit retry confirmation did not start SSH owner", run)
		}
		sc009RunExec(t, model, connect)
		if connectCalls != 1 || model.operation != nil {
			t.Fatalf("run %d: retry network calls=%d operation=%#v", run, connectCalls, model.operation)
		}

		model = loadedModel(t, []app.Connection{current}, true)
		model.connections = ConnectionFuncs{GetFunc: func(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
			return app.ConnectionResult{Connection: current}, nil
		}}
		model.installSSHFailure(newSSHFailureModal(failed, failure))
		sc009RunCommand(t, model, sc009Update(t, model, keyPress("e")))
		if model.screen != screenConnectionForm || model.form == nil || model.modal.isOpen() || model.focusOwner != focusOwnerConnectionForm {
			t.Fatalf("run %d: Edit did not transfer ownership to Details", run)
		}
		sc009Update(t, model, keyPress("esc"))
		if model.activeSSHFailure() == nil || model.screen != screenBrowser {
			t.Fatalf("run %d: cancel Edit did not restore SSH diagnostic", run)
		}
		sc009Update(t, model, keyPress("b"))
		if model.modal.isOpen() || model.browser.selectedID != current.ID {
			t.Fatalf("run %d: Back did not restore captured selection", run)
		}

		for _, recovery := range []recoveryState{recoveryMissing, recoveryConflict} {
			reloads := 0
			model = loadedModel(t, []app.Connection{current}, true)
			model.connections = ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				reloads++
				return app.ListConnectionsResult{Connections: []app.Connection{current}, CatalogRevision: 9}, nil
			}}
			diagnostic := newSSHFailureModal(failed, failure)
			diagnostic.recovery = recovery
			model.installSSHFailure(diagnostic)
			if command := sc009Update(t, model, keyPress("e")); command != nil || model.screen != screenBrowser || !model.modal.isOpen() {
				t.Fatalf("run %d: recovery %v exposed Edit", run, recovery)
			}
			sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
			if reloads != 1 || model.operation != nil || model.activeSSHFailure().recovery != recoveryIdle {
				t.Fatalf("run %d: recovery %v reload result = %d/%#v", run, recovery, reloads, model.operation)
			}
		}

		model = loadedModel(t, []app.Connection{current}, true)
		diagnostic := newSSHFailureModal(failed, failure)
		diagnostic.recovery = recoveryResolving
		model.installSSHFailure(diagnostic)
		if command := sc009Update(t, model, keyPress("r")); command != nil || model.activeSSHFailure().recovery != recoveryResolving {
			t.Fatalf("run %d: resolving exposed Retry/Reload", run)
		}
		if command := sc009Update(t, model, keyPress("e")); command != nil || model.screen != screenBrowser {
			t.Fatalf("run %d: resolving exposed Edit", run)
		}
		sc009Update(t, model, keyPress("esc"))
		if model.modal.isOpen() {
			t.Fatalf("run %d: resolving Back did not close", run)
		}

		model.installSSHFailure(newSSHFailureModal(failed, failure))
		if command := sc009Update(t, model, keyPress("q")); command == nil || !model.modal.isOpen() {
			t.Fatalf("run %d: SSH Quit path unavailable or lost diagnostic", run)
		}

		model = loadedModel(t, []app.Connection{current}, true)
		noDetail := app.SSHFailurePresentation{
			Category:       app.SSHFailureTimeout,
			Stage:          app.SSHFailureStageNetworkConnection,
			Summary:        "SSH connection timed out.",
			Recommendation: "Check the network path and retry.",
		}
		model.installSSHFailure(newSSHFailureModal(failed, noDetail))
		sc009Update(t, model, keyPress("d"))
		if model.activeSSHFailure().detailVisible {
			t.Fatalf("run %d: Detail toggled without technical detail", run)
		}
	}
}

func TestSC009OperationErrorRetryUsesCapturedIntentTwentyRuns(t *testing.T) {
	for run := range sc009Runs {
		t.Run("catalog", func(t *testing.T) {
			first := testConnection("first", syntheticRootID, "/first", 1)
			other := testConnection("other", syntheticRootID, "/other", 1)
			calls := 0
			model := loadedModel(t, []app.Connection{first, other}, true)
			model.connections = ConnectionFuncs{ListFunc: func(_ context.Context, request app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				calls++
				if request.Folder.Path != "/" {
					t.Fatalf("run %d: catalog retry request = %#v", run, request)
				}
				if calls == 1 {
					return app.ListConnectionsResult{}, app.ErrInvalidRequest
				}
				return app.ListConnectionsResult{Connections: []app.Connection{first, other}, CatalogRevision: 9}, nil
			}}
			model.browser.selectedID = first.ID
			model.ownedSelectionID = first.ID
			failure := sc009Update(t, model, keyPress("r"))
			model.browser.selectedID = other.ID
			model.ownedSelectionID = other.ID
			sc009Update(t, model, failure())
			payload := model.modal.payload.(operationErrorPayload)
			if calls != 1 || payload.modal.retry == nil || payload.modal.retry.kind != operationRetryCatalog || payload.modal.target != "/" || !strings.Contains(model.View().Content, "r Retry") {
				t.Fatalf("run %d: catalog failure lost retry intent or catalog target", run)
			}
			retry := sc009Update(t, model, keyPress("r"))
			if retry == nil || model.operation == nil || model.operation.kind != asyncOperationReload || model.operation.target != nil || model.browser.selectedID != other.ID {
				t.Fatalf("run %d: catalog Retry was retargeted", run)
			}
			sc009RunCommand(t, model, retry)
			if calls != 2 || model.modal.isOpen() || model.browser.selectedID != other.ID {
				t.Fatalf("run %d: catalog Retry result calls=%d selection=%q", run, calls, model.browser.selectedID)
			}
		})

		for _, closeKey := range []string{"b", "esc"} {
			captured := testConnection("captured", syntheticRootID, "/captured", 3)
			other := testConnection("other", syntheticRootID, "/other", 7)
			calls := 0
			model := loadedModel(t, []app.Connection{captured, other}, true)
			model.connections = ConnectionFuncs{DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
				calls++
				if selector.ID != captured.ID {
					t.Fatalf("run %d/%s: close setup retargeted delete scope to %q", run, closeKey, selector.ID)
				}
				return app.ConnectionDeleteScope{}, app.ErrInvalidRequest
			}}
			model.browser.selectedID = captured.ID
			model.ownedSelectionID = captured.ID
			failure := sc009Update(t, model, keyPress("d"))
			message := failure()
			model.browser.selectedID = other.ID
			model.ownedSelectionID = other.ID
			sc009Update(t, model, message)
			if calls != 1 || model.modal.kind != modalKindOperationError || !strings.Contains(model.View().Content, "r Retry") {
				t.Fatalf("run %d/%s: retryable failure = calls %d modal %q", run, closeKey, calls, model.modal.kind)
			}
			if command := sc009Update(t, model, keyPress(closeKey)); command != nil || calls != 1 || model.operation != nil || model.modal.isOpen() || model.browser.selectedID != other.ID {
				t.Fatalf("run %d/%s: Back/Esc retried or changed background state", run, closeKey)
			}
		}

		captured := testConnection("captured", syntheticRootID, "/captured", 3)
		other := testConnection("other", syntheticRootID, "/other", 7)
		calls := 0
		var selectors []app.ItemSelector
		model := loadedModel(t, []app.Connection{captured, other}, true)
		model.connections = ConnectionFuncs{DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
			calls++
			selectors = append(selectors, selector)
			if calls == 1 {
				return app.ConnectionDeleteScope{}, app.ErrInvalidRequest
			}
			return app.ConnectionDeleteScope{ID: captured.ID, Path: captured.Path, Host: captured.Host, Revision: captured.Revision}, nil
		}}
		model.browser.selectedID = captured.ID
		model.ownedSelectionID = captured.ID
		failure := sc009Update(t, model, keyPress("d"))
		message := failure()
		model.browser.selectedID = other.ID
		model.ownedSelectionID = other.ID
		sc009Update(t, model, message)
		payload, ok := model.modal.payload.(operationErrorPayload)
		if !ok || payload.modal.retry == nil || payload.modal.retry.kind != operationRetryConnectionDeleteScope || model.modal.target == nil || model.modal.target.id != captured.ID {
			t.Fatalf("run %d: operation error lost its captured retry intent/target", run)
		}
		model.browser.selectedID = other.ID
		model.ownedSelectionID = other.ID
		retry := sc009Update(t, model, keyPress("r"))
		if retry == nil || calls != 1 || model.modal.isOpen() || model.operation == nil || model.operation.target == nil || model.operation.target.id != captured.ID || model.browser.selectedID != other.ID {
			t.Fatalf("run %d: Retry did not start the captured operation", run)
		}
		sc009RunCommand(t, model, retry)
		if calls != 2 || len(selectors) != 2 || selectors[0].ID != captured.ID || selectors[1].ID != captured.ID || model.modal.kind != modalKindDeleteConnection {
			t.Fatalf("run %d: retry selectors=%#v calls=%d modal=%q", run, selectors, calls, model.modal.kind)
		}
		confirmation := model.modal.payload.(deleteConnectionPayload).confirmation
		if confirmation.scope.ID != captured.ID || confirmation.scope.Path != captured.Path || model.browser.selectedID != other.ID {
			t.Fatalf("run %d: Retry used current selection instead of captured target", run)
		}
	}
}

func TestSC009RetryFailureRetainsCapturedIntentTwentyRuns(t *testing.T) {
	for run := range sc009Runs {
		captured := testConnection("captured", syntheticRootID, "/captured", 3)
		other := testConnection("other", syntheticRootID, "/other", 8)
		calls := 0
		model := loadedModel(t, []app.Connection{captured, other}, true)
		model.connections = ConnectionFuncs{DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
			calls++
			if selector.ID != captured.ID {
				t.Fatalf("run %d: Retry retargeted to %q", run, selector.ID)
			}
			return app.ConnectionDeleteScope{}, app.ErrInvalidRequest
		}}
		selectSCNode(model, captured.ID)
		failure := sc009Update(t, model, keyPress("d"))
		sc009Update(t, model, failure())
		selectSCNode(model, other.ID)
		retry := sc009Update(t, model, keyPress("r"))
		if retry == nil || model.modal.isOpen() || model.operation == nil {
			t.Fatalf("run %d: Retry did not transfer captured ownership to an operation", run)
		}
		sc009RunCommand(t, model, retry)
		payload, ok := model.modal.payload.(operationErrorPayload)
		if !ok || payload.modal.retry == nil || payload.modal.retry.kind != operationRetryConnectionDeleteScope || model.modal.target == nil || model.modal.target.id != captured.ID || model.browser.selectedID != other.ID || calls != 2 {
			t.Fatalf("run %d: failed Retry lost intent/target: calls=%d modal=%#v", run, calls, model.modal)
		}
		if !strings.Contains(model.View().Content, "Target: /captured") || !strings.Contains(model.View().Content, "r Retry") {
			t.Fatalf("run %d: failed Retry omitted captured recovery cues", run)
		}
	}
}

func TestSC009SSHResolveNotFoundConflictFromServiceTwentyRuns(t *testing.T) {
	for _, resolvedErr := range []error{app.ErrNotFound, app.ErrConflict} {
		name := "not found"
		want := recoveryMissing
		if resolvedErr == app.ErrConflict {
			name = "conflict"
			want = recoveryConflict
		}
		t.Run(name, func(t *testing.T) {
			for run := range sc009Runs {
				connection := testConnection("connection", syntheticRootID, "/connection", 3)
				attempt := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
				failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()
				calls := 0
				model := loadedModel(t, []app.Connection{connection}, true)
				model.connections = ConnectionFuncs{GetFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionResult, error) {
					calls++
					if selector.ID != connection.ID {
						t.Fatalf("run %d: resolve selector = %#v", run, selector)
					}
					return app.ConnectionResult{}, resolvedErr
				}}
				model.installSSHFailure(newSSHFailureModal(attempt, failure))
				sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
				if calls != 1 || model.operation != nil || model.activeSSHFailure().recovery != want || !model.modal.isOpen() {
					t.Fatalf("run %d: real resolve result = calls %d recovery %v", run, calls, model.activeSSHFailure().recovery)
				}
				view := model.View().Content
				if !strings.Contains(view, "r Reload") || strings.Contains(view, "e Edit") || strings.Contains(view, "r Retry") {
					t.Fatalf("run %d: resolve failure controls are not terminal-state exact", run)
				}
			}
		})
	}
}

func TestSC009ActionModalFailureConflictRetentionTwentyRuns(t *testing.T) {
	type modalFailureCase struct {
		name       string
		kind       modalKind
		payload    func() (capturedTarget, any)
		resultKind operationKind
		start      func(*testing.T, *Model) tea.Cmd
		retained   func(any) string
	}
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 3)
	destination := testFolder("destination", root.ID, "/destination", 2)
	connection := testConnection("connection", folder.ID, "/folder/connection", 4)
	cases := []modalFailureCase{
		{name: "folder create", kind: modalKindFolderCreate, payload: func() (capturedTarget, any) {
			form := newFolderForm(nil, app.ItemSelector{ID: root.ID})
			form.setDestination(root)
			form.input.SetValue("retained-create")
			return capturedTargetFromFolder(root), folderCreatePayload{form: form}
		}, resultKind: operationFolderCreate, start: func(t *testing.T, model *Model) tea.Cmd { return sc009Update(t, model, keyPress("enter")) }, retained: func(payload any) string { return payload.(folderCreatePayload).form.input.Value() }},
		{name: "folder edit", kind: modalKindFolderEdit, payload: func() (capturedTarget, any) {
			form := newFolderForm(&folder, app.ItemSelector{})
			form.input.SetValue("retained-edit")
			return capturedTargetFromFolder(folder), folderEditPayload{form: form}
		}, resultKind: operationFolderRename, start: func(t *testing.T, model *Model) tea.Cmd { return sc009Update(t, model, keyPress("enter")) }, retained: func(payload any) string { return payload.(folderEditPayload).form.input.Value() }},
		{name: "move", kind: modalKindMovePicker, payload: func() (capturedTarget, any) {
			picker := newMovePicker(connection.Node, []app.Folder{destination})
			return capturedTargetFromConnection(connection), movePickerPayload{picker: picker}
		}, resultKind: operationMove, start: func(t *testing.T, model *Model) tea.Cmd { return sc009Update(t, model, keyPress("enter")) }, retained: func(payload any) string { return payload.(movePickerPayload).picker.destination().Path }},
		{name: "delete connection", kind: modalKindDeleteConnection, payload: func() (capturedTarget, any) {
			scope := app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision}
			return capturedTargetFromConnection(connection), deleteConnectionPayload{confirmation: newDeleteConfirmation(scope)}
		}, resultKind: operationDelete, start: func(t *testing.T, model *Model) tea.Cmd { return sc009Update(t, model, keyPress("y")) }, retained: func(payload any) string { return payload.(deleteConnectionPayload).confirmation.scope.Path }},
		{name: "delete folder", kind: modalKindDeleteFolder, payload: func() (capturedTarget, any) {
			scope := app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1, Snapshot: []app.NodeRevision{{ID: folder.ID, Revision: folder.Revision}}}
			return capturedTargetFromFolder(folder), deleteFolderPayload{confirmation: newFolderDeleteConfirmation(scope)}
		}, resultKind: operationFolderDelete, start: func(t *testing.T, model *Model) tea.Cmd { return sc009Update(t, model, keyPress("y")) }, retained: func(payload any) string { return payload.(deleteFolderPayload).confirmation.scope.Path }},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for run := range sc009Runs {
				for _, failure := range []error{app.ErrInvalidRequest, app.ErrConflict} {
					target, payload := test.payload()
					model := New(Config{Width: 80, Height: 24, NoColor: true})
					sc009Snapshot(model, root, []app.Folder{folder, destination}, []app.Connection{connection}, target.id)
					if !model.openGenericModal(test.kind, &target, payload) {
						t.Fatalf("run %d: open failed", run)
					}
					before := test.retained(model.modal.payload)
					command := test.start(t, model)
					if command == nil || model.operation == nil {
						t.Fatalf("run %d: action did not start", run)
					}
					sc009Update(t, model, operationResultMsg{id: model.operation.id, kind: test.resultKind, err: failure})
					if !model.modal.isOpen() || model.modal.kind != test.kind || model.modal.target == nil || model.modal.target.id != target.id || test.retained(model.modal.payload) != before || model.operation != nil {
						t.Fatalf("run %d: %v lost modal owner/payload/target", run, failure)
					}
					if failure == app.ErrConflict {
						if model.modal.conflict == nil || !strings.Contains(model.View().Content, "Warning:") {
							t.Fatalf("run %d: conflict was not embedded", run)
						}
					} else if model.modal.recoverableError == "" || !strings.Contains(model.View().Content, "Error:") {
						t.Fatalf("run %d: failure was not embedded", run)
					}
				}
			}
		})
	}
}

func TestSC009ConnectionFormsRemainInDetailsTwentyRuns(t *testing.T) {
	for _, test := range []struct {
		name string
		key  string
	}{
		{"create", "n"},
		{"edit", "e"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for run := range sc009Runs {
				connection := testConnection("connection", syntheticRootID, "/connection", 2)
				model := loadedModel(t, []app.Connection{connection}, true)
				if test.key == "e" {
					model.browser.selectedID = connection.ID
					model.ownedSelectionID = connection.ID
				}
				sc009Update(t, model, keyPress(test.key))
				view := model.View().Content
				if model.screen != screenConnectionForm || model.form == nil || model.focusOwner != focusOwnerConnectionForm || model.modal.isOpen() {
					t.Fatalf("run %d: form ownership = screen %v focus %v modal %q", run, model.screen, model.focusOwner, model.modal.kind)
				}
				if !strings.Contains(view, "Details") || !strings.Contains(view, "connection") {
					t.Fatalf("run %d: Details form not rendered", run)
				}
			}
		})
	}
}

func TestSC009DisplayedModalControlAliasesReachTerminalOutcomesTwentyRuns(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 2)
	connection := testConnection("connection", folder.ID, "/folder/connection", 3)
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()

	tests := []struct {
		name  string
		setup func(*Model)
		key   tea.KeyPressMsg
		check func(*testing.T, *Model, tea.Cmd, int)
	}{
		{name: "folder_create_save_ctrl_s", setup: func(model *Model) {
			form := newFolderForm(nil, app.ItemSelector{ID: folder.ID})
			form.setDestination(folder)
			form.input.SetValue("created")
			target := capturedTargetFromFolder(folder)
			model.openGenericModal(modalKindFolderCreate, &target, folderCreatePayload{form: form})
		}, key: tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}), check: sc009CheckFailedOperationRetained},
		{name: "folder_edit_save_enter", setup: func(model *Model) {
			form := newFolderForm(&folder, app.ItemSelector{})
			form.input.SetValue("renamed")
			target := capturedTargetFromFolder(folder)
			model.openGenericModal(modalKindFolderEdit, &target, folderEditPayload{form: form})
		}, key: keyPress("enter"), check: sc009CheckFailedOperationRetained},
		{name: "folder_edit_save_ctrl_s", setup: func(model *Model) {
			form := newFolderForm(&folder, app.ItemSelector{})
			form.input.SetValue("renamed")
			target := capturedTargetFromFolder(folder)
			model.openGenericModal(modalKindFolderEdit, &target, folderEditPayload{form: form})
		}, key: tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}), check: sc009CheckFailedOperationRetained},
		{name: "folder_form_quit_ctrl_c", setup: func(model *Model) {
			form := newFolderForm(&folder, app.ItemSelector{})
			target := capturedTargetFromFolder(folder)
			model.openGenericModal(modalKindFolderEdit, &target, folderEditPayload{form: form})
		}, key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), check: sc009CheckQuitOwnerRetained},
		{name: "move_home", setup: func(model *Model) {
			target := capturedTargetFromConnection(connection)
			model.openGenericModal(modalKindMovePicker, &target, movePickerPayload{picker: newMovePicker(connection.Node, []app.Folder{root, folder})})
			model.modal.payload.(movePickerPayload).picker.selected = 1
		}, key: keyPress("g"), check: func(t *testing.T, model *Model, command tea.Cmd, run int) {
			if command != nil || model.modal.payload.(movePickerPayload).picker.selected != 0 {
				t.Fatalf("run %d: Home alias did not select first move target", run)
			}
		}},
		{name: "move_end", setup: func(model *Model) {
			target := capturedTargetFromConnection(connection)
			model.openGenericModal(modalKindMovePicker, &target, movePickerPayload{picker: newMovePicker(connection.Node, []app.Folder{root, folder})})
		}, key: keyPress("G"), check: func(t *testing.T, model *Model, command tea.Cmd, run int) {
			if command != nil || model.modal.payload.(movePickerPayload).picker.selected != 1 {
				t.Fatalf("run %d: End alias did not select last move target", run)
			}
		}},
		{name: "operation_error_back_b", setup: func(model *Model) {
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: newErrorModal("move", connection.Path, app.ErrInvalidRequest)})
		}, key: keyPress("b"), check: sc009CheckModalClosed},
		{name: "operation_error_back_escape", setup: func(model *Model) {
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: newErrorModal("move", connection.Path, app.ErrInvalidRequest)})
		}, key: keyPress("esc"), check: sc009CheckModalClosed},
		{name: "operation_error_quit_q", setup: func(model *Model) {
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: newErrorModal("move", connection.Path, app.ErrInvalidRequest)})
		}, key: keyPress("q"), check: sc009CheckQuitOwnerRetained},
		{name: "operation_error_quit_ctrl_c", setup: func(model *Model) {
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: newErrorModal("move", connection.Path, app.ErrInvalidRequest)})
		}, key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), check: sc009CheckQuitOwnerRetained},
		{name: "ssh_failure_back_b", setup: func(model *Model) {
			model.installSSHFailure(newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, failure))
		}, key: keyPress("b"), check: sc009CheckModalClosed},
		{name: "ssh_failure_back_escape", setup: func(model *Model) {
			model.installSSHFailure(newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, failure))
		}, key: keyPress("esc"), check: sc009CheckModalClosed},
		{name: "ssh_failure_quit_q", setup: func(model *Model) {
			model.installSSHFailure(newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, failure))
		}, key: keyPress("q"), check: sc009CheckQuitOwnerRetained},
		{name: "ssh_failure_quit_ctrl_c", setup: func(model *Model) {
			model.installSSHFailure(newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, failure))
		}, key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), check: sc009CheckQuitOwnerRetained},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for run := range sc009Runs {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				sc009Snapshot(model, root, []app.Folder{folder}, []app.Connection{connection}, connection.ID)
				test.setup(model)
				if !model.modal.isOpen() || model.focusOwner != focusOwnerModal {
					t.Fatalf("run %d: setup did not install modal owner", run)
				}
				command := sc009Update(t, model, test.key)
				test.check(t, model, command, run)
			}
		})
	}
}

func TestSC009DisplayedModalNavigationAliasesRemainLocalTwentyRuns(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 2)
	connection := testConnection("connection", folder.ID, "/folder/connection", 3)
	navigationKeys := []tea.KeyPressMsg{
		tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), keyPress("k"),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), keyPress("j"),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}), keyPress("g"),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}), keyPress("G"),
	}

	for _, pressed := range navigationKeys {
		t.Run("move_picker/"+pressed.String(), func(t *testing.T) {
			for run := range sc009Runs {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				sc009Snapshot(model, root, []app.Folder{folder}, []app.Connection{connection}, connection.ID)
				target := capturedTargetFromConnection(connection)
				model.openGenericModal(modalKindMovePicker, &target, movePickerPayload{picker: newMovePicker(connection.Node, []app.Folder{root, folder})})
				background := model.browser.selectedID
				if command := sc009Update(t, model, pressed); command != nil || model.modal.kind != modalKindMovePicker || model.focusOwner != focusOwnerModal || model.browser.selectedID != background || model.operation != nil {
					t.Fatalf("run %d: move navigation alias %q escaped modal", run, pressed.String())
				}
			}
		})
	}

	confirmations := []struct {
		name    string
		kind    modalKind
		payload any
	}{
		{name: "delete_connection", kind: modalKindDeleteConnection, payload: deleteConnectionPayload{confirmation: newDeleteConfirmation(app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision})}},
		{name: "delete_folder", kind: modalKindDeleteFolder, payload: deleteFolderPayload{confirmation: newFolderDeleteConfirmation(app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1})}},
		{name: "connect", kind: modalKindConnectConfirmation, payload: connectConfirmationPayload{confirmation: newConnectConfirmation(connection)}},
	}
	for _, confirmation := range confirmations {
		for _, pressed := range navigationKeys {
			t.Run(confirmation.name+"/"+pressed.String(), func(t *testing.T) {
				for run := range sc009Runs {
					model := New(Config{Width: 40, Height: 12, NoColor: true})
					sc009Snapshot(model, root, []app.Folder{folder}, []app.Connection{connection}, connection.ID)
					target := capturedTarget{id: connection.ID, revision: connection.Revision, kind: app.NodeKindConnection, path: strings.Repeat("/long-target", 12)}
					if confirmation.kind == modalKindDeleteFolder {
						target = capturedTargetFromFolder(folder)
					}
					model.openGenericModal(confirmation.kind, &target, confirmation.payload)
					background := model.browser.selectedID
					if command := sc009Update(t, model, pressed); command != nil || model.modal.kind != confirmation.kind || model.focusOwner != focusOwnerModal || model.browser.selectedID != background || model.operation != nil {
						t.Fatalf("run %d: confirmation navigation alias %q escaped modal", run, pressed.String())
					}
				}
			})
		}
	}

	lines := make([]string, 30)
	for index := range lines {
		lines[index] = "Help line " + strings.Repeat("x", index+1)
	}
	for _, pressed := range navigationKeys {
		t.Run("help/"+pressed.String(), func(t *testing.T) {
			for run := range sc009Runs {
				model := New(Config{Width: 40, Height: 12, NoColor: true})
				model.openGenericModal(modalKindHelp, nil, helpPayload{lines: lines})
				if command := sc009Update(t, model, pressed); command != nil || model.modal.kind != modalKindHelp || model.focusOwner != focusOwnerModal || model.operation != nil {
					t.Fatalf("run %d: Help navigation alias %q escaped modal", run, pressed.String())
				}
			}
		})
	}

	for _, closeKey := range []tea.KeyPressMsg{keyPress("?"), keyPress("esc")} {
		t.Run("help_close/"+closeKey.String(), func(t *testing.T) {
			for run := range sc009Runs {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				model.openGenericModal(modalKindHelp, nil, helpPayload{lines: lines})
				if command := sc009Update(t, model, closeKey); command != nil || model.modal.isOpen() || model.focusOwner != focusOwnerTree {
					t.Fatalf("run %d: Help close alias %q did not restore opener", run, closeKey.String())
				}
			}
		})
	}

	for _, recovery := range []recoveryState{recoveryMissing, recoveryConflict, recoveryResolving} {
		for _, pressed := range []tea.KeyPressMsg{keyPress("d"), keyPress("b"), keyPress("esc"), keyPress("q"), tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), keyPress("?")} {
			t.Run("ssh_failure_state_"+string(rune('0'+recovery))+"/"+pressed.String(), func(t *testing.T) {
				for run := range sc009Runs {
					model := New(Config{Width: 80, Height: 24, NoColor: true})
					sc009Snapshot(model, root, []app.Folder{folder}, []app.Connection{connection}, connection.ID)
					diagnostic := newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation())
					diagnostic.recovery = recovery
					model.installSSHFailure(diagnostic)
					command := sc009Update(t, model, pressed)
					switch pressed.String() {
					case "d":
						if command != nil || !model.activeSSHFailure().detailVisible || model.modal.kind != modalKindSSHFailure {
							t.Fatalf("run %d: state %v Detail did not remain local", run, recovery)
						}
					case "b", "esc":
						if command != nil || model.modal.isOpen() || model.browser.selectedID != connection.ID {
							t.Fatalf("run %d: state %v Back alias %q did not close locally", run, recovery, pressed.String())
						}
					case "q", "ctrl+c":
						sc009CheckQuitOwnerRetained(t, model, command, run)
					case "?":
						if command != nil || !model.modal.helpVisible || model.modal.kind != modalKindSSHFailure {
							t.Fatalf("run %d: state %v Help did not remain inline", run, recovery)
						}
						sc009Update(t, model, keyPress("esc"))
						if model.modal.helpVisible || model.modal.kind != modalKindSSHFailure {
							t.Fatalf("run %d: state %v Help Esc did not restore diagnostic", run, recovery)
						}
					}
				}
			})
		}
	}

	for _, closeKey := range []tea.KeyPressMsg{keyPress("?"), keyPress("esc")} {
		t.Run("ssh_retry_confirmation_help/"+closeKey.String(), func(t *testing.T) {
			for run := range sc009Runs {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				sc009Snapshot(model, root, []app.Folder{folder}, []app.Connection{connection}, connection.ID)
				diagnostic := newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation())
				diagnostic.recovery = recoveryConfirming
				model.installSSHFailure(diagnostic)
				model.modal.payload = sshFailurePayload{modal: diagnostic, confirmation: newRetryConnectConfirmation(diagnostic.attempt, connection)}
				confirmation := model.modal.payload.(sshFailurePayload).confirmation
				sc009Update(t, model, keyPress("?"))
				if !model.modal.helpVisible || model.modal.kind != modalKindSSHFailure {
					t.Fatalf("run %d: retry confirmation Help replaced owner", run)
				}
				sc009Update(t, model, closeKey)
				payload := model.modal.payload.(sshFailurePayload)
				if model.modal.helpVisible || payload.confirmation != confirmation || payload.modal != diagnostic || model.focusOwner != focusOwnerModal {
					t.Fatalf("run %d: retry confirmation Help close alias %q lost subphase", run, closeKey.String())
				}
			}
		})
	}
}

func sc009CheckFailedOperationRetained(t *testing.T, model *Model, command tea.Cmd, run int) {
	t.Helper()
	if command == nil || model.operation == nil || !model.modal.isOpen() {
		t.Fatalf("run %d: displayed Save did not start modal-owned operation", run)
	}
	kind, target, payload := model.modal.kind, model.modal.target.clone(), model.modal.payload
	sc009RunCommand(t, model, command)
	if model.operation != nil || !model.modal.isOpen() || model.modal.kind != kind || model.modal.target == nil || model.modal.target.id != target.id || model.modal.payload != payload || model.modal.recoverableError == "" || !strings.Contains(model.View().Content, "Error:") {
		t.Fatalf("run %d: failed Save did not clean operation and retain modal target/value/focus", run)
	}
}

func sc009CheckModalClosed(t *testing.T, model *Model, command tea.Cmd, run int) {
	t.Helper()
	if command != nil || model.modal.isOpen() || model.focusOwner != focusOwnerTree {
		t.Fatalf("run %d: displayed Back/Close did not restore opener", run)
	}
}

func sc009CheckQuitOwnerRetained(t *testing.T, model *Model, command tea.Cmd, run int) {
	t.Helper()
	if command == nil {
		t.Fatalf("run %d: displayed Quit alias returned no command", run)
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatalf("run %d: displayed Quit alias returned %T", run, command())
	}
	if !model.modal.isOpen() || model.focusOwner != focusOwnerModal {
		t.Fatalf("run %d: Quit destroyed modal owner before program exit", run)
	}
}
