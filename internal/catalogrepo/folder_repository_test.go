package catalogrepo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
)

func TestFolderRepositoryCRUDSelectorsAndDirectChildren(t *testing.T) {
	repository, _ := testConnectionRepository(t)
	ctx := context.Background()

	root, err := repository.GetFolder(ctx, app.ItemSelector{Path: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if root.Folder.Path != "/" || root.Folder.ParentID != "" || root.Folder.Kind != app.NodeKindFolder {
		t.Fatalf("root = %#v", root.Folder)
	}

	clients := mustCreateFolder(t, repository, app.ItemSelector{ID: root.Folder.ID}, "clients")
	acme := mustCreateFolder(t, repository, app.ItemSelector{Path: "/clients"}, "acme")
	connection, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
		Parent: app.ItemSelector{ID: clients.Folder.ID}, Name: "jump", Host: "jump.example", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}

	byPath, err := repository.GetFolder(ctx, app.ItemSelector{Path: "/clients/acme"})
	if err != nil {
		t.Fatal(err)
	}
	byID, err := repository.GetFolder(ctx, app.ItemSelector{ID: acme.Folder.ID})
	if err != nil {
		t.Fatal(err)
	}
	if byPath.Folder != byID.Folder || byID.Folder.ID != acme.Folder.ID {
		t.Fatalf("folder path/ID projections differ: %#v / %#v", byPath.Folder, byID.Folder)
	}

	children, err := repository.ListChildren(ctx, app.ListChildrenRequest{Folder: app.ItemSelector{Path: "/clients"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(children.Folders) != 1 || children.Folders[0].ID != acme.Folder.ID || len(children.Connections) != 1 || children.Connections[0].ID != connection.Connection.ID {
		t.Fatalf("children = %#v", children)
	}

	renamed, err := repository.RenameFolder(ctx, app.RenameFolderRequest{
		Folder: app.ItemSelector{ID: acme.Folder.ID}, Name: "ACME", Expected: revisionPointer(acme.Folder.Revision),
	})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Folder.ID != acme.Folder.ID || renamed.Folder.Path != "/clients/ACME" || renamed.Folder.Revision != 2 {
		t.Fatalf("renamed = %#v", renamed.Folder)
	}

	archive := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "archive")
	moved, err := repository.MoveFolder(ctx, app.MoveFolderRequest{
		Folder: app.ItemSelector{ID: renamed.Folder.ID}, Destination: app.ItemSelector{ID: archive.Folder.ID}, Expected: revisionPointer(renamed.Folder.Revision),
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Folder.ID != acme.Folder.ID || moved.Folder.Path != "/archive/ACME" || moved.Folder.Revision != 3 {
		t.Fatalf("moved = %#v", moved.Folder)
	}
	if got, err := repository.GetFolder(ctx, app.ItemSelector{Path: "/archive/ACME"}); err != nil || got.Folder.ID != acme.Folder.ID {
		t.Fatalf("moved lookup = %#v, %v", got, err)
	}
}

func TestFolderRepositorySharedNamespaceParentsAndRootInvariants(t *testing.T) {
	repository, _ := testConnectionRepository(t)
	ctx := context.Background()
	folder := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "shared")

	_, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: "shared", Host: "shared.example", AuthMethod: app.AuthMethodAgent,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("connection/folder collision error = %v, want conflict", err)
	}
	connection := mustCreateConnection(t, repository, "not-a-parent")
	if _, err := repository.CreateFolder(ctx, app.CreateFolderRequest{Parent: app.ItemSelector{ID: connection.Connection.ID}, Name: "child"}); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("connection parent error = %v, want wrong kind", err)
	}

	root, err := repository.GetFolder(ctx, app.ItemSelector{Path: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.RenameFolder(ctx, app.RenameFolderRequest{Folder: app.ItemSelector{ID: root.Folder.ID}, Name: "root"}); !errors.Is(err, ErrRootImmutable) {
		t.Fatalf("rename root error = %v", err)
	}
	if _, err := repository.MoveFolder(ctx, app.MoveFolderRequest{Folder: app.ItemSelector{Path: "/"}, Destination: app.ItemSelector{ID: folder.Folder.ID}}); !errors.Is(err, ErrRootImmutable) {
		t.Fatalf("move root error = %v", err)
	}
	if _, err := repository.DeleteFolder(ctx, app.DeleteFolderRequest{Folder: app.ItemSelector{Path: "/"}}); !errors.Is(err, ErrRootImmutable) {
		t.Fatalf("delete root error = %v", err)
	}
}

func TestFolderRepositoryRejectsCyclesNonemptyAndStaleRecursiveScope(t *testing.T) {
	t.Run("cycle and nonempty", func(t *testing.T) {
		repository, _ := testConnectionRepository(t)
		parent := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "parent")
		child := mustCreateFolder(t, repository, app.ItemSelector{ID: parent.Folder.ID}, "child")

		if _, err := repository.MoveFolder(context.Background(), app.MoveFolderRequest{
			Folder: app.ItemSelector{ID: parent.Folder.ID}, Destination: app.ItemSelector{ID: child.Folder.ID}, Expected: revisionPointer(parent.Folder.Revision),
		}); !errors.Is(err, ErrFolderCycle) {
			t.Fatalf("descendant move error = %v, want cycle", err)
		}
		if _, err := repository.DeleteFolder(context.Background(), app.DeleteFolderRequest{
			Folder: app.ItemSelector{ID: parent.Folder.ID}, Expected: revisionPointer(parent.Folder.Revision),
		}); !errors.Is(err, ErrFolderNotEmpty) {
			t.Fatalf("nonrecursive delete error = %v, want nonempty", err)
		}
	})

	t.Run("stale revision", func(t *testing.T) {
		repository, _ := testConnectionRepository(t)
		folder := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "tree")
		connection := mustCreateConnectionIn(t, repository, folder.Folder.ID, "server")
		snapshot := []app.NodeRevision{
			{ID: folder.Folder.ID, Revision: folder.Folder.Revision},
			{ID: connection.Connection.ID, Revision: connection.Connection.Revision},
		}
		host := "changed.example"
		if _, err := repository.UpdateConnection(context.Background(), app.UpdateConnectionRequest{
			Connection: app.ItemSelector{ID: connection.Connection.ID}, Expected: revisionPointer(connection.Connection.Revision), Host: &host,
		}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, repository.store)
		if _, err := repository.DeleteFolder(context.Background(), app.DeleteFolderRequest{
			Folder: app.ItemSelector{ID: folder.Folder.ID}, Expected: revisionPointer(folder.Folder.Revision), Recursive: true, Snapshot: snapshot,
		}); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale revision delete error = %v, want conflict", err)
		}
		if after := catalogRevision(t, repository.store); after != before {
			t.Fatalf("catalog revision after stale delete = %d, want %d", after, before)
		}
	})

	t.Run("stale membership", func(t *testing.T) {
		repository, _ := testConnectionRepository(t)
		folder := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "tree")
		snapshot := []app.NodeRevision{{ID: folder.Folder.ID, Revision: folder.Folder.Revision}}
		mustCreateFolder(t, repository, app.ItemSelector{ID: folder.Folder.ID}, "late")
		if _, err := repository.DeleteFolder(context.Background(), app.DeleteFolderRequest{
			Folder: app.ItemSelector{ID: folder.Folder.ID}, Expected: revisionPointer(folder.Folder.Revision), Recursive: true, Snapshot: snapshot,
		}); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale membership delete error = %v, want conflict", err)
		}
	})
}

