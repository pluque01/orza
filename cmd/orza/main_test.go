package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
)

func TestRealMainUsesInjectedStreamsAndClosesDependencies(t *testing.T) {
	store, err := catalog.Open(mainCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	dependencies := &app.Dependencies{Catalog: store}
	originalBootstrap := bootstrap
	bootstrap = func(context.Context) (*app.Dependencies, error) { return dependencies, nil }
	t.Cleanup(func() { bootstrap = originalBootstrap })

	var stdout, stderr bytes.Buffer
	if got := realMain([]string{"--unknown"}, strings.NewReader(""), &stdout, &stderr); got != cli.ExitUsage {
		t.Fatalf("realMain() = %d, want %d", got, cli.ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid command usage") {
		t.Fatalf("stderr = %q, want safe usage error", stderr.String())
	}
	if err := store.CheckIntegrity(context.Background()); err == nil {
		t.Fatal("catalog remains open after realMain returned")
	}
}

func TestRealMainHelpAndVersionDoNotBootstrap(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "help flag", args: []string{"--help"}, want: "Usage:\n  orza [flags]"},
		{name: "short help flag", args: []string{"connection", "--help"}, want: "Usage:\n  orza connection"},
		{name: "help command", args: []string{"help", "folder"}, want: "Usage:\n  orza folder"},
		{name: "version flag", args: []string{"--version"}, want: "orza " + version + "\n"},
		{name: "version command", args: []string{"version"}, want: "orza " + version + "\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalBootstrap := bootstrap
			bootstrapCalls := 0
			bootstrap = func(context.Context) (*app.Dependencies, error) {
				bootstrapCalls++
				return nil, errors.New("bootstrap must not run")
			}
			t.Cleanup(func() { bootstrap = originalBootstrap })

			var stdout, stderr bytes.Buffer
			if got := realMain(test.args, strings.NewReader(""), &stdout, &stderr); got != cli.ExitSuccess {
				t.Fatalf("realMain() = %d, want %d; stderr=%q", got, cli.ExitSuccess, stderr.String())
			}
			if bootstrapCalls != 0 {
				t.Fatalf("bootstrap calls = %d, want 0", bootstrapCalls)
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), test.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRealMainMapsAndRedactsBootstrapErrors(t *testing.T) {
	originalBootstrap := bootstrap
	bootstrap = func(context.Context) (*app.Dependencies, error) {
		return nil, errors.New("catalog password super-secret")
	}
	t.Cleanup(func() { bootstrap = originalBootstrap })

	var stdout, stderr bytes.Buffer
	got := realMain([]string{"--json"}, strings.NewReader(""), &stdout, &stderr)
	if got != cli.ExitCatalog {
		t.Fatalf("realMain() = %d, want %d", got, cli.ExitCatalog)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), "super-secret") {
		t.Fatalf("stderr leaked bootstrap cause: %q", stderr.String())
	}
	want := "{\"ok\":false,\"error\":{\"code\":\"catalog_unavailable\",\"message\":\"catalog is unavailable; verify its location and permissions\"}}\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRealMainReportsUnavailableCredentialRecoveryAccurately(t *testing.T) {
	originalBootstrap := bootstrap
	bootstrap = func(context.Context) (*app.Dependencies, error) {
		return nil, errors.Join(errors.New("recovery-password-canary"), credential.ErrUnavailable)
	}
	t.Cleanup(func() { bootstrap = originalBootstrap })

	var stdout, stderr bytes.Buffer
	got := realMain([]string{"--json", "connection", "list", "/"}, strings.NewReader(""), &stdout, &stderr)
	if got != cli.ExitSecurity {
		t.Fatalf("realMain() = %d, want %d", got, cli.ExitSecurity)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), "canary") {
		t.Fatalf("stderr leaked bootstrap cause: %q", stderr.String())
	}
	want := "{\"ok\":false,\"error\":{\"code\":\"security_failure\",\"message\":\"secure credential store is unavailable; start or unlock the operating-system credential service and retry\"}}\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRealMainMapsCommandUsageErrors(t *testing.T) {
	store, err := catalog.Open(mainCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	originalBootstrap := bootstrap
	bootstrap = func(context.Context) (*app.Dependencies, error) {
		return &app.Dependencies{Catalog: store}, nil
	}
	t.Cleanup(func() { bootstrap = originalBootstrap })

	var stderr bytes.Buffer
	got := realMain([]string{"--unknown"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if got != cli.ExitUsage {
		t.Fatalf("realMain() = %d, want %d", got, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "invalid command usage") {
		t.Fatalf("stderr = %q, want safe usage error", stderr.String())
	}
}

func TestRealMainOwnsSignalsAndReturnsConventionalExitCode(t *testing.T) {
	tests := []struct {
		name   string
		signal os.Signal
		want   int
	}{
		{name: "interrupt", signal: os.Interrupt, want: 130},
		{name: "sigterm", signal: syscall.Signal(15), want: 143},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && test.want == 143 {
				t.Skip("SIGTERM exit policy is Unix-only")
			}
			originalBootstrap := bootstrap
			originalNotify, originalStop := notifySignals, stopSignals
			signalChannel := make(chan chan<- os.Signal, 1)
			stopped := make(chan struct{}, 1)
			notifySignals = func(channel chan<- os.Signal, _ ...os.Signal) { signalChannel <- channel }
			stopSignals = func(chan<- os.Signal) { stopped <- struct{}{} }
			bootstrap = func(ctx context.Context) (*app.Dependencies, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			t.Cleanup(func() {
				bootstrap = originalBootstrap
				notifySignals = originalNotify
				stopSignals = originalStop
			})

			result := make(chan int, 1)
			var stdout, stderr bytes.Buffer
			go func() { result <- realMain(nil, strings.NewReader(""), &stdout, &stderr) }()
			channel := <-signalChannel
			channel <- test.signal
			select {
			case got := <-result:
				if got != test.want {
					t.Fatalf("realMain() = %d, want %d", got, test.want)
				}
			case <-time.After(time.Second):
				t.Fatal("realMain did not return after signal cancellation")
			}
			select {
			case <-stopped:
			default:
				t.Fatal("root signal notification was not stopped")
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal("signal shutdown emitted command output")
			}
		})
	}
}

func mainCatalogPath(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, catalog.CatalogFileName)
}
