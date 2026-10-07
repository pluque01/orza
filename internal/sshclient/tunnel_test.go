package sshclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
)

type tunnelTestTransport struct{ *forwardingDoubleTransport }

type admittedTestTransport struct {
	*tunnelTestTransport
	entered chan struct{}
}

func (t *admittedTestTransport) Dial(network, address string) (net.Conn, error) {
	t.entered <- struct{}{}
	return t.forwardingDoubleTransport.Dial(network, address)
}

func (t *tunnelTestTransport) NewSession() (remoteSession, error) { panic("tunnel opened session") }

type tunnelRaw struct {
	net.Conn
	scope *forwardingDoubleScope
}

func (r *tunnelRaw) Close() error { return r.scope.RawClose() }
func tunnelTestClient(f *forwardingDoubleTransport) *Client {
	return New(Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return &tunnelRaw{scope: f.forwardingDoubleScope}, nil
	}, Handshake: func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) {
		return &tunnelTestTransport{f}, nil
	}})
}
func tunnelRequest(mode app.TunnelMode) app.TunnelRunRequest {
	return app.TunnelRunRequest{Connection: app.Connection{Host: "host", Port: 22, Username: "user"}, VerifyHost: func(context.Context, app.PresentedHost) error { return nil }, Config: app.TunnelConfig{Mode: mode, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 12345}, Destination: app.TunnelEndpoint{Host: "destination.example", Port: 80}}}
}
func TestTunnelPendingRemoteListenCancellation(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := false
	req := tunnelRequest(app.TunnelRemote)
	req.Ready = func() { ready = true }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("result: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending listen leaked")
	}
	if ready || f.Active() != 0 {
		t.Fatalf("ready=%v workers=%d", ready, f.Active())
	}
}
func TestTunnelRemoteBlockedCleanup(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelRemote)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	l := newForwardingDoubleListener(f.forwardingDoubleScope)
	if err := f.ListenOp.Reply(ctx, l, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, ready)
	forwardingDoubleAwait(t, l.AcceptOp.Started)
	cancel()
	forwardingDoubleAwait(t, l.CloseOp.Started)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked close defeated watchdog")
	}
	if f.Active() != 0 {
		t.Fatalf("workers=%d", f.Active())
	}
}
func TestSOCKSRequest(t *testing.T) {
	for _, tc := range []struct {
		data    []byte
		address string
		code    byte
	}{
		{[]byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80}, "127.0.0.1:80", 0},
		{append([]byte{5, 1, 0, 5, 1, 0, 3, 3}, []byte{'f', 'o', 'o', 1, 187}...), "foo:443", 0},
		{[]byte{5, 1, 0, 5, 2, 0, 1}, "", 7},
		{[]byte{5, 1, 0, 5, 1, 0, 9}, "", 8},
		{[]byte{5, 1, 0, 5, 1, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 255, 255}, "[::1]:65535", 0},
		{[]byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 0}, "", 1},
		{[]byte{5, 1, 0, 5, 3, 0, 1}, "", 7},
		{[]byte{5, 1, 0, 5, 1, 1, 1}, "", 1},
	} {
		r := bytes.NewBuffer(append(tc.data, []byte("payload")...))
		var w bytes.Buffer
		address, code, err := socksRequest(r, &w)
		if address != tc.address || code != tc.code || (err != nil) != (tc.code != 0) {
			t.Fatalf("%v: %q %d %v", tc.data, address, code, err)
		}
		if tc.code == 0 {
			rest, _ := io.ReadAll(r)
			if string(rest) != "payload" {
				t.Fatal("lost pipeline")
			}
		}
	}
}

