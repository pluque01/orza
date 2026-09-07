package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/tui"
)

func TestTUIContextualActionsAndCapturedConnectTarget(t *testing.T) {
	connection := app.Connection{
		Node:       app.Node{ID: "11111111111111111111111111111111", Kind: app.NodeKindConnection, Name: "server", Path: "/server", Revision: 7},
		Host:       "server.test",
		Port:       2222,
		AuthMethod: app.AuthMethodAgent,
	}
	service := tui.ConnectionFuncs{
		ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 1}, nil
		},
		DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
			if selector.ID != connection.ID || selector.Path != "" {
				t.Fatalf("delete scope selector = %#v", selector)
			}
			return app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision}, nil
		},
	}
	model := tui.New(tui.Config{Connections: service, Width: 80, Height: 24, NoColor: true})
	updateTUI(t, model, model.Init()())

	root := model.View().Content
	for _, action := range []string{"n New connection", "f New folder", "r Reload", "? Help", "q Quit"} {
		if !strings.Contains(root, action) {
			t.Fatalf("root Actions omitted %q:\n%s", action, root)
		}
	}
	for _, unavailable := range []string{"c Connect", "e Edit", "m Move", "d Delete"} {
		if strings.Contains(root, unavailable) {
			t.Fatalf("root Actions exposed %q:\n%s", unavailable, root)
		}
	}

	updateTUI(t, model, tuiKey("l"))
	connectionView := model.View().Content
	for _, action := range []string{"c Connect", "n New connection", "f New folder", "e Edit", "m Move", "d Delete", "r Reload", "? Help", "q Quit"} {
		if !strings.Contains(connectionView, action) {
			t.Fatalf("connection Actions omitted %q:\n%s", action, connectionView)
		}
	}
	updateTUI(t, model, tuiKey("c"))
	confirmation := model.View().Content
	for _, captured := range []string{"Path: /server", "Endpoint: server.test:2222", "ID: 11111111111111111111111111111111", "Revision: 7"} {
		if !strings.Contains(confirmation, captured) {
			t.Fatalf("connect confirmation omitted captured %q:\n%s", captured, confirmation)
		}
	}
	updateTUI(t, model, tuiKey("esc"))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	deleteView := model.View().Content
	for _, captured := range []string{"Delete connection?", "Path: /server", "Target: (default)@server.test"} {
		if !strings.Contains(deleteView, captured) {
			t.Fatalf("delete confirmation omitted captured %q:\n%s", captured, deleteView)
		}
	}
}

func TestTUIFolderActionInventory(t *testing.T) {
	root := app.Folder{Node: app.Node{ID: "00000000000000000000000000000000", Kind: app.NodeKindFolder, Name: "/", Path: "/", Revision: 1}}
	folder := app.Folder{Node: app.Node{ID: "22222222222222222222222222222222", ParentID: root.ID, Kind: app.NodeKindFolder, Name: "prod", Path: "/prod", Revision: 2}}
	service := tui.FolderFuncs{
		GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			return app.FolderResult{Folder: root, CatalogRevision: 1}, nil
		},
		ListFunc: func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
			if request.Folder.ID == root.ID {
				return app.ListChildrenResult{Folders: []app.Folder{folder}, CatalogRevision: 1}, nil
			}
			return app.ListChildrenResult{CatalogRevision: 1}, nil
		},
	}
	model := tui.New(tui.Config{Folders: service, Width: 80, Height: 24, NoColor: true})
	updateTUI(t, model, model.Init()())
	updateTUI(t, model, tuiKey("l"))
	view := model.View().Content
	for _, action := range []string{"n New connection", "f New folder", "e Edit", "m Move", "d Delete", "r Reload", "? Help", "q Quit"} {
		if !strings.Contains(view, action) {
			t.Fatalf("folder Actions omitted %q:\n%s", action, view)
		}
	}
	if strings.Contains(view, "c Connect") {
		t.Fatalf("folder Actions exposed Connect:\n%s", view)
	}
}
