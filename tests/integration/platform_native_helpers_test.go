//go:build windows || darwin || linux

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/credential"
)

func exerciseNativeCredentialLifecycle(t *testing.T, verify func(credential.Key) error) {
	t.Helper()
	store := credential.NewStore()
	key := credential.Key{
		Scope:     credential.Scope(fmt.Sprintf("native-integration-%d", os.Getpid())),
		Reference: credential.Reference(fmt.Sprintf("disposable-%d", time.Now().UnixNano())),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stored := false
	t.Cleanup(func() {
		if !stored {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = store.Delete(cleanupCtx, key)
	})

	secret := []byte("orza disposable native-test value")
	defer clear(secret)
	if err := store.Set(ctx, key, secret); err != nil {
		if errors.Is(err, credential.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) {
			t.Skipf("native credential service is unavailable or locked: %v", err)
		}
		t.Fatalf("store disposable credential: %v", err)
	}
	stored = true
	if verify != nil {
		if err := verify(key); err != nil {
			t.Fatalf("verify native credential metadata: %v", err)
		}
	}
	actual, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("read disposable credential: %v", err)
	}
	if !bytes.Equal(actual, secret) {
		t.Fatal("native credential store returned a different disposable value")
	}
	clear(actual)

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete disposable credential: %v", err)
	}
	stored = false
	if _, err := store.Get(ctx, key); !errors.Is(err, credential.ErrNotFound) {
		t.Fatalf("read deleted disposable credential: %v, want ErrNotFound", err)
	}
}

func smokeNativeReleaseExecutable(t *testing.T) {
	t.Helper()
	binary := nativeReleaseExecutable(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--version")
	dataRoot := t.TempDir()
	cmd.Env = environmentWith(map[string]string{
		"HOME":                     dataRoot,
		"LOCALAPPDATA":             dataRoot,
		"XDG_DATA_HOME":            dataRoot,
		"SSH_AUTH_SOCK":            "",
		"DBUS_SESSION_BUS_ADDRESS": "unix:path=" + filepath.Join(dataRoot, "absent-session-bus"),
	})
	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("release executable timed out: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("release executable %q --version failed: %v\n%s", binary, err, output)
	}
	if !strings.Contains(string(output), "orza ") {
		t.Fatalf("release executable version output = %q", output)
	}
}

func nativeReleaseExecutable(t *testing.T) string {
	t.Helper()
	if configured := os.Getenv("ORZA_RELEASE_BINARY"); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured
		}
		t.Skipf("release executable configured by ORZA_RELEASE_BINARY is unavailable: %q", configured)
	}

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("release executable is unavailable: cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	name := "orza"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidates := []string{
		filepath.Join(root, "result", "bin", name),
		filepath.Join(root, "dist", fmt.Sprintf("orza-%s-%s", runtime.GOOS, runtime.GOARCH), name),
		filepath.Join(root, "dist", fmt.Sprintf("orza-%s-%s%s", runtime.GOOS, runtime.GOARCH, filepath.Ext(name))),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skipf("native %s/%s release executable is unavailable; set ORZA_RELEASE_BINARY", runtime.GOOS, runtime.GOARCH)
	return ""
}

func environmentWith(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[name]; !replaced {
			environment = append(environment, entry)
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}
