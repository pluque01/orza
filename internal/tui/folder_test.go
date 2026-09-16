package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestFolderFormsReuseModalBadgeAndSharedFieldHierarchy(t *testing.T) {
	destination := app.Folder{Node: app.Node{ID: "destination", Kind: app.NodeKindFolder, Name: "team", Path: "/team", Revision: 5}}
	original := app.Folder{Node: app.Node{ID: "original", Kind: app.NodeKindFolder, Name: "source", Path: "/team/source", Revision: 7}}

	tests := []struct {
		name       string
		kind       modalKind
		form       *folderForm
		payload    func(*folderForm) any
		badge      string
		values     []string
		forbidden  []string
		lineCount  int
		activeLine int
	}{
		{
			name: "create", kind: modalKindFolderCreate, form: newFolderForm(nil, app.ItemSelector{ID: destination.ID}),
			payload: func(form *folderForm) any { return folderCreatePayload{form: form} }, badge: "[Create Folder]",
			values: []string{"/team", "destination", "draft"}, forbidden: []string{"Create folder", "Target:", "Destination ID:", "Name:"}, lineCount: 4, activeLine: 2,
		},
		{
			name: "edit", kind: modalKindFolderEdit, form: newFolderForm(&original, app.ItemSelector{}),
			payload: func(form *folderForm) any { return folderEditPayload{form: form} }, badge: "[Edit Folder]",
			values: []string{"/team/source", "original/7", "source"}, forbidden: []string{"Edit folder", "Target:", "ID/revision:", "Name:"}, lineCount: 4, activeLine: 2,
		},
	}
	tests[0].form.setDestination(destination)
	tests[0].form.input.SetValue("draft")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plainLines, active := test.form.modalLines(80, newStyles(true))
			coloredLines, coloredActive := test.form.modalLines(80, newStyles(false))
			plain := strings.Join(plainLines, "\n")
			colored := strings.Join(coloredLines, "\n")
			if got := ansi.Strip(colored); got != plain {
				t.Fatalf("color semantics = %q, want %q", got, plain)
			}
			if !strings.Contains(colored, "\x1b[90m") {
				t.Fatalf("descriptive labels were not muted: %q", colored)
			}
			if len(plainLines) != test.lineCount || active != test.activeLine || coloredActive != active {
				t.Fatalf("lines/active = %d/%d/%d, want %d/%d/%d: %#v", len(plainLines), active, coloredActive, test.lineCount, test.activeLine, test.activeLine, plainLines)
			}
			for _, value := range test.values {
				if !strings.Contains(plain, value) {
					t.Fatalf("folder form omitted captured value %q: %q", value, plain)
				}
			}
			valueColumn := -1
			for index, value := range test.values {
				column := strings.Index(plainLines[index], value)
				if column < 0 || valueColumn >= 0 && column != valueColumn {
					t.Fatalf("folder values are not aligned: column %d, want %d in %#v", column, valueColumn, plainLines)
				}
				valueColumn = column
			}
			for _, forbidden := range test.forbidden {
				if strings.Contains(plain, forbidden) {
					t.Fatalf("folder form retained duplicate heading or colon label %q: %q", forbidden, plain)
				}
			}
			if !strings.HasPrefix(plainLines[active], "> ") || plainLines[len(plainLines)-1] != "Ctrl+S/Enter Save  Esc Cancel  F1 Help" {
				t.Fatalf("folder focus/control contract changed: %#v", plainLines)
			}

			model := New(Config{Width: 80, Height: 24, NoColor: true})
			if !model.openGenericModal(test.kind, nil, test.payload(test.form)) {
				t.Fatal("folder modal did not open")
			}
			view := model.View().Content
			if strings.Count(view, test.badge) != 1 || strings.Count(strings.ToLower(view), strings.ToLower(strings.Trim(test.badge, "[]"))) != 1 {
				t.Fatalf("folder modal did not reuse exactly one title badge %q:\n%s", test.badge, view)
			}
		})
	}
}

