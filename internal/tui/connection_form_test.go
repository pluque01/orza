package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

func TestConnectionNormalControlsManualPasteEquivalenceCorpus(t *testing.T) {
	fields := []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldIdentity}
	positions := []string{"start", "middle", "selection"}
	for _, field := range fields {
		for payloadName, payload := range normalEquivalencePayloads() {
			for _, position := range positions {
				t.Run(formLabels[field]+"/"+payloadName+"/"+position, func(t *testing.T) {
					manual := validConnectionForm()
					pasted := validConnectionForm()
					manual.setFocus(field)
					pasted.setFocus(field)
					prepareTextField(&manual.inputs[field], position)
					prepareTextField(&pasted.inputs[field], position)
					if position == "selection" && (!manual.inputs[field].HasSelection() || !pasted.inputs[field].HasSelection()) {
						t.Fatal("selection corpus setup did not select text")
					}

					manual.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyExtended, Text: payload}))
					pasted.update(tea.PasteMsg{Content: payload})
					manualValid := manual.validate()
					pastedValid := pasted.validate()

					if manual.inputs[field].Value() != pasted.inputs[field].Value() || manual.inputs[field].Position() != pasted.inputs[field].Position() {
						t.Fatalf("manual state %q/%d differs from paste %q/%d", manual.inputs[field].Value(), manual.inputs[field].Position(), pasted.inputs[field].Value(), pasted.inputs[field].Position())
					}
					if manual.inputs[field].Error() != pasted.inputs[field].Error() || manualValid != pastedValid || !reflect.DeepEqual(manual.errors, pasted.errors) {
						t.Fatalf("manual validation (%v, %#v, %q) differs from paste (%v, %#v, %q)", manualValid, manual.errors, manual.inputs[field].Error(), pastedValid, pasted.errors, pasted.inputs[field].Error())
					}
				})
			}
		}
	}
}

func normalEquivalencePayloads() map[string]string {
	return map[string]string{
		"plain":      "plain",
		"address":    "user@example.com",
		"port":       "22",
		"path":       "/tmp/id_ed25519",
		"accent":     "café",
		"combining":  "e\u0301",
		"cjk":        "用户",
		"cyrillic":   "пароль",
		"wide-emoji": "界🙂",
		"zwj":        "👩‍💻",
		"1001-runes": strings.Repeat("a", 1001),
		"4096-runes": strings.Repeat("a", 4096),
	}
}

func validConnectionForm() *connectionForm {
	form := newConnectionForm(nil)
	form.inputs[fieldName].SetValue("node")
	form.inputs[fieldFolder].SetValue("/")
	form.inputs[fieldHost].SetValue("host.test")
	form.inputs[fieldPort].SetValue("22")
	form.inputs[fieldUsername].SetValue("deploy")
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodAgent))
	form.inputs[fieldIdentity].SetValue("/tmp/id_ed25519")
	return form
}

func prepareTextField(field *textField, position string) {
	switch position {
	case "start":
		field.SetValue("tail")
		field.SetCursor(0)
	case "middle":
		field.SetValue("book")
		field.SetCursor(2)
	case "selection":
		field.SetValue("replace")
		field.SetCursor(2)
		field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
		field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	}
}