func TestFolderRepositoryRecursiveDeleteUsesExactSnapshot(t *testing.T) {
	repository, store := testConnectionRepository(t)
	tree := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "tree")
	branch := mustCreateFolder(t, repository, app.ItemSelector{ID: tree.Folder.ID}, "branch")
	first := mustCreateConnectionIn(t, repository, tree.Folder.ID, "first")
	second := mustCreateConnectionIn(t, repository, branch.Folder.ID, "second")
	unrelated := mustCreateConnection(t, repository, "unrelated")
	snapshot := []app.NodeRevision{
		{ID: second.Connection.ID, Revision: second.Connection.Revision},
		{ID: tree.Folder.ID, Revision: tree.Folder.Revision},
		{ID: first.Connection.ID, Revision: first.Connection.Revision},
		{ID: branch.Folder.ID, Revision: branch.Folder.Revision},
	}

	before := catalogRevision(t, store)
	deleted, err := repository.DeleteFolder(context.Background(), app.DeleteFolderRequest{
		Folder: app.ItemSelector{Path: "/tree"}, Expected: revisionPointer(tree.Folder.Revision), Recursive: true, Snapshot: snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.FoldersDeleted != 2 || deleted.ConnectionsDeleted != 2 || deleted.CatalogRevision != before+1 {
		t.Fatalf("deleted = %#v", deleted)
	}
	if _, err := repository.GetConnection(context.Background(), app.ItemSelector{ID: unrelated.Connection.ID}); err != nil {
		t.Fatalf("unrelated connection removed: %v", err)
	}
}

func TestFolderRepositoryRecursiveCTEDetectsCorruptionAndDepthLimit(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		parent := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "parent")
		child := mustCreateFolder(t, repository, app.ItemSelector{ID: parent.Folder.ID}, "child")
		if _, err := store.DB().Exec(`UPDATE nodes SET parent_id = ? WHERE id = ?`, child.Folder.ID, parent.Folder.ID); err != nil {
			t.Fatal(err)
		}
		connection, err := store.DB().Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		if _, err := readSubtree(context.Background(), connection, string(parent.Folder.ID)); !errors.Is(err, ErrFolderCycle) {
			t.Fatalf("readSubtree(cycle) error = %v, want cycle", err)
		}
	})

	t.Run("maximum defensive depth", func(t *testing.T) {
		_, store := testConnectionRepository(t)
		parent := rootID(t, store)
		first := ""
		for index := 0; index < maxHierarchyDepth+2; index++ {
			id := fmt.Sprintf("%032x", index+1000)
			if first == "" {
				first = id
			}
			insertTestFolder(t, store, id, fmt.Sprintf("level-%03d", index), parent)
			parent = id
		}
		connection, err := store.DB().Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		if _, err := readSubtree(context.Background(), connection, first); !errors.Is(err, ErrHierarchyTooDeep) {
			t.Fatalf("readSubtree(deep) error = %v, want depth limit", err)
		}
	})
}

