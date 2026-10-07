package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/credential"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestForwardingSecurityRealTrustBeforeRememberedSecret(t *testing.T) {
	for _, status := range []string{"known", "unknown", "changed", "revoked", "bad-password"} {
		t.Run(status, func(t *testing.T) {
			server := startForwardingServer(t, forwardingServerConfig{Password: "forwarding-password-canary"})
			known := ""
			if status == "revoked" {
				known = "@revoked " + knownhosts.Line([]string{knownhosts.Normalize(server.address)}, server.signer.PublicKey()) + "\n"
			}
			f := newForwardingAppFixture(t, known)
			connection := f.create(t, server, "security")
			if status == "known" || status == "bad-password" {
				f.trust(t, server)
			}
			if status == "changed" {
				other := startForwardingServer(t, forwardingServerConfig{})
				endpoint := forwardingEndpoint(t, server.address)
				_, err := f.trusted.TrustHost(context.Background(), app.TrustHostRequest{Host: app.PresentedHost{
					Endpoint: app.HostEndpoint{CanonicalHost: endpoint.Host, Port: endpoint.Port}, KeyAlgorithm: other.signer.PublicKey().Type(), PublicKey: other.signer.PublicKey().Marshal(), FingerprintSHA256: ssh.FingerprintSHA256(other.signer.PublicKey()),
				}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if status == "bad-password" {
				if err := f.store.Set(context.Background(), credential.Key{Scope: f.options.Scope, Reference: credential.Reference(connection.CredentialRef)}, []byte("wrong-password-canary")); err != nil {
					t.Fatal(err)
				}
			}
			before := len(f.store.Calls())
			address, _ := startForwardingDestination(t)
			config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
			snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config))
			if err != nil {
				t.Fatal(err)
			}
			if status == "known" {
				forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
				if err := forwardingExchange(forwardingDial(t, config, config.Destination), []byte("payload-history-canary")); err != nil {
					t.Fatal(err)
				}
				forwardingAppStop(t, f.service, snapshot.ID)
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err = f.service.Wait(ctx, snapshot.ID)
				cancel()
				var failure *app.SSHStartError
				if !errors.As(err, &failure) {
					t.Fatalf("missing classified failure: %v", err)
				}
				want := app.SSHFailureHostTrust
				if status == "bad-password" {
					want = app.SSHFailureAuthenticationDenied
				}
				if failure.Reason() != want {
					t.Fatalf("failure = %v, want %s", err, want)
				}
				forwardingAppState(t, f.service, snapshot.ID, app.TunnelFailed)
			}
			calls := f.store.Calls()[before:]
			gets := 0
			for _, call := range calls {
				if call.Operation == credential.OperationGet {
					gets++
				}
			}
			if status == "known" || status == "bad-password" {
				if gets != 1 {
					t.Fatalf("verified secret reads = %d", gets)
				}
			} else if gets != 0 {
				t.Fatalf("unverified host acquired %d secrets", gets)
			}
			encoded, marshalErr := json.Marshal(f.service.Snapshots())
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			for _, canary := range []string{"forwarding-password-canary", "wrong-password-canary", "payload-history-canary", connection.CredentialRef} {
				if strings.Contains(string(encoded)+fmt.Sprint(err), canary) {
					t.Fatalf("snapshot/error leaked %q", canary)
				}
			}
			forwardingPortReleased(t, config.Listen)
		})
	}
}

