package tui

import (
	"context"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestModelCompleteTreeRevisionRetryAndSecondConflict(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	child := testFolder("child", root.ID, "/child", 1)
	getCalls := 0
	service := FolderFuncs{
		GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			getCalls++
			revision := app.CatalogRevision(getCalls)
			return app.FolderResult{Folder: root, CatalogRevision: revision}, nil
		},
		ListFunc: func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
			if getCalls == 1 {
				return app.ListChildrenResult{CatalogRevision: 2}, nil
			}
			if request.Folder.ID == root.ID {
				return app.ListChildrenResult{Folders: []app.Folder{child}, CatalogRevision: 2}, nil
			}
			return app.ListChildrenResult{CatalogRevision: 2}, nil
		},
	}
	model := New(Config{Folders: service, Width: 80, Height: 24, NoColor: true})
	updateModel(model, model.Init()())
	if getCalls != 2 || !model.browser.hasNode(child.ID) || model.browser.revision != 2 {
		t.Fatalf("revision retry calls=%d child=%v revision=%d", getCalls, model.browser.hasNode(child.ID), model.browser.revision)
	}

	validSnapshot := model.browser.snapshot
	conflicts := 0
	model.folders = FolderFuncs{
		GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			conflicts++
			return app.FolderResult{Folder: root, CatalogRevision: app.CatalogRevision(10 + conflicts)}, nil
		},
		ListFunc: func(context.Context, app.ListChildrenRequest) (app.ListChildrenResult, error) {
			return app.ListChildrenResult{CatalogRevision: 99}, nil
		},
	}
	updateModel(model, model.reloadCommand()())
	if conflicts != 2 || model.modal.kind != modalKindOperationError || model.browser.snapshot.revision != validSnapshot.revision {
		t.Fatalf("second conflict calls=%d modal=%#v revision=%d", conflicts, model.modal, model.browser.snapshot.revision)
	}
}

func TestLoadCatalogSnapshotAcceptsConnectionBeyondDepthTen(t *testing.T) {
	root := testFolder("00000000000000000000000000000000", "", "/", 1)
	children := make(map[app.NodeID]app.ListChildrenResult)
	parent := root
	for depth := 1; depth <= 10; depth++ {
		path := parent.Path
		if path != "/" {
			path += "/"
		}
		folder := testFolder(strings.Repeat("0", 31)+string("123456789a"[depth-1]), parent.ID, path+"level", 1)
		result := children[parent.ID]
		result.Folders = []app.Folder{folder}
		result.CatalogRevision = 1
		children[parent.ID] = result
		parent = folder
	}
	result := children[parent.ID]
	result.Connections = []app.Connection{testConnection("ffffffffffffffffffffffffffffffff", parent.ID, parent.Path+"/too-deep", 1)}
	result.CatalogRevision = 1
	children[parent.ID] = result
	service := FolderFuncs{
		GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			return app.FolderResult{Folder: root, CatalogRevision: 1}, nil
		},
		ListFunc: func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
			return children[request.Folder.ID], nil
		},
	}
	snapshot, err := loadCatalogSnapshot(context.Background(), service)
	_, hasDeepConnection := snapshot.nodes["ffffffffffffffffffffffffffffffff"]
	if err != nil || !hasDeepConnection {
		t.Fatalf("loadCatalogSnapshot() = (%#v, %v), want deep connection", snapshot, err)
	}
}

func TestOwnerlessOperationResultIsRejected(t *testing.T) {
	model := loadedModel(t, nil, true)
	revision := model.browser.revision
	updateModel(model, operationResultMsg{id: 9, kind: operationReload, list: app.ListConnectionsResult{CatalogRevision: 99}})
	if model.browser.revision != revision {
		t.Fatal("ownerless result changed the catalog")
	}
}

