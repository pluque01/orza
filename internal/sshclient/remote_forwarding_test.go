package sshclient

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestTunnelRemote64SlotsAndSerializedOverflowGrace(t *testing.T) {
	service, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelRemote)
	host, port, _ := net.SplitHostPort(service.Addr().String())
	p, _ := strconv.Atoi(port)
	req.Config.Destination = app.TunnelEndpoint{Host: host, Port: uint16(p)}
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	listener := newForwardingDoubleListener(f.forwardingDoubleScope)
	if err := f.ListenOp.Reply(ctx, listener, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, ready)
	for range 64 {
		conn := newForwardingDoubleConn(f.forwardingDoubleScope)
		if err := listener.AcceptOp.Reply(ctx, conn, nil); err != nil {
			t.Fatal(err)
		}
		forwardingDoubleAwait(t, conn.ReadOp.Started)
	}
	overflow := newForwardingDoubleConn(f.forwardingDoubleScope)
	start := time.Now()
	if err := listener.AcceptOp.Reply(ctx, overflow, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, overflow.CloseOp.Started)
	select {
	case <-f.done:
		t.Fatal("capacity alone forced transport close")
	default:
	}
	select {
	case err := <-done:
		if !errors.Is(err, errTunnel) {
			t.Fatalf("result=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("overflow cleanup did not converge")
	}
	if time.Since(start) < time.Second {
		t.Fatal("cleanup grace skipped")
	}
	if f.Active() != 0 {
		t.Fatalf("workers=%d", f.Active())
	}
}

func TestTunnelLocalPendingOpenTimeoutAndLateSuccess(t *testing.T) {
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelLocal)
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, ready)
	source, err := net.Dial("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	forwardingDoubleAwait(t, f.DialOp.Started)
	// The actual non-contextual open remains owned for its full ten-second wait.
	timer := time.NewTimer(10100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case err := <-done:
		t.Fatalf("premature timeout: %v", err)
	}
	late := newForwardingDoubleConn(f.forwardingDoubleScope)
	if err := f.DialOp.Reply(ctx, late, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, late.CloseOp.Started)
	if err := late.CloseOp.Reply(ctx, struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	_ = source.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	_, _ = source.Read(b[:])
	select {
	case err := <-done:
		t.Fatalf("late destination failure killed listener: %v", err)
	default:
	}
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatalf("workers=%d", f.Active())
	}
}

func TestTunnelRemoteRefusalIsControlled(t *testing.T) {
	f := newForwardingDoubleTransport()
	req := tunnelRequest(app.TunnelRemote)
	req.Ready = func() { t.Fatal("ready after remote refusal") }
	var diagnostic string
	req.Diagnostic = func(text string) { diagnostic = text }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(context.Background(), req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	if err := f.ListenOp.Reply(context.Background(), nil, errors.New("secret canary\x1b[31m")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || diagnostic != "Forwarding listener unavailable." {
			t.Fatalf("error=%v diagnostic=%q", err, diagnostic)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("refusal cleanup did not join")
	}
	if f.Active() != 0 {
		t.Fatal("refusal leaked workers")
	}
}

func TestTunnelRemoteLateListenerIsClosedWithoutReady(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelRemote)
	req.Ready = func() { t.Error("late listener became ready") }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	cancel()
	listener := newForwardingDoubleListener(f.forwardingDoubleScope)
	if err := f.ListenOp.Reply(context.Background(), listener, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, listener.CloseOp.Started)
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatal("late listener leaked workers")
	}
}

func TestTunnelRemoteDestinationRefusalKeepsAccepting(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelRemote)
	req.Config.Destination = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	listener := newForwardingDoubleListener(f.forwardingDoubleScope)
	if err := f.ListenOp.Reply(ctx, listener, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, ready)
	for range 2 {
		incoming := newForwardingDoubleConn(f.forwardingDoubleScope)
		if err := listener.AcceptOp.Reply(ctx, incoming, nil); err != nil {
			t.Fatal(err)
		}
		forwardingDoubleAwait(t, incoming.CloseOp.Started)
		if err := incoming.CloseOp.Reply(ctx, struct{}{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatal("refused destinations leaked workers")
	}
}

func TestTunnelRemoteCleanupRetainsAll64Slots(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelRemote)
	req.Config.Destination = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	capacity := make(chan struct{})
	req.Diagnostic = func(text string) {
		if text == "Forwarding client capacity reached." {
			close(capacity)
		}
	}
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	listener := newForwardingDoubleListener(f.forwardingDoubleScope)
	if err := f.ListenOp.Reply(ctx, listener, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, ready)
	for range 64 {
		incoming := newForwardingDoubleConn(f.forwardingDoubleScope)
		if err := listener.AcceptOp.Reply(ctx, incoming, nil); err != nil {
			t.Fatal(err)
		}
		forwardingDoubleAwait(t, incoming.CloseOp.Started)
	}
	overflow := newForwardingDoubleConn(f.forwardingDoubleScope)
	if err := listener.AcceptOp.Reply(ctx, overflow, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, capacity)
	forwardingDoubleAwait(t, overflow.CloseOp.Started)
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatal("pending cleanup workers leaked")
	}
}
