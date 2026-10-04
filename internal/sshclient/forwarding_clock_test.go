package sshclient

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
)

type observedForwardingTimer struct {
	*forwardingDoubleTimer
	duration time.Duration
	stopped  chan<- time.Duration
}

func (t *observedForwardingTimer) Stop() bool {
	stopped := t.forwardingDoubleTimer.Stop()
	t.stopped <- t.duration
	return stopped
}

func observeForwardingClock(clock *forwardingDoubleClock) (forwardingAfterFunc, <-chan time.Duration) {
	stopped := make(chan time.Duration, 64)
	return func(d time.Duration, f func()) forwardingTimer {
		return &observedForwardingTimer{clock.AfterFunc(d, f), d, stopped}
	}, stopped
}

func assertForwardingRawOpen(t *testing.T, scope *forwardingDoubleScope) {
	t.Helper()
	select {
	case <-scope.done:
		t.Fatal("raw socket forced closed before boundary")
	default:
	}
}

func awaitForwardingTimerStop(t *testing.T, stopped <-chan time.Duration, want time.Duration) {
	t.Helper()
	select {
	case duration := <-stopped:
		if duration != want {
			t.Fatalf("stopped timer=%v, want %v", duration, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("completed operation did not cancel its timer")
	}
}

func TestForwardingShutdownWatchdogClockBoundaryAllModes(t *testing.T) {
	for _, mode := range []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic} {
		t.Run(string(mode), func(t *testing.T) {
			clock := &forwardingDoubleClock{now: time.Unix(0, 0)}
			f := newForwardingDoubleTransport()
			client := tunnelTestClient(f)
			client.options.forwardAfterFunc, _ = observeForwardingClock(clock)
			ctx, cancel := context.WithCancel(context.Background())
			req := tunnelRequest(mode)
			req.Config.Listen = unusedTunnelEndpoint(t)
			ready := make(chan struct{})
			req.Ready = func() { close(ready) }
			done := make(chan error, 1)
			joined := make(chan struct{})
			go func() { defer close(joined); done <- client.RunTunnel(ctx, req) }()
			t.Cleanup(func() { cancel(); _ = f.RawClose(); forwardingDoubleAwait(t, joined) })
			blocked := f.CloseOp.Started
			if mode == app.TunnelRemote {
				forwardingDoubleAwait(t, f.ListenOp.Started)
				listener := newForwardingDoubleListener(f.forwardingDoubleScope)
				if err := f.ListenOp.Reply(ctx, listener, nil); err != nil {
					t.Fatal(err)
				}
				blocked = listener.CloseOp.Started
			}
			forwardingDoubleAwait(t, ready)
			if mode != app.TunnelRemote {
				source, err := net.Dial("tcp", req.Config.Listen.String())
				if err != nil {
					t.Fatal(err)
				}
				defer source.Close()
				if mode == app.TunnelDynamic {
					if _, err := source.Write([]byte{5, 1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80}); err != nil {
						t.Fatal(err)
					}
				}
				forwardingDoubleAwait(t, f.DialOp.Started)
				destination := newForwardingDoubleConn(f.forwardingDoubleScope)
				if err := f.DialOp.Reply(ctx, destination, nil); err != nil {
					t.Fatal(err)
				}
				forwardingDoubleAwait(t, destination.ReadOp.Started)
				blocked = destination.CloseOp.Started
			}
			cancel()
			forwardingDoubleAwait(t, blocked)
			clock.Advance(249 * time.Millisecond)
			assertForwardingRawOpen(t, f.forwardingDoubleScope)
			select {
			case err := <-done:
				t.Fatalf("blocked cleanup returned before watchdog: %v", err)
			default:
			}
			clock.Advance(time.Millisecond)
			select {
			case <-f.done:
			default:
				t.Fatal("watchdog did not force raw close at exactly 250ms")
			}
			tunnelResult(t, done)
			if f.Active() != 0 {
				t.Fatalf("workers=%d", f.Active())
			}
			clock.mu.Lock()
			defer clock.mu.Unlock()
			for _, timer := range clock.timers {
				if timer.pending {
					t.Fatal("owned timer survived runtime join")
				}
			}
		})
	}
}

func TestForwardingProtocolGraceClockBoundary(t *testing.T) {
	clock := &forwardingDoubleClock{now: time.Unix(0, 0)}
	scope := newForwardingDoubleScope()
	ctx, cancel := context.WithCancelCause(context.Background())
	afterFunc, _ := observeForwardingClock(clock)
	r := &tunnelRuntime{ctx: ctx, cancel: cancel, raw: newCloseOnce(closerFunc(scope.RawClose)), afterFunc: afterFunc, watchdogDone: make(chan struct{}), converged: make(chan struct{})}
	conn := newForwardingDoubleConn(scope)
	done := make(chan struct{})
	go func() { defer close(done); _ = r.protocolCall(conn.Close) }()
	t.Cleanup(func() {
		r.stop(context.Canceled)
		_ = scope.RawClose()
		forwardingDoubleAwait(t, done)
		close(r.converged)
		forwardingDoubleAwait(t, r.watchdogDone)
	})
	forwardingDoubleAwait(t, conn.CloseOp.Started)
	clock.Advance(999 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatal("protocol grace expired before 1s")
	}
	assertForwardingRawOpen(t, scope)
	clock.Advance(time.Millisecond)
	if !errors.Is(context.Cause(ctx), errTunnel) {
		t.Fatal("protocol grace did not fail transport at exactly 1s")
	}
	assertForwardingRawOpen(t, scope)
	clock.Advance(249 * time.Millisecond)
	assertForwardingRawOpen(t, scope)
	clock.Advance(time.Millisecond)
	forwardingDoubleAwait(t, done)
	if scope.Active() != 0 {
		t.Fatalf("workers=%d", scope.Active())
	}
}

func TestForwardingDeadlineCancelsOrJoinsCallback(t *testing.T) {
	clock := &forwardingDoubleClock{now: time.Unix(0, 0)}
	afterFunc, stopped := observeForwardingClock(clock)
	r := &tunnelRuntime{afterFunc: afterFunc}
	finish := r.deadline(time.Second, func() { t.Error("canceled callback fired") })
	finish()
	awaitForwardingTimerStop(t, stopped, time.Second)
	clock.Advance(time.Second)
	r.afterFunc = func(d time.Duration, f func()) forwardingTimer {
		// Advance callbacks stay nonblocking; deadline's finish must join f.
		return afterFunc(d, func() { go f() })
	}
	started, release := make(chan struct{}), make(chan struct{})
	finish = r.deadline(time.Second, func() { close(started); <-release })
	clock.Advance(time.Second)
	forwardingDoubleAwait(t, started)
	joined := make(chan struct{})
	go func() { finish(); close(joined) }()
	awaitForwardingTimerStop(t, stopped, time.Second)
	select {
	case <-joined:
		t.Fatal("deadline cancellation did not join running callback")
	default:
	}
	close(release)
	forwardingDoubleAwait(t, joined)
}

func TestTunnelExpiredOpenGraceClockBoundary(t *testing.T) {
	t.Parallel()
	clock := &forwardingDoubleClock{now: time.Unix(0, 0)}
	afterFunc, _ := observeForwardingClock(clock)
	registered := make(chan time.Duration, 16)
	f := newForwardingDoubleTransport()
	client := tunnelTestClient(f)
	client.options.forwardAfterFunc = func(d time.Duration, expire func()) forwardingTimer {
		timer := afterFunc(d, expire)
		registered <- d
		return timer
	}
	ctx, cancel := context.WithCancel(context.Background())
	req := tunnelRequest(app.TunnelLocal)
	req.Config.Listen = unusedTunnelEndpoint(t)
	ready := make(chan struct{})
	req.Ready = func() { close(ready) }
	done, joined := make(chan error, 1), make(chan struct{})
	go func() { defer close(joined); done <- client.RunTunnel(ctx, req) }()
	t.Cleanup(func() { cancel(); _ = f.RawClose(); forwardingDoubleAwait(t, joined) })
	forwardingDoubleAwait(t, ready)
	source, err := net.Dial("tcp", req.Config.Listen.String())
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	forwardingDoubleAwait(t, f.DialOp.Started)
	// Keep the real ten-second destination context; only convergence is fake.
	select {
	case duration := <-registered:
		if duration != time.Second {
			t.Fatalf("open grace=%v", duration)
		}
	case err := <-done:
		t.Fatalf("pending open failed before convergence grace: %v", err)
	case <-time.After(12 * time.Second):
		t.Fatal("expired destination open did not arm convergence grace")
	}
	clock.Advance(999 * time.Millisecond)
	assertForwardingRawOpen(t, f.forwardingDoubleScope)
	if f.Active() < 2 {
		t.Fatal("expired open worker was released before convergence")
	}
	select {
	case duration := <-registered:
		t.Fatalf("transport stopping before 1s grace: timer=%v", duration)
	default:
	}
	clock.Advance(time.Millisecond)
	select {
	case duration := <-registered:
		if duration != 250*time.Millisecond {
			t.Fatalf("watchdog=%v", duration)
		}
	default:
		t.Fatal("expired open did not fail transport at exactly 1s")
	}
	clock.Advance(249 * time.Millisecond)
	assertForwardingRawOpen(t, f.forwardingDoubleScope)
	clock.Advance(time.Millisecond)
	select {
	case err := <-done:
		if !errors.Is(err, errTunnel) {
			t.Fatalf("result=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expired open failed to join after raw closure")
	}
	if f.Active() != 0 {
		t.Fatalf("workers=%d", f.Active())
	}
}

func TestTunnelStartupCancellationMatrix(t *testing.T) {
	for _, phase := range []string{"before_network", "network", "network_late_success", "handshake", "trust"} {
		t.Run(phase, func(t *testing.T) {
			f := newForwardingDoubleTransport()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := tunnelTestClient(f)
			req := tunnelRequest(app.TunnelLocal)
			started := make(chan struct{})
			var authCalls atomic.Int64
			client.options.Authenticate = func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
				authCalls.Add(1)
				return nil, nil, errors.New("unexpected authentication")
			}
			req.Ready = func() { t.Error("canceled startup became ready") }
			switch phase {
			case "before_network":
				client.options.DialContext = func(context.Context, string, string) (net.Conn, error) {
					t.Error("network I/O after initial cancellation")
					return nil, context.Canceled
				}
				cancel()
				close(started)
			case "network":
				client.options.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				}
			case "network_late_success":
				client.options.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
					close(started)
					<-ctx.Done()
					return &tunnelRaw{scope: f.forwardingDoubleScope}, nil
				}
			case "handshake":
				client.options.Handshake = func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) {
					close(started)
					<-f.done
					return nil, net.ErrClosed
				}
			case "trust":
				req.VerifyHost = func(ctx context.Context, _ app.PresentedHost) error { close(started); <-ctx.Done(); return ctx.Err() }
				key := testPublicKey(t)
				client.options.Handshake = func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
					return nil, config.HostKeyCallback("", nil, key)
				}
			}
			done := make(chan error, 1)
			go func() { done <- client.RunTunnel(ctx, req) }()
			forwardingDoubleAwait(t, started)
			cancel()
			tunnelResult(t, done)
			if authCalls.Load() != 0 || f.Active() != 0 {
				t.Fatalf("auth calls=%d workers=%d", authCalls.Load(), f.Active())
			}
			if phase != "network" && phase != "before_network" {
				select {
				case <-f.done:
				default:
					t.Fatal("acquired raw socket survived canceled startup")
				}
			}
		})
	}
	// Agent listing and signing cancellation are covered by
	// TestAgentCancellationDuringSignerList and TestTunnelRootCancellationDuringAgentSigning.
}
