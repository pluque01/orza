package sshclient

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// All descendants share the raw socket's lifetime and worker accounting. Idle
// reports convergence of calls already entered, not of future caller goroutines.
type forwardingDoubleScope struct {
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	active int
	idle   chan struct{}
}

func newForwardingDoubleScope() *forwardingDoubleScope {
	idle := make(chan struct{})
	close(idle)
	return &forwardingDoubleScope{done: make(chan struct{}), idle: idle}
}

func (s *forwardingDoubleScope) RawClose() error {
	s.once.Do(func() { close(s.done) })
	return nil
}

func (s *forwardingDoubleScope) enter() func() {
	s.mu.Lock()
	if s.active == 0 {
		s.idle = make(chan struct{})
	}
	s.active++
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.active--
		if s.active == 0 {
			close(s.idle)
		}
	}
}

func (s *forwardingDoubleScope) Idle() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.idle
}

func (s *forwardingDoubleScope) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

type forwardingDoubleResult[T any] struct {
	value T
	err   error
}

// Each Reply releases exactly one call. Started closes on first entry; Calls
// counts all entries. No implicit goroutines or buffered late results are owned.
type forwardingDoubleOp[T any] struct {
	scope   *forwardingDoubleScope
	Started chan struct{}
	start   sync.Once
	Calls   atomic.Int64
	replies chan forwardingDoubleResult[T]
}

func newForwardingDoubleOp[T any](s *forwardingDoubleScope) *forwardingDoubleOp[T] {
	return &forwardingDoubleOp[T]{scope: s, Started: make(chan struct{}), replies: make(chan forwardingDoubleResult[T])}
}

