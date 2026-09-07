package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/tui"
)

func TestTUITreeContextualActionMatrixAndReconciliation(t *testing.T) {
	folders, connections := integrationTreeServices(t)
	for _, path := range []string{"/archive", "/work", "/work/prod"} {
		parent, name := splitTreePath(path)
		if _, err := folders.Create(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: parent}, Name: name}); err != nil {
			t.Fatalf("create fixture folder %s: %v", path, err)
		}
	}
	database, err := connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/work/prod"}, Name: "database", Host: "db.test", Port: 22, AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}

	model := tui.New(tui.Config{Folders: folders, Connections: connections, Width: 80, Height: 24, NoColor: true})
	executeTUICommand(t, model, model.Init())
	if view := model.View().Content; !strings.Contains(view, "> [/] /") || strings.Index(view, "[+] archive") > strings.Index(view, "[+] work") {
		t.Fatalf("initial tree does not select ordered root: %q", view)
	}

	// root -> archive -> work -> expand work -> prod
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("j"))
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("l"))
	assertTreeSelectionPath(t, model, "/work/prod")

	updateTUI(t, model, tuiKey("n"))
	typeText(t, model, "api")
	updateTUI(t, model, tuiKey("tab"))
	typeText(t, model, "api.test")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	api, err := connections.Get(context.Background(), app.ItemSelector{Path: "/work/prod/api"})
	if err != nil {
		t.Fatalf("contextual connection create: %v", err)
	}
	assertTreeSelectionPath(t, model, api.Connection.Path)

	updateTUI(t, model, tuiKey("e"))
	updateTUI(t, model, tuiKey("ctrl+a"))
	updateTUI(t, model, tuiKey("ctrl+k"))
	typeText(t, model, "api-renamed")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	api, err = connections.Get(context.Background(), app.ItemSelector{ID: api.Connection.ID})
	if err != nil || api.Connection.Path != "/work/prod/api-renamed" {
		t.Fatalf("identity-preserving rename = %#v, %v", api.Connection, err)
	}
	assertTreeSelectionPath(t, model, api.Connection.Path)

	updateTUI(t, model, tuiKey("m"))
	updateTUI(t, model, tuiKey("j")) // root -> archive
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("enter")))
	api, err = connections.Get(context.Background(), app.ItemSelector{ID: api.Connection.ID})
	if err != nil || api.Connection.Path != "/archive/api-renamed" {
		t.Fatalf("identity-preserving move = %#v, %v", api.Connection, err)
	}
	assertTreeSelectionPath(t, model, api.Connection.Path)

	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("y")))
	if _, err := connections.Get(context.Background(), app.ItemSelector{ID: api.Connection.ID}); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("connection delete = %v", err)
	}
	assertTreeSelectionPath(t, model, "/archive")

	updateTUI(t, model, tuiKey("f"))
	typeText(t, model, "child")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	child, err := folders.Get(context.Background(), app.ItemSelector{Path: "/archive/child"})
	if err != nil {
		t.Fatalf("contextual folder create: %v", err)
	}
	assertTreeSelectionPath(t, model, child.Folder.Path)

	updateTUI(t, model, tuiKey("e"))
	updateTUI(t, model, tuiKey("ctrl+a"))
	updateTUI(t, model, tuiKey("ctrl+k"))
	typeText(t, model, "child-renamed")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	child, err = folders.Get(context.Background(), app.ItemSelector{ID: child.Folder.ID})
	if err != nil || child.Folder.Path != "/archive/child-renamed" {
		t.Fatalf("folder rename = %#v, %v", child.Folder, err)
	}

	updateTUI(t, model, tuiKey("m"))
	updateTUI(t, model, tuiKey("j"))
	updateTUI(t, model, tuiKey("j")) // skip source subtree and select /work
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("enter")))
	child, err = folders.Get(context.Background(), app.ItemSelector{ID: child.Folder.ID})
	if err != nil || child.Folder.Path != "/work/child-renamed" {
		t.Fatalf("folder move = %#v, %v", child.Folder, err)
	}
	assertTreeSelectionPath(t, model, child.Folder.Path)

	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("y")))
	if _, err := folders.Get(context.Background(), app.ItemSelector{ID: child.Folder.ID}); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("folder delete = %v", err)
	}
	assertTreeSelectionPath(t, model, "/work")

	// Select database and prove confirmation/cancel retains the captured target.
	updateTUI(t, model, tuiKey("g"))
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("j"))
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("l"))
	assertTreeSelectionPath(t, model, database.Connection.Path)
	updateTUI(t, model, tuiKey("c"))
	for _, value := range []string{database.Connection.Path, "db.test:22", string(database.Connection.ID)} {
		if !strings.Contains(model.View().Content, value) {
			t.Fatalf("connect confirmation omitted %q: %q", value, model.View().Content)
		}
	}
	updateTUI(t, model, tuiKey("esc"))
	assertTreeSelectionPath(t, model, database.Connection.Path)

	// A stale selected revision must not open a confirmation for another row.
	changedHost := "changed.test"
	expected := database.Connection.Revision
	if _, err := connections.Update(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: database.Connection.ID}, Expected: &expected, Host: &changedHost}); err != nil {
		t.Fatal(err)
	}
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	if view := model.View().Content; !strings.Contains(view, "Reload") || !strings.Contains(view, database.Connection.Path) {
		t.Fatalf("stale target did not fail against captured path: %q", view)
	}
}