func TestConnectionFormFocusValidationDependenciesAndDiscard(t *testing.T) {
	form := newConnectionForm(nil)
	if got := form.focusedField(); got != fieldName {
		t.Fatalf("initial focus = %v, want name", got)
	}
	form.update(keyPress("tab"))
	if got := form.focusedField(); got != fieldHost {
		t.Fatalf("focus after tab = %v, want host", got)
	}
	form.update(keyPress("shift+tab"))
	if got := form.focusedField(); got != fieldName {
		t.Fatalf("focus after shift-tab = %v, want name", got)
	}

	form.inputs[fieldName].SetValue("bad/name")
	form.inputs[fieldHost].SetValue("")
	form.inputs[fieldPort].SetValue("70000")
	if form.validate() {
		t.Fatal("invalid form passed validation")
	}
	for _, field := range []connectionField{fieldName, fieldHost, fieldPort} {
		if form.errors[field] == "" {
			t.Fatalf("field %v has no validation message", field)
		}
	}
	if !form.dirty() || !form.cancelNeedsConfirmation() {
		t.Fatal("changed form can be discarded without confirmation")
	}

	form.inputs[fieldAuth].SetValue(string(app.AuthMethodKey))
	form.syncDependencies()
	if !form.identityVisible() {
		t.Fatal("key authentication did not reveal identity field")
	}
	form.inputs[fieldIdentity].SetValue("~/.ssh/id_old")
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodAgent))
	form.syncDependencies()
	if form.identityVisible() || form.inputs[fieldIdentity].Value() != "~/.ssh/id_old" {
		t.Fatal("changing method erased or retained visibility of prior key configuration")
	}
}

func TestConnectionFormCanonicalConditionalFocusOrder(t *testing.T) {
	tests := []struct {
		name   string
		method app.AuthMethod
		want   []connectionField
	}{
		{name: "agent skips conditionals", method: app.AuthMethodAgent, want: []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldSave}},
		{name: "key includes identity", method: app.AuthMethodKey, want: []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldIdentity, fieldSave}},
		{name: "password includes remember", method: app.AuthMethodPassword, want: []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldRemember, fieldSave}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := newConnectionForm(nil)
			form.inputs[fieldAuth].SetValue(string(test.method))
			form.syncDependencies()
			if got := form.focusableFields(); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("focusable fields = %v, want %v", got, test.want)
			}
			if visible := form.visibleFields(); len(visible) < 2 || visible[1] != fieldFolder {
				t.Fatalf("read-only Folder missing from visible fields: %v", visible)
			}

			forward := []connectionField{form.focusedField()}
			for range len(test.want) - 1 {
				form.update(keyPress("tab"))
				forward = append(forward, form.focusedField())
			}
			if !reflect.DeepEqual(forward, test.want) {
				t.Fatalf("forward focus = %v, want %v", forward, test.want)
			}

			reverse := []connectionField{form.focusedField()}
			for range len(test.want) - 1 {
				form.update(keyPress("shift+tab"))
				reverse = append(reverse, form.focusedField())
			}
			wantReverse := append([]connectionField(nil), test.want...)
			for left, right := 0, len(wantReverse)-1; left < right; left, right = left+1, right-1 {
				wantReverse[left], wantReverse[right] = wantReverse[right], wantReverse[left]
			}
			if !reflect.DeepEqual(reverse, wantReverse) {
				t.Fatalf("reverse focus = %v, want %v", reverse, wantReverse)
			}
		})
	}
}

func TestConnectionFormConditionalFallbackAndF2Previous(t *testing.T) {
	form := newConnectionForm(nil)
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodKey))
	form.setFocus(fieldIdentity)
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodAgent))
	form.syncDependencies()
	if form.focusedField() != fieldAuth {
		t.Fatalf("hidden Identity focus = %v, want Method", form.focusedField())
	}

	form.inputs[fieldAuth].SetValue(string(app.AuthMethodPassword))
	form.setFocus(fieldRemember)
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodKey))
	form.syncDependencies()
	if form.focusedField() != fieldAuth {
		t.Fatalf("hidden Remember focus = %v, want Method", form.focusedField())
	}

	form.setFocus(fieldHost)
	form.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	if form.focusedField() != fieldName {
		t.Fatalf("F2 focus = %v, want Name", form.focusedField())
	}
	form.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2, Mod: tea.ModShift}))
	if form.focusedField() != fieldName {
		t.Fatal("modified F2 unexpectedly traversed the form")
	}
}