func (o *forwardingDoubleOp[T]) call(ctx context.Context, closed <-chan struct{}) (T, error) {
	leave := o.scope.enter()
	defer leave()
	o.Calls.Add(1)
	o.start.Do(func() { close(o.Started) })
	var zero T
	select {
	case <-o.scope.done:
		return zero, net.ErrClosed
	case <-closed:
		return zero, net.ErrClosed
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
	}
	select {
	case result := <-o.replies:
		return result.value, result.err
	case <-o.scope.done:
		return zero, net.ErrClosed
	case <-closed:
		return zero, net.ErrClosed
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (o *forwardingDoubleOp[T]) Reply(ctx context.Context, value T, err error) error {
	select {
	case <-o.scope.done:
		return net.ErrClosed
	default:
	}
	select {
	case o.replies <- forwardingDoubleResult[T]{value: value, err: err}:
		return nil
	case <-o.scope.done:
		return net.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// A conn also models an SSH channel: Close and CloseWrite can stall on protocol
// writes independently of raw closure. Scripted reads retain unread bytes.
type forwardingDoubleConn struct {
	scope        *forwardingDoubleScope
	ReadOp       *forwardingDoubleOp[[]byte]
	WriteOp      *forwardingDoubleOp[int]
	CloseOp      *forwardingDoubleOp[struct{}]
	CloseWriteOp *forwardingDoubleOp[struct{}]
	closed       chan struct{}
	closeOnce    sync.Once
	writeClosed  atomic.Bool
	readMu       sync.Mutex
	pending      []byte
	readErr      error
}

func newForwardingDoubleConn(s *forwardingDoubleScope) *forwardingDoubleConn {
	return &forwardingDoubleConn{
		scope: s, closed: make(chan struct{}),
		ReadOp: newForwardingDoubleOp[[]byte](s), WriteOp: newForwardingDoubleOp[int](s),
		CloseOp: newForwardingDoubleOp[struct{}](s), CloseWriteOp: newForwardingDoubleOp[struct{}](s),
	}
}

func (c *forwardingDoubleConn) Read(p []byte) (int, error) {
	leave := c.scope.enter()
	defer leave()
	c.readMu.Lock()
	defer c.readMu.Unlock()
	select {
	case <-c.scope.done:
		return 0, net.ErrClosed
	case <-c.closed:
		return 0, net.ErrClosed
	default:
	}
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.pending) == 0 && c.readErr == nil {
		data, err := c.ReadOp.call(context.Background(), c.closed)
		c.pending, c.readErr = append([]byte(nil), data...), err
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	if len(c.pending) != 0 {
		return n, nil
	}
	err := c.readErr
	c.readErr = nil
	return n, err
}

func (c *forwardingDoubleConn) Write(p []byte) (int, error) {
	if c.writeClosed.Load() {
		return 0, net.ErrClosed
	}
	n, err := c.WriteOp.call(context.Background(), c.closed)
	if n < 0 || n > len(p) {
		return 0, errors.New("invalid scripted write count")
	}
	return n, err
}

func (c *forwardingDoubleConn) Close() error {
	leave := c.scope.enter()
	defer leave()
	select {
	case <-c.closed:
		return nil
	default:
	}
	_, err := c.CloseOp.call(context.Background(), c.closed)
	if err == nil || errors.Is(err, net.ErrClosed) {
		c.closeOnce.Do(func() { close(c.closed) })
		return nil
	}
	return err
}

func (c *forwardingDoubleConn) CloseWrite() error {
	leave := c.scope.enter()
	defer leave()
	if c.writeClosed.Load() {
		return nil
	}
	_, err := c.CloseWriteOp.call(context.Background(), c.closed)
	if err == nil {
		c.writeClosed.Store(true)
	}
	return err
}

func (c *forwardingDoubleConn) LocalAddr() net.Addr              { return fakeAddr("local-unverified") }
func (c *forwardingDoubleConn) RemoteAddr() net.Addr             { return fakeAddr("remote-unverified") }
func (c *forwardingDoubleConn) SetDeadline(time.Time) error      { return errors.ErrUnsupported }
func (c *forwardingDoubleConn) SetReadDeadline(time.Time) error  { return errors.ErrUnsupported }
func (c *forwardingDoubleConn) SetWriteDeadline(time.Time) error { return errors.ErrUnsupported }

type forwardingDoubleListener struct {
	AcceptOp *forwardingDoubleOp[net.Conn]
	CloseOp  *forwardingDoubleOp[struct{}]
	closed   chan struct{}
	once     sync.Once
}

func newForwardingDoubleListener(s *forwardingDoubleScope) *forwardingDoubleListener {
	return &forwardingDoubleListener{AcceptOp: newForwardingDoubleOp[net.Conn](s), CloseOp: newForwardingDoubleOp[struct{}](s), closed: make(chan struct{})}
}

func (l *forwardingDoubleListener) Accept() (net.Conn, error) {
	return l.AcceptOp.call(context.Background(), l.closed)
}

func (l *forwardingDoubleListener) Close() error {
	leave := l.CloseOp.scope.enter()
	defer leave()
	select {
	case <-l.closed:
		return nil
	default:
	}
	_, err := l.CloseOp.call(context.Background(), l.closed)
	if err == nil || errors.Is(err, net.ErrClosed) {
		l.once.Do(func() { close(l.closed) })
		return nil
	}
	return err
}

func (l *forwardingDoubleListener) Addr() net.Addr { return fakeAddr("listener-unverified") }

// These are test APIs, not proposed production interfaces. Dial models the
// vendor's non-contextual channel open; DialContext models cancellable raw dial.
type forwardingDoubleTransport struct {
	*forwardingDoubleScope
	DialOp   *forwardingDoubleOp[net.Conn]
	ListenOp *forwardingDoubleOp[net.Listener]
	CloseOp  *forwardingDoubleOp[struct{}]
	WaitOp   *forwardingDoubleOp[struct{}]
}

func newForwardingDoubleTransport() *forwardingDoubleTransport {
	s := newForwardingDoubleScope()
	return &forwardingDoubleTransport{s, newForwardingDoubleOp[net.Conn](s), newForwardingDoubleOp[net.Listener](s), newForwardingDoubleOp[struct{}](s), newForwardingDoubleOp[struct{}](s)}
}

func (f *forwardingDoubleTransport) Dial(_, _ string) (net.Conn, error) {
	return f.DialOp.call(context.Background(), nil)
}
func (f *forwardingDoubleTransport) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return f.DialOp.call(ctx, nil)
}
func (f *forwardingDoubleTransport) Listen(_, _ string) (net.Listener, error) {
	return f.ListenOp.call(context.Background(), nil)
}
func (f *forwardingDoubleTransport) Close() error {
	leave := f.enter()
	defer leave()
	_, err := f.CloseOp.call(context.Background(), nil)
	if err == nil || errors.Is(err, net.ErrClosed) {
		return f.RawClose()
	}
	return err
}
func (f *forwardingDoubleTransport) Wait() error {
	_, err := f.WaitOp.call(context.Background(), nil)
	return err
}

var _ net.Conn = (*forwardingDoubleConn)(nil)
var _ net.Listener = (*forwardingDoubleListener)(nil)

// Advance runs due callbacks synchronously, outside the lock. Callbacks must
// not block; tests launch blocking cleanup separately and join it explicitly.
type forwardingDoubleClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*forwardingDoubleTimer
}

type forwardingDoubleTimer struct {
	clock   *forwardingDoubleClock
	at      time.Time
	f       func()
	pending bool
}

func (c *forwardingDoubleClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *forwardingDoubleClock) AfterFunc(d time.Duration, f func()) *forwardingDoubleTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &forwardingDoubleTimer{c, c.now.Add(d), f, true}
	c.timers = append(c.timers, timer)
	return timer
}

func (t *forwardingDoubleTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasPending := t.pending
	t.pending = false
	return wasPending
}

func (c *forwardingDoubleClock) Advance(d time.Duration) {
	if d < 0 {
		panic("forwarding double clock cannot move backwards")
	}
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []func()
	var pending []*forwardingDoubleTimer
	for _, timer := range c.timers {
		if timer.pending && !timer.at.After(c.now) {
			timer.pending = false
			due = append(due, timer.f)
		} else if timer.pending {
			pending = append(pending, timer)
		}
	}
	c.timers = pending
	c.mu.Unlock()
	for _, f := range due {
		f()
	}
}

func forwardingDoubleAwait(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("forwarding double did not converge")
	}
}

