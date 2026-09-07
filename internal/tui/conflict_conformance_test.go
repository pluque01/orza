package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestT095NavigationReloadNoticeFirstSynchronizedFrame(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "missing"
		wantNotice := "Warning: TARGET NO LONGER EXISTS"
		if changed {
			name = "changed_revision"
			wantNotice = "Warning: CATALOG CHANGED"
		}
		for _, display := range []struct {
			name    string
			width   int
			height  int
			noColor bool
		}{
			{name: "normal", width: 100, height: 30},
			{name: "reduced", width: 40, height: 12},
			{name: "no_color", width: 80, height: 24, noColor: true},
		} {
			t.Run(name+"/"+display.name, func(t *testing.T) {
				initial := newSC012Catalog(app.NodeKindConnection, false, false).initial()
				current := newSC012Catalog(app.NodeKindConnection, !changed, false)
				model := newSC012Model(initial, current, &sc012ServiceCounters{})
				model.width, model.height = display.width, display.height
				model.noColor, model.styles = display.noColor, newStyles(display.noColor)
				model.focusOwner = focusOwnerDetail
				command := sc012Update(t, model, keyPress("r"))
				sc012RunCommand(t, model, command)

				frame := model.View().Content
				if !strings.Contains(frame, wantNotice) || model.detailState.targetID != model.browser.selectedID {
					t.Fatalf("first synchronized frame omitted notice/detail synchronization:\n%s", frame)
				}
				assertUS5FrameBounded(t, frame, display.width, display.height)
				if display.noColor && strings.Contains(frame, "\x1b") {
					t.Fatal("no-color navigation notice emitted ANSI")
				}
			})
		}
	}
}

const sc012ConflictRuns = 20

type sc012Catalog struct {
	root       app.Folder
	parent     app.Folder
	target     app.Node
	connection *app.Connection
	folder     *app.Folder
	sibling    app.Connection
	revision   app.CatalogRevision
	missing    bool
	rootOnly   bool
}

type sc012ServiceCounters struct {
	catalogReads        int
	connectionMutations int
	folderMutations     int
	targetReads         int
	sshStarts           int
}

type sc012ModalCase struct {
	name string
	kind modalKind
	node app.NodeKind
}

func TestSC012NavigationConflictConformance20Runs(t *testing.T) {
	cells := []struct {
		name     string
		missing  bool
		rootOnly bool
	}{
		{name: "missing_nearest_ancestor", missing: true},
		{name: "missing_root_fallback", missing: true, rootOnly: true},
		{name: "same_id_revision_changed"},
	}

	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			for run := range sc012ConflictRuns {
				initial := newSC012Catalog(app.NodeKindConnection, false, false).initial()
				current := newSC012Catalog(app.NodeKindConnection, cell.missing, cell.rootOnly)
				counters := &sc012ServiceCounters{}
				model := newSC012Model(initial, current, counters)
				model.focusOwner = focusOwnerDetail
				model.browser.selectedID = initial.target.ID
				model.ownedSelectionID = initial.target.ID
				model.syncDetail()

				command := sc012Update(t, model, keyPress("r"))
				sc012RunCommand(t, model, command)

				wantID := initial.target.ID
				if cell.missing {
					wantID = initial.parent.ID
					if cell.rootOnly {
						wantID = initial.root.ID
					}
				}
				if model.browser.selectedID != wantID || model.detailState.targetID != wantID || model.focusOwner != focusOwnerDetail {
					t.Fatalf("run %d: navigation result selection=%q detail=%q focus=%v, want %q/detail/focus", run, model.browser.selectedID, model.detailState.targetID, model.focusOwner, wantID)
				}
				if !cell.missing {
					node := model.browser.snapshot.nodes[initial.target.ID]
					if node.node.Revision != initial.target.Revision+1 || !model.browser.changed || model.connectionEdit != nil || model.modal.conflict != nil {
						t.Fatalf("run %d: same-ID revision refresh = revision %d changed=%v form=%v conflict=%v", run, node.node.Revision, model.browser.changed, model.connectionEdit != nil, model.modal.conflict != nil)
					}
				}
				wantReads := 3
				if cell.rootOnly {
					wantReads = 2
				}
				assertSC012ServiceCounts(t, counters, wantReads, 0, run)
			}
		})
	}
}

