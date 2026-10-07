package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

func TestTunnelPreflight(t *testing.T) {
	for _, tt := range []struct {
		args        []string
		interactive bool
		want        int
		unattended  bool
	}{
		{[]string{"tunnel", "dynamic", "/host", "--listen-port", "1080", "--non-interactive"}, false, 0, true},
		{[]string{"--json=false", "tunnel", "local", "--non-interactive=true", "--listen-port=08080", "--destination=[::1]:80", "--", "/host"}, false, 0, true},
		{[]string{"--help=false", "tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"}, false, 0, true},
		{[]string{"--help=false", "tunnel", "dynamic", "/host", "--listen-port=1080"}, false, 2, false},
		{[]string{"tunnel", "remote", "/host", "--listen-port=80", "--destination=host:80"}, true, 0, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=1080"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive=false"}, false, 2, false},
		{[]string{"--json", "tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--destination=host:80", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "local", "/host", "--listen-port=1080", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=+80", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=0", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=65536", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=80", "--listen-address=", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=80", "--listen-address=example.com", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=80", "--listen-address=0.0.0.0", "--non-interactive"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "/host", "--listen-port=80", "--listen-address=0.0.0.0", "--non-interactive", "--acknowledge-exposure"}, false, 0, true},
		{[]string{"tunnel", "wrong", "/host"}, false, 2, false},
		{[]string{"tunnel", "dynamic", "--help"}, false, 0, false},
		{[]string{"--no-color=false", "tunnel", "--json=false", "dynamic", "/host", "--listen-port=1080", "--non-interactive=true"}, false, 0, true},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			policy, err := PreflightTunnel(tt.args, tt.interactive)
			if ExitCode(err) != tt.want || policy.NonInteractive() != tt.unattended {
				t.Fatalf("policy=%+v err=%v exit=%d", policy, err, ExitCode(err))
			}
		})
	}
}

type cliTunnelRunner func(context.Context, app.TunnelRunRequest) error

func (r cliTunnelRunner) RunTunnel(ctx context.Context, request app.TunnelRunRequest) error {
	return r(ctx, request)
}

func TestTunnelForegroundModesAndCancellationJoin(t *testing.T) {
	for _, mode := range []string{"local", "remote", "dynamic"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newConnectionCLIFixture(t)
			_, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cleaned := make(chan struct{})
			runner := cliTunnelRunner(func(ctx context.Context, req app.TunnelRunRequest) error {
				req.Ready()
				<-ctx.Done()
				close(cleaned)
				return ctx.Err()
			})
			service, err := app.NewTunnelService(app.TunnelOptions{Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: &cliCredentialLifecycle{}, HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store, Scope: fixture.scope, Runner: runner, Terminal: fixture.local})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			var stdout bytes.Buffer
			stderr := &tunnelReadyWriter{cancel: cancel}
			root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: fixture.local, Stdout: &stdout, Stderr: stderr})
			args := []string{"tunnel", mode, "/host", "--listen-port=1080", "--non-interactive"}
			if mode != "dynamic" {
				args = append(args, "--destination=service.example:80")
			}
			root.SetArgs(args)
			if err := root.ExecuteContext(ctx); ExitCode(err) != ExitCanceled {
				t.Fatalf("error=%v exit=%d", err, ExitCode(err))
			}
			select {
			case <-cleaned:
			default:
				t.Fatal("returned before cleanup")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout=%q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "Active:") || !strings.Contains(stderr.String(), "/host") || !strings.Contains(stderr.String(), "127.0.0.1:1080") {
				t.Fatalf("readiness=%q", stderr.String())
			}
			if mode == "remote" && (!strings.Contains(stderr.String(), "Requested listener") || !strings.Contains(stderr.String(), "Actual remote listening scope is unverified and depends on server configuration.")) {
				t.Fatalf("remote readiness=%q", stderr.String())
			}
			for _, call := range fixture.local.Calls() {
				if call.Operation == terminal.OperationReadSecret {
					t.Fatal("unattended command prompted")
				}
			}
		})
	}
}

type tunnelReadyWriter struct {
	bytes.Buffer
	cancel  context.CancelFunc
	trigger string
}