func TestForwardingDoubleRawCloseConverges(t *testing.T) {
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	c := newForwardingDoubleConn(f.forwardingDoubleScope)
	l := newForwardingDoubleListener(f.forwardingDoubleScope)
	operations := []struct {
		started <-chan struct{}
		run     func() error
		close   bool
	}{
		{f.DialOp.Started, func() error { _, err := f.Dial("tcp", "destination"); return err }, false},
		{f.ListenOp.Started, func() error { _, err := f.Listen("tcp", "remote"); return err }, false},
		{f.WaitOp.Started, f.Wait, false},
		{f.CloseOp.Started, f.Close, true},
		{l.AcceptOp.Started, func() error { _, err := l.Accept(); return err }, false},
		{l.CloseOp.Started, l.Close, true},
		{c.ReadOp.Started, func() error { _, err := c.Read(make([]byte, 1)); return err }, false},
		{c.WriteOp.Started, func() error { _, err := c.Write([]byte("x")); return err }, false},
		{c.CloseOp.Started, c.Close, true},
		{c.CloseWriteOp.Started, c.CloseWrite, false},
	}
	var workers sync.WaitGroup
	for _, op := range operations {
		workers.Add(1)
		go func() {
			defer workers.Done()
			err := op.run()
			if (op.close && err != nil) || (!op.close && !errors.Is(err, net.ErrClosed)) {
				t.Errorf("raw close result = %v", err)
			}
		}()
		forwardingDoubleAwait(t, op.started)
	}
	if f.Active() < len(operations) {
		t.Fatalf("workers not retained: %d", f.Active())
	}
	select {
	case <-f.Idle():
		t.Fatal("stalled workers reported idle")
	default:
	}
	_ = f.RawClose()
	_ = f.RawClose()
	forwardingDoubleAwait(t, f.Idle())
	joined := make(chan struct{})
	go func() { workers.Wait(); close(joined) }()
	forwardingDoubleAwait(t, joined)
	if f.Active() != 0 {
		t.Fatalf("remaining workers = %d", f.Active())
	}
}