func TestForwardingSecurityUnverifiedRemoteScopeAndNoPersistence(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{RemoteBindHost: "127.0.0.1", Password: "scope-password-canary"})
	f := newForwardingAppFixture(t, "")
	f.trust(t, server)
	connection := f.create(t, server, "scope")
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelRemote, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
	config.Listen.Host = "127.0.0.2"
	baseline, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config))
	if err != nil {
		t.Fatal(err)
	}
	snapshot = forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
	id := server.WaitTransport(t)
	actual := server.RemoteAddress(id, config.Listen.String())
	if actual == "" || actual == config.Listen.String() {
		t.Fatalf("actual=%q requested=%s", actual, config.Listen)
	}
	if snapshot.Scope != "unverified" || snapshot.Config.Listen != config.Listen || !strings.Contains(snapshot.Warning, "unverified") {
		t.Fatalf("scope snapshot = %+v", snapshot)
	}
	actualConfig := config
	actualConfig.Listen = forwardingEndpoint(t, actual)
	if err := forwardingExchange(forwardingDial(t, actualConfig, config.Destination), []byte("scope-payload-canary")); err != nil {
		t.Fatal(err)
	}
	afterTraffic, _ := f.service.Get(snapshot.ID)
	if afterTraffic.Scope != "unverified" {
		t.Fatal("traffic upgraded requested remote scope")
	}
	forwardingAppStop(t, f.service, snapshot.ID)
	forwardingPortReleased(t, actualConfig.Listen)
	persisted, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseline, persisted) {
		t.Fatal("forwarding changed catalog bytes")
	}
	for _, canary := range []string{"scope-password-canary", "scope-payload-canary"} {
		if bytes.Contains(persisted, []byte(canary)) {
			t.Fatalf("persisted %s", canary)
		}
	}
	if err := f.service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := app.NewTunnelService(f.options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.Snapshots()) != 0 {
		t.Fatal("forwarding restored across manager restart")
	}
}

func TestForwardingSecurityRepeatedPeerFailuresBoundedAndRedacted(t *testing.T) {
	const canary = "peer-ansi-bidi-secret-canary"
	server := startForwardingServer(t, forwardingServerConfig{RejectMessage: "\x1b[31m\x00\r\n\u202e" + strings.Repeat(canary, 1024), ChannelRules: map[string]forwardingReplyRule{"direct-tcpip": {Deny: true}}})
	f := newForwardingAppFixture(t, "")
	f.trust(t, server)
	connection := f.create(t, server, "redaction")
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
	baseline, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config))
	if err != nil {
		t.Fatal(err)
	}
	forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
	for range 40 {
		conn := forwardingDial(t, config, config.Destination)
		_, _ = conn.Write([]byte("client-payload-canary"))
		var b [1]byte
		if _, err := conn.Read(b[:]); err == nil {
			t.Fatal("denied destination produced data")
		} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("denied client did not close")
		}
		_ = conn.Close()
	}
	latest, _ := f.service.Get(snapshot.ID)
	if latest.State != app.TunnelActive || latest.Diagnostic == "" || latest.Version > 3 || len([]rune(latest.Diagnostic)) > 256 || len(f.service.Snapshots()) != 1 {
		t.Fatalf("unbounded/repeated failure state = %+v", latest)
	}
	for _, r := range latest.Diagnostic {
		if unicode.IsControl(r) || r == '\u202e' {
			t.Fatal("unsafe diagnostic character")
		}
	}
	encoded, err := json.Marshal(f.service.Snapshots())
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{canary, "client-payload-canary", connection.CredentialRef, server.config.Password} {
		if strings.Contains(string(encoded), value) {
			t.Fatalf("snapshot leaked %q", value)
		}
	}
	forwardingAppStop(t, f.service, snapshot.ID)
	forwardingPortReleased(t, config.Listen)
	persisted, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseline, persisted) {
		t.Fatal("peer failures mutated catalog")
	}
}

func TestForwardingSecurityDynamicNoDestinationHistory(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{})
	f := newForwardingAppFixture(t, "")
	f.trust(t, server)
	connection := f.create(t, server, "proxy")
	config := app.TunnelConfig{Mode: app.TunnelDynamic, Listen: forwardingFreeEndpoint(t)}
	snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config))
	if err != nil {
		t.Fatal(err)
	}
	forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
	for range 3 {
		address, closeService := startForwardingDestination(t)
		destination := forwardingEndpoint(t, address)
		destination.Host = "localhost"
		if err := forwardingExchange(forwardingDial(t, config, destination), []byte("dynamic-history-canary")); err != nil {
			t.Fatal(err)
		}
		closeService()
	}
	latest, _ := f.service.Get(snapshot.ID)
	encoded, err := json.Marshal(latest)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Config.Destination != (app.TunnelEndpoint{}) || strings.Contains(string(encoded), "localhost") || strings.Contains(string(encoded), "dynamic-history-canary") || !strings.Contains(latest.Warning, "not authenticated") {
		t.Fatalf("dynamic history/exposure: %s", encoded)
	}
	forwardingAppStop(t, f.service, snapshot.ID)
	forwardingPortReleased(t, config.Listen)
}

