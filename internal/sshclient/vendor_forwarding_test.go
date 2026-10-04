package sshclient

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
)

type forwardingWriteGate struct {
	net.Conn
	blocked   atomic.Bool
	started   chan struct{}
	once      sync.Once
	closed    chan struct{}
	closeOnce sync.Once
}

func (g *forwardingWriteGate) Write(p []byte) (int, error) {
	if g.blocked.Load() {
		g.once.Do(func() { close(g.started) })
		<-g.closed
		return 0, net.ErrClosed
	}
	return g.Conn.Write(p)
}
func (g *forwardingWriteGate) Close() error {
	g.closeOnce.Do(func() { close(g.closed) })
	return g.Conn.Close()
}

type observedForwardingWrites struct {
	net.Conn
	observe atomic.Bool
	writes  chan struct{}
}

func (c *observedForwardingWrites) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if c.observe.Load() {
		c.writes <- struct{}{}
	}
	return n, err
}

func TestVendorRemoteAcceptQueueDrainsAfterBlockedConfirmation(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverRaw := make(chan *observedForwardingWrites, 1)
	serverDone := make(chan struct{})
	trigger := make(chan struct{})
	var channels sync.WaitGroup
	go func() {
		defer close(serverDone)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		observed := &observedForwardingWrites{Conn: raw, writes: make(chan struct{}, 64)}
		serverRaw <- observed
		defer raw.Close()
		config := &ssh.ServerConfig{NoClientAuth: true}
		config.AddHostKey(signer)
		server, incoming, requests, err := ssh.NewServerConn(observed, config)
		if err != nil {
			return
		}
		defer server.Close()
		go func() {
			for ch := range incoming {
				_ = ch.Reject(ssh.Prohibited, "no sessions")
			}
		}()
		for req := range requests {
			switch req.Type {
			case "tcpip-forward":
				_ = req.Reply(true, nil)
				<-trigger
				for range 16 {
					channels.Add(1)
					go func() {
						defer channels.Done()
						payload := struct {
							Host       string
							Port       uint32
							Origin     string
							OriginPort uint32
						}{"127.0.0.1", 12345, "127.0.0.1", 54321}
						ch, requests, err := server.OpenChannel("forwarded-tcpip", ssh.Marshal(payload))
						if err == nil {
							go ssh.DiscardRequests(requests)
							_ = ch.Close()
						}
					}()
				}
			case "cancel-tcpip-forward": /* Deliberately withhold global reply. */
			}
		}
	}()
	var gate *forwardingWriteGate
	client := New(Options{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		raw, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		gate = &forwardingWriteGate{Conn: raw, started: make(chan struct{}), closed: make(chan struct{})}
		return gate, nil
	}})
	req := tunnelRequest(app.TunnelRemote)
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	req.Connection.Host = host
	p, _ := net.LookupPort("tcp", port)
	req.Connection.Port = uint16(p)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- client.RunTunnel(ctx, req) }()
	observed := <-serverRaw
	t.Cleanup(func() {
		cancel()
		_ = gate.Close()
		_ = observed.Close()
		_ = listener.Close()
		forwardingDoubleAwait(t, serverDone)
		channels.Wait()
	})
	forwardingDoubleAwait(t, ready)
	gate.blocked.Store(true)
	observed.observe.Store(true)
	close(trigger)
	forwardingDoubleAwait(t, gate.started)
	for range 16 {
		forwardingDoubleAwait(t, observed.writes)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("vendor accept queue stranded during raw-close convergence")
	}
}
