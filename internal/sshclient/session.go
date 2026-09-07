package sshclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
)

type sshTransport interface {
	NewSession() (remoteSession, error)
	Close() error
}

type remoteSession interface {
	SetIO(io.Reader, io.Writer, io.Writer)
	RequestPty(string, int, int, ssh.TerminalModes) error
	Shell() error
	Wait() error
	WindowChange(int, int) error
	Close() error
}

type clientTransport struct{ client *ssh.Client }

func (t *clientTransport) NewSession() (remoteSession, error) {
	session, err := t.client.NewSession()
	if err != nil {
		return nil, err
	}
	return &clientSession{session: session}, nil
}

func (t *clientTransport) Close() error { return t.client.Close() }

type clientSession struct{ session *ssh.Session }

func (s *clientSession) SetIO(stdin io.Reader, stdout, stderr io.Writer) {
	s.session.Stdin = stdin
	s.session.Stdout = stdout
	s.session.Stderr = stderr
}

func (s *clientSession) RequestPty(name string, rows, columns int, modes ssh.TerminalModes) error {
	return s.session.RequestPty(name, rows, columns, modes)
}
func (s *clientSession) Shell() error { return s.session.Shell() }
func (s *clientSession) Wait() error  { return s.session.Wait() }
func (s *clientSession) WindowChange(rows, columns int) error {
	return s.session.WindowChange(rows, columns)
}
func (s *clientSession) Close() error { return s.session.Close() }

func runSession(
	ctx context.Context,
	session remoteSession,
	sessionCloser *closeOnce,
	local app.Terminal,
	initial terminal.Size,
	now func() time.Time,
	resizeObserver *resizeLifecycleObserver,
) (app.SSHSessionResult, error) {
	result := app.SSHSessionResult{State: app.SessionFailed, Outcome: app.SessionOutcomeTransportFailure}
	name := os.Getenv("TERM")
	if name == "" {
		name = "xterm-256color"
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}
	if err := session.RequestPty(name, initial.Rows, initial.Columns, modes); err != nil {
		return canceledOrFailed(ctx, result, StagePTY, err)
	}
	resizeCtx, stopResize := context.WithCancel(ctx)
	defer stopResize()
	resizes, err := local.ResizeEvents(resizeCtx)
	if err != nil {
		return canceledOrFailed(ctx, result, StageTerminal, err)
	}
	if err := local.MakeRaw(ctx); err != nil {
		return canceledOrFailed(ctx, result, StageTerminal, err)
	}
	if err := session.Shell(); err != nil {
		return canceledOrFailed(ctx, result, StageShell, err)
	}
	result.StartedAt = now()

	waitResult := make(chan error, 1)
	go func() { waitResult <- session.Wait() }()
	resizeResult := make(chan error, 1)
	resizeObserver.started()
	go forwardResizes(resizeCtx, session, resizes, resizeResult, resizeObserver)
	resizeJoined := false
	joinResize := func() error {
		resizeErr := <-resizeResult
		if !resizeJoined {
			resizeObserver.joined()
			resizeJoined = true
		}
		return resizeErr
	}

	var waitErr error
	select {
	case waitErr = <-waitResult:
		stopResize()
	case resizeErr := <-resizeResult:
		resizeObserver.joined()
		resizeJoined = true
		if resizeErr == nil {
			select {
			case waitErr = <-waitResult:
			case <-ctx.Done():
				_ = sessionCloser.Close()
				waitErr = <-waitResult
			}
		} else {
			_ = sessionCloser.Close()
			<-waitResult
			return canceledOrFailed(ctx, result, StageStream, resizeErr)
		}
	case <-ctx.Done():
		_ = sessionCloser.Close()
		waitErr = <-waitResult
	}
	stopResize()
	if !resizeJoined {
		_ = joinResize()
	}

	if err := ctx.Err(); err != nil {
		result.State = app.SessionCanceled
		result.Outcome = app.SessionOutcomeCanceled
		return result, err
	}
	if waitErr == nil {
		result.State = app.SessionSucceeded
		result.Outcome = app.SessionOutcomeSuccess
		return result, nil
	}
	var exitErr interface {
		error
		ExitStatus() int
	}
	if errors.As(waitErr, &exitErr) {
		status := exitErr.ExitStatus()
		result.State = app.SessionFailed
		result.Outcome = app.SessionOutcomeRemoteFailure
		result.RemoteExitStatus = &status
		return result, &RemoteExitError{status: status}
	}
	return result, stageError(StageStream, waitErr)
}

func forwardResizes(ctx context.Context, session remoteSession, events <-chan terminal.Size, result chan<- error, observer *resizeLifecycleObserver) {
	for {
		select {
		case <-ctx.Done():
			result <- nil
			return
		case size, ok := <-events:
			if !ok {
				result <- nil
				return
			}
			if !size.Valid() {
				continue
			}
			observer.processed()
			if err := session.WindowChange(size.Rows, size.Columns); err != nil {
				result <- err
				return
			}
		}
	}
}

type resizeLifecycleObserver struct {
	start   func()
	process func()
	join    func()
}

func (observer *resizeLifecycleObserver) started() {
	if observer != nil && observer.start != nil {
		observer.start()
	}
}

func (observer *resizeLifecycleObserver) processed() {
	if observer != nil && observer.process != nil {
		observer.process()
	}
}

func (observer *resizeLifecycleObserver) joined() {
	if observer != nil && observer.join != nil {
		observer.join()
	}
}

type closeOnce struct {
	closer io.Closer
	once   sync.Once
	err    error
}

func newCloseOnce(closer io.Closer) *closeOnce {
	if closer == nil {
		return nil
	}
	return &closeOnce{closer: closer}
}

func (c *closeOnce) Close() error {
	if c == nil {
		return nil
	}
	c.once.Do(func() { c.err = c.closer.Close() })
	return c.err
}

type resourceSet struct {
	session   *closeOnce
	transport *closeOnce
	raw       *closeOnce
	auth      *closeOnce
}

func (r *resourceSet) close() error {
	var failures []error
	for _, resource := range []*closeOnce{r.session, r.transport, r.raw, r.auth} {
		if err := resource.Close(); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("release SSH resources: %w", errors.Join(failures...))
}