func TestForwardingSecurityInteractiveTrustAndExposureGates(t *testing.T) {
	for _, decision := range []app.TrustDecision{app.TrustOnce, app.TrustReject} {
		t.Run(string(decision), func(t *testing.T) {
			server := startForwardingServer(t, forwardingServerConfig{})
			f := newForwardingAppFixture(t, "")
			endpoint := forwardingEndpoint(t, server.address)
			created, err := f.connections.Create(context.Background(), app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "interactive", Host: endpoint.Host, Port: endpoint.Port, Username: "tester", AuthMethod: app.AuthMethodPassword})
			if err != nil {
				t.Fatal(err)
			}
			address, _ := startForwardingDestination(t)
			config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
			request := forwardingAppRequest(created.Connection, config)
			request.NonInteractive = false
			trusted, prompted := false, 0
			request.DecideTrust = func(_ context.Context, prompt app.TrustDecisionPrompt) (app.TrustDecision, error) {
				if prompt.Host.FingerprintSHA256 != ssh.FingerprintSHA256(server.signer.PublicKey()) {
					return app.TrustReject, errors.New("wrong presented key")
				}
				trusted = decision == app.TrustOnce
				return decision, nil
			}
			request.ReadSecret = func(context.Context, app.SecretRequest) ([]byte, error) {
				if !trusted {
					return nil, errors.New("secret requested before trust approval")
				}
				prompted++
				return []byte(server.config.Password), nil
			}
			snapshot, err := f.service.Start(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if decision == app.TrustOnce {
				forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
				forwardingAppStop(t, f.service, snapshot.ID)
				if prompted != 1 {
					t.Fatalf("secret prompts = %d", prompted)
				}
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err = f.service.Wait(ctx, snapshot.ID)
				cancel()
				if err == nil || prompted != 0 {
					t.Fatalf("rejected trust error=%v prompts=%d", err, prompted)
				}
			}
			before := len(f.store.Calls())
			request.Config.Listen.Host = "0.0.0.0"
			if _, err := f.service.Start(context.Background(), request); !errors.Is(err, app.ErrInvalidRequest) {
				t.Fatalf("unacknowledged exposure accepted: %v", err)
			}
			if len(f.store.Calls()) != before {
				t.Fatal("exposure validation accessed credentials")
			}
			forwardingPortReleased(t, config.Listen)
		})
	}
}

func TestLocalForwardingStartupTrustCancellationRemainsStopped(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{})
	f := newForwardingAppFixture(t, "")
	connection := f.create(t, server, "cancel-trust")
	before := len(f.store.Calls())
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
	request := forwardingAppRequest(connection, config)
	request.NonInteractive = false
	seen, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	request.DecideTrust = func(context.Context, app.TrustDecisionPrompt) (app.TrustDecision, error) {
		close(seen)
		<-release
		return app.TrustOnce, nil
	}
	snapshot, err := f.service.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	forwardingFixtureWait(t, seen)
	stopped := make(chan error, 1)
	go func() { stopped <- f.service.Stop(context.Background(), snapshot.ID) }()
	forwardingAppState(t, f.service, snapshot.ID, app.TunnelStopping)
	// A late affirmative decision cannot revive a canceled startup or unlock
	// remembered credentials. Stop still owns joining the callback/handshake.
	close(release)
	if err := forwardingFixtureWait(t, stopped); err != nil {
		t.Fatal(err)
	}
	forwardingAppState(t, f.service, snapshot.ID, app.TunnelStopped)
	if len(f.store.Calls()) != before {
		t.Fatal("canceled verification acquired credentials")
	}
	forwardingPortReleased(t, config.Listen)
}
