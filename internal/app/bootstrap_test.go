package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pluque01/orza/internal/catalog"
)

func TestBootstrapOpensChecksAndRecoversInOrder(t *testing.T) {
	path := bootstrapCatalogPath(t)
	var events []string

	dependencies, err := Bootstrap(context.Background(), BootstrapOptions{
		ResolveCatalogPath: func() (string, error) {
			events = append(events, "resolve")
			return path, nil
		},
		OpenCatalog: func(got string) (*catalog.Store, error) {
			events = append(events, "open")
			if got != path {
				t.Fatalf("OpenCatalog path = %q, want %q", got, path)
			}
			return catalog.Open(got)
		},
		RecoverPendingCredentials: func(ctx context.Context, dependencies *Dependencies) error {
			events = append(events, "recover")
			return dependencies.Catalog.CheckIntegrity(ctx)
		},
	})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	t.Cleanup(func() { _ = dependencies.Close() })

	want := []string{"resolve", "open", "recover"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestBootstrapIntegrityFailureSkipsRecoveryAndCloses(t *testing.T) {
	path := bootstrapCatalogPath(t)
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	recovered := false

	dependencies, err := Bootstrap(context.Background(), BootstrapOptions{
		ResolveCatalogPath: func() (string, error) { return path, nil },
		OpenCatalog:        func(string) (*catalog.Store, error) { return store, nil },
		RecoverPendingCredentials: func(context.Context, *Dependencies) error {
			recovered = true
			return nil
		},
	})
	if dependencies != nil {
		t.Fatal("Bootstrap() returned dependencies after integrity failure")
	}
	if err == nil {
		t.Fatal("Bootstrap() error = nil, want integrity failure")
	}
	if recovered {
		t.Fatal("recovery ran before a successful integrity check")
	}
}

func TestBootstrapRecoveryFailureClosesCatalog(t *testing.T) {
	path := bootstrapCatalogPath(t)
	var opened *Dependencies
	const secret = "recovery-secret"

	dependencies, err := Bootstrap(context.Background(), BootstrapOptions{
		ResolveCatalogPath: func() (string, error) { return path, nil },
		RecoverPendingCredentials: func(_ context.Context, dependencies *Dependencies) error {
			opened = dependencies
			return errors.New(secret)
		},
	})
	if dependencies != nil {
		t.Fatal("Bootstrap() returned dependencies after recovery failure")
	}
	if err == nil {
		t.Fatal("Bootstrap() error = nil, want recovery failure")
	}
	if opened == nil {
		t.Fatal("recovery callback was not invoked")
	}
	if checkErr := opened.Catalog.CheckIntegrity(context.Background()); checkErr == nil {
		t.Fatal("catalog remains open after recovery failure")
	}
}

func TestDependenciesCloseIsIdempotent(t *testing.T) {
	dependencies, err := Bootstrap(context.Background(), BootstrapOptions{
		ResolveCatalogPath: func() (string, error) { return bootstrapCatalogPath(t), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := dependencies.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := dependencies.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func bootstrapCatalogPath(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, catalog.CatalogFileName)
}
