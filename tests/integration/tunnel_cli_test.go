package integration

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type tunnelCLIOutput struct {
	bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func (w *tunnelCLIOutput) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if strings.Contains(string(p), "Active:") {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}

func TestTunnelCLIRememberedPasswordAllModesForwardAndCleanUp(t *testing.T) {
	for _, scenario := range []struct {
		mode       app.TunnelMode
		unattended bool
	}{
		{app.TunnelLocal, true}, {app.TunnelRemote, true}, {app.TunnelDynamic, true},
		{app.TunnelLocal, false}, {app.TunnelRemote, false}, {app.TunnelDynamic, false},
	} {
		mode := scenario.mode
		t.Run(string(mode)+"/non-interactive="+strconv.FormatBool(scenario.unattended), func(t *testing.T) {
			server := startForwardingServer(t, forwardingServerConfig{Password: "cli-secret-canary"})
			fixture := newForwardingAppFixture(t, "")
			connection := fixture.create(t, server, "host")
			fixture.trust(t, server)
			address, _ := startForwardingDestination(t)
			destination := forwardingEndpoint(t, address)
			config := app.TunnelConfig{Mode: mode, Listen: forwardingFreeEndpoint(t), Destination: destination}
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			local.SetInteractive(!scenario.unattended)
			if !scenario.unattended {
				local.QueueSecret([]byte("yes"), nil)
			}
			var stdout bytes.Buffer
			stderr := &tunnelCLIOutput{ready: make(chan struct{})}
			root := cli.NewRoot(cli.RootConfig{Connections: fixture.connections, Tunnels: fixture.service, Terminal: local, Stdin: strings.NewReader("stdin-must-not-be-forwarded"), Stdout: &stdout, Stderr: stderr})
			args := []string{"tunnel", string(mode), connection.Path, "--listen-port=" + strconv.Itoa(int(config.Listen.Port))}
			if scenario.unattended {
				args = append(args, "--non-interactive")
			}
			if mode != app.TunnelDynamic {
				args = append(args, "--destination="+destination.String())
			} else {
				config.Destination = app.TunnelEndpoint{}
			}
			root.SetArgs(args)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- root.ExecuteContext(ctx) }()
			select {
			case <-stderr.ready:
			case err := <-done:
				t.Fatalf("startup failed: %v", err)
			case <-ctx.Done():
				t.Fatal("CLI did not report readiness")
			}
			id := server.WaitTransport(t)
			for range 2 {
				if err := forwardingExchange(forwardingDial(t, config, destination), []byte("unchanged-cli-response")); err != nil {
					t.Fatal(err)
				}
			}
			cancel()
			if err := forwardingFixtureWait(t, done); cli.ExitCode(err) != cli.ExitCanceled {
				t.Fatalf("cancel=%v exit=%d", err, cli.ExitCode(err))
			}
			server.CloseTransport(id)
			forwardingPortReleased(t, config.Listen)
			if server.ChannelCount(id, "session") != 0 {
				t.Fatal("tunnel opened an SSH session")
			}
			if stdout.Len() != 0 || strings.Contains(stderr.String(), "cli-secret-canary") || mode == app.TunnelDynamic && strings.Contains(stderr.String(), address) {
				t.Fatalf("streams=%q/%q", stdout.String(), stderr.String())
			}
			if mode == app.TunnelRemote && (!strings.Contains(stderr.String(), "Requested listener") || !strings.Contains(stderr.String(), "Actual remote listening scope is unverified and depends on server configuration.")) {
				t.Fatalf("remote scope=%q", stderr.String())
			}
			prompts := 0
			for _, call := range local.Calls() {
				if call.Operation == terminal.OperationReadSecret {
					prompts++
				}
			}
			wantPrompts := 0
			if !scenario.unattended {
				wantPrompts = 1
			}
			if prompts != wantPrompts {
				t.Fatalf("prompts=%d want=%d", prompts, wantPrompts)
			}
		})
	}
}