func TestModelContextualNavigationConfirmationAndNarrowMode(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/prod", 4)
	model := loadedModel(t, []app.Connection{connection}, true)
	if model.browser.selectedID != syntheticRootID {
		t.Fatalf("initial selection = %q, want root", model.browser.selectedID)
	}
	updateModel(model, keyPress("l"))
	if model.browser.selectedID != connection.ID {
		t.Fatalf("right selected %q", model.browser.selectedID)
	}
	updateModel(model, keyPress("c"))
	payload, ok := model.modal.payload.(connectConfirmationPayload)
	if !ok {
		t.Fatal("connect did not open confirmation")
	}
	captured := payload.confirmation.connection
	copy := model.browser.snapshot.nodes[connection.ID]
	copy.connection.Host = "changed.test"
	if payload.confirmation.connection.Host != captured.Host || !strings.Contains(model.View().Content, "/prod") || !strings.Contains(model.View().Content, "host.test:22") {
		t.Fatalf("confirmation was mutable or incomplete: %q", model.View().Content)
	}
	updateModel(model, keyPress("esc"))
	if model.modal.isOpen() || model.browser.selectedID != connection.ID {
		t.Fatal("cancel did not return to captured node")
	}

	updateModel(model, tea.WindowSizeMsg{Width: 42, Height: 10})
	view := model.View().Content
	for _, value := range []string{"Terminal too small", "Required minimum: 40x12", "? Help", "q Quit"} {
		if !strings.Contains(view, value) {
			t.Fatalf("undersized view omitted %q: %q", value, view)
		}
	}
	updateModel(model, keyPress("h"))
	if model.browser.selectedID != connection.ID {
		t.Fatal("undersized mode mutated the preserved selection")
	}
}

func TestModelPendingSelectionAndFormPreservation(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/prod", 1)
	model := loadedModel(t, []app.Connection{connection}, true)
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("e"))
	form := model.form
	form.inputs[fieldName].SetValue("pending")
	model.operationID = 2
	id, _, _ := model.beginOperationWith(asyncOperationReload, nil, operationOwnerForm)
	newConnection := connection
	newConnection.Revision = 2
	updateModel(model, operationResultMsg{id: id, kind: operationReload, list: app.ListConnectionsResult{Connections: []app.Connection{newConnection}, CatalogRevision: 8}})
	if model.form != form || model.form.inputs[fieldName].Value() != "pending" || !model.browser.changed {
		t.Fatal("reload discarded form content or missed changed catalog")
	}
}

func TestModelStaleContextualDestinationPreservesForm(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 2)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}})
	createCalls := 0
	model := New(Config{
		Width: 80, Height: 24, NoColor: true,
		Folders: FolderFuncs{GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			changed := folder
			changed.Revision++
			return app.FolderResult{Folder: changed, CatalogRevision: 2}, nil
		}},
		Connections: ConnectionFuncs{CreateFunc: func(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
			createCalls++
			return app.ConnectionResult{}, nil
		}},
	})
	model.browser.setSnapshot(snapshot, "")
	model.browser.selectedID = folder.ID
	model.browser.rebuildRows()
	updateModel(model, keyPress("n"))
	form := model.form
	form.inputs[fieldName].SetValue("pending")
	form.inputs[fieldHost].SetValue("pending.test")
	_, command := model.Update(keyPress("ctrl+s"))
	if command == nil {
		t.Fatal("save did not start destination preflight")
	}
	updateModel(model, command())
	if createCalls != 0 || model.form != form || form.inputs[fieldName].Value() != "pending" || model.connectionEdit == nil || model.connectionEdit.conflict == nil {
		t.Fatalf("stale destination mutated or discarded form: calls=%d form=%p conflict=%#v", createCalls, model.form, model.connectionEdit)
	}
}

func TestLateOperationMessageIsIgnored(t *testing.T) {
	model := loadedModel(t, nil, true)
	revision := model.browser.revision
	model.operationID = 8
	id, _, _ := model.beginOperationWith(asyncOperationReload, nil, operationOwnerRoot)
	updateModel(model, operationResultMsg{id: id - 1, kind: operationReload, list: app.ListConnectionsResult{CatalogRevision: 99}})
	if model.browser.revision != revision || model.operation == nil {
		t.Fatal("late result changed model")
	}
}

