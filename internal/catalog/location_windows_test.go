//go:build windows

package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalDataDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	got, err := LocalDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "orza"); got != want {
		t.Fatalf("LocalDataDir() = %q, want %q", got, want)
	}
}

func TestOpenCreatesProtectedCurrentUserDACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orza", "catalog.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := validatePrivatePath(filepath.Dir(path), true); err != nil {
		t.Fatalf("directory DACL: %v", err)
	}
	if err := validatePrivatePath(path, false); err != nil {
		t.Fatalf("catalog DACL: %v", err)
	}
}

func TestOpenRejectsPermissiveDACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, nil, nil,
	); err != nil {
		t.Fatal(err)
	}

	if store, err := Open(filepath.Join(dir, "catalog.db")); err == nil {
		_ = store.Close()
		t.Fatal("Open() accepted a permissive DACL")
	}
}
