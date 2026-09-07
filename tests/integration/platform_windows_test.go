//go:build windows

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/sys/windows"
)

func TestWindowsNativeCredentialManagerLifecycle(t *testing.T) {
	exerciseNativeCredentialLifecycle(t, nil)
}

func TestWindowsNativeOpenSSHAgentNamedPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := sshclient.OpenAgent(ctx)
	if err != nil {
		if !errors.Is(err, sshclient.ErrAgentUnavailable) {
			t.Fatalf("OpenAgent() error = %v, want ErrAgentUnavailable", err)
		}
		t.Skipf("Windows OpenSSH agent named pipe is unavailable: %v", err)
	}
	if client == nil {
		t.Fatal("OpenAgent() returned a nil client")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close Windows OpenSSH agent named pipe: %v", err)
	}
}

func TestWindowsNativeCatalogDACL(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, catalog.CatalogFileName)
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	assertCurrentUserOnlyDACL(t, directory)
	assertCurrentUserOnlyDACL(t, path)
}

func TestWindowsNativeConsoleCaptureRestore(t *testing.T) {
	console := terminal.New(os.Stdin, os.Stdout)
	if !console.Interactive() {
		t.Skip("Windows console capture/restore requires interactive stdin and stdout")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state, err := console.Capture(ctx)
	if err != nil {
		t.Skipf("Windows console state is unavailable for capture/restore: %v", err)
	}
	if err := console.Restore(ctx, state); err != nil {
		t.Fatalf("restore captured Windows console state: %v", err)
	}
}

func TestWindowsNativeReleaseExecutable(t *testing.T) {
	smokeNativeReleaseExecutable(t)
}

func assertCurrentUserOnlyDACL(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("read DACL for %q: %v", path, err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		t.Fatalf("read owner for %q: %v", path, err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("read current Windows user: %v", err)
	}
	if owner == nil || !owner.Equals(user.User.Sid) {
		t.Fatalf("%q is not owned by the current Windows user", path)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatalf("read DACL control for %q: %v", path, err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("%q has an inherited DACL", path)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount == 0 {
		t.Fatalf("%q has no restrictive DACL: %v", path, err)
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatalf("read DACL entry %d for %q: %v", i, path, err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			t.Fatalf("DACL entry %d for %q is not an allow entry", i, path)
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !aceSID.Equals(user.User.Sid) {
			t.Fatalf("DACL entry %d for %q grants another principal", i, path)
		}
	}
}