func TestSC012FormConflictRecoveryConformance20Runs(t *testing.T) {
	conflicts := []struct {
		name string
		kind conflictType
		err  error
	}{
		{name: "missing", kind: conflictTypeMissing, err: app.ErrNotFound},
		{name: "revision_changed", kind: conflictTypeRevisionChanged, err: app.ErrConflict},
	}
	actions := []string{"reload", "back", "esc"}

	for _, conflict := range conflicts {
		for _, action := range actions {
			t.Run(conflict.name+"/"+action, func(t *testing.T) {
				for run := range sc012ConflictRuns {
					initial := newSC012Catalog(app.NodeKindConnection, false, false).initial()
					current := newSC012Catalog(app.NodeKindConnection, conflict.kind == conflictTypeMissing, false)
					counters := &sc012ServiceCounters{}
					model := newSC012Model(initial, current, counters)
					model.connections = sc012ConnectionService(t, counters, initial.target.ID, conflict.err, run)
					model.browser.selectedID = initial.target.ID
					model.ownedSelectionID = initial.target.ID
					sc012Update(t, model, keyPress("e"))
					form := model.form
					form.inputs[fieldHost].SetValue("pending.test")
					form.setFocus(fieldHost)
					model.browser.selectedID = initial.sibling.ID

					save := sc012Update(t, model, keyPress("ctrl+s"))
					sc012RunCommand(t, model, save)
					state := model.connectionEdit
					if state == nil || state.conflict == nil || state.conflict.kind != conflict.kind || state.conflict.target.id != initial.target.ID || state.conflict.target.revision != initial.target.Revision {
						t.Fatalf("run %d: form conflict = %#v", run, state)
					}
					assertSC012FormStable(t, model, form, initial.target.ID, run)
					if blocked := sc012Update(t, model, keyPress("ctrl+s")); blocked != nil || counters.connectionMutations != 1 {
						t.Fatalf("run %d: blocked form persisted: command=%v mutations=%d", run, blocked != nil, counters.connectionMutations)
					}

					switch action {
					case "reload":
						sc012RunCommand(t, model, sc012Update(t, model, keyPress("r")))
						assertSC012FormStable(t, model, form, initial.target.ID, run)
						if model.connectionEdit.conflict == nil || model.connectionEdit.conflict.kind != conflict.kind || model.connectionEdit.conflict.target.id != initial.target.ID {
							t.Fatalf("run %d: Reload lost unresolved form conflict: %#v", run, model.connectionEdit.conflict)
						}
					case "back":
						if command := sc012Update(t, model, keyPress("b")); command != nil || model.form != nil || model.modal.isOpen() || model.browser.selectedID != initial.parent.ID || model.focusOwner != focusOwnerTree {
							t.Fatalf("run %d: Back result command=%v form=%v modal=%v selection=%q focus=%v", run, command != nil, model.form != nil, model.modal.isOpen(), model.browser.selectedID, model.focusOwner)
						}
					case "esc":
						sc012Update(t, model, keyPress("esc"))
						assertSC012FormStable(t, model, form, initial.target.ID, run)
						if conflict := model.connectionEdit.conflict; conflict == nil || conflict.detailVisible || !conflict.blocked {
							t.Fatalf("run %d: Esc form conflict = %#v", run, conflict)
						}
					}
					wantReads := 0
					if action == "reload" {
						wantReads = 3
					}
					assertSC012ServiceCounts(t, counters, wantReads, 1, run)
				}
			})
		}
	}
}