func TestMoveCommandPinsSourceAndDestinationRevisions(t *testing.T) {
	source := app.Node{ID: "11111111111111111111111111111111", Kind: app.NodeKindConnection, Path: "/source", Revision: 3}
	destination := testFolder("22222222222222222222222222222222", "", "/destination", 7)
	var captured app.MoveConnectionRequest
	model := New(Config{Connections: ConnectionFuncs{MoveFunc: func(_ context.Context, request app.MoveConnectionRequest) (app.ConnectionResult, error) {
		captured = request
		return app.ConnectionResult{Connection: app.Connection{Node: source}}, nil
	}}})
	message := model.moveCommand(source, destination)()
	if _, ok := message.(operationResultMsg); !ok {
		t.Fatalf("move command message = %T", message)
	}
	if captured.Expected == nil || *captured.Expected != source.Revision || captured.ExpectedDestination == nil || *captured.ExpectedDestination != destination.Revision || captured.ExpectedSourcePath != source.Path || captured.ExpectedDestinationPath != destination.Path {
		t.Fatalf("move request = %#v", captured)
	}

	folderSource := source
	folderSource.Kind = app.NodeKindFolder
	var capturedFolder app.MoveFolderRequest
	model = New(Config{Folders: FolderFuncs{MoveFunc: func(_ context.Context, request app.MoveFolderRequest) (app.FolderResult, error) {
		capturedFolder = request
		return app.FolderResult{Folder: app.Folder{Node: folderSource}}, nil
	}}})
	_ = model.moveCommand(folderSource, destination)()
	if capturedFolder.Expected == nil || *capturedFolder.Expected != folderSource.Revision || capturedFolder.ExpectedDestination == nil || *capturedFolder.ExpectedDestination != destination.Revision || capturedFolder.ExpectedSourcePath != folderSource.Path || capturedFolder.ExpectedDestinationPath != destination.Path {
		t.Fatalf("folder move request = %#v", capturedFolder)
	}
}

func TestModelRoutesPasteOnlyToFocusedNormalControl(t *testing.T) {
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.screen = screenConnectionForm
	model.form = validConnectionForm()
	model.form.setFocus(fieldHost)
	before := make([]string, fieldIdentity+1)
	for field := fieldName; field <= fieldIdentity; field++ {
		before[field] = model.form.inputs[field].Value()
	}

	_, command := model.Update(tea.PasteMsg{Content: "-pasted-用户"})
	if command != nil {
		t.Fatal("normal-field paste returned an application command")
	}
	for field := fieldName; field <= fieldIdentity; field++ {
		want := before[field]
		if field == fieldHost {
			want += "-pasted-用户"
		}
		if got := model.form.inputs[field].Value(); got != want {
			t.Fatalf("field %v = %q, want %q", field, got, want)
		}
	}

	model.form.setFocus(fieldSave)
	_, command = model.Update(tea.PasteMsg{Content: "ignored"})
	if command != nil {
		t.Fatal("paste on a non-editable control returned a command")
	}
	for field := fieldName; field <= fieldIdentity; field++ {
		want := before[field]
		if field == fieldHost {
			want += "-pasted-用户"
		}
		if model.form.inputs[field].Value() != want {
			t.Fatalf("non-editable focus routed paste to field %v", field)
		}
	}

	model.form = nil
	folderForm := newFolderForm(nil, app.ItemSelector{Path: "/"})
	folderForm.setDestination(rootFolder())
	folderForm.input.SetValue("folder")
	model.openGenericModal(modalKindFolderCreate, nil, folderCreatePayload{form: folderForm})
	updateModel(model, tea.PasteMsg{Content: "-name"})
	if folderForm.input.Value() != "folder-name" {
		t.Fatalf("folder paste = %q", folderForm.input.Value())
	}
}

