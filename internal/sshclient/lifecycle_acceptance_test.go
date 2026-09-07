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

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
)

func TestSC016TransportCleanupLifecycleMatrix20Runs(t *testing.T) {
	stages := []string{
		"terminal-size", "target-resolution", "dial", "host-trust", "authentication", "handshake", "session",
		"pty", "resize-source", "raw-mode", "shell", "active-wait", "active-resize",
	}
	for run := range 20 {
		for _, failure := range []string{"timeout", "network"} {
			for _, stage := range stages {
				t.Run(failure+"/"+stage, func(t *testing.T) {
					runSC016LifecycleCase(t, run, failure, stage)
				})
			}
		}
	}
}

func runSC016LifecycleCase(t *testing.T, run int, failure, stage string) {
	t.Helper()
	ctx := newSC016Context()
	injected := errors.New("injected network interruption")
	fail := func() error {
		if failure == "timeout" {
			ctx.trigger(context.DeadlineExceeded)
			return context.DeadlineExceeded
		}
		return injected
	}

	var sessionCloses, transportCloses, connectionCloses, authCloses atomic.Int32
	var resizeStarts, resizeProcesses, resizeJoins atomic.Int32
	closed := make(chan struct{})
	resized := make(chan struct{})
	var resizeOnce sync.Once
	remote := &fakeSession{
		close: func() error {
			if sessionCloses.Add(1) == 1 {
				close(closed)
			}
			return nil
		},
	}
	transport := &fakeTransport{session: remote, close: func() error { transportCloses.Add(1); return nil }}
	connection := &fakeConn{close: func() error { connectionCloses.Add(1); return nil }}
	local := &sc016Terminal{Fake: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})}

	expected := sc016ExpectedCleanup(stage)
	local.restore = func() {
		got := sc016CleanupCounts{sessionCloses.Load(), transportCloses.Load(), connectionCloses.Load(), authCloses.Load()}
		if got != expected {
			t.Errorf("run %02d cleanup before terminal restore = %+v, want %+v", run+1, got, expected)
		}
		active := stage == "active-wait" || stage == "active-resize"
		wantResize := sc016ResizeCounts{}
		if active {
			wantResize = sc016ResizeCounts{start: 1, process: 1, join: 1}
		}
		gotResize := sc016ResizeCounts{resizeStarts.Load(), resizeProcesses.Load(), resizeJoins.Load()}
		if gotResize != wantResize {
			t.Errorf("run %02d resize forwarding before terminal restore = %+v, want %+v", run+1, gotResize, wantResize)
		}
	}
	if stage == "terminal-size" {
		local.size = fail
	}
	if stage == "resize-source" {
		local.resizeEvents = func(context.Context) (<-chan terminal.Size, error) { return nil, fail() }
	}
	if stage == "raw-mode" {
		local.makeRaw = fail
	}
	if stage == "active-wait" || stage == "active-resize" {
		local.activeResize = true
	}

	options := Options{
		resize: &resizeLifecycleObserver{
			start: func() { resizeStarts.Add(1) }, process: func() { resizeProcesses.Add(1) }, join: func() { resizeJoins.Add(1) },
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			if stage == "target-resolution" {
				return nil, &net.DNSError{Err: failure, Name: "injected.invalid", IsTimeout: failure == "timeout", IsTemporary: failure == "network"}
			}
			if stage == "dial" {
				return nil, fail()
			}
			return connection, nil
		},
		Authenticate: func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			closer := closerFunc(func() error { authCloses.Add(1); return nil })
			if stage == "authentication" {
				return nil, closer, fail()
			}
			return []ssh.AuthMethod{ssh.Password("not-recorded")}, closer, nil
		},
	}
	options.Handshake = func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
		if err := config.HostKeyCallback("", nil, testPublicKeyNoTest()); err != nil {
			return nil, err
		}
		if _, err := config.AuthCallback(new(ssh.ClientAuthContext)); err != nil {
			return nil, err
		}
		if stage == "handshake" {
			return nil, fail()
		}
		return transport, nil
	}
	if stage == "session" {
		transport.newSession = func() (remoteSession, error) { return nil, fail() }
	}
	if stage == "pty" {
		remote.requestPty = func(string, int, int, ssh.TerminalModes) error { return fail() }
	}
	if stage == "shell" {
		remote.shell = fail
	}
	if stage == "active-wait" {
		remote.resize = func(int, int) error {
			resizeOnce.Do(func() { close(resized) })
			return nil
		}
		remote.wait = func() error {
			<-resized
			if failure == "timeout" {
				_ = fail()
				<-closed
				return context.DeadlineExceeded
			}
			return injected
		}
	}
	if stage == "active-resize" {
		remote.resize = func(int, int) error {
			return fail()
		}
		remote.wait = func() error { <-closed; return injected }
	}

	request := testRequest(local)
	request.VerifyHost = func(context.Context, app.PresentedHost) error {
		if stage == "host-trust" {
			return fail()
		}
		return nil
	}
	result, err := New(options).Run(ctx, request)
	if err == nil {
		t.Fatalf("run %02d %s/%s unexpectedly succeeded", run+1, failure, stage)
	}
	if failure == "timeout" {
		if stage == "target-resolution" {
			var normalized *app.SSHStartError
			if !errors.As(err, &normalized) || normalized.Reason() != app.SSHFailureTimeout || normalized.Stage() != app.SSHFailureStageTargetResolution {
				t.Fatalf("run %02d target-resolution timeout = %v", run+1, err)
			}
		} else if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("run %02d %s timeout error = %v", run+1, stage, err)
		}
	}
	active := stage == "active-wait" || stage == "active-resize"
	if active != !result.StartedAt.IsZero() {
		t.Fatalf("run %02d %s/%s StartedAt = %v, active=%v", run+1, failure, stage, result.StartedAt, active)
	}
	got := sc016CleanupCounts{sessionCloses.Load(), transportCloses.Load(), connectionCloses.Load(), authCloses.Load()}
	if got != expected {
		t.Fatalf("run %02d %s/%s cleanup = %+v, want %+v", run+1, failure, stage, got, expected)
	}
	wantResize := sc016ResizeCounts{}
	if active {
		wantResize = sc016ResizeCounts{start: 1, process: 1, join: 1}
	}
	gotResize := sc016ResizeCounts{resizeStarts.Load(), resizeProcesses.Load(), resizeJoins.Load()}
	if gotResize != wantResize {
		t.Fatalf("run %02d %s/%s resize forwarding = %+v, want %+v", run+1, failure, stage, gotResize, wantResize)
	}
	assertTerminalRestored(t, local.Fake)
}