func TestSC012TargetBearingModalConflictRecoveryConformance20Runs(t *testing.T) {
	modals := []sc012ModalCase{
		{name: "folder_create", kind: modalKindFolderCreate, node: app.NodeKindFolder},
		{name: "folder_edit", kind: modalKindFolderEdit, node: app.NodeKindFolder},
		{name: "move_picker", kind: modalKindMovePicker, node: app.NodeKindConnection},
		{name: "delete_connection", kind: modalKindDeleteConnection, node: app.NodeKindConnection},
		{name: "delete_folder", kind: modalKindDeleteFolder, node: app.NodeKindFolder},
	}
	conflicts := []struct {
		name string
		kind conflictType
		err  error
	}{
		{name: "missing", kind: conflictTypeMissing, err: app.ErrNotFound},
		{name: "revision_changed", kind: conflictTypeRevisionChanged, err: app.ErrConflict},
	}
	actions := []string{"reload", "back", "esc"}

	for _, modalCase := range modals {
		for _, conflict := range conflicts {
			for _, action := range actions {
				t.Run(modalCase.name+"/"+conflict.name+"/"+action, func(t *testing.T) {
					for run := range sc012ConflictRuns {
						initial := newSC012Catalog(modalCase.node, false, false).initial()
						current := newSC012Catalog(modalCase.node, conflict.kind == conflictTypeMissing, false)
						counters := &sc012ServiceCounters{}
						model := newSC012Model(initial, current, counters)
						sc012InstallModalConflictServices(t, counters, model, current, initial.target.ID, conflict.err, run)
						model.focusOwner = focusOwnerDetail
						model.browser.selectedID = initial.sibling.ID
						model.ownedSelectionID = initial.sibling.ID
						target := sc012CapturedTarget(initial)
						payload := sc012ModalPayload(modalCase.kind, initial, target)
						if !model.openGenericModal(modalCase.kind, &target, payload) {
							t.Fatalf("run %d: modal open failed", run)
						}
						operation := sc012StartModalAction(t, model, modalCase.kind)
						cleanupCalls := 0
						model.operation.cancel = func() { cleanupCalls++ }
						sc012Update(t, model, operation())
						readsBeforeRecovery := counters.catalogReads
						mutationsBeforeRecovery := counters.connectionMutations + counters.folderMutations
						if cleanupCalls != 1 || model.operation != nil {
							t.Fatalf("run %d: modal conflict cleanup=%d operation=%#v", run, cleanupCalls, model.operation)
						}
						stablePayload := model.modal.payload
						if model.modal.conflict == nil || model.modal.conflict.kind != conflict.kind || model.modal.conflict.target.id != target.id || model.modal.conflict.target.revision != target.revision {
							t.Fatalf("run %d: modal conflict = %#v", run, model.modal.conflict)
						}
						if command := sc012Update(t, model, keyPress("y")); command != nil || counters.connectionMutations+counters.folderMutations != mutationsBeforeRecovery || counters.sshStarts != 0 {
							t.Fatalf("run %d: blocked modal authorized current row: command=%v mutations=%d", run, command != nil, counters.totalMutations())
						}

						switch action {
						case "reload":
							sc012RunCommand(t, model, sc012Update(t, model, keyPress("r")))
							if !model.modal.isOpen() || model.modal.kind != modalCase.kind || model.focusOwner != focusOwnerModal || !reflect.DeepEqual(model.modal.payload, stablePayload) {
								t.Fatalf("run %d: Reload lost modal kind/focus/payload", run)
							}
							if model.modal.conflict == nil || model.modal.conflict.kind != conflict.kind || model.modal.conflict.target.id != target.id || model.modal.conflict.target.revision != target.revision {
								t.Fatalf("run %d: Reload retargeted modal conflict: %#v", run, model.modal.conflict)
							}
						case "back":
							if command := sc012Update(t, model, keyPress("b")); command != nil || model.modal.isOpen() || model.browser.selectedID != initial.parent.ID || model.focusOwner != focusOwnerDetail {
								t.Fatalf("run %d: Back modal result command=%v open=%v selection=%q focus=%v", run, command != nil, model.modal.isOpen(), model.browser.selectedID, model.focusOwner)
							}
						case "esc":
							sc012Update(t, model, keyPress("esc"))
							if model.modal.conflict == nil || model.modal.conflict.detailVisible || !model.modal.conflict.blocked || model.focusOwner != focusOwnerModal || !reflect.DeepEqual(model.modal.payload, stablePayload) {
								t.Fatalf("run %d: Esc did not retain compact modal owner", run)
							}
						}
						wantReads := readsBeforeRecovery
						if action == "reload" {
							wantReads += 3
							if modalCase.node == app.NodeKindFolder && conflict.kind == conflictTypeRevisionChanged {
								wantReads++
							}
						}
						if counters.catalogReads != wantReads || counters.connectionMutations+counters.folderMutations != mutationsBeforeRecovery || counters.targetReads != 0 || counters.sshStarts != 0 {
							t.Fatalf("run %d: modal service counts reads=%d mutations=%d target_reads=%d ssh_starts=%d, want %d/%d/0/0", run, counters.catalogReads, counters.connectionMutations+counters.folderMutations, counters.targetReads, counters.sshStarts, wantReads, mutationsBeforeRecovery)
						}
					}
				})
			}
		}
	}
}