func TestTUITreeDetailsDirectChildrenEmptyAndFallback(t *testing.T) {
	folders, connections := integrationTreeServices(t)
	for _, path := range []string{"/empty", "/team", "/team/nested"} {
		parent, name := splitTreePath(path)
		if _, err := folders.Create(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: parent}, Name: name}); err != nil {
			t.Fatalf("create fixture folder %s: %v", path, err)
		}
	}
	direct, err := connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/team"}, Name: "direct", Host: "direct.test", Port: 2201, Username: "deploy", AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/team/nested"}, Name: "deep", Host: "deep.test", Port: 2202, AuthMethod: app.AuthMethodAgent}); err != nil {
		t.Fatal(err)
	}

	model := tui.New(tui.Config{Folders: folders, Connections: connections, Width: 80, Height: 24, NoColor: true})
	executeTUICommand(t, model, model.Init())
	rootView := model.View().Content
	for _, want := range []string{"n New connection", "f New folder", "r Reload", "? Help", "q Quit"} {
		if !strings.Contains(rootView, want) {
			t.Fatalf("root Actions omitted %q:\n%s", want, rootView)
		}
	}
	for _, unavailable := range []string{"c Connect", "e Edit", "m Move", "d Delete"} {
		if strings.Contains(rootView, unavailable) {
			t.Fatalf("root Actions exposed %q:\n%s", unavailable, rootView)
		}
	}

	// root -> empty
	updateTUI(t, model, tuiKey("l"))
	view := model.View().Content
	for _, want := range []string{"[*] Tree", "[ ] Details", "Kind: Folder", "Path: /empty", "Direct connections: 0", "No direct connections."} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty-folder Details omitted %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "e Edit") || !strings.Contains(view, "d Delete") || strings.Contains(view, "c Connect") {
		t.Fatalf("folder Actions inventory is incorrect:\n%s", view)
	}

	// empty -> team; folder detail must exclude nested descendants.
	updateTUI(t, model, tuiKey("j"))
	view = model.View().Content
	for _, want := range []string{"Path: /team", "Direct connections: 1", "direct: direct.test:2201"} {
		if !strings.Contains(view, want) {
			t.Fatalf("folder Details omitted %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "deep.test") {
		t.Fatalf("folder Details included descendant connection:\n%s", view)
	}

	// Expand team and select its direct connection after the nested folder.
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("j"))
	updateTUI(t, model, tuiKey("j"))
	view = model.View().Content
	for _, want := range []string{"Kind: Connection", "Path: /team/direct", "Endpoint: direct.test:2201", "User: deploy", "Method: agent"} {
		if !strings.Contains(view, want) {
			t.Fatalf("connection Details omitted %q:\n%s", want, view)
		}
	}
	for _, want := range []string{"c Connect", "n New connection", "f New folder", "e Edit", "m Move", "d Delete", "r Reload", "? Help", "q Quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("connection Actions omitted %q:\n%s", want, view)
		}
	}

	scope, err := connections.DeleteScope(context.Background(), app.ItemSelector{ID: direct.Connection.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connections.Delete(context.Background(), scope.Request()); err != nil {
		t.Fatal(err)
	}
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("r")))
	view = model.View().Content
	if !strings.Contains(view, ">   [-] team") || !strings.Contains(view, "Path: /team") || !strings.Contains(view, "Direct connections: 0") || strings.Contains(view, "Path: /team/direct") {
		t.Fatalf("missing selection did not atomically fall back to parent:\n%s", view)
	}
}