func TestTunnelCLIUnattendedTrustAndCredentialsFailWithoutInput(t *testing.T) {
	for _, scenario := range []string{"unknown", "changed", "revoked", "missing", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			server := startForwardingServer(t, forwardingServerConfig{Password: "cli-secret-canary"})
			known := ""
			if scenario == "revoked" {
				known = "@revoked " + knownhosts.Line([]string{server.address}, server.signer.PublicKey()) + "\n"
			}
			fixture := newForwardingAppFixture(t, known)
			connection := fixture.create(t, server, "host")
			if scenario != "unknown" && scenario != "changed" {
				fixture.trust(t, server)
			}
			if scenario == "changed" {
				other := startForwardingServer(t, forwardingServerConfig{})
				_, err := fixture.trusted.TrustHost(context.Background(), app.TrustHostRequest{Host: app.PresentedHost{Endpoint: app.HostEndpoint{CanonicalHost: connection.Host, Port: connection.Port}, KeyAlgorithm: other.signer.PublicKey().Type(), PublicKey: other.signer.PublicKey().Marshal(), FingerprintSHA256: ssh.FingerprintSHA256(other.signer.PublicKey())}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing" {
				fixture.store.SetFault(credential.OperationGet, credential.ErrNotFound)
			}
			if scenario == "unavailable" {
				fixture.store.SetFault(credential.OperationGet, credential.ErrUnavailable)
			}
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			var stdout, stderr bytes.Buffer
			listen := forwardingFreeEndpoint(t)
			root := cli.NewRoot(cli.RootConfig{Connections: fixture.connections, Tunnels: fixture.service, Terminal: local, Stdout: &stdout, Stderr: &stderr})
			root.SetArgs([]string{"tunnel", "dynamic", connection.Path, "--listen-port=" + strconv.Itoa(int(listen.Port)), "--non-interactive"})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := root.ExecuteContext(ctx)
			if cli.ExitCode(err) != cli.ExitSecurity {
				t.Fatalf("error=%v exit=%d", err, cli.ExitCode(err))
			}
			if err := cli.WriteError(&stderr, false, err); err != nil {
				t.Fatal(err)
			}
			if stdout.Len() != 0 || strings.Contains(stderr.String(), "cli-secret-canary") {
				t.Fatalf("streams=%q/%q", stdout.String(), stderr.String())
			}
			for _, call := range local.Calls() {
				if call.Operation == terminal.OperationReadSecret {
					t.Fatal("failed unattended invocation prompted")
				}
			}
			forwardingPortReleased(t, listen)
		})
	}
}

func TestTunnelCLIBindingAndPolicyFailureIdentifyRequestedEndpoint(t *testing.T) {
	for _, mode := range []string{"local", "remote"} {
		t.Run(mode, func(t *testing.T) {
			config := forwardingServerConfig{}
			if mode == "remote" {
				config.GlobalRules = map[string]forwardingReplyRule{"tcpip-forward": {Deny: true}}
			}
			server := startForwardingServer(t, config)
			fixture := newForwardingAppFixture(t, "")
			connection := fixture.create(t, server, "host")
			fixture.trust(t, server)
			listen := forwardingFreeEndpoint(t)
			if mode == "local" {
				occupied, err := net.Listen("tcp", listen.String())
				if err != nil {
					t.Fatal(err)
				}
				defer occupied.Close()
			}
			var stdout, stderr bytes.Buffer
			root := cli.NewRoot(cli.RootConfig{Connections: fixture.connections, Tunnels: fixture.service, Stdout: &stdout, Stderr: &stderr})
			root.SetArgs([]string{"tunnel", mode, connection.Path, "--listen-port=" + strconv.Itoa(int(listen.Port)), "--destination=127.0.0.1:80", "--non-interactive"})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := root.ExecuteContext(ctx)
			if cli.ExitCode(err) != cli.ExitTransport {
				t.Fatalf("err=%v exit=%d", err, cli.ExitCode(err))
			}
			if err := cli.WriteError(&stderr, false, err); err != nil {
				t.Fatal(err)
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), listen.String()) || !strings.Contains(stderr.String(), "server forwarding policy before retrying") || strings.Contains(stderr.String(), "Active:") {
				t.Fatalf("streams=%q/%q", stdout.String(), stderr.String())
			}
		})
	}
}
