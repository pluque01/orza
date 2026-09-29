package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/pluque01/orza/internal/credential"
)

func TestCommandServiceForwardsCapturedConnectionAndExactCommand(t *testing.T) {
	repository := newServiceRepository()
	runner := &fakeCommandRunner{run: func(_ context.Context, request SSHCommandRequest) (SSHSessionResult, error) {
		if request.Connection.ID != repository.connection.ID || request.Connection.Revision != repository.connection.Revision || request.Command != `printf 'one two' | cat` {
			t.Fatalf("request = %+v", request)
		}
		if request.Stdin == nil || request.Stdout == nil || request.Stderr == nil {
			t.Fatal("command streams were not forwarded")
		}
		return SSHSessionResult{State: SessionSucceeded, Outcome: SessionOutcomeSuccess}, nil
	}}
	service := mustCommandService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{result: HostTrustResult{Status: HostTrustKnown}}, &fakeTrustedHosts{}, credential.NewFake(), runner)
	stdin, stdout, stderr := bytes.NewBufferString("input"), new(bytes.Buffer), new(bytes.Buffer)
	result, err := service.Run(context.Background(), CommandRequest{Connection: ItemSelector{Path: repository.connection.Path}, Command: `printf 'one two' | cat`, Stdin: stdin, Stdout: stdout, Stderr: stderr})
	if err != nil || result.Connection != repository.connection || runner.calls != 1 {
		t.Fatalf("Run() = %+v, %v; calls=%d", result, err, runner.calls)
	}
}

func TestCommandServiceRejectsStaleTrustAndUnavailableSecretWithoutRunningCommand(t *testing.T) {
	for _, test := range []struct {
		name     string
		trust    HostTrustResult
		expected *Revision
		run      func(SSHCommandRequest) error
		want     error
	}{
		{name: "stale revision", expected: revisionPointerValue(2), want: ErrConflict},
		{name: "unknown host before secret", trust: HostTrustResult{Status: HostTrustUnknown}, run: func(request SSHCommandRequest) error {
			return request.VerifyHost(context.Background(), presentedHost())
		}, want: ErrTrustDecisionMissing},
		{name: "noninteractive secret", trust: HostTrustResult{Status: HostTrustKnown}, run: func(request SSHCommandRequest) error {
			if err := request.VerifyHost(context.Background(), presentedHost()); err != nil {
				return err
			}
			_, err := request.Secret(context.Background(), SecretRequest{Kind: SecretPassword})
			return err
		}, want: ErrNonInteractive},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := newServiceRepository()
			repository.connection.AuthMethod = AuthMethodPassword
			runner := &fakeCommandRunner{run: func(_ context.Context, request SSHCommandRequest) (SSHSessionResult, error) {
				if test.run != nil {
					return SSHSessionResult{}, test.run(request)
				}
				return SSHSessionResult{}, errors.New("runner called")
			}}
			service := mustCommandService(t, repository, &fakeCredentialLifecycle{repository: repository}, &fakeHostTrust{result: test.trust}, &fakeTrustedHosts{}, credential.NewFake(), runner)
			_, err := service.Run(context.Background(), CommandRequest{Connection: ItemSelector{ID: repository.connection.ID}, Expected: test.expected, Command: "true"})
			if !errors.Is(err, test.want) {
				t.Fatalf("Run() error = %v, want %v", err, test.want)
			}
			if test.expected != nil && runner.calls != 0 {
				t.Fatal("stale request reached runner")
			}
		})
	}
}

func mustCommandService(t *testing.T, repository ConnectionRepository, lifecycle CredentialLifecycle, trust HostTrust, trusted TrustedHostRepository, store CredentialStore, runner SSHCommandRunner) *CommandService {
	t.Helper()
	service, err := NewCommandService(CommandOptions{Connections: repository, Credentials: lifecycle, HostTrust: trust, TrustedHosts: trusted, Store: store, Scope: "catalog", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type fakeCommandRunner struct {
	run   func(context.Context, SSHCommandRequest) (SSHSessionResult, error)
	calls int
}

func (f *fakeCommandRunner) RunCommand(ctx context.Context, request SSHCommandRequest) (SSHSessionResult, error) {
	f.calls++
	return f.run(ctx, request)
}
