package sshclient

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestDynamicNegotiationDeadlineDoesNotLimitEstablishedIdle(t *testing.T) {
	t.Parallel()
	service, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := service.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(14 * time.Second))
		data, _ := io.ReadAll(conn)
		_, _ = conn.Write(data)
	}()
	f := &socketTunnelTransport{scope: newForwardingDoubleScope(), dial: func(string) (net.Conn, error) { return net.Dial("tcp", service.Addr().String()) }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelDynamic)
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- socketTunnelClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, ready)
	healthy, err := net.Dial("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer healthy.Close()
	_ = healthy.SetDeadline(time.Now().Add(14 * time.Second))
	_, _ = healthy.Write([]byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80})
	var reply [12]byte
	if _, err := io.ReadFull(healthy, reply[:]); err != nil || reply[3] != 0 {
		t.Fatalf("reply=%v err=%v", reply, err)
	}
	stalled, err := net.Dial("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer stalled.Close()
	_ = stalled.SetReadDeadline(time.Now().Add(12 * time.Second))
	var b [1]byte
	start := time.Now()
	if _, err := stalled.Read(b[:]); err != io.EOF {
		t.Fatalf("negotiation did not close: %v", err)
	}
	if time.Since(start) < 9*time.Second {
		t.Fatal("negotiation closed prematurely")
	}
	_, _ = healthy.Write([]byte("still alive"))
	_ = healthy.(*net.TCPConn).CloseWrite()
	data, err := io.ReadAll(healthy)
	if err != nil || string(data) != "still alive" {
		t.Fatalf("healthy idle was limited: %q %v", data, err)
	}
	cancel()
	tunnelResult(t, done)
	forwardingDoubleAwait(t, serverDone)
}

func TestRemoteListenHasNoDestinationStartupDeadline(t *testing.T) {
	t.Parallel()
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, tunnelRequest(app.TunnelRemote)) }()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	timer := time.NewTimer(10100 * time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-done:
		t.Fatalf("fixed startup deadline: %v", err)
	case <-timer.C:
	}
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatal("startup worker leaked")
	}
}

func TestDynamicSuccessWaitsForDestination(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelDynamic)
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, ready)
	conn, err := net.Dial("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80})
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, f.DialOp.Started)
	_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	var reply [10]byte
	if _, err := conn.Read(reply[:]); err == nil {
		t.Fatal("premature proxy success")
	}
	destination := newForwardingDoubleConn(f.forwardingDoubleScope)
	if err := f.DialOp.Reply(ctx, destination, nil); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[1] != 0 {
		t.Fatalf("reply=%v err=%v", reply, err)
	}
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatal("proxy workers leaked")
	}
}
