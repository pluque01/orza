//go:build windows

package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func LocalDataDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(base, "orza"), nil
}

func prepareCatalogPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve catalog path: %w", err)
	}
	if err := rejectNonLocalWindowsPath(absPath); err != nil {
		return "", err
	}
	dir := filepath.Dir(absPath)

	dirCreated := false
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create catalog directory: %w", err)
		}
		dirCreated = true
	} else if err != nil {
		return "", fmt.Errorf("inspect catalog directory: %w", err)
	}
	if dirCreated {
		if err := setPrivateDACL(dir, true); err != nil {
			return "", err
		}
	}
	if err := validatePrivatePath(dir, true); err != nil {
		return "", err
	}

	fileCreated := false
	if _, err := os.Lstat(absPath); errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(absPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if createErr != nil {
			return "", fmt.Errorf("create catalog file: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", fmt.Errorf("close new catalog file: %w", closeErr)
		}
		fileCreated = true
	} else if err != nil {
		return "", fmt.Errorf("inspect catalog file: %w", err)
	}
	if fileCreated {
		if err := setPrivateDACL(absPath, false); err != nil {
			return "", err
		}
	}
	if err := validatePrivatePath(absPath, false); err != nil {
		return "", err
	}
	return absPath, nil
}

func setPrivateDACL(path string, directory bool) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	var pinner runtime.Pinner
	pinner.Pin(sid)
	defer pinner.Unpin()

	inheritance := uint32(windows.NO_INHERITANCE)
	if directory {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entries := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("build private DACL for %q: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil,
	); err != nil {
		return fmt.Errorf("set private DACL on %q: %w", path, err)
	}
	return nil
}

func validatePrivatePath(path string, directory bool) error {
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
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode private path %q: %w", path, err)
	}
	attributes, err := windows.GetFileAttributes(path16)
	if err != nil {
		return fmt.Errorf("read attributes for %q: %w", path, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("private path %q must not be a reparse point", path)
	}

	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read DACL for %q: %w", path, err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("read owner for %q: %w", path, err)
	}
	user, err := currentUserSID()
	if err != nil {
		return err
	}
	if owner == nil || !owner.Equals(user) {
		return fmt.Errorf("private path %q is not owned by the current user", path)
	}
	control, _, err := sd.Control()
	if err != nil {
		return fmt.Errorf("read DACL control for %q: %w", path, err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("private path %q has an inherited DACL", path)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount == 0 {
		return fmt.Errorf("private path %q has no restrictive DACL", path)
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return fmt.Errorf("read DACL entry for %q: %w", path, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("private path %q has an unsupported DACL entry", path)
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !aceSID.Equals(user) {
			return fmt.Errorf("private path %q grants access to another principal", path)
		}
	}
	return nil
}

func currentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read current Windows user: %w", err)
	}
	return user.User.Sid, nil
}

func rejectNonLocalWindowsPath(path string) error {
	volume := filepath.VolumeName(path)
	if strings.HasPrefix(volume, `\\`) {
		return errors.New("catalog must not be stored on a network path")
	}
	for _, variable := range []string{"OneDrive", "OneDriveCommercial", "OneDriveConsumer"} {
		if root := os.Getenv(variable); root != "" && pathWithin(path, root) {
			return fmt.Errorf("catalog must not be stored under %s", variable)
		}
	}
	return nil
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, `..\`)
}