func TestSC012PendingOperationConflictRecoveryConformance20Runs(t *testing.T) {
	operations := []struct {
		name string
		kind asyncOperationKind
	}{
		{name: "reload", kind: asyncOperationReload},
		{name: "save", kind: asyncOperationSave},
		{name: "ssh_start", kind: asyncOperationSSHStart},
	}
	conflicts := []struct {
		name string
		kind conflictType
		err  error
	}{
		{name: "missing", kind: conflictTypeMissing, err: app.ErrNotFound},
		{name: "revision_changed", kind: conflictTypeRevisionChanged, err: app.ErrConflict},
	}
	actions := []string{"reload", "back", "esc"}

	for _, operation := range operations {
		for _, conflict := range conflicts {
			for _, action := range actions {
				t.Run(operation.name+"/"+conflict.name+"/"+action, func(t *testing.T) {
					for run := range sc012ConflictRuns {
						fixture := newOperationConflictFixture(t, operation.kind, conflict.kind)
						stableForm := fixture.model.form
						stableFormHost := ""
						stableFormFocus := fieldName
						if stableForm != nil {
							stableFormHost = stableForm.inputs[fieldHost].Value()
							stableFormFocus = stableForm.focusedField()
						}
						cleanupCalls := installOperationCleanupProbe(t, fixture, run)
						sc012Update(t, fixture.model, operationConflictMessage(fixture, conflict.err))
						if *cleanupCalls != 1 || fixture.model.operation != nil {
							t.Fatalf("run %d: pending conflict dispatched before cleanup: calls=%d operation=%#v", run, *cleanupCalls, fixture.model.operation)
						}
						published := operationPublishedConflict(fixture.model)
						if published == nil || published.kind != conflict.kind || published.target.id != fixture.connection.ID || published.target.revision != fixture.connection.Revision {
							t.Fatalf("run %d: pending conflict = %#v", run, published)
						}
						stablePayload := fixture.model.modal.payload

						other := testConnection("other", syntheticRootID, "/other", 9)
						fixture.model.browser.setConnectionsPending(app.ListConnectionsResult{Connections: []app.Connection{fixture.connection, other}, CatalogRevision: 8}, other.ID)
						fixture.model.browser.selectedID = other.ID
						fixture.model.ownedSelectionID = other.ID
						counters := &sc012ServiceCounters{}
						observed := fixture.connection
						observed.Revision++
						values := []app.Connection{other}
						if conflict.kind == conflictTypeRevisionChanged {
							values = append(values, observed)
						}
						fixture.model.connections = sc012ReloadConnectionService(counters, values)
						switch action {
						case "reload":
							sc012RunCommand(t, fixture.model, sc012Update(t, fixture.model, keyPress("r")))
							published = operationPublishedConflict(fixture.model)
							if published == nil || published.kind != conflict.kind || published.target.id != fixture.connection.ID || published.target.revision != fixture.connection.Revision {
								t.Fatalf("run %d: pending Reload retargeted conflict: %#v", run, published)
							}
							if operation.kind == asyncOperationSSHStart {
								if !fixture.model.modal.isOpen() || !reflect.DeepEqual(fixture.model.modal.payload, stablePayload) || fixture.model.focusOwner != focusOwnerModal {
									t.Fatalf("run %d: pending modal Reload lost values/focus", run)
								}
							} else {
								if fixture.model.form != stableForm || stableForm.inputs[fieldHost].Value() != stableFormHost || stableForm.focusedField() != stableFormFocus || fixture.model.focusOwner != focusOwnerConnectionForm || fixture.model.connectionEdit.target.id != fixture.connection.ID {
									t.Fatalf("run %d: pending form Reload lost target/values/focus", run)
								}
							}
						case "back":
							sc012Update(t, fixture.model, keyPress("b"))
							if fixture.model.modal.isOpen() || fixture.model.form != nil || fixture.model.browser.selectedID != syntheticRootID {
								t.Fatalf("run %d: pending Back did not use root fallback: modal=%v form=%v selection=%q", run, fixture.model.modal.isOpen(), fixture.model.form != nil, fixture.model.browser.selectedID)
							}
						case "esc":
							sc012Update(t, fixture.model, keyPress("esc"))
							published = operationPublishedConflict(fixture.model)
							if published == nil || published.detailVisible || !published.blocked || published.target.id != fixture.connection.ID {
								t.Fatalf("run %d: pending Esc conflict = %#v", run, published)
							}
						}
						wantReads := 0
						if action == "reload" {
							wantReads = 1
						}
						assertSC012ServiceCounts(t, counters, wantReads, 0, run)
					}
				})
			}
		}
	}
}

