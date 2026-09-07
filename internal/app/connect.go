package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

type ConnectOptions struct {
	Connections  ConnectionRepository
	Credentials  CredentialLifecycle
	HostTrust    HostTrust
	TrustedHosts TrustedHostRepository
	Store        CredentialStore
	Scope        credential.Scope
	Runner       SSHSessionRunner
	Terminal     Terminal
}

type ConnectService struct {
	connections  ConnectionRepository
	credentials  CredentialLifecycle
	hostTrust    HostTrust
	trustedHosts TrustedHostRepository
	store        CredentialStore
	scope        credential.Scope
	runner       SSHSessionRunner
	terminal     Terminal
}

func NewConnectService(options ConnectOptions) (*ConnectService, error) {
	if options.Connections == nil || options.Credentials == nil || options.HostTrust == nil || options.TrustedHosts == nil || options.Store == nil || options.Scope == "" || options.Runner == nil || options.Terminal == nil {
		return nil, safeUseCaseError("configure connect service", "", ErrInvalidRequest)
	}
	return &ConnectService{
		connections:  options.Connections,
		credentials:  options.Credentials,
		hostTrust:    options.HostTrust,
		trustedHosts: options.TrustedHosts,
		store:        options.Store,
		scope:        options.Scope,
		runner:       options.Runner,
		terminal:     options.Terminal,
	}, nil
}

func (s *ConnectService) Connect(ctx context.Context, request ConnectRequest) (ConnectResult, error) {
	result := ConnectResult{Session: failedSessionResult(ctx)}
	target := selectorTarget(request.Connection)
	if err := contextError(ctx); err != nil {
		return result, safeUseCaseError("connect", target, err)
	}
	if err := validateSelector(request.Connection); err != nil {
		return result, safeUseCaseError("connect", target, err)
	}
	if request.Expected != nil && *request.Expected == 0 {
		return result, safeUseCaseError("connect", target, ErrInvalidRequest)
	}
	if !s.terminal.Interactive() {
		return result, safeUseCaseError("connect", target, ErrNonInteractive)
	}
	connectionResult, err := s.connections.GetConnection(ctx, request.Connection)
	if err != nil {
		return result, safeUseCaseError("resolve connection", target, err)
	}
	result.Connection = connectionResult.Connection
	result.Attempt = SSHAttemptTarget{
		ID:       result.Connection.ID,
		Revision: result.Connection.Revision,
		Path:     result.Connection.Path,
		Host:     result.Connection.Host,
		Port:     result.Connection.Port,
	}
	if request.Expected != nil && *request.Expected != result.Connection.Revision {
		return result, safeUseCaseError("connect", target, ErrConflict)
	}
	if err := s.credentials.Recover(ctx); err != nil {
		failure := normalizeConnectFailure(SSHFailureStageCredential, err)
		result.Session = sessionResultForFailure(failure)
		return result, safeUseCaseError("recover credentials before connect", target, failure)
	}

	gate := &hostVerificationGate{
		connection:   result.Connection,
		trust:        s.hostTrust,
		trustedHosts: s.trustedHosts,
		decide:       request.DecideTrust,
	}
	secret := func(secretCtx context.Context, secretRequest SecretRequest) ([]byte, error) {
		if !gate.verifiedHost() {
			return nil, safeUseCaseError("request authentication secret", target, ErrHostNotVerified)
		}
		if err := contextError(secretCtx); err != nil {
			return nil, safeUseCaseError("request authentication secret", target, err)
		}
		switch secretRequest.Kind {
		case SecretPassword:
			if result.Connection.CredentialRef != "" {
				stored, storeErr := s.store.Get(secretCtx, credential.Key{
					Scope:     s.scope,
					Reference: credential.Reference(result.Connection.CredentialRef),
				})
				if storeErr == nil {
					return stored, nil
				}
				wipeSecret(stored)
				if !errors.Is(storeErr, credential.ErrNotFound) && !errors.Is(storeErr, credential.ErrUnavailable) {
					cause := safeUseCaseError("read remembered password", target, storeErr)
					return nil, NewSSHStartError(SSHFailureCredentialUnavailable, SSHFailureStageCredential, "secure store", cause)
				}
			}
			return s.readPresentedSecret(secretCtx, secretRequest, terminal.SecretPrompt{Message: "Password"}, target, request.ReadSecret)
		case SecretPassphrase:
			return s.readPresentedSecret(secretCtx, secretRequest, terminal.SecretPrompt{Message: "Private key passphrase"}, target, request.ReadSecret)
		default:
			return nil, safeUseCaseError("request authentication secret", target, ErrInvalidRequest)
		}
	}

	session, runErr := s.runner.Run(ctx, SSHSessionRequest{
		Connection: result.Connection,
		VerifyHost: gate.verify,
		Secret:     secret,
		Activate:   request.Activate,
		Terminal:   s.terminal,
	})
	result.Session = session
	if runErr == nil && ctx.Err() != nil {
		failure := NormalizeSSHStartError(SSHFailureStageUnknown, ctx.Err())
		result.Session = sessionResultForFailure(failure)
		return result, safeUseCaseError("run SSH session", target, failure)
	}
	if runErr != nil {
		if !sessionWasActive(result.Session) {
			cause := runErr
			var normalized *SSHStartError
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) && !errors.As(runErr, &normalized) {
				cause = ctx.Err()
			}
			failure := normalizeConnectFailure(SSHFailureStageUnknown, cause)
			result.Session = sessionResultForFailure(failure)
			return result, safeUseCaseError("run SSH session", target, failure)
		}
		if result.Session.State == "" {
			result.Session = failedSessionResult(ctx)
		}
		return result, safeUseCaseError("run SSH session", target, runErr)
	}
	if result.Session.State == "" || result.Session.Outcome == "" {
		failure := normalizeConnectFailure(SSHFailureStageUnknown, errors.New("session runner returned an incomplete result"))
		result.Session = sessionResultForFailure(failure)
		return result, safeUseCaseError("run SSH session", target, failure)
	}
	return result, nil
}

