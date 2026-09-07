// Package sshclient implements interactive SSH transport and session ownership.
package sshclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
)

// Stage identifies the operation that failed without exposing credentials.
type Stage string

const (
	StageTerminal       Stage = "terminal"
	StageHostVerify     Stage = "host_verification"
	StageAuthentication Stage = "authentication"
	StageDial           Stage = "dial"
	StageHandshake      Stage = "handshake"
	StageSession        Stage = "session"
	StagePTY            Stage = "pty"
	StageShell          Stage = "shell"
	StageStream         Stage = "stream"
	StageCleanup        Stage = "cleanup"
)

// Error is a transport or local-session failure at a stable lifecycle stage.
type Error struct {
	Stage Stage
	Err   error
}

func (e *Error) Error() string {
	if e == nil {
		return "ssh unknown failed"
	}
	return fmt.Sprintf("ssh %s failed", safeStage(e.Stage))
}
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func safeStage(stage Stage) Stage {
	switch stage {
	case StageTerminal, StageHostVerify, StageAuthentication, StageDial, StageHandshake,
		StageSession, StagePTY, StageShell, StageStream, StageCleanup:
		return stage
	default:
		return "unknown"
	}
}

// RemoteExitError reports a valid nonzero status returned by the remote process.
type RemoteExitError struct{ status int }

func (e *RemoteExitError) Error() string {
	return fmt.Sprintf("remote session exited with status %d", e.status)
}

// Status returns the remote process status.
func (e *RemoteExitError) Status() int { return e.status }

// AuthenticationFunc lazily constructs authentication methods. It is first
// called by the SSH protocol only after host-key verification has succeeded.
// The returned closer owns resources such as an agent connection.
type AuthenticationFunc func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error)

type dialContextFunc func(context.Context, string, string) (net.Conn, error)
type handshakeFunc func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error)

// Options exposes I/O and protocol seams used by deterministic lifecycle tests.
// Zero values select production implementations.
type Options struct {
	Authenticate AuthenticationFunc
	DialContext  dialContextFunc
	Handshake    handshakeFunc
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Now          func() time.Time
	resize       *resizeLifecycleObserver
}

// Client owns all resources for one Run invocation.
type Client struct{ options Options }

