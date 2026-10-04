package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/sshclient"
	"golang.org/x/crypto/ssh"
)

func forwardingEndpoint(t *testing.T, address string) app.TunnelEndpoint {
	t.Helper()
	host, text, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return app.TunnelEndpoint{Host: host, Port: uint16(port)}
}

func forwardingFreeEndpoint(t *testing.T) app.TunnelEndpoint {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := forwardingEndpoint(t, listener.Addr().String())
	_ = listener.Close()
	return endpoint
}

func forwardingConnection(t *testing.T, server *forwardingTestServer) app.Connection {
	e := forwardingEndpoint(t, server.address)
	return app.Connection{Host: e.Host, Port: e.Port, Username: "tester", AuthMethod: app.AuthMethodPassword}
}

type forwardingRun struct {
	cancel   context.CancelFunc
	done     chan error
	finished chan struct{}
	ready    chan struct{}
	config   app.TunnelConfig
}

func startForwardingRun(t *testing.T, server *forwardingTestServer, config app.TunnelConfig, diagnostics ...func(string)) *forwardingRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &forwardingRun{cancel: cancel, done: make(chan error, 1), finished: make(chan struct{}), ready: make(chan struct{}), config: config}
	client := sshclient.New(sshclient.Options{})
	connection := forwardingConnection(t, server)
	var diagnostic func(string)
	if len(diagnostics) != 0 {
		diagnostic = diagnostics[0]
	}
	go func() {
		defer close(run.finished)
		run.done <- client.RunTunnel(ctx, app.TunnelRunRequest{
			Connection: connection, Config: config, Diagnostic: diagnostic,
			VerifyHost: func(_ context.Context, host app.PresentedHost) error {
				if host.FingerprintSHA256 != ssh.FingerprintSHA256(server.signer.PublicKey()) {
					return errors.New("wrong fixture identity")
				}
				return nil
			},
			Secret: func(context.Context, app.SecretRequest) ([]byte, error) { return []byte(server.config.Password), nil },
			Ready:  func() { close(run.ready) },
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-run.finished:
		case <-time.After(2 * time.Second):
			t.Error("forwarding runtime did not clean up")
		}
	})
	return run
}

func (r *forwardingRun) waitReady(t *testing.T) {
	t.Helper()
	select {
	case <-r.ready:
	case err := <-r.done:
		t.Fatalf("tunnel startup: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("tunnel did not become ready")
	}
}

func (r *forwardingRun) stop(t *testing.T) {
	t.Helper()
	r.cancel()
	if err := forwardingFixtureWait(t, r.done); !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped tunnel = %v", err)
	}
}

func forwardingPortReleased(t *testing.T, endpoint app.TunnelEndpoint) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		listener, err := net.Listen("tcp", endpoint.String())
		if err == nil {
			_ = listener.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("port %s not released: %v", endpoint, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func forwardingDial(t *testing.T, config app.TunnelConfig, destination app.TunnelEndpoint) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", config.Listen.String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if config.Mode == app.TunnelDynamic {
		if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
			t.Fatal(err)
		}
		var method [2]byte
		if _, err := io.ReadFull(conn, method[:]); err != nil || method != [2]byte{5, 0} {
			t.Fatalf("SOCKS method = %v, %v", method, err)
		}
		// Keep domain names unresolved until the SSH destination opener.
		host := destination.Host
		request := append([]byte{5, 1, 0, 3, byte(len(host))}, []byte(host)...)
		request = append(request, byte(destination.Port>>8), byte(destination.Port))
		if _, err := conn.Write(request); err != nil {
			t.Fatal(err)
		}
		var reply [10]byte
		if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[0] != 5 || reply[1] != 0 {
			t.Fatalf("SOCKS CONNECT = %v, %v", reply, err)
		}
	}
	return conn
}

func forwardingExchange(conn net.Conn, payload []byte) error {
	written := make(chan error, 1)
	go func() {
		_, err := conn.Write(payload)
		if err == nil {
			err = conn.(interface{ CloseWrite() error }).CloseWrite()
		}
		written <- err
	}()
	response, err := io.ReadAll(conn)
	if writeErr := <-written; err == nil {
		err = writeErr
	}
	if err == nil && !bytes.Equal(response, append(bytes.Clone(payload), []byte("after-eof")...)) {
		err = io.ErrUnexpectedEOF
	}
	_ = conn.Close()
	return err
}

