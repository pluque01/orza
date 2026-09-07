//go:build windows

package app

import (
	"path/filepath"
	"testing"

	"github.com/pluque01/orza/internal/catalog"
)

func catalogPathForSagaTest(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), catalog.CatalogFileName)
}
