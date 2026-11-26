package catalogrepo

import (
	"errors"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
)

func TestConnectionActivityPersistsByIDAcrossRenameAndMove(t *testing.T) {
	ctx := t.Context()
	path := testCatalogPath(t)
	open := func() *catalog.Store {
		t.Helper()
		store, err := catalog.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		return store
	}
	store := open()
	repository := NewRepository(store)
	created, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: "old", Host: "saved.invalid", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := app.ItemSelector{ID: created.Connection.ID}
	if favourite, err := repository.ToggleConnectionFavourite(ctx, id.ID); err != nil || !favourite {
		t.Fatalf("toggle favourite = %t, %v", favourite, err)
	}
	latest := time.Date(2025, 10, 8, 9, 7, 6, 54, time.UTC)
	for _, opened := range []time.Time{latest, latest.Add(-time.Nanosecond)} {
		if err := repository.RecordConnectionOpened(ctx, id.ID, opened); err != nil {
			t.Fatal(err)
		}
	}
	activity, err := repository.GetConnection(ctx, id)
	if err != nil || activity.Connection.Revision != created.Connection.Revision || activity.CatalogRevision != created.CatalogRevision {
		t.Fatalf("activity changed configuration revisions: %+v, %v", activity, err)
	}
	name := "renamed"
	if _, err := repository.UpdateConnection(ctx, app.UpdateConnectionRequest{Connection: id, Name: &name}); err != nil {
		t.Fatal(err)
	}
	folder, err := repository.CreateFolder(ctx, app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/"}, Name: "archive"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.MoveConnection(ctx, app.MoveConnectionRequest{Connection: id, Destination: app.ItemSelector{ID: folder.Folder.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	repository = NewRepository(open())
	saved, err := repository.GetConnection(ctx, id)
	if err != nil || saved.Connection.Path != "/archive/renamed" || !saved.Connection.Favourite || !saved.Connection.LastOpenedAt.Equal(latest) {
		t.Fatalf("reopened catalog returned %+v, %v", saved, err)
	}
	if _, err := repository.DeleteConnection(ctx, app.DeleteConnectionRequest{Connection: id}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordConnectionOpened(ctx, id.ID, latest); !errors.Is(err, ErrNotFound) {
		t.Fatalf("recording a deleted ID returned %v", err)
	}
	if _, err := repository.ToggleConnectionFavourite(ctx, id.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("toggling a deleted ID returned %v", err)
	}
	replacement, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
		Parent: app.ItemSelector{ID: folder.Folder.ID}, Name: name, Host: "new.invalid", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil || replacement.Connection.ID == id.ID || replacement.Connection.Favourite || !replacement.Connection.LastOpenedAt.IsZero() {
		t.Fatalf("replacement inherited activity: %+v, %v", replacement, err)
	}
}