func (s *ConnectService) readPresentedSecret(ctx context.Context, request SecretRequest, prompt terminal.SecretPrompt, target string, read func(context.Context, SecretRequest) ([]byte, error)) ([]byte, error) {
	if read == nil {
		return s.readTerminalSecret(ctx, prompt, target)
	}
	secret, err := read(ctx, request)
	if err != nil {
		wipeSecret(secret)
		return nil, terminalSecretError(target, err)
	}
	return secret, nil
}

func (s *ConnectService) readTerminalSecret(ctx context.Context, prompt terminal.SecretPrompt, target string) ([]byte, error) {
	secret, err := s.terminal.ReadSecret(ctx, prompt)
	if err != nil {
		wipeSecret(secret)
		return nil, terminalSecretError(target, err)
	}
	return secret, nil
}

func terminalSecretError(target string, err error) error {
	cause := safeUseCaseError("read secret from terminal", target, err)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return NormalizeSSHStartError(SSHFailureStageCredential, cause)
	}
	return NewSSHStartError(SSHFailureCredentialUnavailable, SSHFailureStageCredential, "secret prompt", cause)
}

type hostVerificationGate struct {
	mu           sync.Mutex
	connection   Connection
	trust        HostTrust
	trustedHosts TrustedHostRepository
	decide       TrustDecisionFunc
	verified     bool
}

