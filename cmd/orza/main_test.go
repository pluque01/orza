package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/hostkey"
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

func TestTunnelInvalidPreflightNeverBootstraps(t *testing.T) {
	original := bootstrap
	defer func() { bootstrap = original }()
	bootstrap = func(context.Context) (*app.Dependencies, error) {
		t.Fatal("invalid tunnel reached bootstrap")
		return nil, nil
	}
	for _, args := range [][]string{
		{"--json", "tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"},
		{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive=false"},
		{"tunnel", "local", "/host", "--listen-port=1080", "--non-interactive"},
		{"tunnel", "dynamic", "/host", "--listen-port=0", "--non-interactive"},
		{"tunnel", "dynamic", "--", "/host", "--listen-port=1080", "--non-interactive"},
		{"--help=false", "tunnel", "dynamic", "/host", "--listen-port=1080"},
	} {
		var stdout, stderr bytes.Buffer
		if got := realMain(args, strings.NewReader(""), &stdout, &stderr); got != cli.ExitUsage {
			t.Fatalf("args=%v exit=%d stderr=%q", args, got, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout=%q", stdout.String())
		}
	}
}

func TestTunnelPolicySelectedBeforeBootstrap(t *testing.T) {
	original := bootstrap
	defer func() { bootstrap = original }()
	called := false
	bootstrap = func(ctx context.Context) (*app.Dependencies, error) {
		called = true
		if !credential.IsNonInteractive(ctx) {
			t.Fatal("no-UI policy missing before recovery")
		}
		return nil, credential.ErrUnavailable
	}
	for _, args := range [][]string{
		{"--json=false", "tunnel", "local", "--non-interactive=true", "--destination=host:80", "--listen-port=8080", "--", "/host"},
		{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive", "--no-color=false"},
		{"--help=false", "tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive=true"},
	} {
		called = false
		var stdout, stderr bytes.Buffer
		if got := realMain(args, strings.NewReader(""), &stdout, &stderr); got != cli.ExitCatalog || !called {
			t.Fatalf("exit=%d bootstrap=%v stderr=%q", got, called, stderr.String())
		}
		if stdout.Len() != 0 || strings.HasPrefix(stderr.String(), "{") {
			t.Fatalf("streams=%q/%q", stdout.String(), stderr.String())
		}
	}
}

type mainTunnelRunner func(context.Context, app.TunnelRunRequest) error

func (r mainTunnelRunner) RunTunnel(ctx context.Context, req app.TunnelRunRequest) error {
	return r(ctx, req)
}

func TestTunnelSignalsReturnOnlyAfterOwnerCleanup(t *testing.T) {
	for _, tt := range []struct {
		signal os.Signal
		want   int
	}{{os.Interrupt, 130}, {syscall.Signal(15), 143}} {
		t.Run(strconv.Itoa(tt.want), func(t *testing.T) {
			if runtime.GOOS == "windows" && tt.want == 143 {
				t.Skip("SIGTERM is Unix-only")
			}
			store, err := catalog.Open(mainCatalogPath(t))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			repository := catalogrepo.NewRepository(store)
			secrets := credential.NewFake()
			saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), secrets, credential.Scope("test"), nil)
			if err != nil {
				t.Fatal(err)
			}
			connections, err := app.NewConnectionService(repository, saga)
			if err != nil {
				t.Fatal(err)
			}
			_, err = connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
			if err != nil {
				t.Fatal(err)
			}
			trusted := catalog.NewTrustedHostRepository(store)
			policy, err := hostkey.NewCatalogPolicy(trusted)
			if err != nil {
				t.Fatal(err)
			}
			started, stopping, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			tunnels, err := app.NewTunnelService(app.TunnelOptions{Connections: repository, Credentials: saga, HostTrust: policy, TrustedHosts: hostkey.NewCatalogTrustedHostAdapter(trusted), Store: secrets, Scope: credential.Scope("test"), Runner: mainTunnelRunner(func(ctx context.Context, req app.TunnelRunRequest) error {
				req.Ready()
				close(started)
				<-ctx.Done()
				close(stopping)
				<-release
				return ctx.Err()
			})})
			if err != nil {
				t.Fatal(err)
			}
			oldBootstrap, oldNotify, oldStop := bootstrap, notifySignals, stopSignals
			defer func() { bootstrap, notifySignals, stopSignals = oldBootstrap, oldNotify, oldStop }()
			signalChannel := make(chan chan<- os.Signal, 1)
			notifySignals = func(ch chan<- os.Signal, _ ...os.Signal) { signalChannel <- ch }
			stopSignals = func(chan<- os.Signal) {}
			bootstrap = func(ctx context.Context) (*app.Dependencies, error) {
				if !credential.IsNonInteractive(ctx) {
					t.Error("policy was not set before bootstrap")
				}
				return &app.Dependencies{Catalog: store, Connections: connections, Tunnels: tunnels}, nil
			}
			result := make(chan int, 1)
			var stdout, stderr bytes.Buffer
			go func() {
				result <- realMain([]string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"}, strings.NewReader(""), &stdout, &stderr)
			}()
			channel := <-signalChannel
			<-started
			channel <- tt.signal
			select {
			case <-stopping:
			case <-time.After(time.Second):
				t.Fatal("signal did not stop runtime")
			}
			select {
			case code := <-result:
				t.Fatalf("returned before cleanup: %d", code)
			default:
			}
			close(release)
			select {
			case code := <-result:
				if code != tt.want {
					t.Fatalf("exit=%d want=%d", code, tt.want)
				}
			case <-time.After(time.Second):
				t.Fatal("owner did not join cleanup")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout=%q", stdout.String())
			}
			if err := store.CheckIntegrity(context.Background()); err == nil {
				t.Fatal("catalog remains open after signal result")
			}
		})
	}
}
