//go:build linux

package integration

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/sshclient"
)

func TestLinuxNativeSecretServiceLifecycleOrUnavailableMapping(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("Secret Service is unavailable: DBUS_SESSION_BUS_ADDRESS is not set")
	}
	exerciseNativeCredentialLifecycle(t, nil)
}

func TestLinuxNativeSecretServiceHeadlessFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "runtime"))
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(home, "absent-session-bus"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	key := credential.Key{Scope: "native-headless", Reference: "disposable"}
	err := credential.NewStore().Set(ctx, key, []byte("disposable native-test value"))
	if !errors.Is(err, credential.ErrUnavailable) {
		t.Fatalf("headless Secret Service Set() error = %v, want ErrUnavailable", err)
	}
	entries, readErr := os.ReadDir(home)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("headless credential attempt created insecure fallback data in %q", home)
	}
}

func TestLinuxNativeAgentSocketControlledHandling(t *testing.T) {
	t.Run("missing environment", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		client, err := sshclient.OpenAgent(context.Background())
		if client != nil {
			_ = client.Close()
		}
		if !errors.Is(err, sshclient.ErrAgentUnavailable) {
			t.Fatalf("OpenAgent() error = %v, want ErrAgentUnavailable", err)
		}
	})

	t.Run("absent socket", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "absent-agent.sock"))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		client, err := sshclient.OpenAgent(ctx)
		if client != nil {
			_ = client.Close()
		}
		if !errors.Is(err, sshclient.ErrAgentUnavailable) {
			t.Fatalf("OpenAgent() error = %v, want ErrAgentUnavailable", err)
		}
	})

	t.Run("controlled socket", func(t *testing.T) {
		directory, err := os.MkdirTemp("/tmp", "orza-agent-")
		if err != nil {
			t.Skipf("short temporary path for controlled Unix agent socket is unavailable: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(directory) })
		socket := filepath.Join(directory, "agent.sock")
		listener, err := net.Listen("unix", socket)
		if err != nil {
			t.Skipf("controlled Unix agent socket is unavailable: %v", err)
		}
		defer listener.Close()
		accepted := make(chan net.Conn, 1)
		go func() {
			connection, acceptErr := listener.Accept()
			if acceptErr == nil {
				accepted <- connection
			}
		}()
		t.Setenv("SSH_AUTH_SOCK", socket)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		client, err := sshclient.OpenAgent(ctx)
		if err != nil {
			t.Fatalf("connect to controlled Unix agent socket: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("close controlled Unix agent socket: %v", err)
		}
		select {
		case connection := <-accepted:
			_ = connection.Close()
		case <-ctx.Done():
			t.Fatal("controlled Unix agent socket did not accept the connection")
		}
	})
}

func TestLinuxNativeCatalogModes(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, catalog.CatalogFileName)
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	assertLinuxPOSIXMode(t, directory, 0o700)
	assertLinuxPOSIXMode(t, path, 0o600)
}

func TestLinuxNativeReleaseExecutable(t *testing.T) {
	smokeNativeReleaseExecutable(t)
}

func assertLinuxPOSIXMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%q mode = %04o, want %04o", path, got, want)
	}
}