func (g *hostVerificationGate) verify(ctx context.Context, presented PresentedHost) error {
	g.markUnverified()
	if err := contextError(ctx); err != nil {
		return NormalizeSSHStartError(SSHFailureStageHostTrust, safeUseCaseError("verify host", g.connection.Path, err))
	}
	if !sameEndpoint(g.connection, presented.Endpoint) {
		return hostTrustFailure("rejected", safeUseCaseError("verify host", g.connection.Path, ErrHostRejected))
	}
	checked, err := g.trust.CheckHost(ctx, clonePresentedHost(presented))
	if err != nil {
		return NormalizeSSHStartError(SSHFailureStageHostTrust, safeUseCaseError("check host trust", g.connection.Path, err))
	}

	switch checked.Status {
	case HostTrustKnown:
		g.markVerified()
		return nil
	case HostTrustRevoked:
		return hostTrustFailure("revoked", safeUseCaseError("verify host", g.connection.Path, ErrHostRevoked))
	case HostTrustUnknown, HostTrustChanged:
		if checked.Status == HostTrustChanged && (checked.Known == nil || checked.Known.Revision == 0) {
			return NormalizeSSHStartError(SSHFailureStageHostTrust, safeUseCaseError("verify host", g.connection.Path, ErrInvalidRequest))
		}
		if g.decide == nil {
			return hostTrustFailure(string(checked.Status), safeUseCaseError("verify host", g.connection.Path, ErrTrustDecisionMissing))
		}
	default:
		return NormalizeSSHStartError(SSHFailureStageHostTrust, safeUseCaseError("verify host", g.connection.Path, ErrInvalidRequest))
	}

	prompt := TrustDecisionPrompt{Host: clonePresentedHost(presented), Status: checked.Status, Known: cloneTrustedHost(checked.Known)}
	decision, decisionErr := g.decide(ctx, prompt)
	if decisionErr != nil {
		cause := safeUseCaseError("decide host trust", g.connection.Path, decisionErr)
		if errors.Is(decisionErr, context.Canceled) || errors.Is(decisionErr, context.DeadlineExceeded) {
			return NormalizeSSHStartError(SSHFailureStageHostTrust, cause)
		}
		return hostTrustFailure(string(checked.Status), cause)
	}
	switch decision {
	case TrustReject:
		return hostTrustFailure("rejected", safeUseCaseError("verify host", g.connection.Path, ErrHostRejected))
	case TrustOnce:
		g.markVerified()
		return nil
	case TrustPersist:
		var expected *Revision
		if checked.Known != nil {
			revision := checked.Known.Revision
			expected = &revision
		}
		_, persistErr := g.trustedHosts.TrustHost(ctx, TrustHostRequest{Host: clonePresentedHost(presented), ExpectedRevision: expected})
		if persistErr != nil {
			cause := safeUseCaseError("persist host trust", g.connection.Path, persistErr)
			if errors.Is(persistErr, context.Canceled) || errors.Is(persistErr, context.DeadlineExceeded) {
				return NormalizeSSHStartError(SSHFailureStageHostTrust, cause)
			}
			return hostTrustFailure(string(checked.Status), cause)
		}
		g.markVerified()
		return nil
	default:
		return NormalizeSSHStartError(SSHFailureStageHostTrust, safeUseCaseError("verify host", g.connection.Path, ErrInvalidRequest))
	}
}

func hostTrustFailure(detail string, cause error) error {
	return NewSSHStartError(SSHFailureHostTrust, SSHFailureStageHostTrust, detail, cause)
}

func (g *hostVerificationGate) markVerified() {
	g.mu.Lock()
	g.verified = true
	g.mu.Unlock()
}

func (g *hostVerificationGate) markUnverified() {
	g.mu.Lock()
	g.verified = false
	g.mu.Unlock()
}

func (g *hostVerificationGate) verifiedHost() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.verified
}

func sameEndpoint(connection Connection, endpoint HostEndpoint) bool {
	want := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(connection.Host)), ".")
	got := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(endpoint.CanonicalHost)), ".")
	return want != "" && want == got && connection.Port == endpoint.Port
}

func clonePresentedHost(host PresentedHost) PresentedHost {
	host.PublicKey = bytes.Clone(host.PublicKey)
	return host
}

func cloneTrustedHost(host *TrustedHost) *TrustedHost {
	if host == nil {
		return nil
	}
	cloned := *host
	cloned.PublicKey = bytes.Clone(host.PublicKey)
	return &cloned
}

func failedSessionResult(ctx context.Context) SSHSessionResult {
	if ctx != nil && ctx.Err() != nil {
		return canceledSessionResult()
	}
	return SSHSessionResult{State: SessionFailed, Outcome: SessionOutcomeTransportFailure}
}

func canceledSessionResult() SSHSessionResult {
	return SSHSessionResult{State: SessionCanceled, Outcome: SessionOutcomeCanceled}
}

func sessionResultForFailure(failure *SSHStartError) SSHSessionResult {
	presentation := failure.Presentation()
	if failure.Reason() == SSHFailureCanceled {
		return SSHSessionResult{State: SessionCanceled, Outcome: SessionOutcomeCanceled, Failure: &presentation}
	}
	return SSHSessionResult{State: SessionFailed, Outcome: SessionOutcomeTransportFailure, Failure: &presentation}
}

func sessionWasActive(result SSHSessionResult) bool {
	return !result.StartedAt.IsZero() || result.RemoteExitStatus != nil
}

func normalizeConnectFailure(stage SSHFailureStage, cause error) *SSHStartError {
	failure := NormalizeSSHStartError(stage, cause)
	if direct, ok := cause.(*SSHStartError); ok && direct == failure {
		return failure
	}
	var existing *SSHStartError
	if errors.As(cause, &existing) && existing == failure {
		presentation := existing.Presentation()
		return NewSSHStartError(existing.Reason(), existing.Stage(), presentation.TechnicalDetail, cause)
	}
	return failure
}