func newSC012Catalog(kind app.NodeKind, missing, rootOnly bool) sc012Catalog {
	root := testFolder("root", "", "/", 1)
	parent := testFolder("parent", root.ID, "/parent", 2)
	sibling := testConnection("current-row", parent.ID, "/parent/current-row", 9)
	catalog := sc012Catalog{root: root, parent: parent, sibling: sibling, revision: 2, missing: missing, rootOnly: rootOnly}
	if kind == app.NodeKindFolder {
		target := testFolder("captured", parent.ID, "/parent/captured", 4)
		if !missing {
			target.Revision++
			target.Name = "captured-current"
		}
		catalog.target = target.Node
		catalog.folder = &target
	} else {
		target := testConnection("captured", parent.ID, "/parent/captured", 4)
		if !missing {
			target.Revision++
			target.Host = "current.test"
		}
		catalog.target = target.Node
		catalog.connection = &target
	}
	return catalog
}

func (catalog sc012Catalog) initial() sc012Catalog {
	catalog.revision = 1
	catalog.missing = false
	catalog.rootOnly = false
	if catalog.folder != nil {
		folder := *catalog.folder
		folder.Revision--
		folder.Name = "captured"
		catalog.folder = &folder
		catalog.target = folder.Node
	} else {
		connection := *catalog.connection
		connection.Revision--
		connection.Host = "host.test"
		catalog.connection = &connection
		catalog.target = connection.Node
	}
	return catalog
}

func newSC012Model(initial, current sc012Catalog, counters *sc012ServiceCounters) *Model {
	model := New(Config{
		Width: 80, Height: 24, NoColor: true,
		Connections: sc012ReloadConnectionService(counters, nil),
		Folders:     sc012FolderService(counters, current),
		Connect: ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
			counters.sshStarts++
			return app.ConnectResult{}, app.ErrInvalidRequest
		}),
	})
	setSC012Snapshot(model, initial, initial.target.ID)
	return model
}

func setSC012Snapshot(model *Model, catalog sc012Catalog, selected app.NodeID) {
	snapshot := newCatalogSnapshot(catalog.root, catalog.revision)
	if !catalog.rootOnly {
		_ = snapshot.addChildren(catalog.root.ID, app.ListChildrenResult{Folders: []app.Folder{catalog.parent}, CatalogRevision: catalog.revision})
		children := app.ListChildrenResult{Connections: []app.Connection{catalog.sibling}, CatalogRevision: catalog.revision}
		if !catalog.missing {
			if catalog.folder != nil {
				children.Folders = []app.Folder{*catalog.folder}
			} else {
				children.Connections = append(children.Connections, *catalog.connection)
			}
		}
		_ = snapshot.addChildren(catalog.parent.ID, children)
	}
	model.browser.setSnapshot(snapshot, selected)
	model.browser.selectedID = selected
	model.browser.expandAncestors(selected)
	model.browser.rebuildRows()
	model.ownedSelectionID = model.browser.selectedID
	model.syncDetail()
}