func TestFolderRepositoryChecksWritableSchema(t *testing.T) {
	repository, store := testConnectionRepository(t)
	if _, err := store.DB().Exec(`UPDATE catalog_meta SET schema_version = schema_version + 1`); err != nil {
		t.Fatal(err)
	}
	_, err := repository.CreateFolder(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/"}, Name: "blocked"})
	if !errors.Is(err, catalog.ErrNewerSchema) {
		t.Fatalf("CreateFolder() error = %v, want newer schema", err)
	}
}

func TestRepositoryRejectsStaleCreateParentsAndMoveDestinationsAtomically(t *testing.T) {
	repository, store := testConnectionRepository(t)
	ctx := context.Background()
	parent := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "parent")
	destination := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "destination")
	connection := mustCreateConnection(t, repository, "connection")
	folder := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "folder")

	beforeMutation := catalogRevision(t, store)
	renamed, err := repository.RenameFolder(ctx, app.RenameFolderRequest{
		Folder: app.ItemSelector{ID: parent.Folder.ID}, Name: "renamed-parent", Expected: revisionPointer(parent.Folder.Revision),
	})
	if err != nil {
		t.Fatal(err)
	}
	renamedDestination, err := repository.RenameFolder(ctx, app.RenameFolderRequest{
		Folder: app.ItemSelector{ID: destination.Folder.ID}, Name: "renamed-destination", Expected: revisionPointer(destination.Folder.Revision),
	})
	if err != nil {
		t.Fatal(err)
	}
	if catalogRevision(t, store) != beforeMutation+2 {
		t.Fatal("destination setup mutations did not commit")
	}

	staleParent := revisionPointer(parent.Folder.Revision)
	staleDestination := revisionPointer(destination.Folder.Revision)
	beforeConflicts := catalogRevision(t, store)
	mutations := []struct {
		name   string
		mutate func() error
	}{
		{name: "create connection", mutate: func() error {
			_, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
				Parent: app.ItemSelector{ID: renamed.Folder.ID}, ExpectedParent: staleParent,
				Name: "late-connection", Host: "late.example", AuthMethod: app.AuthMethodAgent,
			})
			return err
		}},
		{name: "create folder", mutate: func() error {
			_, err := repository.CreateFolder(ctx, app.CreateFolderRequest{
				Parent: app.ItemSelector{ID: renamed.Folder.ID}, ExpectedParent: staleParent, Name: "late-folder",
			})
			return err
		}},
		{name: "move connection", mutate: func() error {
			_, err := repository.MoveConnection(ctx, app.MoveConnectionRequest{
				Connection: app.ItemSelector{ID: connection.Connection.ID}, Destination: app.ItemSelector{ID: renamedDestination.Folder.ID},
				Expected: revisionPointer(connection.Connection.Revision), ExpectedDestination: staleDestination,
			})
			return err
		}},
		{name: "move folder", mutate: func() error {
			_, err := repository.MoveFolder(ctx, app.MoveFolderRequest{
				Folder: app.ItemSelector{ID: folder.Folder.ID}, Destination: app.ItemSelector{ID: renamedDestination.Folder.ID},
				Expected: revisionPointer(folder.Folder.Revision), ExpectedDestination: staleDestination,
			})
			return err
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if err := mutation.mutate(); !errors.Is(err, ErrConflict) {
				t.Fatalf("error = %v, want conflict", err)
			}
			if got := catalogRevision(t, store); got != beforeConflicts {
				t.Fatalf("catalog revision = %d, want %d", got, beforeConflicts)
			}
		})
	}
	if _, err := repository.GetConnection(ctx, app.ItemSelector{Path: "/renamed-parent/late-connection"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale-parent connection exists: %v", err)
	}
	if _, err := repository.GetFolder(ctx, app.ItemSelector{Path: "/renamed-parent/late-folder"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale-parent folder exists: %v", err)
	}
	gotConnection, err := repository.GetConnection(ctx, app.ItemSelector{ID: connection.Connection.ID})
	if err != nil || gotConnection.Connection.ParentID != connection.Connection.ParentID || gotConnection.Connection.Revision != connection.Connection.Revision {
		t.Fatalf("connection changed after stale destination: %#v, %v", gotConnection.Connection, err)
	}
	gotFolder, err := repository.GetFolder(ctx, app.ItemSelector{ID: folder.Folder.ID})
	if err != nil || gotFolder.Folder.ParentID != folder.Folder.ParentID || gotFolder.Folder.Revision != folder.Folder.Revision {
		t.Fatalf("folder changed after stale destination: %#v, %v", gotFolder.Folder, err)
	}
}

func TestRepositoryRejectsAncestorPathDriftAtomically(t *testing.T) {
	ctx := context.Background()
	assertConflict := func(t *testing.T, store *catalog.Store, before app.CatalogRevision, err error) {
		t.Helper()
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("error = %v, want conflict", err)
		}
		if got := catalogRevision(t, store); got != before {
			t.Fatalf("catalog revision = %d, want %d", got, before)
		}
	}

	t.Run("connection create after parent ancestor rename", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		ancestor := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "ancestor")
		parent := mustCreateFolder(t, repository, app.ItemSelector{ID: ancestor.Folder.ID}, "parent")
		if _, err := repository.RenameFolder(ctx, app.RenameFolderRequest{Folder: app.ItemSelector{ID: ancestor.Folder.ID}, Name: "renamed", Expected: revisionPointer(ancestor.Folder.Revision)}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, store)
		_, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
			Parent: app.ItemSelector{ID: parent.Folder.ID}, ExpectedParent: revisionPointer(parent.Folder.Revision), ExpectedParentPath: parent.Folder.Path,
			Name: "late", Host: "late.example", AuthMethod: app.AuthMethodAgent,
		})
		assertConflict(t, store, before, err)
		if _, err := repository.GetConnection(ctx, app.ItemSelector{Path: "/renamed/parent/late"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("connection was created after ancestor rename: %v", err)
		}
	})

	t.Run("folder create after parent ancestor move", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		ancestor := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "ancestor")
		parent := mustCreateFolder(t, repository, app.ItemSelector{ID: ancestor.Folder.ID}, "parent")
		archive := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "archive")
		if _, err := repository.MoveFolder(ctx, app.MoveFolderRequest{Folder: app.ItemSelector{ID: ancestor.Folder.ID}, Destination: app.ItemSelector{ID: archive.Folder.ID}, Expected: revisionPointer(ancestor.Folder.Revision)}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, store)
		_, err := repository.CreateFolder(ctx, app.CreateFolderRequest{
			Parent: app.ItemSelector{ID: parent.Folder.ID}, ExpectedParent: revisionPointer(parent.Folder.Revision), ExpectedParentPath: parent.Folder.Path, Name: "late",
		})
		assertConflict(t, store, before, err)
		if _, err := repository.GetFolder(ctx, app.ItemSelector{Path: "/archive/ancestor/parent/late"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("folder was created after ancestor move: %v", err)
		}
	})

	t.Run("connection move after source ancestor rename", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		ancestor := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "ancestor")
		source := mustCreateConnectionIn(t, repository, ancestor.Folder.ID, "source")
		destination := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "destination")
		if _, err := repository.RenameFolder(ctx, app.RenameFolderRequest{Folder: app.ItemSelector{ID: ancestor.Folder.ID}, Name: "renamed", Expected: revisionPointer(ancestor.Folder.Revision)}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, store)
		_, err := repository.MoveConnection(ctx, app.MoveConnectionRequest{
			Connection: app.ItemSelector{ID: source.Connection.ID}, Destination: app.ItemSelector{ID: destination.Folder.ID},
			Expected: revisionPointer(source.Connection.Revision), ExpectedDestination: revisionPointer(destination.Folder.Revision),
			ExpectedSourcePath: source.Connection.Path, ExpectedDestinationPath: destination.Folder.Path,
		})
		assertConflict(t, store, before, err)
		got, getErr := repository.GetConnection(ctx, app.ItemSelector{ID: source.Connection.ID})
		if getErr != nil || got.Connection.ParentID != ancestor.Folder.ID || got.Connection.Revision != source.Connection.Revision {
			t.Fatalf("source changed after conflict: %#v, %v", got.Connection, getErr)
		}
	})

	t.Run("connection move after destination ancestor move", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		source := mustCreateConnection(t, repository, "source")
		ancestor := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "ancestor")
		destination := mustCreateFolder(t, repository, app.ItemSelector{ID: ancestor.Folder.ID}, "destination")
		archive := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "archive")
		if _, err := repository.MoveFolder(ctx, app.MoveFolderRequest{Folder: app.ItemSelector{ID: ancestor.Folder.ID}, Destination: app.ItemSelector{ID: archive.Folder.ID}, Expected: revisionPointer(ancestor.Folder.Revision)}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, store)
		_, err := repository.MoveConnection(ctx, app.MoveConnectionRequest{
			Connection: app.ItemSelector{ID: source.Connection.ID}, Destination: app.ItemSelector{ID: destination.Folder.ID},
			Expected: revisionPointer(source.Connection.Revision), ExpectedDestination: revisionPointer(destination.Folder.Revision),
			ExpectedSourcePath: source.Connection.Path, ExpectedDestinationPath: destination.Folder.Path,
		})
		assertConflict(t, store, before, err)
		got, getErr := repository.GetConnection(ctx, app.ItemSelector{ID: source.Connection.ID})
		if getErr != nil || got.Connection.ParentID != source.Connection.ParentID || got.Connection.Revision != source.Connection.Revision {
			t.Fatalf("source changed after conflict: %#v, %v", got.Connection, getErr)
		}
	})

	t.Run("folder move after source ancestor move", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		ancestor := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "ancestor")
		source := mustCreateFolder(t, repository, app.ItemSelector{ID: ancestor.Folder.ID}, "source")
		destination := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "destination")
		archive := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "archive")
		if _, err := repository.MoveFolder(ctx, app.MoveFolderRequest{Folder: app.ItemSelector{ID: ancestor.Folder.ID}, Destination: app.ItemSelector{ID: archive.Folder.ID}, Expected: revisionPointer(ancestor.Folder.Revision)}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, store)
		_, err := repository.MoveFolder(ctx, app.MoveFolderRequest{
			Folder: app.ItemSelector{ID: source.Folder.ID}, Destination: app.ItemSelector{ID: destination.Folder.ID},
			Expected: revisionPointer(source.Folder.Revision), ExpectedDestination: revisionPointer(destination.Folder.Revision),
			ExpectedSourcePath: source.Folder.Path, ExpectedDestinationPath: destination.Folder.Path,
		})
		assertConflict(t, store, before, err)
		got, getErr := repository.GetFolder(ctx, app.ItemSelector{ID: source.Folder.ID})
		if getErr != nil || got.Folder.ParentID != ancestor.Folder.ID || got.Folder.Revision != source.Folder.Revision {
			t.Fatalf("source changed after conflict: %#v, %v", got.Folder, getErr)
		}
	})

	t.Run("folder move after destination ancestor rename", func(t *testing.T) {
		repository, store := testConnectionRepository(t)
		source := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "source")
		ancestor := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "ancestor")
		destination := mustCreateFolder(t, repository, app.ItemSelector{ID: ancestor.Folder.ID}, "destination")
		if _, err := repository.RenameFolder(ctx, app.RenameFolderRequest{Folder: app.ItemSelector{ID: ancestor.Folder.ID}, Name: "renamed", Expected: revisionPointer(ancestor.Folder.Revision)}); err != nil {
			t.Fatal(err)
		}
		before := catalogRevision(t, store)
		_, err := repository.MoveFolder(ctx, app.MoveFolderRequest{
			Folder: app.ItemSelector{ID: source.Folder.ID}, Destination: app.ItemSelector{ID: destination.Folder.ID},
			Expected: revisionPointer(source.Folder.Revision), ExpectedDestination: revisionPointer(destination.Folder.Revision),
			ExpectedSourcePath: source.Folder.Path, ExpectedDestinationPath: destination.Folder.Path,
		})
		assertConflict(t, store, before, err)
		got, getErr := repository.GetFolder(ctx, app.ItemSelector{ID: source.Folder.ID})
		if getErr != nil || got.Folder.ParentID != source.Folder.ParentID || got.Folder.Revision != source.Folder.Revision {
			t.Fatalf("source changed after conflict: %#v, %v", got.Folder, getErr)
		}
	})
}