func TestTUIScrollbarKeyboardOnlyOverflow(t *testing.T) {
	folders, connections := integrationTreeServices(t)
	for index := 0; index < 12; index++ {
		name := fmt.Sprintf("destination-%02d", index)
		if _, err := folders.Create(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/"}, Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	longFolderName := strings.Repeat("y", 50)
	longFolder, err := folders.Create(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/"}, Name: longFolderName})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 30; index++ {
		name := fmt.Sprintf("host-%02d", index)
		if _, err := connections.Create(context.Background(), app.CreateConnectionRequest{
			Parent: app.ItemSelector{Path: "/"}, Name: name, Host: name + ".test", Port: 22, AuthMethod: app.AuthMethodAgent,
		}); err != nil {
			t.Fatal(err)
		}
	}
	longName := strings.Repeat("z", 60)
	_, err = connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: longName, Host: strings.Repeat("long-host.", 12) + "test", Port: 2222, AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	longErrorConnection, err := connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{ID: longFolder.Folder.ID}, Name: longName, Host: "stale-target.test", Port: 22, AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}

	model := tui.New(tui.Config{Folders: folders, Connections: connections, Width: 40, Height: 12, NoColor: true})
	executeTUICommand(t, model, model.Init())
	for _, key := range []string{"j", "j", "G", "k", "g"} {
		updateTUI(t, model, tuiKey(key))
		assertKeyboardOnlyOverflowView(t, model, "Tree after "+key, "Tree", "[*] Tree")
	}

	updateTUI(t, model, tuiKey("G"))
	assertKeyboardOnlyOverflowView(t, model, "Tree end", "Tree", ">   [ssh] ")
	updateTUI(t, model, tuiKey("tab"))
	updateTUI(t, model, tuiKey("G"))
	assertKeyboardOnlyOverflowView(t, model, "Details end", "Details", "Method: agent")
	updateTUI(t, model, tuiKey("tab"))

	updateTUI(t, model, tuiKey("n"))
	assertKeyboardOnlyOverflowView(t, model, "connection form", "Details", "New connection")
	updateTUI(t, model, tuiKey("tab"))
	assertKeyboardOnlyOverflowView(t, model, "connection form next field", "Details", "Host:")
	updateTUI(t, model, tuiKey("f1"))
	assertKeyboardOnlyOverflowView(t, model, "form Help", "Help", "Connection form Help")
	updateTUI(t, model, tuiKey("esc"))
	updateTUI(t, model, tuiKey("esc"))

	updateTUI(t, model, tuiKey("m"))
	updateTUI(t, model, tuiKey("G"))
	updateTUI(t, model, tuiKey("k"))
	assertKeyboardOnlyOverflowView(t, model, "move picker", "Move", "> /destination-11")
	updateTUI(t, model, tuiKey("esc"))

	updateTUI(t, model, tuiKey("c"))
	assertKeyboardOnlyOverflowView(t, model, "connect confirmation", "Connect", "Connect confirmation")
	updateTUI(t, model, tuiKey("esc"))

	updateTUI(t, model, tuiKey("g"))
	updateTUI(t, model, tuiKey("l"))
	for range 12 {
		updateTUI(t, model, tuiKey("j"))
	}
	updateTUI(t, model, tuiKey("l"))
	updateTUI(t, model, tuiKey("l"))
	assertKeyboardOnlyOverflowView(t, model, "long stale target", "Tree", ">     [ssh]")

	changedHost := "changed-before-delete.test"
	expected := longErrorConnection.Connection.Revision
	if _, err := connections.Update(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: longErrorConnection.Connection.ID}, Expected: &expected, Host: &changedHost}); err != nil {
		t.Fatal(err)
	}
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	assertKeyboardOnlyOverflowView(t, model, "recoverable error", "Operation Error", "Recoverable operation error")
	updateTUI(t, model, tuiKey("G"))
	assertKeyboardOnlyOverflowView(t, model, "recoverable error end", "Operation Error", "b/Esc Back")
	updateTUI(t, model, tuiKey("esc"))

	updateTUI(t, model, tuiKey("?"))
	assertKeyboardOnlyOverflowView(t, model, "Tree Help", "Help", "Help")
	updateTUI(t, model, tuiKey("esc"))
}

