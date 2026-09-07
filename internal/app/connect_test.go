package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

func TestConnectServiceRecoversVerifiesPersistsThenReadsRememberedPassword(t *testing.T) {
	repository := newServiceRepository()
	repository.connection.AuthMethod = AuthMethodPassword
	repository.connection.CredentialRef = serviceCredentialRef
	lifecycle := &fakeCredentialLifecycle{repository: repository}
	trust := &fakeHostTrust{result: HostTrustResult{Status: HostTrustChanged, Known: &TrustedHost{Revision: 4}}}
	trusted := &fakeTrustedHosts{}
	store := credential.NewFake()
	secret := []byte("remembered-connect-password")
	if err := store.Set(context.Background(), credential.Key{Scope: "catalog", Reference: serviceCredentialRef}, secret); err != nil {
		t.Fatal(err)
	}
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	activated := false
	runner := &fakeSessionRunner{run: func(ctx context.Context, request SSHSessionRequest) (SSHSessionResult, error) {
		if lifecycle.recovered != 1 {
			t.Fatal("session started before credential recovery")
		}
		if _, err := request.Secret(ctx, SecretRequest{Kind: SecretPassword, Credential: serviceCredentialRef}); !errors.Is(err, ErrHostNotVerified) {
			t.Fatalf("secret before host verification error = %v", err)
		}
		if err := request.VerifyHost(ctx, presentedHost()); err != nil {
			return SSHSessionResult{}, err
		}
		got, err := request.Secret(ctx, SecretRequest{Kind: SecretPassword, Credential: serviceCredentialRef})
		if err != nil {
			return SSHSessionResult{}, err
		}
		defer wipeSecret(got)
		if string(got) != string(secret) {
			t.Fatalf("password = %q", got)
		}
		if request.Activate == nil {
			t.Fatal("session activation boundary was not propagated")
		}
		if err := request.Activate(ctx); err != nil {
			return SSHSessionResult{}, err
		}
		return SSHSessionResult{State: SessionSucceeded, Outcome: SessionOutcomeSuccess}, nil
	}}
	service := mustConnectService(t, repository, lifecycle, trust, trusted, store, runner, local)

	decisions := 0
	result, err := service.Connect(context.Background(), ConnectRequest{
		Connection: ItemSelector{ID: repository.connection.ID},
		Activate: func(context.Context) error {
			activated = true
			return nil
		},
		DecideTrust: func(_ context.Context, prompt TrustDecisionPrompt) (TrustDecision, error) {
			decisions++
			if prompt.Status != HostTrustChanged || prompt.Known == nil || prompt.Host.FingerprintSHA256 == "" {
				t.Fatalf("trust prompt = %#v", prompt)
			}
			return TrustPersist, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !activated {
		t.Fatal("session activation boundary did not run")
	}
	if result.Session.Outcome != SessionOutcomeSuccess || result.Connection.ID != repository.connection.ID {
		t.Fatalf("connect result = %#v", result)
	}
	if decisions != 1 || trusted.calls != 1 || trusted.request.ExpectedRevision == nil || *trusted.request.ExpectedRevision != 4 {
		t.Fatalf("decisions = %d, persisted = %#v", decisions, trusted)
	}
	if hasTerminalCall(local.Calls(), terminal.OperationReadSecret) {
		t.Fatalf("remembered password unexpectedly prompted: %#v", local.Calls())
	}
}

func TestConnectServiceTrustOnceAndPromptFallback(t *testing.T) {
	repository := newServiceRepository()
	repository.connection.AuthMethod = AuthMethodPassword
	repository.connection.CredentialRef = serviceCredentialRef
	lifecycle := &fakeCredentialLifecycle{repository: repository}
	trust := &fakeHostTrust{result: HostTrustResult{Status: HostTrustUnknown}}
	trusted := &fakeTrustedHosts{}
	store := credential.NewFake()
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueSecret([]byte("prompted-password"), nil)
	runner := &fakeSessionRunner{run: func(ctx context.Context, request SSHSessionRequest) (SSHSessionResult, error) {
		if err := request.VerifyHost(ctx, presentedHost()); err != nil {
			return SSHSessionResult{}, err
		}
		secret, err := request.Secret(ctx, SecretRequest{Kind: SecretPassword, Prompt: "Password"})
		if err != nil {
			return SSHSessionResult{}, err
		}
		defer wipeSecret(secret)
		if string(secret) != "prompted-password" {
			t.Fatalf("prompted secret = %q", secret)
		}
		return SSHSessionResult{State: SessionSucceeded, Outcome: SessionOutcomeSuccess}, nil
	}}
	service := mustConnectService(t, repository, lifecycle, trust, trusted, store, runner, local)

	_, err := service.Connect(context.Background(), ConnectRequest{
		Connection:  ItemSelector{ID: repository.connection.ID},
		DecideTrust: func(context.Context, TrustDecisionPrompt) (TrustDecision, error) { return TrustOnce, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if trusted.calls != 0 {
		t.Fatal("trust-once persisted a host")
	}
	if !hasTerminalCall(local.Calls(), terminal.OperationReadSecret) {
		t.Fatal("missing terminal password prompt")
	}
}

func TestConnectServiceRejectCancellationAndSafeErrors(t *testing.T) {
	t.Run("reject prevents secrets", func(t *testing.T) {
		repository := newServiceRepository()
		repository.connection.AuthMethod = AuthMethodPassword
		lifecycle := &fakeCredentialLifecycle{repository: repository}
		local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
		local.QueueSecret([]byte("must-not-be-read"), nil)
		runner := &fakeSessionRunner{run: func(ctx context.Context, request SSHSessionRequest) (SSHSessionResult, error) {
			err := request.VerifyHost(ctx, presentedHost())
			return SSHSessionResult{}, err
		}}
		service := mustConnectService(t, repository, lifecycle, &fakeHostTrust{result: HostTrustResult{Status: HostTrustUnknown}}, &fakeTrustedHosts{}, credential.NewFake(), runner, local)
		result, err := service.Connect(context.Background(), ConnectRequest{
			Connection:  ItemSelector{ID: repository.connection.ID},
			DecideTrust: func(context.Context, TrustDecisionPrompt) (TrustDecision, error) { return TrustReject, nil },
		})
		if !errors.Is(err, ErrHostRejected) || result.Session.State != SessionFailed {
			t.Fatalf("Connect() = %#v, %v", result, err)
		}
		if hasTerminalCall(local.Calls(), terminal.OperationReadSecret) {
			t.Fatal("secret prompted after host rejection")
		}
	})

	t.Run("canceled before dependencies", func(t *testing.T) {
		repository := newServiceRepository()
		lifecycle := &fakeCredentialLifecycle{repository: repository}
		runner := &fakeSessionRunner{}
		service := mustConnectService(t, repository, lifecycle, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := service.Connect(ctx, ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
		if !errors.Is(err, context.Canceled) || result.Session.Outcome != SessionOutcomeCanceled || runner.calls != 0 {
			t.Fatalf("Connect() = %#v, %v; runner calls = %d", result, err, runner.calls)
		}
	})

	t.Run("decision error is redacted", func(t *testing.T) {
		const canary = "trust-secret-canary"
		repository := newServiceRepository()
		lifecycle := &fakeCredentialLifecycle{repository: repository}
		runner := &fakeSessionRunner{run: func(ctx context.Context, request SSHSessionRequest) (SSHSessionResult, error) {
			return SSHSessionResult{}, request.VerifyHost(ctx, presentedHost())
		}}
		service := mustConnectService(t, repository, lifecycle, &fakeHostTrust{result: HostTrustResult{Status: HostTrustUnknown}}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))
		_, err := service.Connect(context.Background(), ConnectRequest{
			Connection: ItemSelector{ID: repository.connection.ID},
			DecideTrust: func(context.Context, TrustDecisionPrompt) (TrustDecision, error) {
				return TrustReject, errors.New(canary)
			},
		})
		if err == nil || strings.Contains(err.Error(), canary) {
			t.Fatalf("unsafe decision error = %v", err)
		}
	})
}

func TestConnectExpectedRevisionRejectsBeforeRecoveryOrNetwork(t *testing.T) {
	repository := newServiceRepository()
	lifecycle := &fakeCredentialLifecycle{repository: repository}
	runner := &fakeSessionRunner{}
	trust := &fakeHostTrust{result: HostTrustResult{Status: HostTrustKnown}}
	service := mustConnectService(t, repository, lifecycle, trust, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))
	expected := repository.connection.Revision
	repository.connection.Revision++

	result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}, Expected: &expected})
	if !errors.Is(err, ErrConflict) || result.Connection.ID != repository.connection.ID {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
	if lifecycle.recovered != 0 || runner.calls != 0 || trust.calls != 0 {
		t.Fatalf("stale connect crossed side-effect boundary: recovery=%d runner=%d trust=%d", lifecycle.recovered, runner.calls, trust.calls)
	}
}

func TestConnectNormalizesCredentialRecoveryFailure(t *testing.T) {
	tests := []struct {
		name       string
		recovery   error
		wantReason SSHFailureReason
		wantStage  SSHFailureStage
	}{
		{
			name:       "untyped",
			recovery:   errors.New("recover-password-canary"),
			wantReason: SSHFailureUnexpected,
			wantStage:  SSHFailureStageCredential,
		},
		{
			name: "wrapped typed",
			recovery: safeUseCaseError("credential recovery", "", NewSSHStartError(
				SSHFailureCredentialUnavailable,
				SSHFailureStageCredential,
				"secure store",
				errors.New("typed-recover-password-canary"),
			)),
			wantReason: SSHFailureCredentialUnavailable,
			wantStage:  SSHFailureStageCredential,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newServiceRepository()
			lifecycle := &fakeCredentialLifecycle{repository: repository, err: test.recovery}
			runner := &fakeSessionRunner{}
			service := mustConnectService(t, repository, lifecycle, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))

			result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
			if !errors.Is(err, test.recovery) {
				t.Fatalf("recovery wrapper was not preserved: %v", err)
			}
			var failure *SSHStartError
			if !errors.As(err, &failure) || failure.Reason() != test.wantReason || failure.Stage() != test.wantStage {
				t.Fatalf("Connect() failure = %#v, %v", result.Session.Failure, err)
			}
			if result.Session.State != SessionFailed || result.Session.Outcome != SessionOutcomeTransportFailure || result.Session.Failure == nil || result.Session.Failure.Category != test.wantReason || result.Session.Failure.Stage != test.wantStage {
				t.Fatalf("Connect() session = %#v", result.Session)
			}
			if runner.calls != 0 || strings.Contains(err.Error(), "canary") || strings.Contains(result.Session.Failure.TechnicalDetail, "canary") {
				t.Fatalf("unsafe recovery failure or runner call: session=%#v err=%v calls=%d", result.Session, err, runner.calls)
			}
		})
	}
}

func TestConnectNormalizesNilErrorIncompleteRunnerResult(t *testing.T) {
	const canary = "incomplete-password-canary"
	repository := newServiceRepository()
	runner := &fakeSessionRunner{run: func(context.Context, SSHSessionRequest) (SSHSessionResult, error) {
		return SSHSessionResult{Failure: &SSHFailurePresentation{TechnicalDetail: canary}}, nil
	}}
	service := mustConnectService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))

	result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
	var failure *SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != SSHFailureUnexpected || failure.Stage() != SSHFailureStageUnknown {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
	if result.Session.State != SessionFailed || result.Session.Outcome != SessionOutcomeTransportFailure || result.Session.Failure == nil || result.Session.Failure.Category != SSHFailureUnexpected || result.Session.Failure.Stage != SSHFailureStageUnknown {
		t.Fatalf("Connect() session = %#v", result.Session)
	}
	if strings.Contains(err.Error(), canary) || strings.Contains(result.Session.Failure.TechnicalDetail, canary) {
		t.Fatalf("incomplete result canary leaked: %#v, %v", result.Session.Failure, err)
	}
}

func TestConnectCapturesResolvedAttemptTarget(t *testing.T) {
	repository := newServiceRepository()
	service := mustConnectService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), &fakeSessionRunner{}, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))

	result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
	if err != nil {
		t.Fatal(err)
	}
	want := SSHAttemptTarget{ID: repository.connection.ID, Revision: repository.connection.Revision, Path: repository.connection.Path, Host: repository.connection.Host, Port: repository.connection.Port}
	if result.Attempt != want {
		t.Fatalf("Attempt = %#v, want %#v", result.Attempt, want)
	}
	if result.Attempt.Host == result.Connection.Username || result.Attempt.Path == result.Connection.CredentialRef || result.Attempt.Path == result.Connection.IdentityFile {
		t.Fatalf("attempt target contains a secret-bearing field: %#v", result.Attempt)
	}
}