func (w *tunnelReadyWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	trigger := w.trigger
	if trigger == "" {
		trigger = "Active:"
	}
	if strings.Contains(string(p), trigger) {
		w.cancel()
	}
	return n, err
}

func TestTunnelInteractiveReviewExposureAndTrust(t *testing.T) {
	for _, tt := range []struct {
		name, address string
		ack           bool
		answers       []string
		want          int
		calls         int
	}{
		{"cancel default", "127.0.0.1", false, []string{""}, 5, 0},
		{"separate exposure", "0.0.0.0", false, []string{"yes", "yes", "once"}, 0, 1},
		{"flag does not skip target", "0.0.0.0", true, []string{"no"}, 5, 0},
		{"trust separately rejected", "127.0.0.1", true, []string{"yes", "reject"}, 5, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newConnectionCLIFixture(t)
			_, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
			if err != nil {
				t.Fatal(err)
			}
			for _, answer := range tt.answers {
				fixture.local.QueueSecret([]byte(answer), nil)
			}
			calls := 0
			service, err := app.NewTunnelService(app.TunnelOptions{Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: &cliCredentialLifecycle{}, HostTrust: &cliHostTrust{result: app.HostTrustResult{Status: app.HostTrustUnknown}}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store, Scope: fixture.scope, Runner: cliTunnelRunner(func(ctx context.Context, req app.TunnelRunRequest) error {
				calls++
				return req.VerifyHost(ctx, app.PresentedHost{Endpoint: app.HostEndpoint{CanonicalHost: "host.example", Port: 22}, KeyAlgorithm: "ssh-ed25519", FingerprintSHA256: "SHA256:test"})
			}), Terminal: fixture.local})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			var stdout, stderr bytes.Buffer
			root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: fixture.local, Stdout: &stdout, Stderr: &stderr})
			args := []string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--listen-address=" + tt.address}
			if tt.ack {
				args = append(args, "--acknowledge-exposure")
			}
			root.SetArgs(args)
			err = root.ExecuteContext(context.Background())
			if ExitCode(err) != tt.want || calls != tt.calls {
				t.Fatalf("err=%v exit=%d calls=%d stderr=%q", err, ExitCode(err), calls, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout=%q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "SSH host.example:22") {
				t.Fatalf("target=%q", stderr.String())
			}
		})
	}
}

type cancelTunnelTerminal struct {
	*terminal.Fake
	entered  chan struct{}
	finished chan struct{}
}

func (t *cancelTunnelTerminal) ReadSecret(ctx context.Context, _ terminal.SecretPrompt) ([]byte, error) {
	close(t.entered)
	<-ctx.Done()
	close(t.finished)
	return nil, ctx.Err()
}

func TestTunnelConfirmationCancellationJoinsInput(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	_, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}
	local := &cancelTunnelTerminal{Fake: fixture.local, entered: make(chan struct{}), finished: make(chan struct{})}
	service := new(app.TunnelService) // Input cancellation must never reach Start.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: local, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	root.SetArgs([]string{"tunnel", "dynamic", "/host", "--listen-port=1080"})
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()
	select {
	case <-local.entered:
	case <-time.After(time.Second):
		t.Fatal("input did not start")
	}
	cancel()
	select {
	case err := <-done:
		if ExitCode(err) != 5 {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("input did not cancel")
	}
	select {
	case <-local.finished:
	default:
		t.Fatal("returned before input cleanup")
	}
}

type failingTunnelWriter struct{ bytes.Buffer }

func (w *failingTunnelWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "Active:") {
		return 0, errors.New("output-canary")
	}
	return w.Buffer.Write(p)
}