func TestModelPasteShortcutIsolationCorpusAndBoundaries(t *testing.T) {
	shortcuts := []struct {
		name    string
		payload string
	}{
		{name: "quit", payload: "q"},
		{name: "new-connection", payload: "n"},
		{name: "new-folder", payload: "f"},
		{name: "edit", payload: "e"},
		{name: "move", payload: "m"},
		{name: "delete", payload: "d"},
		{name: "connect", payload: "c"},
		{name: "reload", payload: "r"},
		{name: "help", payload: "?"},
		{name: "enter", payload: "\r"},
		{name: "escape", payload: "\x1b"},
		{name: "tab", payload: "\t"},
		{name: "shift-tab", payload: "\x1b[Z"},
		{name: "ctrl-c", payload: "\x03"},
		{name: "ctrl-s", payload: "\x13"},
		{name: "ctrl-r", payload: "\x12"},
		{name: "up", payload: "\x1b[A"},
		{name: "down", payload: "\x1b[B"},
		{name: "right", payload: "\x1b[C"},
		{name: "left", payload: "\x1b[D"},
		{name: "home", payload: "\x1b[H"},
		{name: "end", payload: "\x1b[F"},
	}
	for _, shortcut := range shortcuts {
		for _, surrounded := range []bool{false, true} {
			mode := "alone"
			payload := shortcut.payload
			if surrounded {
				mode = "surrounded"
				payload = "before" + payload + "after"
			}
			t.Run(shortcut.name+"/"+mode, func(t *testing.T) {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				model.screen = screenConnectionForm
				model.form = validConnectionForm()
				model.form.setFocus(fieldName)
				form := model.form
				beforeHost := form.inputs[fieldHost].Value()

				_, command := model.Update(tea.PasteMsg{Content: payload})
				if command != nil {
					t.Fatal("pasted shortcut returned a navigation, mutation, save, connect, or quit command")
				}
				if model.screen != screenConnectionForm || model.form != form || model.operation != nil || model.modal.isOpen() {
					t.Fatal("pasted shortcut changed root action state")
				}
				if form.focusedField() != fieldName || form.inputs[fieldHost].Value() != beforeHost {
					t.Fatal("pasted shortcut navigated or reached an unfocused field")
				}
				if want := isolatedPasteValue("node", payload); form.inputs[fieldName].Value() != want {
					t.Fatalf("pasted shortcut value = %q, want isolated text %q", form.inputs[fieldName].Value(), want)
				}
			})
		}
	}

	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.screen = screenConnectionForm
	model.form = validConnectionForm()
	before := model.form.inputs[fieldName].Value()
	for _, boundary := range []tea.Msg{tea.PasteStartMsg{}, tea.PasteEndMsg{}} {
		_, command := model.Update(boundary)
		if command != nil || model.form.inputs[fieldName].Value() != before || model.screen != screenConnectionForm {
			t.Fatal("paste boundary message caused an application action")
		}
	}

	browser := loadedModel(t, nil, true)
	_, command := browser.Update(tea.PasteMsg{Content: "qnfemdcr?"})
	if command != nil || browser.screen != screenBrowser || browser.modal.isOpen() {
		t.Fatal("paste without a focused field dispatched browser shortcuts")
	}
}

func isolatedPasteValue(before, payload string) string {
	var normalized strings.Builder
	for _, r := range payload {
		switch r {
		case '\r', '\n', '\u0085', '\u2028', '\u2029':
			continue
		}
		if unicode.IsControl(r) {
			return before
		}
		normalized.WriteRune(r)
	}
	return before + normalized.String()
}

