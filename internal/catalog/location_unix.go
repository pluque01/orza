//go:build linux || darwin

package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

func LocalDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "orza"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "orza"), nil
	}
	return filepath.Join(home, ".local", "share", "orza"), nil
}

func prepareCatalogPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve catalog path: %w", err)
	}
	dir := filepath.Dir(absPath)

	if err := rejectSymlinkComponents(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create catalog directory: %w", err)
	}
	if err := validateUnixPath(dir, true); err != nil {
		return "", err
	}

	info, err := os.Lstat(absPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		file, createErr := os.OpenFile(absPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if createErr != nil {
			return "", fmt.Errorf("create catalog file: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", fmt.Errorf("close new catalog file: %w", closeErr)
		}
	case err != nil:
		return "", fmt.Errorf("inspect catalog file: %w", err)
	case info.Mode()&os.ModeSymlink != 0:
		return "", errors.New("catalog file must not be a symlink")
	}
	if err := validateUnixPath(absPath, false); err != nil {
		return "", err
	}
	return absPath, nil
}

func rejectSymlinkComponents(path string) error {
	volume := filepath.VolumeName(path)
	remainder := path[len(volume):]
	current := volume + string(filepath.Separator)
	for _, component := range splitPath(remainder) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect catalog path component %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("catalog path component %q is a symlink", current)
		}
	}
	return nil
}

func splitPath(path string) []string {
	var components []string
	for path != "" && path != string(filepath.Separator) && path != "." {
		dir, base := filepath.Split(path)
		if base != "" {
			components = append([]string{base}, components...)
		}
		path = filepath.Clean(dir)
	}
	return components
}

func validateUnixPath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect private path %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private path %q must not be a symlink", path)
	}
	if directory && !info.IsDir() {
		return fmt.Errorf("catalog directory %q is not a directory", path)
	}
	if !directory && !info.Mode().IsRegular() {
		return fmt.Errorf("catalog path %q is not a regular file", path)
	}
	wantMode := os.FileMode(0o600)
	if directory {
		wantMode = 0o700
	}
	if info.Mode().Perm() != wantMode {
		return fmt.Errorf("private path %q has mode %04o, want %04o", path, info.Mode().Perm(), wantMode)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine owner of private path %q", path)
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("private path %q is owned by uid %d, want uid %d", path, stat.Uid, os.Geteuid())
	}
	return nil
}