func testForwardingModeMatrix(t *testing.T, mode app.TunnelMode) {
	address, _ := startForwardingDestination(t)
	destination := forwardingEndpoint(t, address)
	destination.Host = "localhost"
	server := startForwardingServer(t, forwardingServerConfig{ChannelRules: map[string]forwardingReplyRule{"session": {Deny: true}}})
	runs := make([]*forwardingRun, 2)
	for i := range runs {
		config := app.TunnelConfig{Mode: mode, Listen: forwardingFreeEndpoint(t), Destination: destination}
		if mode == app.TunnelDynamic {
			config.Destination = app.TunnelEndpoint{}
		}
		runs[i] = startForwardingRun(t, server, config)
		runs[i].waitReady(t)
		id := server.WaitTransport(t)
		t.Cleanup(func() {
			if count := server.ChannelCount(id, "session"); count != 0 {
				t.Errorf("tunnel opened %d session channels", count)
			}
			if mode == app.TunnelRemote && server.ChannelCount(id, "direct-tcpip") != 0 {
				t.Error("remote mode dialed destination on host side")
			}
			if mode == app.TunnelRemote && server.ChannelCount(id, "forwarded-tcpip") < 2 {
				t.Error("remote clients did not use forwarded-tcpip")
			}
			if mode != app.TunnelRemote && server.ChannelCount(id, "direct-tcpip") < 2 {
				t.Error("local/proxy clients did not use direct-tcpip")
			}
		})
	}
	var wait sync.WaitGroup
	results := make(chan error, 4)
	for _, run := range runs {
		for range 2 {
			conn := forwardingDial(t, run.config, destination)
			wait.Add(1)
			go func() {
				defer wait.Done()
				results <- forwardingExchange(conn, bytes.Repeat([]byte{0, 255, 128, 13, 10, 42}, 32768))
			}()
		}
	}
	for range 4 {
		if err := forwardingFixtureWait(t, results); err != nil {
			t.Fatal(err)
		}
	}
	wait.Wait()
	// Stop with an admitted idle client, not only after clean client EOF.
	idle := forwardingDial(t, runs[0].config, destination)
	runs[0].stop(t)
	_ = idle.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, err := idle.Read(b[:]); err == nil {
		t.Fatal("stopped client remained readable")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("stopped client did not close")
	}
	forwardingPortReleased(t, runs[0].config.Listen)
	if err := forwardingExchange(forwardingDial(t, runs[1].config, destination), []byte("surviving-tunnel")); err != nil {
		t.Fatal(err)
	}
	runs[1].stop(t)
	forwardingPortReleased(t, runs[1].config.Listen)
}

func TestLocalForwardingTwoTunnelsTwoClients(t *testing.T) {
	testForwardingModeMatrix(t, app.TunnelLocal)
}

func TestLocalForwardingPendingOpenCancellationAndOccupiedPort(t *testing.T) {
	seen := make(chan struct{}, 1)
	server := startForwardingServer(t, forwardingServerConfig{ChannelRules: map[string]forwardingReplyRule{"direct-tcpip": {Absent: true, Seen: seen}}})
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
	run := startForwardingRun(t, server, config)
	run.waitReady(t)
	conn := forwardingDial(t, config, config.Destination)
	forwardingFixtureWait(t, seen)
	run.stop(t)
	_ = conn.Close()
	forwardingPortReleased(t, config.Listen)
	occupied, err := net.Listen("tcp", config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	run = startForwardingRun(t, server, config)
	if err := forwardingFixtureWait(t, run.done); err == nil {
		t.Fatal("occupied port accepted")
	}
	select {
	case <-run.ready:
		t.Fatal("occupied tunnel became ready")
	default:
	}
}

func TestLocalForwardingHostSideDNSAndDestinationRecovery(t *testing.T) {
	address, closeService := startForwardingDestination(t)
	destination := forwardingEndpoint(t, address)
	destination.Host = "host-side-only.invalid"
	server := startForwardingServer(t, forwardingServerConfig{DestinationAliases: map[string]string{destination.Host: "127.0.0.1"}})
	config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: destination}
	run := startForwardingRun(t, server, config)
	run.waitReady(t)
	id := server.WaitTransport(t)
	if err := forwardingExchange(forwardingDial(t, config, destination), []byte("host-resolved-domain")); err != nil {
		t.Fatal(err)
	}
	closeService()
	conn := forwardingDial(t, config, destination)
	var b [1]byte
	if _, err := conn.Read(b[:]); err == nil {
		t.Fatal("unavailable destination remained open")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("unavailable destination client did not close")
	}
	_ = conn.Close()
	_, closeReplacement := startForwardingDestination(t, address)
	defer closeReplacement()
	if err := forwardingExchange(forwardingDial(t, config, destination), []byte("same-tunnel-recovered")); err != nil {
		t.Fatal(err)
	}
	if server.ChannelCount(id, "direct-tcpip") != 3 || server.ChannelCount(id, "session") != 0 {
		t.Fatal("wrong local channel protocol")
	}
	run.stop(t)
	forwardingPortReleased(t, config.Listen)
}

func TestLocalForwardingCanceledDelayedOpenCannotReactivate(t *testing.T) {
	seen := make(chan struct{}, 1)
	release := make(chan struct{})
	server := startForwardingServer(t, forwardingServerConfig{ChannelRules: map[string]forwardingReplyRule{"direct-tcpip": {Release: release, Seen: seen}}})
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
	run := startForwardingRun(t, server, config)
	run.waitReady(t)
	id := server.WaitTransport(t)
	conn := forwardingDial(t, config, config.Destination)
	forwardingFixtureWait(t, seen)
	run.cancel()
	// Allow the real channel open to succeed after owner cancellation. Its late
	// result must still belong to the canceled runtime and be closed before return.
	close(release)
	if err := forwardingFixtureWait(t, run.done); !errors.Is(err, context.Canceled) {
		t.Fatalf("late open result = %v", err)
	}
	_ = conn.Close()
	server.CloseTransport(id)
	if server.AcceptedChannelCount(id, "direct-tcpip") != 1 {
		t.Fatal("delayed channel did not actually succeed after cancellation")
	}
	forwardingPortReleased(t, config.Listen)
}