// New returns an SSH session runner. Authentication is deliberately injected
// by the authentication adapter so host verification can remain protocol-ordered.
func New(options Options) *Client {
	if options.Authenticate == nil {
		options.Authenticate = func(ctx context.Context, request app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			result, err := BuildAuthMethod(ctx, request.Connection, request.Secret)
			if err != nil {
				return nil, nil, err
			}
			return []ssh.AuthMethod{result.Method}, result, nil
		}
	}
	if options.DialContext == nil {
		options.DialContext = (&net.Dialer{}).DialContext
	}
	if options.Handshake == nil {
		options.Handshake = handshake
	}
	if options.Stdin == nil {
		options.Stdin = os.Stdin
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Client{options: options}
}

// Run establishes and owns one interactive session. Network and authentication
// resources are closed before the captured terminal is restored.
func (c *Client) Run(ctx context.Context, request app.SSHSessionRequest) (result app.SSHSessionResult, runErr error) {
	result.State = app.SessionFailed
	result.Outcome = app.SessionOutcomeTransportFailure
	if err := validateRequest(ctx, request); err != nil {
		return result, err
	}

	state, err := request.Terminal.Capture(ctx)
	if err != nil {
		return result, normalizePreActiveFailure(StageTerminal, err)
	}
	resources := &resourceSet{}
	defer func() {
		cleanupErr := resources.close()
		restoreErr := request.Terminal.Restore(context.WithoutCancel(ctx), state)
		if cleanupErr != nil {
			runErr = errors.Join(runErr, stageError(StageCleanup, cleanupErr))
		}
		if restoreErr != nil {
			runErr = errors.Join(runErr, stageError(StageTerminal, restoreErr))
		}
		if cleanupErr != nil || restoreErr != nil {
			result.State = app.SessionFailed
			result.Outcome = app.SessionOutcomeTransportFailure
		}
	}()

	size, err := request.Terminal.Size(ctx)
	if err != nil {
		return result, normalizePreActiveFailure(StageTerminal, err)
	}
	address := net.JoinHostPort(request.Connection.Host, strconv.Itoa(int(request.Connection.Port)))
	raw, err := c.options.DialContext(ctx, "tcp", address)
	if err != nil {
		return canceledOrFailed(ctx, result, StageDial, err)
	}
	if raw == nil {
		return result, normalizePreActiveFailure(StageDial, errors.New("dial returned no connection"))
	}
	resources.raw = newCloseOnce(raw)

	config, authState := c.clientConfig(ctx, request)
	transport, err := c.options.Handshake(ctx, raw, address, config)
	resources.auth = newCloseOnce(authState.closer())
	if err != nil {
		if authErr := authState.err(); authErr != nil {
			return canceledOrFailed(ctx, result, StageAuthentication, authErr)
		}
		var hostErr *hostVerificationError
		if errors.As(err, &hostErr) {
			return canceledOrFailed(ctx, result, StageHostVerify, hostErr.err)
		}
		return canceledOrFailed(ctx, result, StageHandshake, err)
	}
	if transport == nil {
		return result, normalizePreActiveFailure(StageHandshake, errors.New("handshake returned no client"))
	}
	resources.transport = newCloseOnce(transport)

	remote, err := transport.NewSession()
	if err != nil {
		return canceledOrFailed(ctx, result, StageSession, err)
	}
	if remote == nil {
		return result, normalizePreActiveFailure(StageSession, errors.New("SSH client returned no session"))
	}
	resources.session = newCloseOnce(remote)
	remote.SetIO(c.options.Stdin, c.options.Stdout, c.options.Stderr)
	if request.Activate != nil {
		if err := request.Activate(ctx); err != nil {
			return canceledOrFailed(ctx, result, StageTerminal, err)
		}
	}

	sessionResult, err := runSession(ctx, remote, resources.session, request.Terminal, size, c.options.Now, c.options.resize)
	return sessionResult, err
}

type authState struct {
	mu          sync.Mutex
	initialized bool
	methods     []ssh.AuthMethod
	next        int
	resource    io.Closer
	failure     error
}

type authFailureSource interface {
	authFailure() error
}

var errAuthenticationDenied = errors.New("authentication denied")

func (c *Client) clientConfig(ctx context.Context, request app.SSHSessionRequest) (*ssh.ClientConfig, *authState) {
	state := &authState{}
	config := &ssh.ClientConfig{User: request.Connection.Username}
	config.HostKeyCallback = func(_ string, remote net.Addr, key ssh.PublicKey) error {
		presented := app.PresentedHost{
			Endpoint: app.HostEndpoint{
				CanonicalHost: request.Connection.Host,
				Port:          request.Connection.Port,
			},
			KeyAlgorithm:      key.Type(),
			PublicKey:         append([]byte(nil), key.Marshal()...),
			FingerprintSHA256: ssh.FingerprintSHA256(key),
		}
		if remote != nil {
			presented.RemoteAddress = remote.String()
		}
		if err := request.VerifyHost(ctx, presented); err != nil {
			return &hostVerificationError{err: err}
		}
		return nil
	}
	config.AuthCallback = func(*ssh.ClientAuthContext) (ssh.AuthMethod, error) {
		state.mu.Lock()
		defer state.mu.Unlock()
		if !state.initialized {
			state.initialized = true
			state.methods, state.resource, state.failure = c.options.Authenticate(ctx, request)
			if state.failure != nil {
				return nil, state.failure
			}
		}
		if source, ok := state.resource.(authFailureSource); ok {
			if err := source.authFailure(); err != nil {
				state.failure = err
				return nil, err
			}
		}
		if state.next >= len(state.methods) {
			if state.next > 0 {
				state.failure = errAuthenticationDenied
			} else {
				state.failure = errors.New("authentication methods unavailable")
			}
			return nil, state.failure
		}
		method := state.methods[state.next]
		state.next++
		return method, nil
	}
	return config, state
}

func (s *authState) closer() io.Closer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resource
}

func (s *authState) err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}

type hostVerificationError struct{ err error }

func (e *hostVerificationError) Error() string { return e.err.Error() }
func (e *hostVerificationError) Unwrap() error { return e.err }