func TestRepositoryPathSnapshotsAllowUnrelatedCatalogChanges(t *testing.T) {
	repository, _ := testConnectionRepository(t)
	ctx := context.Background()
	parent := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "parent")
	destination := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "destination")
	connection := mustCreateConnection(t, repository, "connection")
	folder := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "folder")
	unrelated := mustCreateFolder(t, repository, app.ItemSelector{Path: "/"}, "unrelated")
	if _, err := repository.RenameFolder(ctx, app.RenameFolderRequest{Folder: app.ItemSelector{ID: unrelated.Folder.ID}, Name: "changed", Expected: revisionPointer(unrelated.Folder.Revision)}); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
		Parent: app.ItemSelector{ID: parent.Folder.ID}, ExpectedParent: revisionPointer(parent.Folder.Revision), ExpectedParentPath: parent.Folder.Path,
		Name: "created-connection", Host: "created.example", AuthMethod: app.AuthMethodAgent,
	}); err != nil {
		t.Fatalf("connection create after unrelated change: %v", err)
	}
	if _, err := repository.CreateFolder(ctx, app.CreateFolderRequest{
		Parent: app.ItemSelector{ID: parent.Folder.ID}, ExpectedParent: revisionPointer(parent.Folder.Revision), ExpectedParentPath: parent.Folder.Path, Name: "created-folder",
	}); err != nil {
		t.Fatalf("folder create after unrelated change: %v", err)
	}
	if _, err := repository.MoveConnection(ctx, app.MoveConnectionRequest{
		Connection: app.ItemSelector{ID: connection.Connection.ID}, Destination: app.ItemSelector{ID: destination.Folder.ID},
		Expected: revisionPointer(connection.Connection.Revision), ExpectedDestination: revisionPointer(destination.Folder.Revision),
		ExpectedSourcePath: connection.Connection.Path, ExpectedDestinationPath: destination.Folder.Path,
	}); err != nil {
		t.Fatalf("connection move after unrelated change: %v", err)
	}
	if _, err := repository.MoveFolder(ctx, app.MoveFolderRequest{
		Folder: app.ItemSelector{ID: folder.Folder.ID}, Destination: app.ItemSelector{ID: destination.Folder.ID},
		Expected: revisionPointer(folder.Folder.Revision), ExpectedDestination: revisionPointer(destination.Folder.Revision),
		ExpectedSourcePath: folder.Folder.Path, ExpectedDestinationPath: destination.Folder.Path,
	}); err != nil {
		t.Fatalf("folder move after unrelated change: %v", err)
	}
}

func mustCreateFolder(t *testing.T, repository *Repository, parent app.ItemSelector, name string) app.FolderResult {
	t.Helper()
	result, err := repository.CreateFolder(context.Background(), app.CreateFolderRequest{Parent: parent, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustCreateConnectionIn(t *testing.T, repository *Repository, parent app.NodeID, name string) app.ConnectionResult {
	t.Helper()
	result, err := repository.CreateConnection(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{ID: parent}, Name: name, Host: name + ".example", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
