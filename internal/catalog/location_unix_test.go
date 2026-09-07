//go:build linux || darwin

package catalog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocalDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if runtime.GOOS == "linux" {
		xdg := filepath.Join(t.TempDir(), "data")
		t.Setenv("XDG_DATA_HOME", xdg)
		got, err := LocalDataDir()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(xdg, "orza"); got != want {
			t.Fatalf("LocalDataDir() = %q, want %q", got, want)
		}

		t.Setenv("XDG_DATA_HOME", "")
		got, err = LocalDataDir()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, ".local", "share", "orza"); got != want {
			t.Fatalf("LocalDataDir() fallback = %q, want %q", got, want)
		}
		return
	}

	got, err := LocalDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "Library", "Application Support", "orza"); got != want {
		t.Fatalf("LocalDataDir() = %q, want %q", got, want)
	}
}

func TestOpenCreatesRestrictiveModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "orza")
	path := filepath.Join(dir, "catalog.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("directory mode = %04o, want 0700", got)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("catalog mode = %04o, want 0600", got)
	}
}

func TestOpenRejectsSymlinks(t *testing.T) {
	realDir := filepath.Join(t.TempDir(), "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(filepath.Join(linkDir, "catalog.db")); err == nil {
		_ = store.Close()
		t.Fatal("Open() accepted symlink directory")
	}

	realFile := filepath.Join(realDir, "real.db")
	if err := os.WriteFile(realFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	linkFile := filepath.Join(realDir, "catalog.db")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(linkFile); err == nil {
		_ = store.Close()
		t.Fatal("Open() accepted symlink catalog")
	}
}

func TestOpenRejectsBroadPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(filepath.Join(dir, "catalog.db")); err == nil {
		_ = store.Close()
		t.Fatal("Open() accepted broad directory permissions")
	}

	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "catalog.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("Open() accepted broad catalog permissions")
	}
}
