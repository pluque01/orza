//go:build darwin

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/terminal"
)

func TestDarwinNativeNonSynchronizingKeychainLifecycle(t *testing.T) {
	if reason := darwinKeychainTestUnavailable(); reason != "" {
		t.Skip(reason)
	}
	exerciseNativeCredentialLifecycle(t, verifyDarwinKeychainNotSynchronizing)
}

func TestDarwinNativeCatalogModes(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, catalog.CatalogFileName)
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	assertPOSIXMode(t, directory, 0o700)
	assertPOSIXMode(t, path, 0o600)
}

func TestDarwinNativeTerminalCaptureRestore(t *testing.T) {
	local := terminal.New(os.Stdin, os.Stdout)
	if !local.Interactive() {
		t.Skip("macOS terminal capture/restore requires interactive stdin and stdout")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state, err := local.Capture(ctx)
	if err != nil {
		t.Skipf("macOS terminal state is unavailable for capture/restore: %v", err)
	}
	if err := local.Restore(ctx, state); err != nil {
		t.Fatalf("restore captured macOS terminal state: %v", err)
	}
}

func TestDarwinNativeReleaseExecutable(t *testing.T) {
	smokeNativeReleaseExecutable(t)
}

func assertPOSIXMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%q mode = %04o, want %04o", path, got, want)
	}
}