func TestForwardingDoubleCancellationAndLateOpen(t *testing.T) {
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := f.DialContext(ctx, "tcp", "raw")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("dial cancellation = %v", err)
		}
	}()
	forwardingDoubleAwait(t, f.DialOp.Started)
	cancel()
	forwardingDoubleAwait(t, done)
	forwardingDoubleAwait(t, f.Idle())

	// A fresh non-contextual open remains owned after caller cancellation and
	// can deliver late success, unlike DialContext above.
	f.DialOp = newForwardingDoubleOp[net.Conn](f.forwardingDoubleScope)
	c := newForwardingDoubleConn(f.forwardingDoubleScope)
	done = make(chan struct{})
	go func() {
		defer close(done)
		conn, err := f.Dial("tcp", "late")
		if err != nil || conn != c {
			t.Errorf("late open = %v, %v", conn, err)
			return
		}
		_ = conn.Close()
	}()
	forwardingDoubleAwait(t, f.DialOp.Started)
	if err := f.DialOp.Reply(context.Background(), c, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, c.CloseOp.Started)
	if err := c.CloseOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, done)
	forwardingDoubleAwait(t, f.Idle())
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestForwardingDoubleScriptedIOAndHalfClose(t *testing.T) {
	s := newForwardingDoubleScope()
	t.Cleanup(func() { _ = s.RawClose() })
	c := newForwardingDoubleConn(s)
	done := make(chan struct{})
	go func() {
		defer close(done)
		n, err := c.Write([]byte("request"))
		if n != 7 || err != nil {
			t.Errorf("write = %d, %v", n, err)
		}
		if err := c.CloseWrite(); err != nil {
			t.Error(err)
		}
		if _, err := c.Write([]byte("closed")); !errors.Is(err, net.ErrClosed) {
			t.Errorf("half-close write = %v", err)
		}
		data, err := io.ReadAll(c)
		if string(data) != "response" || err != nil {
			t.Errorf("reverse data = %q, %v", data, err)
		}
	}()
	forwardingDoubleAwait(t, c.WriteOp.Started)
	if err := c.WriteOp.Reply(context.Background(), 7, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, c.CloseWriteOp.Started)
	if err := c.CloseWriteOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, c.ReadOp.Started)
	if err := c.ReadOp.Reply(context.Background(), []byte("response"), io.EOF); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, done)
	forwardingDoubleAwait(t, s.Idle())
}