func sc012FolderService(counters *sc012ServiceCounters, catalog sc012Catalog) FolderFuncs {
	return FolderFuncs{
		GetFunc: func(_ context.Context, selector app.ItemSelector) (app.FolderResult, error) {
			counters.catalogReads++
			if selector.Path == "/" || selector.ID == catalog.root.ID {
				return app.FolderResult{Folder: catalog.root, CatalogRevision: catalog.revision}, nil
			}
			if catalog.folder != nil && selector.ID == catalog.folder.ID && !catalog.missing {
				return app.FolderResult{Folder: *catalog.folder, CatalogRevision: catalog.revision}, nil
			}
			return app.FolderResult{}, app.ErrNotFound
		},
		ListFunc: func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
			counters.catalogReads++
			switch request.Folder.ID {
			case catalog.root.ID:
				result := app.ListChildrenResult{CatalogRevision: catalog.revision}
				if !catalog.rootOnly {
					result.Folders = []app.Folder{catalog.parent}
				}
				return result, nil
			case catalog.parent.ID:
				result := app.ListChildrenResult{Connections: []app.Connection{catalog.sibling}, CatalogRevision: catalog.revision}
				if !catalog.missing {
					if catalog.folder != nil {
						result.Folders = []app.Folder{*catalog.folder}
					} else {
						result.Connections = append(result.Connections, *catalog.connection)
					}
				}
				return result, nil
			default:
				return app.ListChildrenResult{CatalogRevision: catalog.revision}, nil
			}
		},
		CreateFunc: func(context.Context, app.CreateFolderRequest) (app.FolderResult, error) {
			counters.folderMutations++
			return app.FolderResult{}, app.ErrInvalidRequest
		},
		RenameFunc: func(context.Context, app.RenameFolderRequest) (app.FolderResult, error) {
			counters.folderMutations++
			return app.FolderResult{}, app.ErrInvalidRequest
		},
		MoveFunc: func(context.Context, app.MoveFolderRequest) (app.FolderResult, error) {
			counters.folderMutations++
			return app.FolderResult{}, app.ErrInvalidRequest
		},
		DeleteScopeFunc: func(context.Context, app.ItemSelector) (app.FolderDeleteScope, error) {
			counters.folderMutations++
			return app.FolderDeleteScope{}, app.ErrInvalidRequest
		},
		DeleteFunc: func(context.Context, app.DeleteFolderRequest) (app.DeleteFolderResult, error) {
			counters.folderMutations++
			return app.DeleteFolderResult{}, app.ErrInvalidRequest
		},
	}
}

func sc012ConnectionService(t *testing.T, counters *sc012ServiceCounters, captured app.NodeID, resultErr error, run int) ConnectionFuncs {
	service := sc012ReloadConnectionService(counters, nil)
	service.ListFunc = nil
	service.UpdateFunc = func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
		counters.connectionMutations++
		if request.Connection.ID != captured {
			t.Fatalf("run %d: form persistence retargeted to current row %q", run, request.Connection.ID)
		}
		return app.ConnectionResult{}, resultErr
	}
	return service
}

func sc012InstallModalConflictServices(t *testing.T, counters *sc012ServiceCounters, model *Model, catalog sc012Catalog, captured app.NodeID, resultErr error, run int) {
	t.Helper()
	connections := sc012ReloadConnectionService(counters, nil)
	connections.MoveFunc = func(_ context.Context, request app.MoveConnectionRequest) (app.ConnectionResult, error) {
		counters.connectionMutations++
		if request.Connection.ID != captured {
			t.Fatalf("run %d: modal move retargeted request to %q", run, request.Connection.ID)
		}
		return app.ConnectionResult{}, resultErr
	}
	connections.DeleteFunc = func(_ context.Context, request app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
		counters.connectionMutations++
		if request.Connection.ID != captured {
			t.Fatalf("run %d: modal delete retargeted request to %q", run, request.Connection.ID)
		}
		return app.DeleteConnectionResult{}, resultErr
	}
	model.connections = connections

	folders := sc012FolderService(counters, catalog)
	folders.CreateFunc = func(_ context.Context, request app.CreateFolderRequest) (app.FolderResult, error) {
		counters.folderMutations++
		if request.Parent.ID != captured {
			t.Fatalf("run %d: modal folder create retargeted request to %q", run, request.Parent.ID)
		}
		return app.FolderResult{}, resultErr
	}
	folders.RenameFunc = func(_ context.Context, request app.RenameFolderRequest) (app.FolderResult, error) {
		counters.folderMutations++
		if request.Folder.ID != captured {
			t.Fatalf("run %d: modal folder rename retargeted request to %q", run, request.Folder.ID)
		}
		return app.FolderResult{}, resultErr
	}
	folders.DeleteFunc = func(_ context.Context, request app.DeleteFolderRequest) (app.DeleteFolderResult, error) {
		counters.folderMutations++
		if request.Folder.ID != captured {
			t.Fatalf("run %d: modal folder delete retargeted request to %q", run, request.Folder.ID)
		}
		return app.DeleteFolderResult{}, resultErr
	}
	model.folders = folders
}