func TestFolderFormSanitizesDynamicValuesBeforeLabelStyling(t *testing.T) {
	unsafe := "team\x1b[31m\n\u202e"
	destination := app.Folder{Node: app.Node{ID: app.NodeID(unsafe), Kind: app.NodeKindFolder, Path: "/" + unsafe, Revision: 9}}
	form := newFolderForm(nil, app.ItemSelector{ID: destination.ID})
	form.setDestination(destination)
	form.input.SetValue(unsafe)
	capturedInput := form.input.Value()

	lines, _ := form.modalLines(200, newStyles(false))
	view := strings.Join(lines, "\n")
	if strings.Contains(view, "\x1b[31m") || strings.Contains(view, "\u202e") {
		t.Fatalf("folder form styled unsanitized dynamic content: %q", view)
	}
	for _, want := range []string{safeText(destination.Path, 200), safeText(string(destination.ID), 200), safeText(form.input.Value(), 200)} {
		if !strings.Contains(ansi.Strip(view), want) {
			t.Fatalf("folder form omitted safe projection %q: %q", want, ansi.Strip(view))
		}
	}
	if form.destination.Path != destination.Path || form.destination.ID != destination.ID || form.destination.Revision != destination.Revision || form.input.Value() != capturedInput {
		t.Fatal("folder rendering changed captured dynamic values")
	}
}

func TestFolderNameManualPasteEquivalenceCorpus(t *testing.T) {
	for payloadName, payload := range normalEquivalencePayloads() {
		for _, position := range []string{"start", "middle", "selection"} {
			t.Run(payloadName+"/"+position, func(t *testing.T) {
				manual := newFolderForm(nil, app.ItemSelector{Path: "/"})
				pasted := newFolderForm(nil, app.ItemSelector{Path: "/"})
				prepareTextField(&manual.input, position)
				prepareTextField(&pasted.input, position)
				if position == "selection" && (!manual.input.HasSelection() || !pasted.input.HasSelection()) {
					t.Fatal("selection corpus setup did not select text")
				}

				manual.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyExtended, Text: payload}))
				pasted.update(tea.PasteMsg{Content: payload})
				manualValid := manual.validate()
				pastedValid := pasted.validate()

				if manual.input.Value() != pasted.input.Value() || manual.input.Position() != pasted.input.Position() || manual.input.Error() != pasted.input.Error() {
					t.Fatalf("manual state %q/%d/%q differs from paste %q/%d/%q", manual.input.Value(), manual.input.Position(), manual.input.Error(), pasted.input.Value(), pasted.input.Position(), pasted.input.Error())
				}
				if manualValid != pastedValid || manual.err != pasted.err {
					t.Fatalf("manual validation %v/%q differs from paste %v/%q", manualValid, manual.err, pastedValid, pasted.err)
				}
			})
		}
	}
}

func TestContextualFolderDestinationAndCancelState(t *testing.T) {
	root := app.Folder{Node: app.Node{ID: "00000000000000000000000000000001", Kind: app.NodeKindFolder, Name: ".", Path: "/", Revision: 1}}
	child := app.Folder{Node: app.Node{ID: "11111111111111111111111111111111", ParentID: root.ID, Kind: app.NodeKindFolder, Name: "clients", Path: "/clients", Revision: 1}}
	connection := app.Connection{Node: app.Node{ID: "22222222222222222222222222222222", ParentID: child.ID, Kind: app.NodeKindConnection, Name: "alpha", Path: "/clients/alpha", Revision: 1}, Host: "alpha.test", Port: 22, AuthMethod: app.AuthMethodAgent}
	snapshot := newCatalogSnapshot(root, 2)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{child}})
	_ = snapshot.addChildren(child.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(snapshot, "")

	updateModel(model, keyPress("f"))
	payload, ok := model.modal.payload.(folderCreatePayload)
	if !ok || payload.form.destination == nil || payload.form.destination.ID != root.ID {
		t.Fatalf("root folder destination = %#v", model.modal.payload)
	}
	updateModel(model, keyPress("esc"))
	if model.screen != screenBrowser || model.browser.selectedID != root.ID {
		t.Fatal("cancel changed root selection")
	}

	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("f"))
	payload, ok = model.modal.payload.(folderCreatePayload)
	if !ok || payload.form.destination == nil || payload.form.destination.ID != child.ID {
		t.Fatalf("folder destination = %#v", model.modal.payload)
	}
	updateModel(model, keyPress("esc"))
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("f"))
	payload, ok = model.modal.payload.(folderCreatePayload)
	if !ok || payload.form.destination == nil || payload.form.destination.ID != child.ID {
		t.Fatalf("connection sibling destination = %#v", model.modal.payload)
	}
	updateModel(model, keyPress("esc"))
	if model.browser.selectedID != connection.ID {
		t.Fatal("connection selection was not preserved across cancel")
	}

	model.browser.home()
	for _, action := range []string{"e", "m", "d"} {
		updateModel(model, keyPress(action))
		if model.modal.kind != modalKindOperationError || model.browser.selectedID != root.ID {
			t.Fatalf("root action %q was not rejected safely", action)
		}
		updateModel(model, keyPress("esc"))
	}
}

