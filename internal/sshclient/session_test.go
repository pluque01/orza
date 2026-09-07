package sshclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
)

func TestRunStreamsIORequestsPTYAndPropagatesResize(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 90, Rows: 25})
	local.QueueResize(terminal.Size{Columns: 120, Rows: 40})
	stdin := bytes.NewBufferString("input")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	var ptyRows, ptyColumns int
	var resizes [][2]int
	resized := make(chan struct{})
	remote := &fakeSession{
		requestPty: func(_ string, rows, columns int, modes ssh.TerminalModes) error {
			ptyRows, ptyColumns = rows, columns
			if modes[ssh.ECHO] != 1 {
				t.Fatal("remote PTY echo was not enabled")
			}
			return nil
		},
		resize: func(rows, columns int) error {
			resizes = append(resizes, [2]int{rows, columns})
			close(resized)
			return nil
		},
		wait: func() error {
			<-resized
			return nil
		},
	}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	client.options.Stdin, client.options.Stdout, client.options.Stderr = stdin, stdout, stderr

	result, err := client.Run(context.Background(), testRequest(local))
	if err != nil {
		t.Fatal(err)
	}
	if ptyRows != 25 || ptyColumns != 90 {
		t.Fatalf("PTY = %dx%d", ptyColumns, ptyRows)
	}
	if !reflect.DeepEqual(resizes, [][2]int{{40, 120}}) {
		t.Fatalf("resizes = %v", resizes)
	}
	if remote.stdin != stdin || remote.stdout != stdout || remote.stderr != stderr {
		t.Fatal("session I/O was not connected directly")
	}
	if result.State != app.SessionSucceeded || result.StartedAt.IsZero() {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunMapsRemoteExitStatus(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	remote := &fakeSession{wait: func() error { return exitStatusError(42) }}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	result, err := client.Run(context.Background(), testRequest(local))
	var remoteErr *RemoteExitError
	if !errors.As(err, &remoteErr) || remoteErr.Status() != 42 {
		t.Fatalf("Run error = %v", err)
	}
	if result.State != app.SessionFailed || result.Outcome != app.SessionOutcomeRemoteFailure || result.RemoteExitStatus == nil || *result.RemoteExitStatus != 42 {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunMapsWaitFailureToTransport(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	wantErr := errors.New("connection reset")
	client := successfulClient(local, &fakeTransport{session: &fakeSession{wait: func() error { return wantErr }}}, &fakeConn{})
	result, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageStream || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if result.Outcome != app.SessionOutcomeTransportFailure {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunCancellationClosesSessionAndRestoresWithUncanceledContext(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan struct{})
	closed := make(chan struct{})
	remote := &fakeSession{
		wait: func() error {
			close(waiting)
			<-closed
			return errors.New("closed")
		},
		close: func() error {
			close(closed)
			return nil
		},
	}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	go func() {
		<-waiting
		cancel()
	}()
	result, err := client.Run(ctx, testRequest(local))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v", err)
	}
	if result.State != app.SessionCanceled || result.Outcome != app.SessionOutcomeCanceled {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunResizeFailureClosesSession(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize(terminal.Size{Columns: 100, Rows: 30})
	wantErr := errors.New("window change failed")
	closed := make(chan struct{})
	remote := &fakeSession{
		resize: func(int, int) error { return wantErr },
		wait: func() error {
			<-closed
			return errors.New("closed")
		},
		close: func() error { close(closed); return nil },
	}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	result, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageStream || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if result.Outcome != app.SessionOutcomeTransportFailure {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunShellFailureRestoresRawTerminal(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	wantErr := errors.New("shell rejected")
	remote := &fakeSession{shell: func() error { return wantErr }}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	result, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageShell || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if result.Outcome != app.SessionOutcomeTransportFailure {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunRawModeFailureDoesNotStartShell(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	wantErr := errors.New("raw mode unavailable")
	local.SetFault(terminal.OperationMakeRaw, wantErr)
	shellCalled := false
	remote := &fakeSession{shell: func() error { shellCalled = true; return nil }}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	_, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageTerminal || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if shellCalled {
		t.Fatal("shell started after raw mode failure")
	}
	assertTerminalRestored(t, local)
}

func TestRunRestoreFailureIsReturned(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	wantErr := errors.New("restore failed")
	local.SetFault(terminal.OperationRestore, wantErr)
	client := successfulClient(local, &fakeTransport{session: &fakeSession{wait: func() error { return nil }}}, &fakeConn{})
	result, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageTerminal || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if result.State != app.SessionFailed || result.Outcome != app.SessionOutcomeTransportFailure {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunCleanupFailureOverridesSuccessfulOutcome(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	wantErr := errors.New("session close failed")
	remote := &fakeSession{wait: func() error { return nil }, close: func() error { return wantErr }}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	result, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageCleanup || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if result.State != app.SessionFailed || result.Outcome != app.SessionOutcomeTransportFailure {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

type exitStatusError int

func (e exitStatusError) Error() string   { return "remote failure" }
func (e exitStatusError) ExitStatus() int { return int(e) }

func TestCloseOnceIsConcurrentAndDeterministic(t *testing.T) {
	count := 0
	closer := newCloseOnce(closerFunc(func() error {
		count++
		time.Sleep(time.Millisecond)
		return nil
	}))
	done := make(chan struct{}, 2)
	go func() { _ = closer.Close(); done <- struct{}{} }()
	go func() { _ = closer.Close(); done <- struct{}{} }()
	<-done
	<-done
	if count != 1 {
		t.Fatalf("close count = %d", count)
	}
}

func TestRunDirectStreamsLargeStdoutAndStderrWithoutRetention(t *testing.T) {
	const streamSize = 64 << 20
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	stdout := newCountingWriter()
	stderr := newCountingWriter()
	remote := &fakeSession{wait: func() error {
		chunk := bytes.Repeat([]byte("stream"), 4096)
		for _, writer := range []io.Writer{stdout, stderr} {
			remaining := streamSize
			for remaining > 0 {
				write := min(remaining, len(chunk))
				if _, err := writer.Write(chunk[:write]); err != nil {
					return err
				}
				remaining -= write
			}
		}
		return nil
	}}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	client.options.Stdout, client.options.Stderr = stdout, stderr

	result, err := client.Run(context.Background(), testRequest(local))
	if err != nil || result.State != app.SessionSucceeded {
		t.Fatalf("Run() = %+v, %v", result, err)
	}
	if stdout.bytes.Load() != streamSize || stderr.bytes.Load() != streamSize {
		t.Fatalf("stream counts = stdout %d stderr %d", stdout.bytes.Load(), stderr.bytes.Load())
	}
	if remote.stdout != stdout || remote.stderr != stderr {
		t.Fatal("remote streams were not connected directly to terminal writers")
	}
}

func TestRunTimeoutAndNetworkCleanupExactlyOnceMatrix(t *testing.T) {
	for run := 0; run < 20; run++ {
		for _, test := range []struct {
			name    string
			waitErr error
			timeout bool
		}{
			{name: "timeout", timeout: true},
			{name: "network", waitErr: errors.New("connection reset")},
		} {
			t.Run(test.name, func(t *testing.T) {
				local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
				local.QueueResize()
				ctx := context.Background()
				cancel := func() {}
				if test.timeout {
					var cancelContext context.CancelFunc
					ctx, cancelContext = context.WithTimeout(ctx, time.Millisecond)
					cancel = cancelContext
				}
				defer cancel()
				var sessionCloses, transportCloses, connectionCloses, authCloses atomic.Int32
				closed := make(chan struct{})
				remote := &fakeSession{
					wait: func() error {
						if !test.timeout {
							return test.waitErr
						}
						<-closed
						return context.DeadlineExceeded
					},
					close: func() error {
						if sessionCloses.Add(1) == 1 {
							close(closed)
						}
						return nil
					},
				}
				transport := &fakeTransport{session: remote, close: func() error { transportCloses.Add(1); return nil }}
				connection := &fakeConn{close: func() error { connectionCloses.Add(1); return nil }}
				client := New(Options{
					DialContext: func(context.Context, string, string) (net.Conn, error) { return connection, nil },
					Authenticate: func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
						return []ssh.AuthMethod{ssh.Password("not-recorded")}, closerFunc(func() error { authCloses.Add(1); return nil }), nil
					},
					Handshake: func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
						if err := config.HostKeyCallback("", nil, testPublicKeyNoTest()); err != nil {
							return nil, err
						}
						if _, err := config.AuthCallback(new(ssh.ClientAuthContext)); err != nil {
							return nil, err
						}
						return transport, nil
					},
				})
				result, err := client.Run(ctx, testRequest(local))
				if test.timeout && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("timeout error = %v", err)
				}
				if !test.timeout && result.Outcome != app.SessionOutcomeTransportFailure {
					t.Fatalf("network result = %+v, %v", result, err)
				}
				if sessionCloses.Load() != 1 || transportCloses.Load() != 1 || connectionCloses.Load() != 1 || authCloses.Load() != 1 {
					t.Fatalf("cleanup counts = session %d transport %d connection %d auth %d", sessionCloses.Load(), transportCloses.Load(), connectionCloses.Load(), authCloses.Load())
				}
				assertTerminalRestored(t, local)
			})
		}
	}
}

type countingWriter struct {
	bytes atomic.Int64
}

func newCountingWriter() *countingWriter { return &countingWriter{} }

func (w *countingWriter) Write(value []byte) (int, error) {
	w.bytes.Add(int64(len(value)))
	return len(value), nil
}
