package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestRemoteForwardingTwoTunnelsTwoClients(t *testing.T) {
	testForwardingModeMatrix(t, app.TunnelRemote)
}
func TestDynamicForwardingTwoTunnelsTwoClients(t *testing.T) {
	testForwardingModeMatrix(t, app.TunnelDynamic)
}

func TestRemoteForwardingRefusedAndMissingGlobalReplies(t *testing.T) {
	for _, mode := range []string{"refused", "missing-start", "missing-cancel"} {
		t.Run(mode, func(t *testing.T) {
			seen := make(chan struct{}, 1)
			request := "tcpip-forward"
			if mode == "missing-cancel" {
				request = "cancel-tcpip-forward"
			}
			server := startForwardingServer(t, forwardingServerConfig{GlobalRules: map[string]forwardingReplyRule{request: {Deny: mode == "refused", Absent: mode != "refused", Seen: seen}}})
			address, _ := startForwardingDestination(t)
			config := app.TunnelConfig{Mode: app.TunnelRemote, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
			run := startForwardingRun(t, server, config)
			id := server.WaitTransport(t)
			if mode == "missing-cancel" {
				run.waitReady(t)
			} else {
				forwardingFixtureWait(t, seen)
			}
			if mode == "refused" {
				if err := forwardingFixtureWait(t, run.done); err == nil {
					t.Fatal("remote refusal succeeded")
				}
			} else {
				run.stop(t)
				if mode == "missing-cancel" {
					forwardingFixtureWait(t, seen)
				}
			}
			server.CloseTransport(id)
			forwardingPortReleased(t, config.Listen)
		})
	}
}

func TestRemoteForwardingVendorFloodAndWithheldClose(t *testing.T) {
	for _, withheld := range []bool{false, true} {
		name := "flood"
		if withheld {
			name = "withheld-close"
		}
		t.Run(name, func(t *testing.T) {
			server := startForwardingServer(t, forwardingServerConfig{})
			address, _ := startForwardingDestination(t)
			config := app.TunnelConfig{Mode: app.TunnelRemote, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
			capacity := make(chan struct{}, 1)
			run := startForwardingRun(t, server, config, func(text string) {
				if text == "Forwarding client capacity reached." {
					select {
					case capacity <- struct{}{}:
					default:
					}
				}
			})
			run.waitReady(t)
			id := server.WaitTransport(t)
			if withheld {
				conn := forwardingDial(t, config, config.Destination)
				if _, err := conn.Write([]byte("established")); err != nil {
					t.Fatal(err)
				}
				response := make([]byte, len("established"))
				if _, err := io.ReadFull(conn, response); err != nil {
					t.Fatal(err)
				}
				blocked := server.HoldTransportReads(id)
				_ = conn.(*net.TCPConn).CloseWrite()
				forwardingFixtureWait(t, blocked)
				run.stop(t)
				// The synthetic partition cannot observe client raw closure until
				// released. Explicit server teardown is not a scope/release guarantee.
				server.CloseTransport(id)
			} else {
				done := server.Flood(id, config.Listen.Host, config.Listen.Port, 96)
				forwardingFixtureWait(t, capacity)
				run.stop(t)
				server.CloseTransport(id)
				forwardingFixtureWait(t, done)
			}
			forwardingPortReleased(t, config.Listen)
			if server.ChannelCount(id, "session") != 0 {
				t.Fatal("remote forwarding opened a session")
			}
		})
	}
}

func TestRemoteDynamicForwardingDestinationFailureIsolation(t *testing.T) {
	for _, mode := range []app.TunnelMode{app.TunnelRemote, app.TunnelDynamic} {
		t.Run(string(mode), func(t *testing.T) {
			address, closeService := startForwardingDestination(t)
			destination := forwardingEndpoint(t, address)
			server := startForwardingServer(t, forwardingServerConfig{})
			config := app.TunnelConfig{Mode: mode, Listen: forwardingFreeEndpoint(t), Destination: destination}
			if mode == app.TunnelDynamic {
				config.Destination = app.TunnelEndpoint{}
			}
			run := startForwardingRun(t, server, config)
			run.waitReady(t)
			if mode == app.TunnelDynamic {
				bad := forwardingDial(t, config, destination)
				_ = bad.Close()
				// A malformed negotiation is an ordinary proxy-client failure.
				bad, err := net.DialTimeout("tcp", config.Listen.String(), time.Second)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = bad.Write([]byte{4, 1, 0})
				_ = bad.Close()
			}
			if err := forwardingExchange(forwardingDial(t, config, destination), []byte("healthy-before-refusal")); err != nil {
				t.Fatal(err)
			}
			closeService()
			// The runtime remains live when the fixed destination disappears.
			conn, err := net.DialTimeout("tcp", config.Listen.String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			if mode == app.TunnelRemote {
				var b [1]byte
				if _, err := conn.Read(b[:]); err == nil {
					t.Fatal("unavailable remote destination produced data")
				} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
					t.Fatal("remote destination refusal did not close client")
				}
			} else {
				if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
					t.Fatal(err)
				}
				var method [2]byte
				if _, err := io.ReadFull(conn, method[:]); err != nil {
					t.Fatal(err)
				}
				request := []byte{5, 1, 0, 1, 127, 0, 0, 1, byte(destination.Port >> 8), byte(destination.Port)}
				if _, err := conn.Write(request); err != nil {
					t.Fatal(err)
				}
				var reply [10]byte
				if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[1] != 1 {
					t.Fatalf("destination refusal reply = %v, %v", reply, err)
				}
			}
			_ = conn.Close()
			select {
			case err := <-run.done:
				t.Fatalf("client failure killed tunnel: %v", err)
			default:
			}
			_, closeReplacement := startForwardingDestination(t, address)
			defer closeReplacement()
			if err := forwardingExchange(forwardingDial(t, config, destination), []byte("recovered-after-refusal")); err != nil {
				t.Fatal(err)
			}
			run.stop(t)
			forwardingPortReleased(t, config.Listen)
		})
	}
}

func TestDynamicForwardingHostSideDNSAndPipelinedPayload(t *testing.T) {
	address, _ := startForwardingDestination(t)
	destination := forwardingEndpoint(t, address)
	destination.Host = "proxy-host-only.invalid"
	server := startForwardingServer(t, forwardingServerConfig{DestinationAliases: map[string]string{destination.Host: "127.0.0.1"}})
	config := app.TunnelConfig{Mode: app.TunnelDynamic, Listen: forwardingFreeEndpoint(t)}
	run := startForwardingRun(t, server, config)
	run.waitReady(t)
	id := server.WaitTransport(t)
	conn, err := net.DialTimeout("tcp", config.Listen.String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	var method [2]byte
	if _, err := io.ReadFull(conn, method[:]); err != nil || method != [2]byte{5, 0} {
		t.Fatalf("method = %v, %v", method, err)
	}
	request := append([]byte{5, 1, 0, 3, byte(len(destination.Host))}, []byte(destination.Host)...)
	request = append(request, byte(destination.Port>>8), byte(destination.Port))
	request = append(request, []byte("pipelined-payload")...)
	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	var reply [10]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[1] != 0 {
		t.Fatalf("CONNECT reply = %v, %v", reply, err)
	}
	response, err := io.ReadAll(conn)
	if err != nil || string(response) != "pipelined-payloadafter-eof" {
		t.Fatalf("pipelined response = %q, %v", response, err)
	}
	if server.ChannelCount(id, "direct-tcpip") != 1 || server.ChannelCount(id, "session") != 0 {
		t.Fatal("wrong dynamic channel protocol")
	}
	run.stop(t)
	forwardingPortReleased(t, config.Listen)
}

func TestRemoteForwardingCanceledLateListener(t *testing.T) {
	seen := make(chan struct{}, 1)
	release := make(chan struct{})
	server := startForwardingServer(t, forwardingServerConfig{GlobalRules: map[string]forwardingReplyRule{"tcpip-forward": {Seen: seen, Release: release}}})
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelRemote, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
	run := startForwardingRun(t, server, config)
	id := server.WaitTransport(t)
	forwardingFixtureWait(t, seen)
	run.cancel()
	close(release)
	if err := forwardingFixtureWait(t, run.done); !errors.Is(err, context.Canceled) {
		t.Fatalf("late listener result = %v", err)
	}
	select {
	case <-run.ready:
		t.Fatal("canceled remote startup became ready")
	default:
	}
	server.CloseTransport(id)
	forwardingPortReleased(t, config.Listen)
}

func TestForwardingAllModesBlockedPeerWritesConverge(t *testing.T) {
	for _, mode := range []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic} {
		t.Run(string(mode), func(t *testing.T) {
			server := startForwardingServer(t, forwardingServerConfig{})
			address, _ := startForwardingDestination(t)
			destination := forwardingEndpoint(t, address)
			config := app.TunnelConfig{Mode: mode, Listen: forwardingFreeEndpoint(t), Destination: destination}
			if mode == app.TunnelDynamic {
				config.Destination = app.TunnelEndpoint{}
			}
			run := startForwardingRun(t, server, config)
			run.waitReady(t)
			id := server.WaitTransport(t)
			conn := forwardingDial(t, config, destination)
			if _, err := conn.Write([]byte("established")); err != nil {
				t.Fatal(err)
			}
			response := make([]byte, len("established"))
			if _, err := io.ReadFull(conn, response); err != nil {
				t.Fatal(err)
			}
			blocked := server.HoldTransportReads(id)
			written := make(chan error, 1)
			go func() { _, err := conn.Write(bytes.Repeat([]byte{42}, 8<<20)); written <- err }()
			forwardingFixtureWait(t, blocked)
			run.stop(t)
			_ = conn.Close()
			forwardingFixtureWait(t, written)
			server.CloseTransport(id)
			forwardingPortReleased(t, config.Listen)
		})
	}
}
