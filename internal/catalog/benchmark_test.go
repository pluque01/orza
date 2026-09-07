package catalog_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/credential"
)

const (
	benchmarkConnections = 1000
	benchmarkFolders     = 100
	benchmarkDepth       = 10
)

type catalogScaleFixture struct {
	repository *catalogrepo.Repository
	folders    *app.FolderService
	tree       app.Folder
	deepPath   string
}

func BenchmarkCatalogScale(b *testing.B) {
	fixture := newCatalogScaleFixture(b)
	ctx := context.Background()

	listed, err := fixture.repository.ListConnections(ctx, app.ListConnectionsRequest{Folder: app.ItemSelector{ID: fixture.tree.ID}})
	if err != nil || len(listed.Connections) != benchmarkConnections {
		b.Fatalf("scale connection fixture = %d, %v", len(listed.Connections), err)
	}
	deepest, err := fixture.repository.GetFolder(ctx, app.ItemSelector{Path: fixture.deepPath})
	if err != nil || pathDepth(deepest.Folder.Path) != benchmarkDepth {
		b.Fatalf("depth fixture = %q, %v", deepest.Folder.Path, err)
	}
	scope, err := fixture.folders.DeleteScope(ctx, app.ItemSelector{ID: fixture.tree.ID})
	if err != nil || scope.Folders != benchmarkFolders || scope.Connections != benchmarkConnections || len(scope.Snapshot) != benchmarkFolders+benchmarkConnections {
		b.Fatalf("recursive fixture = %#v, %v", scope, err)
	}

	b.Run("PathLookupDepth10", func(b *testing.B) {
		var result app.FolderResult
		var lookupErr error
		b.ResetTimer()
		for range b.N {
			result, lookupErr = fixture.repository.GetFolder(ctx, app.ItemSelector{Path: fixture.deepPath})
		}
		b.StopTimer()
		b.ReportMetric(benchmarkDepth, "levels")
		if lookupErr != nil || result.Folder.Path != fixture.deepPath {
			b.Fatalf("path lookup = %q, %v", result.Folder.Path, lookupErr)
		}
	})

	b.Run("List1000Connections", func(b *testing.B) {
		var result app.ListConnectionsResult
		var listErr error
		b.ResetTimer()
		for range b.N {
			result, listErr = fixture.repository.ListConnections(ctx, app.ListConnectionsRequest{Folder: app.ItemSelector{ID: fixture.tree.ID}})
		}
		b.StopTimer()
		b.ReportMetric(benchmarkConnections, "connections")
		if listErr != nil || len(result.Connections) != benchmarkConnections {
			b.Fatalf("list result = %d, %v", len(result.Connections), listErr)
		}
	})

	b.Run("RecursiveValidation1100Nodes", func(b *testing.B) {
		var result app.FolderDeleteScope
		var scopeErr error
		b.ResetTimer()
		for range b.N {
			result, scopeErr = fixture.folders.DeleteScope(ctx, app.ItemSelector{ID: fixture.tree.ID})
		}
		b.StopTimer()
		b.ReportMetric(benchmarkFolders+benchmarkConnections, "nodes")
		if scopeErr != nil || len(result.Snapshot) != benchmarkFolders+benchmarkConnections {
			b.Fatalf("recursive validation = %d, %v", len(result.Snapshot), scopeErr)
		}
	})
}

func newCatalogScaleFixture(tb testing.TB) catalogScaleFixture {
	tb.Helper()
	directory := filepath.Join(tb.TempDir(), "orza")
	if err := os.Mkdir(directory, 0o700); err != nil {
		tb.Fatal(err)
	}
	store, err := catalog.Open(filepath.Join(directory, catalog.CatalogFileName))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = store.Close() })
	repository := catalogrepo.NewRepository(store)
	ctx := context.Background()

	treeResult, err := repository.CreateFolder(ctx, app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/"}, Name: "tree"})
	if err != nil {
		tb.Fatal(err)
	}
	deepPath := ""
	for branch := range 11 {
		parent := treeResult.Folder.ID
		for level := range 9 {
			name := fmt.Sprintf("branch-%02d-level-%02d", branch, level)
			created, createErr := repository.CreateFolder(ctx, app.CreateFolderRequest{Parent: app.ItemSelector{ID: parent}, Name: name})
			if createErr != nil {
				tb.Fatal(createErr)
			}
			parent = created.Folder.ID
			if branch == 0 && level == 8 {
				deepPath = created.Folder.Path
			}
		}
	}
	for index := range benchmarkConnections {
		name := fmt.Sprintf("connection-%04d", index)
		_, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
			Parent: app.ItemSelector{ID: treeResult.Folder.ID}, Name: name,
			Host: name + ".example", Port: 22, AuthMethod: app.AuthMethodAgent,
		})
		if err != nil {
			tb.Fatal(err)
		}
	}

	var folderCount int
	if err := store.DB().QueryRow(`SELECT count(*) FROM nodes WHERE kind = 'folder' AND parent_id IS NOT NULL`).Scan(&folderCount); err != nil {
		tb.Fatal(err)
	}
	if folderCount != benchmarkFolders {
		tb.Fatalf("folder fixture count = %d, want %d", folderCount, benchmarkFolders)
	}
	saga, err := app.NewCredentialSaga(
		app.NewCatalogCredentialOperationRepository(store), credential.NewFake(),
		credential.Scope("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), nil,
	)
	if err != nil {
		tb.Fatal(err)
	}
	folders, err := app.NewFolderService(repository, saga)
	if err != nil {
		tb.Fatal(err)
	}
	return catalogScaleFixture{repository: repository, folders: folders, tree: treeResult.Folder, deepPath: deepPath}
}

func pathDepth(path string) int {
	depth := 0
	for _, character := range path {
		if character == '/' {
			depth++
		}
	}
	return depth
}
