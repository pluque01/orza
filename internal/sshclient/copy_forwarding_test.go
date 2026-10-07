package sshclient

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// SSH Close can succeed without receiving the peer's EOF or close response.
type withheldReadChannel struct{ *forwardingDoubleConn }

func (c *withheldReadChannel) Read(p []byte) (int, error) {
	data, err := c.ReadOp.call(context.Background(), nil)
	return copy(p, data), err
}

func TestForwardingCopyErrorJoinsReadAfterSuccessfulClose(t *testing.T) {
	clock := &forwardingDoubleClock{now: time.Unix(0, 0)}
	afterFunc, stopped := observeForwardingClock(clock)
	scope := newForwardingDoubleScope()
	ctx, cancel := context.WithCancelCause(context.Background())
	r := &tunnelRuntime{ctx: ctx, cancel: cancel, raw: newCloseOnce(closerFunc(scope.RawClose)), watchdogDone: make(chan struct{}), converged: make(chan struct{}), afterFunc: afterFunc}
	a := newForwardingDoubleConn(scope)
	b := &withheldReadChannel{newForwardingDoubleConn(scope)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.copy(a, b, newCloseOnce(a), newCloseOnce(b))
	}()
	t.Cleanup(func() {
		r.stop(context.Canceled)
		_ = scope.RawClose()
		forwardingDoubleAwait(t, done)
		close(r.converged)
		forwardingDoubleAwait(t, r.watchdogDone)
	})
	forwardingDoubleAwait(t, a.ReadOp.Started)
	forwardingDoubleAwait(t, b.ReadOp.Started)
	if err := a.ReadOp.Reply(context.Background(), nil, errors.New("copy failed")); err != nil {
		t.Fatal(err)
	}
	for _, conn := range []*forwardingDoubleConn{a, b.forwardingDoubleConn} {
		forwardingDoubleAwait(t, conn.CloseOp.Started)
		if err := conn.CloseOp.Reply(context.Background(), struct{}{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Both Close timers must be canceled first: only the copy-join timer may
	// trigger transport failure after the successful protocol closes.
	for range 2 {
		awaitForwardingTimerStop(t, stopped, time.Second)
	}
	clock.Advance(999 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatal("copy convergence grace expired early")
	}
	assertForwardingRawOpen(t, scope)
	clock.Advance(time.Millisecond)
	if !errors.Is(context.Cause(ctx), errTunnel) {
		t.Fatalf("copy convergence failure: cause=%v", context.Cause(ctx))
	}
	clock.Advance(249 * time.Millisecond)
	assertForwardingRawOpen(t, scope)
	clock.Advance(time.Millisecond)
	forwardingDoubleAwait(t, done)
	if scope.Active() != 0 {
		t.Fatalf("remaining workers=%d", scope.Active())
	}
	select {
	case <-scope.done:
	default:
		t.Fatal("reverse Read completed without raw closure")
	}
}

var _ net.Conn = (*withheldReadChannel)(nil)

func TestForwardingCopyHealthyHalfCloseWaitsForDelayedResponse(t *testing.T) {
	clock := &forwardingDoubleClock{now: time.Unix(0, 0)}
	afterFunc, stopped := observeForwardingClock(clock)
	scope := newForwardingDoubleScope()
	ctx, cancel := context.WithCancelCause(context.Background())
	r := &tunnelRuntime{ctx: ctx, cancel: cancel, raw: newCloseOnce(closerFunc(scope.RawClose)), watchdogDone: make(chan struct{}), converged: make(chan struct{}), afterFunc: afterFunc}
	a, b := newForwardingDoubleConn(scope), newForwardingDoubleConn(scope)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.copy(a, b, newCloseOnce(a), newCloseOnce(b))
	}()
	t.Cleanup(func() {
		r.stop(context.Canceled)
		_ = scope.RawClose()
		forwardingDoubleAwait(t, done)
		close(r.converged)
		forwardingDoubleAwait(t, r.watchdogDone)
	})
	forwardingDoubleAwait(t, a.ReadOp.Started)
	forwardingDoubleAwait(t, b.ReadOp.Started)
	if err := a.ReadOp.Reply(context.Background(), nil, io.EOF); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, b.CloseWriteOp.Started)
	if err := b.CloseWriteOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	awaitForwardingTimerStop(t, stopped, time.Second)
	clock.Advance(24 * time.Hour)
	select {
	case <-ctx.Done():
		t.Fatal("healthy half-close imposed a convergence deadline")
	case <-done:
		t.Fatal("half-close truncated the delayed response")
	default:
	}
	assertForwardingRawOpen(t, scope)
	if err := b.ReadOp.Reply(context.Background(), []byte("response"), io.EOF); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, a.WriteOp.Started)
	if err := a.WriteOp.Reply(context.Background(), len("response"), nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, a.CloseWriteOp.Started)
	if err := a.CloseWriteOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, done)
	if ctx.Err() != nil || scope.Active() != 0 {
		t.Fatalf("healthy response: cause=%v workers=%d", context.Cause(ctx), scope.Active())
	}
}
