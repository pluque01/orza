package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestDescriptorDispatchRejectsUnlistedObjectKeys(t *testing.T) {
	tests := []struct {
		name      string
		selection app.NodeID
		keys      []string
	}{
		{name: "root", selection: syntheticRootID, keys: []string{"c", "e", "m", "d"}},
		{name: "folder", selection: "folder", keys: []string{"c"}},
	}
	folder := testFolder("folder", syntheticRootID, "/folder", 1)
	connection := testConnection("connection", folder.ID, "/folder/server", 1)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := New(Config{Width: 80, Height: 24, NoColor: true})
			snapshot := newCatalogSnapshot(rootFolder(), 1)
			if !snapshot.addChildren(syntheticRootID, app.ListChildrenResult{Folders: []app.Folder{folder}}) ||
				!snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
				t.Fatal("invalid test snapshot")
			}
			model.browser.setSnapshot(snapshot, test.selection)
			model.ownedSelectionID = model.browser.selectedID
			for _, value := range test.keys {
				before := model.browser.selectedID
				_, command := model.Update(keyPress(value))
				if command != nil || model.browser.selectedID != before || model.form != nil || model.modal.isOpen() || model.operation != nil {
					t.Fatalf("unlisted key %q changed %s context", value, test.name)
				}
			}
		})
	}
}

func TestDescriptorDispatchCtrlCAliasesQuit(t *testing.T) {
	model := loadedModel(t, nil, true)
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if command == nil {
		t.Fatal("Ctrl+C did not dispatch the descriptor-backed Quit action")
	}
	message := command()
	if _, ok := message.(tea.QuitMsg); !ok {
		t.Fatalf("Ctrl+C command = %T, want tea.QuitMsg", message)
	}
}

func TestConnectionCreationDispatchCapturesParent(t *testing.T) {
	root := rootFolder()
	folder := testFolder("folder", root.ID, "/folder", 3)
	connection := testConnection("connection", folder.ID, "/folder/server", 5)
	snapshot := newCatalogSnapshot(root, 1)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}}) ||
		!snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
		t.Fatal("invalid test snapshot")
	}

	for _, test := range []struct {
		key  string
		name string
	}{
		{key: "n", name: "connection"},
		{key: "f", name: "folder"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := New(Config{Width: 80, Height: 24, NoColor: true})
			model.browser.setSnapshot(snapshot, connection.ID)
			model.ownedSelectionID = connection.ID
			updateModel(model, keyPress(test.key))
			switch test.key {
			case "n":
				if model.form == nil || model.form.destination == nil || model.form.destination.ID != folder.ID || model.form.destination.Revision != folder.Revision {
					t.Fatalf("new connection destination = %#v, want captured parent", model.form)
				}
			case "f":
				payload, ok := model.modal.payload.(folderCreatePayload)
				if !ok || payload.form.destination == nil || payload.form.destination.ID != folder.ID || payload.form.destination.Revision != folder.Revision {
					t.Fatalf("new folder payload = %#v, want captured parent", model.modal.payload)
				}
			}
		})
	}
}
