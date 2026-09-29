package app

import (
	"context"
	"errors"
	"strings"

	"github.com/pluque01/orza/internal/credential"
)

type CommandOptions struct {
	Connections  ConnectionRepository
	Credentials  CredentialLifecycle
	HostTrust    HostTrust
	TrustedHosts TrustedHostRepository
	Store        CredentialStore
	Scope        credential.Scope
	Runner       SSHCommandRunner
}

type CommandService struct {
	connections  ConnectionRepository
	credentials  CredentialLifecycle
	hostTrust    HostTrust
	trustedHosts TrustedHostRepository
	store        CredentialStore
	scope        credential.Scope
	runner       SSHCommandRunner
}

func NewCommandService(options CommandOptions) (*CommandService, error) {
	if options.Connections == nil || options.Credentials == nil || options.HostTrust == nil || options.TrustedHosts == nil || options.Store == nil || options.Scope == "" || options.Runner == nil {
		return nil, safeUseCaseError("configure command service", "", ErrInvalidRequest)
	}
	return &CommandService{options.Connections, options.Credentials, options.HostTrust, options.TrustedHosts, options.Store, options.Scope, options.Runner}, nil
}

func (s *CommandService) Run(ctx context.Context, request CommandRequest) (result CommandResult, err error) {
	result.Session = failedSessionResult(ctx)
	target := selectorTarget(request.Connection)
	if err := contextError(ctx); err != nil {
		return result, safeUseCaseError("execute command", target, err)
	}
	if err := validateSelector(request.Connection); err != nil || strings.TrimSpace(request.Command) == "" {
		return result, safeUseCaseError("execute command", target, ErrInvalidRequest)
	}
	if request.Expected != nil && *request.Expected == 0 {
		return result, safeUseCaseError("execute command", target, ErrInvalidRequest)
	}
	resolved, err := s.connections.GetConnection(ctx, request.Connection)
	if err != nil {
		return result, safeUseCaseError("resolve connection", target, err)
	}
	result.Connection = resolved.Connection
	result.Attempt = SSHAttemptTarget{ID: result.Connection.ID, Revision: result.Connection.Revision, Path: result.Connection.Path, Host: result.Connection.Host, Port: result.Connection.Port}
	if request.Expected != nil && *request.Expected != result.Connection.Revision {
		return result, safeUseCaseError("execute command", target, ErrConflict)
	}
	if err := s.credentials.Recover(ctx); err != nil {
		return result, safeUseCaseError("recover credentials before command", target, normalizeConnectFailure(SSHFailureStageCredential, err))
	}
	gate := &hostVerificationGate{connection: result.Connection, trust: s.hostTrust, trustedHosts: s.trustedHosts}
	secret := func(secretCtx context.Context, secretRequest SecretRequest) ([]byte, error) {
		if !gate.verifiedHost() {
			return nil, safeUseCaseError("request authentication secret", target, ErrHostNotVerified)
		}
		if err := contextError(secretCtx); err != nil {
			return nil, safeUseCaseError("request authentication secret", target, err)
		}
		if secretRequest.Kind == SecretPassword && result.Connection.CredentialRef != "" {
			stored, storeErr := s.store.Get(secretCtx, credential.Key{Scope: s.scope, Reference: credential.Reference(result.Connection.CredentialRef)})
			if storeErr == nil {
				return stored, nil
			}
			wipeSecret(stored)
			if !errors.Is(storeErr, credential.ErrNotFound) && !errors.Is(storeErr, credential.ErrUnavailable) {
				return nil, NewSSHStartError(SSHFailureCredentialUnavailable, SSHFailureStageCredential, "secure store", storeErr)
			}
		}
		return nil, NewSSHStartError(SSHFailureCredentialUnavailable, SSHFailureStageCredential, "secret prompt", ErrNonInteractive)
	}
	result.Session, err = s.runner.RunCommand(ctx, SSHCommandRequest{Connection: result.Connection, Command: request.Command, VerifyHost: gate.verify, Secret: secret, Stdin: request.Stdin, Stdout: request.Stdout, Stderr: request.Stderr})
	if result.Session.RemoteExitStatus != nil {
		return result, err
	}
	if err != nil {
		failure := normalizeConnectFailure(SSHFailureStageSessionSetup, err)
		result.Session = sessionResultForFailure(failure)
		return result, safeUseCaseError("execute command", result.Connection.Path, failure)
	}
	return result, nil
}