func assertKeyboardOnlyOverflowView(t *testing.T, model *tui.Model, surface, panelTitle, active string) {
	t.Helper()
	view := model.View().Content
	if !strings.Contains(view, active) {
		t.Fatalf("keyboard-only %s omitted active content %q:\n%s", surface, active, view)
	}
	if strings.Contains(view, "↑ more") || strings.Contains(view, "↓ more") {
		t.Fatalf("keyboard-only %s has no marker-free scrollbar:\n%s", surface, view)
	}
	assertActivePanelScrollbar(t, view, surface, panelTitle)
	if strings.Contains(strings.ToLower(view), "mouse") {
		t.Fatalf("keyboard-only %s exposed a mouse dependency:\n%s", surface, view)
	}
}

func assertActivePanelScrollbar(t *testing.T, view, surface, title string) {
	t.Helper()
	rows := strings.Split(view, "\n")
	activeTitle := "[*] " + title
	for top, row := range rows {
		marker := strings.Index(row, activeTitle)
		if marker < 0 {
			continue
		}
		leftByte := strings.LastIndex(row[:marker], "┌")
		rightRelative := strings.Index(row[marker:], "┐")
		if leftByte < 0 || rightRelative < 0 {
			continue
		}
		left := ansi.StringWidth(row[:leftByte])
		right := ansi.StringWidth(row[:marker+rightRelative])
		for body := top + 1; body < len(rows); body++ {
			segment := ansi.Cut(rows[body], left, right+1)
			if strings.HasPrefix(segment, "└") && strings.HasSuffix(segment, "┘") {
				break
			}
			if strings.HasSuffix(segment, "█│") || strings.HasSuffix(segment, "││") || strings.HasSuffix(segment, "#│") || strings.HasSuffix(segment, "|│") {
				return
			}
		}
		t.Fatalf("keyboard-only %s active panel %q has no surface-local scrollbar:\n%s", surface, activeTitle, view)
	}
	t.Fatalf("keyboard-only %s did not retain expected focus title %q:\n%s", surface, activeTitle, view)
}

func integrationTreeServices(t *testing.T) (*app.FolderService, *app.ConnectionService) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := catalog.Open(filepath.Join(directory, catalog.CatalogFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository := catalogrepo.NewRepository(store)
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), credential.NewFake(), credential.Scope("ffffffffffffffffffffffffffffffff"), nil)
	if err != nil {
		t.Fatal(err)
	}
	folders, err := app.NewFolderService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := app.NewConnectionService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	return folders, connections
}

func splitTreePath(path string) (string, string) {
	index := strings.LastIndex(path, "/")
	parent := path[:index]
	if parent == "" {
		parent = "/"
	}
	return parent, path[index+1:]
}

func assertTreeSelectionPath(t *testing.T, model *tui.Model, path string) {
	t.Helper()
	if !strings.Contains(model.View().Content, "Path: "+path) {
		t.Fatalf("selection path is not %s:\n%s", path, model.View().Content)
	}
}