func TestConnectionFormFolderIsReadOnlyAndExcludedFromTraversal(t *testing.T) {
	tests := []struct {
		name string
		form *connectionForm
	}{
		{name: "create", form: newConnectionForm(nil)},
		{name: "edit", form: newConnectionForm(&app.Connection{Node: app.Node{ID: "11111111111111111111111111111111", ParentID: "22222222222222222222222222222222", Name: "node", Path: "/team/node", Revision: 3}, Host: "host.test", Port: 22, AuthMethod: app.AuthMethodAgent})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := test.form
			original := form.inputs[fieldFolder].Value()

			form.update(keyPress("tab"))
			if form.focusedField() != fieldHost {
				t.Fatalf("Tab focused %v, want Host", form.focusedField())
			}
			form.update(keyPress("shift+tab"))
			if form.focusedField() != fieldName {
				t.Fatalf("Shift+Tab focused %v, want Name", form.focusedField())
			}

			// Exercise the input boundary directly as a defense against stale focus.
			form.inputs[fieldName].Blur()
			form.focus = fieldFolder
			form.inputs[fieldFolder].Focus()
			form.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyExtended, Text: "/retargeted"}))
			form.update(tea.PasteMsg{Content: "/also-retargeted"})
			if got := form.inputs[fieldFolder].Value(); got != original {
				t.Fatalf("read-only Folder changed to %q, want %q", got, original)
			}
		})
	}
}

func TestConnectionFormDynamicViewportKeepsActiveErrorsAndSaveVisible(t *testing.T) {
	form := validConnectionForm()
	form.setFocus(fieldHost)
	form.errors[fieldHost] = "host validation failed"
	form.setDimensions(32, 4)
	projection := form.project(newStyles(true))
	joined := strings.Join(projection.lines, "\n")
	if !strings.Contains(joined, "Host:") || !strings.Contains(joined, "host validation failed") {
		t.Fatalf("focused field/error not visible in reduced viewport: %q", joined)
	}
	if !projection.hasPrevious || !projection.hasNext || !projection.scrollbar.visible {
		t.Fatalf("reduced viewport overflow = previous %v next %v scrollbar %#v, view %q", projection.hasPrevious, projection.hasNext, projection.scrollbar, joined)
	}
	if strings.Contains(joined, viewportPreviousLabel) || strings.Contains(joined, viewportNextLabel) {
		t.Fatalf("reduced viewport rendered legacy markers: %q", joined)
	}

	form.setFocus(fieldSave)
	form.setFormError("persistence failed safely")
	projection = form.project(newStyles(true))
	joined = strings.Join(projection.lines, "\n")
	if !strings.Contains(joined, "persistence failed safely") || !strings.Contains(joined, "Save connection") {
		t.Fatalf("form error/Save not visible in reduced viewport: %q", joined)
	}
	if !projection.hasPrevious || projection.hasNext || !projection.scrollbar.visible {
		t.Fatalf("Save projection overflow metadata = previous %v next %v scrollbar %#v", projection.hasPrevious, projection.hasNext, projection.scrollbar)
	}
	for _, input := range form.inputs {
		if input.Width() > 32 {
			t.Fatalf("input width %d exceeds form width", input.Width())
		}
	}
}

func TestContextualConnectionDestinationSuccessfulSaveTwentyRuns(t *testing.T) {
	for run := range 20 {
		var created []app.Connection
		var captured app.CreateConnectionRequest
		service := ConnectionFuncs{
			ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				return app.ListConnectionsResult{Connections: append([]app.Connection(nil), created...), CatalogRevision: app.CatalogRevision(run + 1)}, nil
			},
			CreateFunc: func(_ context.Context, request app.CreateConnectionRequest) (app.ConnectionResult, error) {
				captured = request
				connection := app.Connection{Node: app.Node{ID: app.NodeID("created-" + string(rune('a'+run))), ParentID: syntheticRootID, Kind: app.NodeKindConnection, Name: request.Name, Path: "/" + request.Name, Revision: 1}, Host: request.Host, Port: request.Port, AuthMethod: request.AuthMethod}
				created = []app.Connection{connection}
				return app.ConnectionResult{Connection: connection, CatalogRevision: app.CatalogRevision(run + 2)}, nil
			},
		}
		model := New(Config{Connections: service, Width: 80, Height: 24, NoColor: true})
		updateModel(model, model.Init()())
		updateModel(model, keyPress("n"))
		model.form.inputs[fieldName].SetValue("node")
		model.form.inputs[fieldHost].SetValue("node.test")
		_, createCommand := model.Update(keyPress("ctrl+s"))
		if createCommand == nil {
			t.Fatalf("run %d: save did not start", run)
		}
		_, reloadCommand := model.Update(createCommand())
		if reloadCommand == nil {
			t.Fatalf("run %d: save did not request reconciliation", run)
		}
		updateModel(model, reloadCommand())
		if captured.Parent.Path != "/" || model.browser.selectedID != created[0].ID {
			t.Fatalf("run %d: parent=%#v selection=%q", run, captured.Parent, model.browser.selectedID)
		}
	}
}