func TestConnectClassifiesPreActiveDeadlineAndCancellation(t *testing.T) {
	tests := []struct {
		name        string
		runErr      error
		wantReason  SSHFailureReason
		wantState   SessionState
		wantOutcome SessionOutcome
	}{
		{"deadline", context.DeadlineExceeded, SSHFailureTimeout, SessionFailed, SessionOutcomeTransportFailure},
		{"cancellation", context.Canceled, SSHFailureCanceled, SessionCanceled, SessionOutcomeCanceled},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newServiceRepository()
			runner := &fakeSessionRunner{run: func(context.Context, SSHSessionRequest) (SSHSessionResult, error) {
				return SSHSessionResult{}, test.runErr
			}}
			service := mustConnectService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))

			result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
			if !errors.Is(err, test.runErr) || result.Session.State != test.wantState || result.Session.Outcome != test.wantOutcome || result.Session.Failure == nil || result.Session.Failure.Category != test.wantReason {
				t.Fatalf("Connect() = %#v, %v", result, err)
			}
			if result.Attempt.ID != repository.connection.ID {
				t.Fatalf("failed attempt target = %#v", result.Attempt)
			}
		})
	}
}

func TestConnectPreservesNormalizedPreActiveFailure(t *testing.T) {
	const canary = "raw-startup-secret"
	repository := newServiceRepository()
	normalized := NewSSHStartError(SSHFailureAuthenticationDenied, SSHFailureStageAuthentication, "server rejected available authentication methods", errors.New(canary))
	runner := &fakeSessionRunner{run: func(context.Context, SSHSessionRequest) (SSHSessionResult, error) {
		return SSHSessionResult{}, normalized
	}}
	service := mustConnectService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))

	result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
	var got *SSHStartError
	if !errors.As(err, &got) || got != normalized {
		t.Fatalf("normalized cause was not preserved: %v", err)
	}
	if result.Session.Failure == nil || result.Session.Failure.Category != SSHFailureAuthenticationDenied || strings.Contains(err.Error(), canary) {
		t.Fatalf("unsafe or missing presentation: %#v, %v", result.Session.Failure, err)
	}
}