func validateRequest(ctx context.Context, request app.SSHSessionRequest) error {
	if ctx == nil {
		return errors.New("ssh session: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if request.Terminal == nil {
		return errors.New("ssh session: nil terminal")
	}
	if request.VerifyHost == nil {
		return errors.New("ssh session: nil host verifier")
	}
	if request.Connection.Host == "" || request.Connection.Port == 0 || request.Connection.Username == "" {
		return errors.New("ssh session: incomplete connection endpoint")
	}
	return nil
}

func canceledOrFailed(ctx context.Context, result app.SSHSessionResult, stage Stage, err error) (app.SSHSessionResult, error) {
	if contextErr := ctx.Err(); contextErr != nil {
		if errors.Is(contextErr, context.Canceled) {
			result.State = app.SessionCanceled
			result.Outcome = app.SessionOutcomeCanceled
		} else {
			result.State = app.SessionFailed
			result.Outcome = app.SessionOutcomeTransportFailure
		}
		return result, normalizePreActiveFailure(stage, contextErr)
	}
	result.State = app.SessionFailed
	result.Outcome = app.SessionOutcomeTransportFailure
	return result, normalizePreActiveFailure(stage, err)
}

func normalizePreActiveFailure(stage Stage, cause error) error {
	var normalized *app.SSHStartError
	if errors.As(cause, &normalized) {
		return normalized
	}
	staged := stageError(stage, cause)
	applicationStage := failureStage(stage)
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return app.NormalizeSSHStartError(applicationStage, staged)
	}

	switch stage {
	case StageDial:
		if network := classifyNetworkError(staged); network != nil {
			return network
		}
	case StageHostVerify:
		detail := "rejected"
		if errors.Is(cause, app.ErrHostRevoked) {
			detail = "revoked"
		}
		return app.NewSSHStartError(app.SSHFailureHostTrust, applicationStage, detail, staged)
	case StageAuthentication:
		if errors.Is(cause, errAuthenticationDenied) {
			return app.NewSSHStartError(app.SSHFailureAuthenticationDenied, applicationStage, "server rejected available authentication methods", staged)
		}
		return app.NewSSHStartError(app.SSHFailureCredentialUnavailable, applicationStage, "", staged)
	case StageHandshake:
		return app.NewSSHStartError(app.SSHFailureSSHNegotiation, applicationStage, "handshake", staged)
	case StageSession:
		return app.NewSSHStartError(app.SSHFailureSSHNegotiation, applicationStage, "session", staged)
	case StagePTY:
		return app.NewSSHStartError(app.SSHFailureSSHNegotiation, applicationStage, "pty", staged)
	case StageShell:
		return app.NewSSHStartError(app.SSHFailureSSHNegotiation, applicationStage, "shell", staged)
	}
	return app.NewSSHStartError(app.SSHFailureUnexpected, applicationStage, "", staged)
}

func failureStage(stage Stage) app.SSHFailureStage {
	switch stage {
	case StageDial:
		return app.SSHFailureStageNetworkConnection
	case StageHostVerify:
		return app.SSHFailureStageHostTrust
	case StageHandshake:
		return app.SSHFailureStageSSHNegotiation
	case StageAuthentication:
		return app.SSHFailureStageAuthentication
	case StageSession, StagePTY, StageShell:
		return app.SSHFailureStageSessionSetup
	case StageTerminal:
		return app.SSHFailureStageLocalTerminal
	default:
		return app.SSHFailureStageUnknown
	}
}

func stageError(stage Stage, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Stage: stage, Err: err}
}

func handshake(ctx context.Context, raw net.Conn, address string, config *ssh.ClientConfig) (sshTransport, error) {
	type response struct {
		transport sshTransport
		err       error
	}
	responses := make(chan response, 1)
	go func() {
		connection, channels, requests, err := ssh.NewClientConn(raw, address, config)
		if err != nil {
			responses <- response{err: err}
			return
		}
		responses <- response{transport: &clientTransport{client: ssh.NewClient(connection, channels, requests)}}
	}()
	select {
	case response := <-responses:
		return response.transport, response.err
	case <-ctx.Done():
		_ = raw.Close()
		response := <-responses
		if response.transport != nil {
			_ = response.transport.Close()
		}
		return nil, ctx.Err()
	}
}
