package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/credential"
)

func TestSC014CatalogAppServiceByteIdentityRoundTrip(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, catalog.CatalogFileName)
	store, folders, connections := sc014Services(t, path)

	type expectedPair struct {
		folder     app.Folder
		connection app.Connection
	}
	values := []string{
		"ASCII-case",
		" e\u0301-combining ",
		"界-CJK",
		"👩\u200d💻-ZWJ",
	}
	want := make([]expectedPair, 0, len(values))
	for _, value := range values {
		folderResult, err := folders.Create(context.Background(), app.CreateFolderRequest{
			Parent: app.ItemSelector{Path: "/"}, Name: "folder-" + value,
		})
		if err != nil {
			t.Fatalf("create folder %q: %v", value, err)
		}
		connectionResult, err := connections.Create(context.Background(), app.CreateConnectionRequest{
			Parent: app.ItemSelector{ID: folderResult.Folder.ID}, Name: "connection-" + value,
			Host: "host-" + value, Port: 22, Username: "user-" + value,
			AuthMethod: app.AuthMethodAgent, CredentialIntent: app.CredentialKeep,
		})
		if err != nil {
			t.Fatalf("create connection %q: %v", value, err)
		}
		want = append(want, expectedPair{folder: folderResult.Folder, connection: connectionResult.Connection})
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, folders, connections = sc014Services(t, path)
	t.Cleanup(func() { _ = store.Close() })
	for _, expected := range want {
		folderResult, err := folders.Get(context.Background(), app.ItemSelector{ID: expected.folder.ID})
		if err != nil {
			t.Fatalf("get folder %q after reopen: %v", expected.folder.Name, err)
		}
		if folderResult.Folder.Name != expected.folder.Name || folderResult.Folder.Path != expected.folder.Path {
			t.Fatalf("folder byte identity changed after persistence: got name/path %q/%q, want %q/%q", folderResult.Folder.Name, folderResult.Folder.Path, expected.folder.Name, expected.folder.Path)
		}
		connectionResult, err := connections.Get(context.Background(), app.ItemSelector{ID: expected.connection.ID})
		if err != nil {
			t.Fatalf("get connection %q after reopen: %v", expected.connection.Name, err)
		}
		got := connectionResult.Connection
		if got.Name != expected.connection.Name || got.Path != expected.connection.Path || got.Host != expected.connection.Host || got.Username != expected.connection.Username {
			t.Fatalf("connection byte identity changed after persistence: got name/path/host/user %q/%q/%q/%q, want %q/%q/%q/%q", got.Name, got.Path, got.Host, got.Username, expected.connection.Name, expected.connection.Path, expected.connection.Host, expected.connection.Username)
		}
	}
}

func sc014Services(t *testing.T, path string) (*catalog.Store, *app.FolderService, *app.ConnectionService) {
	t.Helper()
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	repository := catalogrepo.NewRepository(store)
	saga, err := app.NewCredentialSaga(
		app.NewCatalogCredentialOperationRepository(store),
		credential.NewFake(),
		credential.Scope("14141414141414141414141414141414"),
		nil,
	)
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	folders, err := app.NewFolderService(repository, saga)
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	connections, err := app.NewConnectionService(repository, saga)
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	return store, folders, connections
}