func TestConnectPreservesHostTrustStatusAndApplicationWrappers(t *testing.T) {
	tests := []struct {
		name   string
		status HostTrustStatus
		decide TrustDecisionFunc
		cause  error
		detail string
	}{
		{"unknown rejected", HostTrustUnknown, func(context.Context, TrustDecisionPrompt) (TrustDecision, error) { return TrustReject, nil }, ErrHostRejected, "rejected"},
		{"changed decision failure", HostTrustChanged, func(context.Context, TrustDecisionPrompt) (TrustDecision, error) {
			return TrustReject, errors.New("private decision text")
		}, nil, "changed"},
		{"revoked", HostTrustRevoked, nil, ErrHostRevoked, "revoked"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newServiceRepository()
			known := (*TrustedHost)(nil)
			if test.status == HostTrustChanged {
				known = &TrustedHost{Revision: 1}
			}
			runner := &fakeSessionRunner{run: func(ctx context.Context, request SSHSessionRequest) (SSHSessionResult, error) {
				return SSHSessionResult{}, request.VerifyHost(ctx, presentedHost())
			}}
			service := mustConnectService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{result: HostTrustResult{Status: test.status, Known: known}}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))
			result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}, DecideTrust: test.decide})
			var failure *SSHStartError
			var wrapper *UseCaseError
			if !errors.As(err, &failure) || !errors.As(err, &wrapper) || failure.Reason() != SSHFailureHostTrust || failure.Stage() != SSHFailureStageHostTrust || failure.Presentation().TechnicalDetail != test.detail {
				t.Fatalf("Connect() = %#v, %v", result, err)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("cause lost: %v", err)
			}
			if result.Attempt.ID != repository.connection.ID {
				t.Fatalf("attempt lost: %#v", result.Attempt)
			}
		})
	}
}