type sc016CleanupCounts struct {
	session, transport, connection, auth int32
}

type sc016ResizeCounts struct {
	start, process, join int32
}

func sc016ExpectedCleanup(stage string) sc016CleanupCounts {
	switch stage {
	case "terminal-size", "target-resolution", "dial":
		return sc016CleanupCounts{}
	case "host-trust":
		return sc016CleanupCounts{connection: 1}
	case "authentication", "handshake":
		return sc016CleanupCounts{connection: 1, auth: 1}
	case "session":
		return sc016CleanupCounts{transport: 1, connection: 1, auth: 1}
	default:
		return sc016CleanupCounts{session: 1, transport: 1, connection: 1, auth: 1}
	}
}

type sc016Context struct {
	done chan struct{}
	once sync.Once
	mu   sync.Mutex
	err  error
}

func newSC016Context() *sc016Context                            { return &sc016Context{done: make(chan struct{})} }
func (c *sc016Context) Deadline() (deadline time.Time, ok bool) { return time.Time{}, false }
func (c *sc016Context) Done() <-chan struct{}                   { return c.done }
func (c *sc016Context) Value(any) any                           { return nil }
func (c *sc016Context) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
func (c *sc016Context) trigger(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.done)
	})
}

type sc016Terminal struct {
	*terminal.Fake
	size         func() error
	resizeEvents func(context.Context) (<-chan terminal.Size, error)
	makeRaw      func() error
	restore      func()
	activeResize bool
}

func (t *sc016Terminal) Size(ctx context.Context) (terminal.Size, error) {
	if t.size != nil {
		return terminal.Size{}, t.size()
	}
	return t.Fake.Size(ctx)
}
func (t *sc016Terminal) ResizeEvents(ctx context.Context) (<-chan terminal.Size, error) {
	if t.resizeEvents != nil {
		return t.resizeEvents(ctx)
	}
	if t.activeResize {
		queued := make(chan terminal.Size, 1)
		queued <- terminal.Size{Columns: 100, Rows: 30}
		close(queued)
		return queued, nil
	}
	return t.Fake.ResizeEvents(ctx)
}
func (t *sc016Terminal) MakeRaw(ctx context.Context) error {
	if t.makeRaw != nil {
		return t.makeRaw()
	}
	return t.Fake.MakeRaw(ctx)
}
func (t *sc016Terminal) Restore(ctx context.Context, state terminal.State) error {
	if t.restore != nil {
		t.restore()
	}
	return t.Fake.Restore(ctx, state)
}