func TestConnectionDestinationFromSelectedConnectionAndCancel(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 1)
	connection := testConnection("connection", folder.ID, "/folder/current", 1)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}})
	_ = snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(snapshot, "")
	model.browser.selectedID = connection.ID
	model.browser.expandAncestors(connection.ID)
	model.browser.rebuildRows()
	updateModel(model, keyPress("n"))
	if model.form.destination == nil || model.form.destination.ID != folder.ID || model.form.inputs[fieldFolder].Value() != folder.Path {
		t.Fatalf("connection destination = %#v, %q", model.form.destination, model.form.inputs[fieldFolder].Value())
	}
	if model.form.dirty() {
		t.Fatal("contextual destination made a new form dirty")
	}
	updateModel(model, keyPress("esc"))
	if model.screen != screenBrowser || model.browser.selectedID != connection.ID {
		t.Fatal("cancel changed browser state")
	}
}

func TestEditFormBuildsPinnedRequestAndConflictRemainsRecoverable(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "11111111111111111111111111111111", Name: "old", Path: "/old", Revision: 4}, Host: "old.example", Port: 22, AuthMethod: app.AuthMethodAgent}
	form := newConnectionForm(&connection)
	form.inputs[fieldHost].SetValue("new.example")
	request, ok := form.updateRequest()
	if !ok || request.Expected == nil || *request.Expected != 4 || request.Host == nil || *request.Host != "new.example" {
		t.Fatalf("update request = %+v, ok %v", request, ok)
	}
	modal := newErrorModal("save connection", connection.Path, &app.UseCaseError{})
	modal.kind = app.ErrorKindConflict
	view := strings.Join(operationErrorLines(modal, 80), "\n")
	if !strings.Contains(view, "Reload") || !strings.Contains(view, "never overwritten") {
		t.Fatalf("conflict modal is not actionable: %q", view)
	}
}

func TestRememberedCreateRejectsDestinationMutationBetweenPreflightAndWrite(t *testing.T) {
	destination := testFolder("11111111111111111111111111111111", "00000000000000000000000000000001", "/destination", 4)
	form := validConnectionForm()
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodPassword))
	form.setDestination(destination)
	request, ok := form.createRequest()
	if !ok || request.ExpectedParent == nil || *request.ExpectedParent != destination.Revision || request.ExpectedParentPath != destination.Path {
		t.Fatalf("create request = %#v, valid = %v", request, ok)
	}

	persisted := false
	folders := FolderFuncs{GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
		observed := destination
		destination.Path = "/renamed/destination"
		return app.FolderResult{Folder: observed}, nil
	}}
	connections := ConnectionFuncs{CreateFunc: func(_ context.Context, request app.CreateConnectionRequest) (app.ConnectionResult, error) {
		if request.ExpectedParentPath != destination.Path {
			return app.ConnectionResult{}, app.ErrConflict
		}
		persisted = true
		return app.ConnectionResult{}, nil
	}}
	localTerminal := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	localTerminal.QueueSecret([]byte("password"), nil)
	command := &credentialMutationCommand{
		ctx: context.Background(), terminal: localTerminal, service: connections, folders: folders,
		destination: form.destination, create: &request,
	}
	if err := command.Run(); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("Run() error = %v, want conflict", err)
	}
	if persisted {
		t.Fatal("remembered-password connection persisted under a changed destination")
	}
}

