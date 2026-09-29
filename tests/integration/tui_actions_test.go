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

func TestTUIBrowserLegendAndHelpUseSingularTitles(t *testing.T) {
	frames := make(map[bool][2]string, 2)
	for _, noColor := range []bool{true, false} {
		service := tui.ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{CatalogRevision: 1}, nil
		}}
		model := tui.New(tui.Config{Connections: service, Width: 80, Height: 24, NoColor: noColor})
		updateTUI(t, model, model.Init()())
		actions := ansi.Strip(model.View().Content)
		assertCompactBrowserLegend(t, actions, false)
		if strings.Contains(actions, "Actions") {
			t.Fatalf("no-color=%t browser retained an Actions panel:\n%s", noColor, actions)
		}

		updateTUI(t, model, tuiKey("?"))
		help := ansi.Strip(model.View().Content)
		if strings.Count(help, "[*] Help") != 1 {
			t.Fatalf("no-color=%t Help title is missing or duplicated:\n%s", noColor, help)
		}
		updateTUI(t, model, tuiKey("G"))
		help = ansi.Strip(model.View().Content)
		assertHelpActions(t, help, "n New connection", "f New folder", "r Reload", "? Help", "q Quit")
		updateTUI(t, model, tuiKey("esc"))
		assertCompactBrowserLegend(t, ansi.Strip(model.View().Content), false)
		frames[noColor] = [2]string{actions, help}
	}
	if frames[false] != frames[true] {
		t.Fatal("ANSI-stripped browser legend/Help frames differ from no-color frames")
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
	assertCompactBrowserLegend(t, root, false)
	assertBrowserHelpActions(t, model, "r Reload", "? Help")

	updateTUI(t, model, tuiKey("l"))
	connectionView := model.View().Content
	assertCompactBrowserLegend(t, connectionView, true)
	assertBrowserHelpActions(t, model, "e Edit", "m Move", "d Delete", "r Reload", "? Help")
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
	assertCompactBrowserLegend(t, view, false)
	assertBrowserHelpActions(t, model, "e Edit", "m Move", "d Delete", "r Reload", "? Help")
}

func assertCompactBrowserLegend(t *testing.T, view string, connect bool) {
	t.Helper()
	if strings.Contains(view, "Actions") {
		t.Fatalf("browser retained Actions panel:\n%s", view)
	}
	want := []string{"Up/k Move up", "Down/j Move down", "n New connection", "f New folder", "q Quit"}
	if connect {
		want = append(want, "c Connect")
	}
	for _, action := range want {
		if !strings.Contains(view, action) {
			t.Fatalf("browser legend omitted %q:\n%s", action, view)
		}
	}
	for _, hidden := range []string{"e Edit", "m Move", "d Delete", "r Reload", "? Help"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("browser legend exposed secondary action %q:\n%s", hidden, view)
		}
	}
}

func assertBrowserHelpActions(t *testing.T, model *tui.Model, actions ...string) {
	t.Helper()
	updateTUI(t, model, tuiKey("?"))
	updateTUI(t, model, tuiKey("G"))
	assertHelpActions(t, ansi.Strip(model.View().Content), actions...)
	updateTUI(t, model, tuiKey("esc"))
}

func assertHelpActions(t *testing.T, view string, actions ...string) {
	t.Helper()
	for _, action := range actions {
		found := false
		for _, line := range strings.Split(view, "\n") {
			for _, cell := range strings.Split(line, "│") {
				if strings.HasPrefix(strings.Join(strings.Fields(cell), " "), action) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatalf("Help omitted action %q:\n%s", action, view)
		}
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