func sc012ReloadConnectionService(counters *sc012ServiceCounters, values []app.Connection) ConnectionFuncs {
	return ConnectionFuncs{
		ListFunc: func(_ context.Context, request app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			counters.catalogReads++
			return app.ListConnectionsResult{Connections: values, CatalogRevision: 9}, nil
		},
		GetFunc: func(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
			counters.targetReads++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		CreateFunc: func(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
			counters.connectionMutations++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		UpdateFunc: func(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
			counters.connectionMutations++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		MoveFunc: func(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error) {
			counters.connectionMutations++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		DeleteScopeFunc: func(context.Context, app.ItemSelector) (app.ConnectionDeleteScope, error) {
			counters.connectionMutations++
			return app.ConnectionDeleteScope{}, app.ErrInvalidRequest
		},
		DeleteFunc: func(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
			counters.connectionMutations++
			return app.DeleteConnectionResult{}, app.ErrInvalidRequest
		},
	}
}

func sc012CapturedTarget(catalog sc012Catalog) capturedTarget {
	return capturedTarget{
		id:          catalog.target.ID,
		revision:    catalog.target.Revision,
		kind:        catalog.target.Kind,
		path:        catalog.target.Path,
		ancestorIDs: []app.NodeID{catalog.parent.ID, catalog.root.ID},
	}
}

func sc012ModalPayload(kind modalKind, catalog sc012Catalog, target capturedTarget) any {
	switch kind {
	case modalKindFolderCreate:
		form := newFolderForm(nil, app.ItemSelector{ID: target.id})
		form.setDestination(*catalog.folder)
		form.input.SetValue("pending-folder")
		return folderCreatePayload{form: form}
	case modalKindFolderEdit:
		form := newFolderForm(catalog.folder, app.ItemSelector{})
		form.input.SetValue("pending-folder")
		return folderEditPayload{form: form}
	case modalKindMovePicker:
		picker := newMovePicker(catalog.connection.Node, []app.Folder{catalog.root, catalog.parent})
		picker.selected = 1
		return movePickerPayload{picker: picker}
	case modalKindDeleteConnection:
		connection := *catalog.connection
		return deleteConnectionPayload{confirmation: newDeleteConfirmation(app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision})}
	case modalKindDeleteFolder:
		folder := *catalog.folder
		return deleteFolderPayload{confirmation: newFolderDeleteConfirmation(app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1})}
	default:
		panic("unsupported SC-012 modal")
	}
}

func sc012StartModalAction(t *testing.T, model *Model, kind modalKind) tea.Cmd {
	t.Helper()
	var key string
	switch kind {
	case modalKindFolderCreate, modalKindFolderEdit, modalKindMovePicker:
		key = "enter"
	case modalKindDeleteConnection, modalKindDeleteFolder:
		key = "y"
	default:
		t.Fatalf("unsupported SC-012 modal action %q", kind)
	}
	command := sc012Update(t, model, keyPress(key))
	if command == nil || model.operation == nil || model.operation.target == nil || model.modal.target == nil || model.operation.target.id != model.modal.target.id {
		t.Fatalf("modal %q did not start an operation for its captured target", kind)
	}
	return command
}

func sc012Update(t *testing.T, model *Model, message tea.Msg) tea.Cmd {
	t.Helper()
	updated, command := model.Update(message)
	if updated != model {
		t.Fatal("Update replaced the root model")
	}
	return command
}

func sc012RunCommand(t *testing.T, model *Model, command tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatal("expected command")
	}
	sc012Update(t, model, command())
}

func assertSC012FormStable(t *testing.T, model *Model, form *connectionForm, target app.NodeID, run int) {
	t.Helper()
	if model.form != form || form.inputs[fieldHost].Value() != "pending.test" || form.focusedField() != fieldHost || model.focusOwner != focusOwnerConnectionForm || model.connectionEdit == nil || model.connectionEdit.target.id != target {
		t.Fatalf("run %d: form target/values/focus changed: form=%v host=%q field=%v focus=%v target=%#v", run, model.form == form, form.inputs[fieldHost].Value(), form.focusedField(), model.focusOwner, model.connectionEdit)
	}
}

func assertSC012ServiceCounts(t *testing.T, counters *sc012ServiceCounters, reads, mutations, run int) {
	t.Helper()
	if counters.catalogReads != reads || counters.connectionMutations != mutations || counters.folderMutations != 0 || counters.targetReads != 0 || counters.sshStarts != 0 {
		t.Fatalf("run %d: service counts reads=%d connection_mutations=%d folder_mutations=%d target_reads=%d ssh_starts=%d, want %d/%d/0/0/0", run, counters.catalogReads, counters.connectionMutations, counters.folderMutations, counters.targetReads, counters.sshStarts, reads, mutations)
	}
}

func (counters *sc012ServiceCounters) totalMutations() int {
	return counters.connectionMutations + counters.folderMutations + counters.sshStarts
}