func TestConnectionFormCreateKeepsCapturedDestinationAuthoritative(t *testing.T) {
	destination := testFolder("11111111111111111111111111111111", "00000000000000000000000000000001", "/destination", 4)
	form := validConnectionForm()
	form.setDestination(destination)
	form.inputs[fieldFolder].SetValue("/retargeted")

	request, ok := form.createRequest()
	if !ok {
		t.Fatal("create request was invalid")
	}
	if request.Parent.ID != destination.ID || request.Parent.Path != "" || request.ExpectedParent == nil || *request.ExpectedParent != destination.Revision || request.ExpectedParentPath != destination.Path {
		t.Fatalf("create request lost captured destination: %#v", request)
	}
}

func TestConnectionFormCreateDestinationRacesNeverRetarget(t *testing.T) {
	destination := testFolder("11111111111111111111111111111111", "00000000000000000000000000000001", "/destination", 4)
	tests := []struct {
		name       string
		get        func(context.Context, app.ItemSelector) (app.FolderResult, error)
		createErr  error
		wantErr    error
		wantWrites int
	}{
		{
			name: "renamed",
			get: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
				renamed := destination
				renamed.Path = "/renamed"
				renamed.Revision++
				return app.FolderResult{Folder: renamed}, nil
			},
			wantErr: app.ErrConflict,
		},
		{
			name: "deleted",
			get: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
				return app.FolderResult{}, app.ErrNotFound
			},
			wantErr: app.ErrNotFound,
		},
		{
			name: "path recreated under another ID",
			get: func(_ context.Context, selector app.ItemSelector) (app.FolderResult, error) {
				if selector.Path == destination.Path {
					recreated := destination
					recreated.ID = "22222222222222222222222222222222"
					return app.FolderResult{Folder: recreated}, nil
				}
				return app.FolderResult{}, app.ErrNotFound
			},
			wantErr: app.ErrNotFound,
		},
		{
			name: "changed after preflight",
			get: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
				return app.FolderResult{Folder: destination}, nil
			},
			createErr:  app.ErrConflict,
			wantErr:    app.ErrConflict,
			wantWrites: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := validConnectionForm()
			form.setDestination(destination)
			request, ok := form.createRequest()
			if !ok {
				t.Fatal("create request was invalid")
			}

			writes := 0
			model := New(Config{
				Width: 80, Height: 24, NoColor: true,
				Folders: FolderFuncs{GetFunc: func(ctx context.Context, selector app.ItemSelector) (app.FolderResult, error) {
					if selector.ID != destination.ID || selector.Path != "" {
						t.Fatalf("destination lookup was not pinned to ID: %#v", selector)
					}
					return test.get(ctx, selector)
				}},
				Connections: ConnectionFuncs{CreateFunc: func(_ context.Context, got app.CreateConnectionRequest) (app.ConnectionResult, error) {
					writes++
					if got.Parent.ID != destination.ID || got.ExpectedParent == nil || *got.ExpectedParent != destination.Revision || got.ExpectedParentPath != destination.Path {
						t.Fatalf("write was not pinned to captured destination: %#v", got)
					}
					return app.ConnectionResult{}, test.createErr
				}},
			})
			command := model.createCommand(request, false, &destination)
			if command == nil {
				t.Fatal("create command did not start")
			}
			message, ok := command().(operationResultMsg)
			if !ok {
				t.Fatalf("create command returned %T", message)
			}
			if !errors.Is(message.err, test.wantErr) {
				t.Fatalf("create error = %v, want %v", message.err, test.wantErr)
			}
			if writes != test.wantWrites {
				t.Fatalf("create writes = %d, want %d", writes, test.wantWrites)
			}
		})
	}
}
