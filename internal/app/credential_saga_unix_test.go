//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pluque01/orza/internal/catalog"
)

func catalogPathForSagaTest(t *testing.T) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(directory, catalog.CatalogFileName)
}
