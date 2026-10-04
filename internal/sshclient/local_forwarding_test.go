package sshclient

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
)

type socketTunnelTransport struct {
	scope  *forwardingDoubleScope
	dial   func(string) (net.Conn, error)
	listen func(string) (net.Listener, error)
}

func (f *socketTunnelTransport) NewSession() (remoteSession, error)       { panic("session acquisition") }
func (f *socketTunnelTransport) Dial(_, address string) (net.Conn, error) { return f.dial(address) }
func (f *socketTunnelTransport) Listen(_, address string) (net.Listener, error) {
	return f.listen(address)
}
func (f *socketTunnelTransport) Wait() error  { <-f.scope.done; return net.ErrClosed }
func (f *socketTunnelTransport) Close() error { return f.scope.RawClose() }
func socketTunnelClient(f *socketTunnelTransport) *Client {
	return New(Options{DialContext: func(context.Context, string, string) (net.Conn, error) { return &tunnelRaw{scope: f.scope}, nil }, Handshake: func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) { return f, nil }})
}
func unusedTunnelEndpoint(t *testing.T) app.TunnelEndpoint {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().(*net.TCPAddr)
	_ = l.Close()
	return app.TunnelEndpoint{Host: "127.0.0.1", Port: uint16(address.Port)}
}
func tunnelResult(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("result=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel failed to join")
	}
}

func TestTunnelModesTwoClientsHalfCloseAndIndependentStop(t *testing.T) {
	for _, mode := range []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic} {
		t.Run(string(mode), func(t *testing.T) {
			service, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			var services sync.WaitGroup
			services.Add(1)
			go func() {
				defer services.Done()
				for {
					conn, err := service.Accept()
					if err != nil {
						return
					}
					services.Add(1)
					go func() {
						defer services.Done()
						defer conn.Close()
						data, _ := io.ReadAll(conn)
						_, _ = conn.Write(append([]byte("response:"), data...))
					}()
				}
			}()
			defer services.Wait()
			defer service.Close()
			stops := make([]context.CancelFunc, 2)
			results := make([]chan error, 2)
			requests := make([]app.TunnelRunRequest, 2)
			for i := range 2 {
				req := tunnelRequest(mode)
				req.Config.Listen = unusedTunnelEndpoint(t)
				requests[i] = req
				if mode == app.TunnelRemote {
					host, port, _ := net.SplitHostPort(service.Addr().String())
					p, _ := strconv.Atoi(port)
					req.Config.Destination = app.TunnelEndpoint{Host: host, Port: uint16(p)}
				}
				f := &socketTunnelTransport{scope: newForwardingDoubleScope()}
				f.dial = func(address string) (net.Conn, error) {
					if address != "destination.example:80" {
						t.Errorf("destination not passed unresolved: %q", address)
					}
					return net.Dial("tcp", service.Addr().String())
				}
				f.listen = func(address string) (net.Listener, error) {
					if address != req.Config.Listen.String() {
						t.Errorf("requested remote endpoint changed: %q", address)
					}
					return net.Listen("tcp", address)
				}
				ctx, cancel := context.WithCancel(context.Background())
				stops[i] = cancel
				t.Cleanup(cancel)
				ready := make(chan struct{})
				req.Ready = func() { close(ready) }
				results[i] = make(chan error, 1)
				go func() { results[i] <- socketTunnelClient(f).RunTunnel(ctx, req) }()
				forwardingDoubleAwait(t, ready)
			}
			exchange := func(index int) {
				t.Helper()
				var clients sync.WaitGroup
				for range 2 {
					clients.Add(1)
					go func() {
						defer clients.Done()
						conn, err := net.Dial("tcp", requests[index].Config.Listen.String())
						if err != nil {
							t.Error(err)
							return
						}
						defer conn.Close()
						_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
						if mode == app.TunnelDynamic {
							_, _ = conn.Write(append([]byte{5, 1, 0, 5, 1, 0, 3, 19}, []byte("destination.example\x00\x50payload")...))
							var reply [12]byte
							if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[1] != 0 || reply[3] != 0 {
								t.Errorf("SOCKS reply=%v err=%v", reply, err)
								return
							}
						} else {
							_, _ = conn.Write([]byte("payload"))
						}
						_ = conn.(*net.TCPConn).CloseWrite()
						data, err := io.ReadAll(conn)
						if err != nil || string(data) != "response:payload" {
							t.Errorf("data=%q err=%v", data, err)
						}
					}()
				}
				clients.Wait()
			}
			exchange(0)
			exchange(1)
			stops[0]()
			tunnelResult(t, results[0])
			exchange(1)
			stops[1]()
			tunnelResult(t, results[1])
			for _, req := range requests {
				l, err := net.Listen("tcp", req.Config.Listen.String())
				if err != nil {
					t.Errorf("port not reusable: %v", err)
				} else {
					_ = l.Close()
				}
			}
		})
	}
}

func TestTunnelLocalRefusalDoesNotKillListener(t *testing.T) {
	f := newForwardingDoubleTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := tunnelRequest(app.TunnelLocal)
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	diagnostic := make(chan string, 1)
	req.Diagnostic = func(text string) {
		select {
		case diagnostic <- text:
		default:
		}
	}
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, ready)
	for range 2 {
		conn, err := net.Dial("tcp", req.Config.Listen.String())
		if err != nil {
			t.Fatal(err)
		}
		if err := f.DialOp.Reply(ctx, nil, errors.New("secret canary\x1b[31m")); err != nil {
			t.Fatal(err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var b [1]byte
		if _, err := conn.Read(b[:]); !errors.Is(err, io.EOF) {
			t.Errorf("refused client not closed: %v", err)
		}
		_ = conn.Close()
	}
	select {
	case text := <-diagnostic:
		if text != "Forwarding destination unavailable." {
			t.Fatalf("unsafe diagnostic: %q", text)
		}
	default:
		t.Fatal("no diagnostic")
	}
	cancel()
	tunnelResult(t, done)
	if f.Active() != 0 {
		t.Fatal("workers leaked")
	}
}

func TestTunnelLocalConflictAndTransportLoss(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	req := tunnelRequest(app.TunnelLocal)
	req.Config.Listen.Port = uint16(occupied.Addr().(*net.TCPAddr).Port)
	req.Ready = func() { t.Fatal("ready for occupied port") }
	f := newForwardingDoubleTransport()
	if err := tunnelTestClient(f).RunTunnel(context.Background(), req); err == nil {
		t.Fatal("occupied listener accepted")
	}
	if f.Active() != 0 {
		t.Fatal("conflict leaked workers")
	}
	f = newForwardingDoubleTransport()
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done := make(chan error, 1)
	go func() { done <- tunnelTestClient(f).RunTunnel(context.Background(), req) }()
	forwardingDoubleAwait(t, ready)
	if err := f.WaitOp.Reply(context.Background(), struct{}{}, net.ErrClosed); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, errTunnel) {
			t.Fatalf("result=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("transport loss did not join")
	}
	listener, err := net.Listen("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	_ = listener.Close()
	if f.Active() != 0 {
		t.Fatal("loss leaked workers")
	}
}