func TestHelpDocumentsUnsupportedBracketedPasteLimitation(t *testing.T) {
	model := loadedModel(t, nil, true)
	updateModel(model, keyPress("n"))
	updateModel(model, tea.WindowSizeMsg{Width: 160, Height: 24})
	updateModel(model, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	view := model.View().Content
	payload, ok := model.modal.payload.(helpPayload)
	completeHelp := strings.Join(strings.Fields(strings.Join(payload.lines, " ")), "")
	if !ok || !strings.Contains(view, "Help") || !strings.Contains(completeHelp, "bracketed-pastesupport") || !strings.Contains(completeHelp, "cannotbedistinguishedfromtyping") || !strings.Contains(completeHelp, "neverreadstheoperatingsystemclipboard") {
		t.Fatalf("help omitted unsupported-terminal limitation: payload=%q view=%q", completeHelp, view)
	}
}

func TestSSHFailureRecoveryResolvesCapturedIDAndRequiresConfirmation(t *testing.T) {
	failed := app.SSHAttemptTarget{ID: "captured", Revision: 3, Path: "/old", Host: "2001:db8::1", Port: 22}
	current := testConnection("captured", syntheticRootID, "/new", 4)
	current.Host, current.Port = "2001:db8::2", 2202
	getCalls, connectCalls := 0, 0
	model := loadedModel(t, []app.Connection{testConnection("selected", syntheticRootID, "/selected", 1)}, true)
	model.connections = ConnectionFuncs{GetFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionResult, error) {
		getCalls++
		if selector.ID != failed.ID || selector.Path != "" {
			t.Fatalf("recovery selector = %#v", selector)
		}
		return app.ConnectionResult{Connection: current}, nil
	}}
	model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
		connectCalls++
		return app.ConnectResult{}, nil
	})
	model.installSSHFailure(newSSHFailureModal(failed, app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", nil).Presentation()))

	updateModel(model, keyPress("d"))
	if !model.activeSSHFailure().detailVisible {
		t.Fatal("dedicated detail key did not expand the failure")
	}
	_, command := model.Update(keyPress("r"))
	if command == nil || getCalls != 0 || connectCalls != 0 {
		t.Fatalf("retry did work eagerly: command=%v get=%d connect=%d", command != nil, getCalls, connectCalls)
	}
	updateModel(model, command())
	payload, ok := model.modal.payload.(sshFailurePayload)
	if getCalls != 1 || connectCalls != 0 || !ok || payload.confirmation == nil {
		t.Fatalf("resolution did not open confirmation: get=%d connect=%d modal=%#v", getCalls, connectCalls, model.modal)
	}
	view := model.View().Content
	for _, want := range []string{"Previous path: /old", "Previous endpoint: [2001:db8::1]:22", "Path: /new", "Endpoint: [2001:db8::2]:2202", "Revision: 4"} {
		if !strings.Contains(view, want) {
			t.Fatalf("changed-target confirmation omitted %q: %q", want, view)
		}
	}
	updateModel(model, keyPress("enter"))
	payload, _ = model.modal.payload.(sshFailurePayload)
	if payload.modal == nil || payload.confirmation != nil || connectCalls != 0 {
		t.Fatal("canceled retry did not return to the diagnostic without network")
	}

	_, command = model.Update(keyPress("r"))
	updateModel(model, command())
	_, command = model.Update(keyPress("y"))
	if command == nil || connectCalls != 0 {
		t.Fatal("explicit confirmation did not defer SSH execution")
	}
}

func TestSSHFailureEditBackMissingAndBrowserDeleteScopes(t *testing.T) {
	failed := app.SSHAttemptTarget{ID: "captured", Revision: 2, Path: "/failed", Host: "failed.test", Port: 22}
	current := testConnection("captured", syntheticRootID, "/current", 3)
	model := loadedModel(t, []app.Connection{current}, true)
	model.connections = ConnectionFuncs{GetFunc: func(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
		return app.ConnectionResult{Connection: current}, nil
	}}
	model.installSSHFailure(newSSHFailureModal(failed, app.NewSSHStartError(app.SSHFailureUnexpected, app.SSHFailureStageUnknown, "", nil).Presentation()))
	_, command := model.Update(keyPress("e"))
	updateModel(model, command())
	if model.screen != screenConnectionForm || model.form == nil || model.form.original.ID != failed.ID {
		t.Fatal("edit did not resolve the captured connection")
	}
	updateModel(model, keyPress("esc"))
	if model.activeSSHFailure() == nil || model.screen != screenBrowser {
		t.Fatal("canceled recovery edit did not return to the diagnostic")
	}
	updateModel(model, keyPress("esc"))
	if model.modal.isOpen() || model.browser.selectedID != failed.ID {
		t.Fatal("back did not select the captured node")
	}

	deleteCalls := 0
	model.connections = ConnectionFuncs{DeleteScopeFunc: func(context.Context, app.ItemSelector) (app.ConnectionDeleteScope, error) {
		deleteCalls++
		return app.ConnectionDeleteScope{}, app.ErrNotFound
	}}
	_, command = model.Update(keyPress("d"))
	if command == nil {
		t.Fatal("browser d no longer invokes delete")
	}
	_ = command()
	if deleteCalls != 1 {
		t.Fatalf("browser delete calls = %d", deleteCalls)
	}

	model.installSSHFailure(newSSHFailureModal(failed, app.NewSSHStartError(app.SSHFailureUnexpected, app.SSHFailureStageUnknown, "", nil).Presentation()))
	model.connections = ConnectionFuncs{GetFunc: func(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
		return app.ConnectionResult{}, app.ErrNotFound
	}}
	_, command = model.Update(keyPress("r"))
	updateModel(model, command())
	if modal := model.activeSSHFailure(); modal == nil || modal.recovery != recoveryMissing || !strings.Contains(model.View().Content, "/failed") || !strings.Contains(model.View().Content, "r Reload") {
		t.Fatalf("missing recovery lost context: %#v %q", modal, model.View().Content)
	}
	updateModel(model, keyPress("esc"))
	if model.operation != nil || model.status != "TARGET NO LONGER EXISTS" {
		t.Fatalf("missing-target back was not stable: operation=%#v status=%q", model.operation, model.status)
	}
}

func TestRetryCASConflictRestoresOriginalFailureContext(t *testing.T) {
	original := app.SSHAttemptTarget{ID: "captured", Revision: 2, Path: "/old", Host: "old.test", Port: 22}
	current := testConnection("captured", syntheticRootID, "/current", 3)
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", nil).Presentation()
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.operationID = 7
	_, _, _ = model.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerModal)
	expected := current.Revision
	updateModel(model, sessionFinishedMsg{
		id: 8, attempt: app.SSHAttemptTarget{ID: current.ID, Revision: current.Revision, Path: current.Path, Host: current.Host, Port: current.Port},
		recoveryAttempt: original, recoveryFailure: &failure,
		result: app.ConnectResult{Connection: current}, err: app.ErrConflict,
	})
	if modal := model.activeSSHFailure(); modal == nil || modal.attempt != original || modal.recovery != recoveryConflict {
		t.Fatalf("CAS conflict context = %#v (expected revision %d)", modal, expected)
	}
}

func loadedModel(t *testing.T, values []app.Connection, noColor bool) *Model {
	t.Helper()
	service := ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
		return app.ListConnectionsResult{Connections: values, CatalogRevision: 7}, nil
	}}
	model := New(Config{Connections: service, Width: 80, Height: 24, NoColor: noColor})
	updateModel(model, model.Init()())
	return model
}

func updateModel(model *Model, msg tea.Msg) {
	updated, _ := model.Update(msg)
	if updated != model {
		panic("model update replaced the root pointer")
	}
}

func keyPress(value string) tea.KeyPressMsg {
	switch value {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	case "shift+tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	case "ctrl+s":
		return tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl})
	default:
		runes := []rune(value)
		return tea.KeyPressMsg(tea.Key{Code: runes[0], Text: value})
	}
}
