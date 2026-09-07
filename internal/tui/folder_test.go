package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

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