func TestForwardingDoubleListenerRepliesAndRefusal(t *testing.T) {
	f := newForwardingDoubleTransport()
	t.Cleanup(func() { _ = f.RawClose() })
	l := newForwardingDoubleListener(f.forwardingDoubleScope)
	c := newForwardingDoubleConn(f.forwardingDoubleScope)
	refused := errors.New("scripted refusal")
	done := make(chan struct{})
	go func() {
		defer close(done)
		listener, err := f.Listen("tcp", "remote")
		if err != nil || listener != l {
			t.Errorf("listen = %v, %v", listener, err)
			return
		}
		conn, err := listener.Accept()
		if err != nil || conn != c {
			t.Errorf("accept = %v, %v", conn, err)
		}
		_, err = f.Dial("tcp", "refused")
		if !errors.Is(err, refused) {
			t.Errorf("refusal = %v", err)
		}
		_ = listener.Close()
		_, err = listener.Accept()
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("closed accept = %v", err)
		}
	}()
	forwardingDoubleAwait(t, f.ListenOp.Started)
	if err := f.ListenOp.Reply(context.Background(), l, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, l.AcceptOp.Started)
	if err := l.AcceptOp.Reply(context.Background(), c, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, f.DialOp.Started)
	if err := f.DialOp.Reply(context.Background(), nil, refused); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, l.CloseOp.Started)
	if err := l.CloseOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, done)
	forwardingDoubleAwait(t, f.Idle())
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestForwardingDoubleClock(t *testing.T) {
	c := &forwardingDoubleClock{now: time.Unix(0, 0)}
	var calls int
	stopped := c.AfterFunc(time.Second, func() { t.Error("stopped timer fired") })
	if !stopped.Stop() || stopped.Stop() {
		t.Fatal("timer stop was not idempotent")
	}
	timer := c.AfterFunc(250*time.Millisecond, func() { calls++; _ = c.Now() })
	c.Advance(249 * time.Millisecond)
	if calls != 0 {
		t.Fatal("watchdog fired early")
	}
	c.Advance(time.Millisecond)
	if calls != 1 || timer.Stop() {
		t.Fatal("watchdog did not fire exactly once")
	}
	c.Advance(time.Second)
	if calls != 1 || !c.Now().Equal(time.Unix(0, 0).Add(1250*time.Millisecond)) {
		t.Fatal("clock advance mismatch")
	}
}

func TestForwardingDoublePartialRead(t *testing.T) {
	s := newForwardingDoubleScope()
	t.Cleanup(func() { _ = s.RawClose() })
	c := newForwardingDoubleConn(s)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var data []byte
		for {
			var buffer [2]byte
			n, err := c.Read(buffer[:])
			data = append(data, buffer[:n]...)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Errorf("partial read = %v", err)
				return
			}
		}
		if string(data) != "payload" {
			t.Errorf("partial read lost bytes: %q", data)
		}
	}()
	forwardingDoubleAwait(t, c.ReadOp.Started)
	if err := c.ReadOp.Reply(context.Background(), []byte("payload"), io.EOF); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, done)
	if c.ReadOp.Calls.Load() != 1 {
		t.Fatalf("scripted read calls = %d", c.ReadOp.Calls.Load())
	}
	forwardingDoubleAwait(t, s.Idle())
}

func TestForwardingDoubleIndependentTransports(t *testing.T) {
	first, second := newForwardingDoubleTransport(), newForwardingDoubleTransport()
	t.Cleanup(func() { _ = first.RawClose(); _ = second.RawClose() })
	firstDone, secondDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(firstDone)
		if err := first.Wait(); !errors.Is(err, net.ErrClosed) {
			t.Errorf("first transport wait = %v", err)
		}
	}()
	go func() {
		defer close(secondDone)
		if err := second.Wait(); err != nil {
			t.Errorf("second transport wait = %v", err)
		}
	}()
	forwardingDoubleAwait(t, first.WaitOp.Started)
	forwardingDoubleAwait(t, second.WaitOp.Started)
	_ = first.RawClose()
	forwardingDoubleAwait(t, firstDone)
	forwardingDoubleAwait(t, first.Idle())
	if second.Active() != 1 {
		t.Fatalf("unrelated transport workers = %d", second.Active())
	}
	if err := second.WaitOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, secondDone)

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		if err := second.Close(); err != nil {
			t.Errorf("graceful transport close = %v", err)
		}
	}()
	forwardingDoubleAwait(t, second.CloseOp.Started)
	if err := second.CloseOp.Reply(context.Background(), struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	forwardingDoubleAwait(t, closed)
	forwardingDoubleAwait(t, second.Idle())
	forwardingDoubleAwait(t, second.done)
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}