func TestTunnel64PendingOpensRetainedUntilCancellation(t *testing.T) {
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	client := tunnelTestClient(f)
	entered := make(chan struct{})
	client.options.Handshake = func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) {
		return &admittedTestTransport{&tunnelTestTransport{f}, entered}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelLocal)
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- client.RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, ready)
	for range 64 {
		conn, err := net.Dial("tcp", req.Config.Listen.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		forwardingDoubleAwait(t, entered)
	}
	rejected, err := net.Dial("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer rejected.Close()
	_ = rejected.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, err := rejected.Read(b[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("overflow not rejected: %v", err)
	}
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatalf("workers=%d", f.Active())
	}
	if f.DialOp.Calls.Load() != 64 {
		t.Fatalf("pending opens=%d", f.DialOp.Calls.Load())
	}
}

func TestTunnelTrustBeforeAuthentication(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := tunnelTestClient(f)
	verified := false
	client.options.Authenticate = func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
		if !verified {
			t.Error("authentication before trust")
		}
		return []ssh.AuthMethod{ssh.Password("test")}, nil, nil
	}
	client.options.Handshake = func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
		if err := config.HostKeyCallback("", nil, testPublicKey(t)); err != nil {
			return nil, err
		}
		if _, err := config.AuthCallback(new(ssh.ClientAuthContext)); err != nil {
			return nil, err
		}
		return &tunnelTestTransport{f}, nil
	}
	req := tunnelRequest(app.TunnelLocal)
	req.Config.Listen = unusedTunnelEndpoint(t)
	req.VerifyHost = func(context.Context, app.PresentedHost) error { verified = true; return nil }
	req.Ready = cancel
	if err := client.RunTunnel(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("result=%v", err)
	}
	if !verified {
		t.Fatal("trust not checked")
	}
}
func FuzzSOCKSRequest(f *testing.F) {
	f.Add([]byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80})
	f.Add([]byte{5, 1, 0, 5, 1, 0, 3, 3, 'f', 'o', 'o', 1, 187})
	f.Add([]byte{5, 1, 2})
	f.Add([]byte{5, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		var w bytes.Buffer
		_, _, _ = socksRequest(bytes.NewReader(data), &w)
		if w.Len() > 12 {
			t.Fatal("unbounded reply")
		}
	})
}

func TestSOCKSUnsupportedMethodsAndTruncation(t *testing.T) {
	var response bytes.Buffer
	_, code, err := socksRequest(bytes.NewReader([]byte{5, 2, 1, 2}), &response)
	if err == nil || code != 0 || !bytes.Equal(response.Bytes(), []byte{5, 255}) {
		t.Fatalf("unsupported methods: code=%d err=%v reply=%v", code, err, response.Bytes())
	}
	valid := []byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80}
	for length := 0; length < len(valid); length++ {
		response.Reset()
		if _, _, err := socksRequest(bytes.NewReader(valid[:length]), &response); err == nil {
			t.Fatalf("accepted truncation at %d", length)
		}
	}
	response.Reset()
	if err := socksReply(&response, 0); err != nil || !bytes.Equal(response.Bytes(), []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}) {
		t.Fatal("invalid neutral success reply")
	}
}

func TestTunnelCanceledHandshakeOwnsRawSocket(t *testing.T) {
	f := newForwardingDoubleTransport()
	client := tunnelTestClient(f)
	started := make(chan struct{})
	client.options.Handshake = func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) {
		close(started)
		<-f.done
		return nil, net.ErrClosed
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.RunTunnel(ctx, tunnelRequest(app.TunnelLocal)) }()
	forwardingDoubleAwait(t, started)
	cancel()
	tunnelResult(t, done)
	forwardingDoubleAwait(t, f.done)
}

func TestTunnelRejectedTrustNeverAuthenticates(t *testing.T) {
	f := newForwardingDoubleTransport()
	client := tunnelTestClient(f)
	client.options.Authenticate = func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
		t.Fatal("authentication after rejected trust")
		return nil, nil, nil
	}
	client.options.Handshake = func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
		return nil, config.HostKeyCallback("", nil, testPublicKey(t))
	}
	req := tunnelRequest(app.TunnelLocal)
	req.VerifyHost = func(context.Context, app.PresentedHost) error { return app.ErrHostRevoked }
	req.Ready = func() { t.Fatal("ready after trust failure") }
	if err := client.RunTunnel(context.Background(), req); !errors.Is(err, app.ErrHostRevoked) {
		t.Fatalf("result=%v", err)
	}
	forwardingDoubleAwait(t, f.done)
}