func TestConnectKeepsPrimaryNormalizedFailureAheadOfJoinedCleanup(t *testing.T) {
	repository := newServiceRepository()
	primaryCause := errors.New("private primary")
	cleanupCause := errors.New("private cleanup")
	primary := NewSSHStartError(SSHFailureSSHNegotiation, SSHFailureStageSSHNegotiation, "handshake", primaryCause)
	runner := &fakeSessionRunner{run: func(context.Context, SSHSessionRequest) (SSHSessionResult, error) {
		return SSHSessionResult{}, errors.Join(primary, cleanupCause)
	}}
	service := mustConnectService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{}, &fakeTrustedHosts{}, credential.NewFake(), runner, terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}))
	result, err := service.Connect(context.Background(), ConnectRequest{Connection: ItemSelector{ID: repository.connection.ID}})
	var got *SSHStartError
	if !errors.As(err, &got) || got.Reason() != primary.Reason() || got.Stage() != primary.Stage() || !errors.Is(err, primaryCause) || !errors.Is(err, cleanupCause) {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
	if result.Session.Failure == nil || result.Session.Failure.Category != SSHFailureSSHNegotiation {
		t.Fatalf("failure = %#v", result.Session.Failure)
	}
}

func mustConnectService(t *testing.T, repository ConnectionRepository, lifecycle CredentialLifecycle, trust HostTrust, trusted TrustedHostRepository, store CredentialStore, runner SSHSessionRunner, local Terminal) *ConnectService {
	t.Helper()
	service, err := NewConnectService(ConnectOptions{
		Connections:  repository,
		Credentials:  lifecycle,
		HostTrust:    trust,
		TrustedHosts: trusted,
		Store:        store,
		Scope:        "catalog",
		Runner:       runner,
		Terminal:     local,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func presentedHost() PresentedHost {
	return PresentedHost{
		Endpoint:          HostEndpoint{CanonicalHost: "server.example", Port: 22},
		KeyAlgorithm:      "ssh-ed25519",
		PublicKey:         []byte("public-key"),
		FingerprintSHA256: "SHA256:public",
	}
}

type fakeHostTrust struct {
	result HostTrustResult
	err    error
	calls  int
}

func (f *fakeHostTrust) CheckHost(context.Context, PresentedHost) (HostTrustResult, error) {
	f.calls++
	return f.result, f.err
}

type fakeTrustedHosts struct {
	request TrustHostRequest
	calls   int
	err     error
}

func (f *fakeTrustedHosts) GetTrustedHost(context.Context, HostEndpoint) (TrustedHost, error) {
	return TrustedHost{}, errors.New("unused")
}

func (f *fakeTrustedHosts) TrustHost(_ context.Context, request TrustHostRequest) (TrustedHost, error) {
	f.calls++
	f.request = request
	return TrustedHost{Revision: 1}, f.err
}

type fakeSessionRunner struct {
	run   func(context.Context, SSHSessionRequest) (SSHSessionResult, error)
	calls int
}

func (f *fakeSessionRunner) Run(ctx context.Context, request SSHSessionRequest) (SSHSessionResult, error) {
	f.calls++
	if f.run == nil {
		return SSHSessionResult{State: SessionSucceeded, Outcome: SessionOutcomeSuccess}, nil
	}
	return f.run(ctx, request)
}

func hasTerminalCall(calls []terminal.Call, operation terminal.Operation) bool {
	for _, call := range calls {
		if call.Operation == operation {
			return true
		}
	}
	return false
}