func TestFolderFormMovePickerAndRecursiveConfirmation(t *testing.T) {
	source := app.Folder{Node: app.Node{ID: "11111111111111111111111111111111", Kind: app.NodeKindFolder, Name: "source", Path: "/source", Revision: 4}}
	descendant := app.Folder{Node: app.Node{ID: "22222222222222222222222222222222", ParentID: source.ID, Kind: app.NodeKindFolder, Name: "child", Path: "/source/child", Revision: 1}}
	destination := app.Folder{Node: app.Node{ID: "33333333333333333333333333333333", Kind: app.NodeKindFolder, Name: "archive", Path: "/archive", Revision: 2}}

	create := newFolderForm(nil, app.ItemSelector{Path: "/source"})
	create.setDestination(source)
	create.input.SetValue("new")
	request, ok := create.createRequest()
	if !ok || request.Parent.ID != source.ID || request.ExpectedParent == nil || *request.ExpectedParent != source.Revision || request.ExpectedParentPath != source.Path || request.Name != "new" {
		t.Fatalf("create request = %#v, %v", request, ok)
	}
	rename := newFolderForm(&source, app.ItemSelector{})
	rename.input.SetValue("renamed")
	renameRequest, ok := rename.renameRequest()
	if !ok || renameRequest.Expected == nil || *renameRequest.Expected != 4 || renameRequest.Name != "renamed" {
		t.Fatalf("rename request = %#v, %v", renameRequest, ok)
	}

	picker := newMovePicker(source.Node, []app.Folder{source, descendant, destination})
	if picker.enabled(source.ID) || picker.enabled(descendant.ID) || !picker.enabled(destination.ID) {
		t.Fatalf("folder move exclusions are incorrect: %#v", picker.targets)
	}
	connection := app.Connection{Node: app.Node{ID: "44444444444444444444444444444444", Kind: app.NodeKindConnection, Name: "web", Path: "/source/web", Revision: 3}}
	connectionPicker := newMovePicker(connection.Node, []app.Folder{source, descendant, destination})
	if !connectionPicker.enabled(descendant.ID) {
		t.Fatal("connection picker incorrectly excluded a descendant folder")
	}

	scope := app.FolderDeleteScope{ID: source.ID, Path: source.Path, Revision: source.Revision, Folders: 2, Connections: 3, RememberedCredentials: 1, Snapshot: []app.NodeRevision{{ID: source.ID, Revision: 4}}}
	modal := newFolderDeleteConfirmation(scope)
	view := strings.Join(deleteFolderLines(scope, 80), "\n")
	if modal.confirmed(keyPress("enter")) || !strings.Contains(view, "2 folders, 3 connections, 1 remembered credential") {
		t.Fatalf("recursive confirmation = %q", view)
	}
}

func TestFolderDeleteChangedScopeShowsReloadConflict(t *testing.T) {
	scope := app.FolderDeleteScope{ID: "11111111111111111111111111111111", Path: "/tree", Revision: 1, Folders: 1, Snapshot: []app.NodeRevision{{ID: "11111111111111111111111111111111", Revision: 1}}}
	reloads := 0
	model := New(Config{Width: 80, Height: 24, NoColor: true, Folders: FolderFuncs{
		GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			return app.FolderResult{Folder: app.Folder{Node: app.Node{ID: "00000000000000000000000000000001", Kind: app.NodeKindFolder, Path: "/", Revision: 1}}, CatalogRevision: 2}, nil
		},
		ListFunc: func(context.Context, app.ListChildrenRequest) (app.ListChildrenResult, error) {
			reloads++
			return app.ListChildrenResult{CatalogRevision: 2}, nil
		},
		DeleteFunc: func(context.Context, app.DeleteFolderRequest) (app.DeleteFolderResult, error) {
			return app.DeleteFolderResult{}, app.ErrConflict
		},
	}})
	target := capturedTarget{id: scope.ID, revision: scope.Revision, kind: app.NodeKindFolder, path: scope.Path}
	model.openGenericModal(modalKindDeleteFolder, &target, deleteFolderPayload{confirmation: newFolderDeleteConfirmation(scope)})
	_, command := model.Update(keyPress("y"))
	if command == nil {
		t.Fatal("explicit delete confirmation did not start")
	}
	updateModel(model, command())
	if model.modal.conflict == nil || !strings.Contains(model.View().Content, "Reload") {
		t.Fatalf("stale scope did not request reload: %q", model.View().Content)
	}
	_, command = model.Update(keyPress("r"))
	if command == nil {
		t.Fatal("stale scope reload did not start")
	}
	updateModel(model, command())
	if reloads != 1 || model.modal.recoverableError != "" {
		t.Fatalf("stale scope reload = %d, modal %#v", reloads, model.modal)
	}
}