func TestTunnelOutputFailureStopsAndJoins(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	_, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}
	cleaned := make(chan struct{})
	service, err := app.NewTunnelService(app.TunnelOptions{Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: &cliCredentialLifecycle{}, HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store, Scope: fixture.scope, Runner: cliTunnelRunner(func(ctx context.Context, req app.TunnelRunRequest) error {
		req.Ready()
		<-ctx.Done()
		close(cleaned)
		return ctx.Err()
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	output := &failingTunnelWriter{}
	root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: fixture.local, Stdout: &bytes.Buffer{}, Stderr: output})
	root.SetArgs([]string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"})
	err = root.ExecuteContext(context.Background())
	if ExitCode(err) != ExitInternal || strings.Contains(err.Error(), "canary") {
		t.Fatalf("err=%v exit=%d", err, ExitCode(err))
	}
	select {
	case <-cleaned:
	default:
		t.Fatal("output failure returned before cleanup")
	}
}

func TestTunnelPinsCapturedRevision(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	created, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewTunnelService(app.TunnelOptions{Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: &cliCredentialLifecycle{}, HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store, Scope: fixture.scope, Runner: cliTunnelRunner(func(context.Context, app.TunnelRunRequest) error { t.Error("stale target reached network"); return nil })})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	output := &callbackWriter{callback: func() {
		host := "changed.example"
		_, err := fixture.connections.Update(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: created.Connection.ID}, Expected: &created.Connection.Revision, Host: &host})
		if err != nil {
			t.Fatal(err)
		}
	}}
	fixture.local.QueueSecret([]byte("yes"), nil)
	root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: fixture.local, Stdout: &bytes.Buffer{}, Stderr: output})
	root.SetArgs([]string{"tunnel", "dynamic", "/host", "--listen-port=1080"})
	if err := root.ExecuteContext(context.Background()); ExitCode(err) != ExitConflict {
		t.Fatalf("err=%v exit=%d", err, ExitCode(err))
	}
}

func TestTunnelCoalescesDiagnosticsWithoutDestinationHistory(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	_, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewTunnelService(app.TunnelOptions{Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: &cliCredentialLifecycle{}, HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store, Scope: fixture.scope, Runner: cliTunnelRunner(func(ctx context.Context, req app.TunnelRunRequest) error {
		for range 10000 {
			req.Diagnostic("Forwarding destination unavailable.")
			req.Diagnostic("secret-canary destination.example:80")
		}
		req.Ready()
		<-ctx.Done()
		return ctx.Err()
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output := &tunnelReadyWriter{cancel: cancel, trigger: "Notice:"}
	root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: fixture.local, Stdout: &bytes.Buffer{}, Stderr: output})
	root.SetArgs([]string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"})
	if err := root.ExecuteContext(ctx); ExitCode(err) != ExitCanceled {
		t.Fatalf("err=%v", err)
	}
	if strings.Count(output.String(), "Notice:") != 1 || strings.Contains(output.String(), "canary") || strings.Contains(output.String(), "destination.example") {
		t.Fatalf("diagnostics=%q", output.String())
	}
}

func TestTunnelExplicitEmbeddingStopReturnsSuccess(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	_, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "host", Host: "host.example", Port: 22, AuthMethod: app.AuthMethodAgent})
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewTunnelService(app.TunnelOptions{Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: &cliCredentialLifecycle{}, HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store, Scope: fixture.scope, Runner: cliTunnelRunner(func(ctx context.Context, req app.TunnelRunRequest) error {
		req.Ready()
		<-ctx.Done()
		return ctx.Err()
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	output := &tunnelReadyWriter{cancel: func() {
		if err := service.Stop(context.Background(), 1); err != nil {
			t.Error(err)
		}
	}}
	root := NewRoot(RootConfig{Connections: fixture.connections, Tunnels: service, Terminal: fixture.local, Stdout: &bytes.Buffer{}, Stderr: output})
	root.SetArgs([]string{"tunnel", "dynamic", "/host", "--listen-port=1080", "--non-interactive"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("explicit stop=%v exit=%d", err, ExitCode(err))
	}
}

func TestTunnelErrorMappings(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want int
	}{{app.ErrInvalidRequest, 2}, {app.ErrNotFound, 3}, {app.ErrConflict, 4}, {context.Canceled, 5}, {app.ErrHostNotVerified, 6}, {errors.New("secret-canary"), 1}} {
		err := tunnelCommandError(tt.err, "/host", "127.0.0.1:80")
		if ExitCode(err) != tt.want || strings.Contains(err.Error(), "canary") {
			t.Fatalf("err=%v exit=%d want=%d", err, ExitCode(err), tt.want)
		}
	}
}
