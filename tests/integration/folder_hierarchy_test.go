package integration

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
)

func TestFolderHierarchyCLIAndTUIServiceParity(t *testing.T) {
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
	secureStore := credential.NewFake()
	scope := credential.Scope("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), secureStore, scope, nil)
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

	execute := func(args ...string) (string, error) {
		var output bytes.Buffer
		root := cli.NewRoot(cli.RootConfig{Version: "test", Folders: folders, Connections: connections, Terminal: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}), Stdin: strings.NewReader(""), Stdout: &output, Stderr: &bytes.Buffer{}})
		root.SetArgs(args)
		err := root.ExecuteContext(context.Background())
		return output.String(), err
	}
	for _, path := range []string{"/clients", "/clients/acme", "/clients/acme/prod", "/archive"} {
		if _, err := execute("folder", "create", path); err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
	}

	created, err := connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/clients/acme/prod"}, Name: "db", Host: "db.test", Port: 22,
		AuthMethod: app.AuthMethodPassword, CredentialIntent: app.CredentialRemember, Password: []byte("secret"),
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialKey := credential.Key{Scope: scope, Reference: credential.Reference(created.Connection.CredentialRef)}
	if _, ok := secureStore.Lookup(credentialKey); !ok {
		t.Fatal("remembered credential was not stored")
	}

	model := tui.New(tui.Config{Folders: folders, Connections: connections, Width: 80, Height: 24, NoColor: true})
	executeTUICommand(t, model, model.Init())

	// Root is selected and expanded. Select clients/acme/prod by its visible
	// tree rows, then move prod to archive.
	assertTreeSelectionPath(t, model, "/")
	updateTUI(t, model, tuiKey("j"))
	updateTUI(t, model, tuiKey("j"))
	assertTreeSelectionPath(t, model, "/clients")
	updateTUI(t, model, tuiKey("enter"))
	updateTUI(t, model, tuiKey("j"))
	assertTreeSelectionPath(t, model, "/clients/acme")
	updateTUI(t, model, tuiKey("enter"))
	updateTUI(t, model, tuiKey("j"))
	assertTreeSelectionPath(t, model, "/clients/acme/prod")
	updateTUI(t, model, tuiKey("m"))
	updateTUI(t, model, tuiKey("esc"))
	assertTreeSelectionPath(t, model, "/clients/acme/prod")
	updateTUI(t, model, tuiKey("m"))
	updateTUI(t, model, tuiKey("j"))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("enter")))
	assertTreeSelectionPath(t, model, "/archive/prod")

	// Navigate to the moved folder, then move its connection first to archive
	// and finally across the hierarchy into clients/acme.
	updateTUI(t, model, tuiKey("enter"))
	updateTUI(t, model, tuiKey("j"))
	assertTreeSelectionPath(t, model, "/archive/prod/db")
	updateTUI(t, model, tuiKey("m"))
	updateTUI(t, model, tuiKey("j"))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("enter")))
	assertTreeSelectionPath(t, model, "/archive/db")
	updateTUI(t, model, tuiKey("m"))
	for range 4 {
		updateTUI(t, model, tuiKey("j"))
	}
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("enter")))
	assertTreeSelectionPath(t, model, "/clients/acme/db")

	// Create and rename a folder through the TUI, then observe both TUI
	// mutations through the independent CLI projection.
	updateTUI(t, model, tuiKey("g"))
	updateTUI(t, model, tuiKey("j"))
	assertTreeSelectionPath(t, model, "/archive")
	updateTUI(t, model, tuiKey("?"))
	if view := model.View().Content; !strings.Contains(view, "Help") || !strings.Contains(view, "n New connection") {
		t.Fatalf("standalone Help view = %q", view)
	}
	updateTUI(t, model, tuiKey("esc"))
	assertTreeSelectionPath(t, model, "/archive")
	updateTUI(t, model, tuiKey("f"))
	typeText(t, model, "canceled-folder")
	updateTUI(t, model, tuiKey("f1"))
	updateTUI(t, model, tuiKey("esc"))
	if view := model.View().Content; !strings.Contains(view, "canceled-folder") {
		t.Fatalf("inline Help did not restore folder field: %q", view)
	}
	updateTUI(t, model, tuiKey("esc"))
	if _, err := folders.Get(context.Background(), app.ItemSelector{Path: "/archive/canceled-folder"}); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("folder Cancel persisted data: %v", err)
	}
	updateTUI(t, model, tuiKey("f"))
	typeText(t, model, "tui-created")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	assertTreeSelectionPath(t, model, "/archive/tui-created")
	updateTUI(t, model, tuiKey("e"))
	updateTUI(t, model, tuiKey("ctrl+a"))
	updateTUI(t, model, tuiKey("ctrl+k"))
	typeText(t, model, "tui-renamed")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	updateTUI(t, model, tuiKey("enter"))
	if _, err := folders.Get(context.Background(), app.ItemSelector{Path: "/archive/tui-renamed"}); err != nil {
		t.Fatalf("folder delete Cancel changed catalog: %v", err)
	}

	archive, err := folders.List(context.Background(), app.ListChildrenRequest{Folder: app.ItemSelector{Path: "/archive"}})
	if err != nil || len(archive.Folders) != 2 || len(archive.Connections) != 0 {
		t.Fatalf("shared persisted result = %#v, %v", archive, err)
	}
	if output, err := execute("folder", "show", "/archive/tui-renamed"); err != nil || !strings.Contains(output, "/archive/tui-renamed") {
		t.Fatalf("CLI did not observe TUI folder result = %q, %v", output, err)
	}
	if output, err := execute("connection", "show", "/clients/acme/db"); err != nil || !strings.Contains(output, "/clients/acme/db") {
		t.Fatalf("CLI did not observe TUI connection result = %q, %v", output, err)
	}

	acme, err := folders.Get(context.Background(), app.ItemSelector{Path: "/clients/acme"})
	if err != nil {
		t.Fatal(err)
	}
	clients, err := folders.Get(context.Background(), app.ItemSelector{Path: "/clients"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := folders.Move(context.Background(), app.MoveFolderRequest{Folder: app.ItemSelector{ID: clients.Folder.ID}, Destination: app.ItemSelector{ID: acme.Folder.ID}, Expected: &clients.Folder.Revision}); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("cycle move = %v", err)
	}

	deleteScope, err := folders.DeleteScope(context.Background(), app.ItemSelector{Path: "/clients"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := folders.Create(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/clients"}, Name: "late"}); err != nil {
		t.Fatal(err)
	}
	if _, err := folders.Delete(context.Background(), deleteScope.Request(true)); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale recursive scope = %v", err)
	}
	deleteScope, err = folders.DeleteScope(context.Background(), app.ItemSelector{Path: "/clients"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := folders.Delete(context.Background(), deleteScope.Request(true)); err != nil {
		t.Fatal(err)
	}
	if _, ok := secureStore.Lookup(credentialKey); ok {
		t.Fatal("recursive folder deletion left an orphaned credential")
	}
}
