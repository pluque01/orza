package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/tui"
)

type tuiCapturedField struct {
	label string
	value string
}

func TestTUIActionsAndHelpUseSingularTitles(t *testing.T) {
	frames := make(map[bool][2]string, 2)
	for _, noColor := range []bool{true, false} {
		service := tui.ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{CatalogRevision: 1}, nil
		}}
		model := tui.New(tui.Config{Connections: service, Width: 80, Height: 24, NoColor: noColor})
		updateTUI(t, model, model.Init()())
		actions := ansi.Strip(model.View().Content)
		if strings.Count(actions, "[ ] Actions") != 1 {
			t.Fatalf("no-color=%t Actions title count != 1:\n%s", noColor, actions)
		}

		updateTUI(t, model, tuiKey("?"))
		help := ansi.Strip(model.View().Content)
		if strings.Count(help, "[*] Help") != 1 || strings.Count(help, "Help") != 2 {
			t.Fatalf("no-color=%t Help title is missing or duplicated in its body:\n%s", noColor, help)
		}
		for _, action := range []string{"n New connection", "f New folder", "r Reload", "? Help", "q Quit"} {
			if !strings.Contains(help, action) {
				t.Fatalf("no-color=%t Help omitted root action %q:\n%s", noColor, action, help)
			}
		}
		updateTUI(t, model, tuiKey("esc"))
		if strings.Count(ansi.Strip(model.View().Content), "[ ] Actions") != 1 {
			t.Fatalf("no-color=%t Esc did not restore Actions", noColor)
		}
		frames[noColor] = [2]string{actions, help}
	}
	if frames[false] != frames[true] {
		t.Fatal("ANSI-stripped Actions/Help frames differ from no-color frames")
	}
}

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
	if !strings.Contains(root, "[ ] Actions") {
		t.Fatalf("root omitted Actions title:\n%s", root)
	}
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
	assertColonlessCapturedFields(t, confirmation, []tuiCapturedField{
		{"Path", "/server"},
		{"Endpoint", "server.test:2222"},
		{"ID", "11111111111111111111111111111111"},
		{"Revision", "7"},
	})
	updateTUI(t, model, tuiKey("esc"))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	deleteView := model.View().Content
	if !strings.Contains(deleteView, "Delete") || !strings.Contains(deleteView, "permanently delete connection") {
		t.Fatalf("delete confirmation omitted its type or action:\n%s", deleteView)
	}
	assertColonlessCapturedFields(t, deleteView, []tuiCapturedField{
		{"Path", "/server"},
		{"Target", "(default)@server.test:2222"},
		{"ID/revision", "11111111111111111111111111111111/7"},
	})
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

func assertColonlessCapturedFields(t *testing.T, view string, fields []tuiCapturedField) {
	t.Helper()
	plain := ansi.Strip(view)
	for _, field := range fields {
		if strings.Contains(plain, field.label+":") {
			t.Fatalf("captured field label %q retained a colon:\n%s", field.label, plain)
		}
		want := field.label + " " + field.value
		found := false
		for _, line := range strings.Split(plain, "\n") {
			for _, panelCell := range strings.Split(line, "│") {
				if strings.Join(strings.Fields(panelCell), " ") == want {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("confirmation omitted colonless captured field %q:\n%s", want, plain)
		}
	}
}
