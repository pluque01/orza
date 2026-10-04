package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/credential"
)

type tunnelRunnerFunc func(context.Context, TunnelRunRequest) error

func (f tunnelRunnerFunc) RunTunnel(ctx context.Context, r TunnelRunRequest) error { return f(ctx, r) }

func tunnelTestService(t *testing.T, runner tunnelRunnerFunc) (*TunnelService, *serviceRepository, *fakeCredentialLifecycle, *fakeHostTrust, *credential.Fake) {
	t.Helper()
	repo := newServiceRepository()
	life := &fakeCredentialLifecycle{repository: repo}
	trust := &fakeHostTrust{result: HostTrustResult{Status: HostTrustKnown}}
	store := credential.NewFake()
	s, err := NewTunnelService(TunnelOptions{Connections: repo, Credentials: life, HostTrust: trust, TrustedHosts: &fakeTrustedHosts{}, Store: store, Scope: "catalog", Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s, repo, life, trust, store
}

func tunnelTestRequest(repo *serviceRepository) TunnelRequest {
	rev := repo.connection.Revision
	return TunnelRequest{Connection: ItemSelector{ID: repo.connection.ID}, Expected: &rev, NonInteractive: true,
		Config: TunnelConfig{Mode: TunnelLocal, Listen: TunnelEndpoint{Port: 8080}, Destination: TunnelEndpoint{Host: "service.example", Port: 80}}}
}

func TestTunnelRejectsStaleBeforeNetwork(t *testing.T) {
	s, repo, life, _, _ := tunnelTestService(t, func(context.Context, TunnelRunRequest) error { t.Error("runner called"); return nil })
	r := tunnelTestRequest(repo)
	repo.connection.Revision++
	if _, err := s.Start(context.Background(), r); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale = %v", err)
	}
	if life.recovered != 0 || len(s.Snapshots()) != 0 {
		t.Fatal("stale crossed admission boundary")
	}
}

func TestTunnelSecurityAndSafeWait(t *testing.T) {
	for _, status := range []HostTrustStatus{HostTrustKnown, HostTrustUnknown, HostTrustChanged, HostTrustRevoked} {
		t.Run(string(status), func(t *testing.T) {
			s, repo, _, trust, store := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
				if _, err := r.Secret(ctx, SecretRequest{Kind: SecretPassword}); !errors.Is(err, ErrHostNotVerified) {
					return fmt.Errorf("missing secret gate: %w", err)
				}
				if err := r.VerifyHost(ctx, presentedHost()); err != nil {
					return err
				}
				secret, err := r.Secret(ctx, SecretRequest{Kind: SecretPassword})
				defer wipeSecret(secret)
				if err != nil {
					return err
				}
				if string(secret) != "tunnel-secret-canary" {
					return errors.New("wrong password")
				}
				return NewSSHStartError(SSHFailureAuthenticationDenied, SSHFailureStageAuthentication, "", errors.New("tunnel-secret-canary"))
			})
			trust.result = HostTrustResult{Status: status, Known: &TrustedHost{Revision: 1}}
			repo.connection.AuthMethod = AuthMethodPassword
			repo.connection.CredentialRef = serviceCredentialRef
			repo.connection.IdentityFile = "/private/tunnel-identity-canary"
			if err := store.Set(context.Background(), credential.Key{Scope: "catalog", Reference: serviceCredentialRef}, []byte("tunnel-secret-canary")); err != nil {
				t.Fatal(err)
			}
			r := tunnelTestRequest(repo)
			snap, err := s.Start(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Wait(context.Background(), snap.ID)
			var failure *SSHStartError
			if !errors.As(err, &failure) {
				t.Fatalf("lost safe failure: %v", err)
			}
			if status == HostTrustKnown && failure.Reason() != SSHFailureAuthenticationDenied {
				t.Fatalf("reason = %s", failure.Reason())
			}
			if status != HostTrustKnown && failure.Reason() != SSHFailureHostTrust {
				t.Fatalf("reason = %s", failure.Reason())
			}
			if strings.Contains(fmt.Sprint(s.Snapshots(), err), "tunnel-secret-canary") || strings.Contains(fmt.Sprint(s.Snapshots()), serviceCredentialRef) || strings.Contains(fmt.Sprint(s.Snapshots()), "tunnel-identity-canary") {
				t.Fatal("unsafe snapshot/error")
			}
		})
	}
}

func TestTunnelRecoveryFailurePreservesSafeClassification(t *testing.T) {
	s, repo, life, _, store := tunnelTestService(t, func(context.Context, TunnelRunRequest) error {
		t.Error("runner started after recovery failure")
		return nil
	})
	life.err = NewSSHStartError(SSHFailureCredentialUnavailable, SSHFailureStageCredential, "secure store", errors.New("recovery-canary"))
	snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Wait(context.Background(), snap.ID)
	var failure *SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != SSHFailureCredentialUnavailable || failure.Stage() != SSHFailureStageCredential {
		t.Fatalf("recovery failure = %v", err)
	}
	if strings.Contains(fmt.Sprint(err, s.Snapshots()), "recovery-canary") || len(store.Calls()) != 0 {
		t.Fatal("unsafe recovery failure")
	}
}

func TestTunnelWrongEndpointAndMissingRetryNeverAcquireSecrets(t *testing.T) {
	s, repo, _, trust, store := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
		host := presentedHost()
		host.Endpoint.Port++
		return r.VerifyHost(ctx, host)
	})
	snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Wait(context.Background(), snap.ID); !errors.Is(err, ErrHostRejected) {
		t.Fatalf("wrong endpoint = %v", err)
	}
	if trust.calls != 0 || len(store.Calls()) != 0 {
		t.Fatal("wrong endpoint crossed security boundary")
	}
	repo.getErr = ErrNotFound
	before, _ := s.Get(snap.ID)
	if _, err := s.Retry(context.Background(), snap.ID, tunnelTestRequest(repo)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing retry = %v", err)
	}
	if after, _ := s.Get(snap.ID); before != after {
		t.Fatal("missing retry changed entry")
	}
}

func TestTunnelKeepsControlledAdapterFailureContext(t *testing.T) {
	for _, text := range []string{"Forwarding listener unavailable.", "Forwarding client capacity reached.", "Proxy request rejected.", "Forwarding destination unavailable."} {
		t.Run(text, func(t *testing.T) {
			s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
				r.Diagnostic(text)
				return errors.New("raw-adapter-canary")
			})
			snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Wait(context.Background(), snap.ID); err == nil {
				t.Fatal("missing failure")
			}
			if got, _ := s.Get(snap.ID); got.State != TunnelFailed || got.Diagnostic != text {
				t.Fatalf("lost controlled context: %#v", got)
			}
		})
	}
}

func TestTunnelUnattendedNeverPromptsOrFallsBack(t *testing.T) {
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
		if err := r.VerifyHost(ctx, presentedHost()); err != nil {
			return err
		}
		_, err := r.Secret(ctx, SecretRequest{Kind: SecretPassword})
		return err
	})
	repo.connection.AuthMethod = AuthMethodPassword
	repo.connection.CredentialRef = serviceCredentialRef
	r := tunnelTestRequest(repo)
	r.ReadSecret = func(context.Context, SecretRequest) ([]byte, error) { t.Error("prompt called"); return nil, nil }
	if _, err := s.Start(context.Background(), r); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unattended callbacks accepted: %v", err)
	}
	r.ReadSecret = nil
	snap, err := s.Start(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	var failure *SSHStartError
	if err := s.Wait(context.Background(), snap.ID); !errors.As(err, &failure) || failure.Reason() != SSHFailureCredentialUnavailable {
		t.Fatalf("missing password = %v", err)
	}
}

func TestTunnelRequiresConfirmedStableTarget(t *testing.T) {
	s, repo, _, _, _ := tunnelTestService(t, func(context.Context, TunnelRunRequest) error { t.Error("unconfirmed runner called"); return nil })
	for _, change := range []func(*TunnelRequest){
		func(r *TunnelRequest) { r.Expected = nil },
		func(r *TunnelRequest) { r.Connection = ItemSelector{Path: repo.connection.Path} },
	} {
		r := tunnelTestRequest(repo)
		change(&r)
		if _, err := s.Start(context.Background(), r); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("unconfirmed request accepted: %v", err)
		}
	}
}

func TestTunnelInteractiveTrustAndCredentialPolicy(t *testing.T) {
	for _, mode := range []AuthMethod{AuthMethodPassword, AuthMethodKey, AuthMethodAgent} {
		t.Run(string(mode), func(t *testing.T) {
			prompted := 0
			s, repo, _, trust, store := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
				if err := r.VerifyHost(ctx, presentedHost()); err != nil {
					return err
				}
				kind := SecretPassword
				if mode == AuthMethodKey {
					kind = SecretPassphrase
				}
				bytes, err := r.Secret(ctx, SecretRequest{Kind: kind})
				defer wipeSecret(bytes)
				if mode == AuthMethodAgent {
					if !errors.Is(err, ErrInvalidRequest) {
						return errors.New("agent accessed password")
					}
					return nil
				}
				if err != nil {
					return err
				}
				if string(bytes) != "interactive-canary" {
					return errors.New("wrong prompted value")
				}
				return nil
			})
			repo.connection.AuthMethod = mode
			trust.result = HostTrustResult{Status: HostTrustUnknown}
			r := tunnelTestRequest(repo)
			r.NonInteractive = false
			r.DecideTrust = func(context.Context, TrustDecisionPrompt) (TrustDecision, error) { return TrustOnce, nil }
			r.ReadSecret = func(context.Context, SecretRequest) ([]byte, error) {
				prompted++
				return []byte("interactive-canary"), nil
			}
			snap, err := s.Start(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Wait(context.Background(), snap.ID); err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode == AuthMethodAgent {
				want = 0
			}
			if prompted != want || len(store.Calls()) != 0 {
				t.Fatalf("prompt count = %d; store = %v", prompted, store.Calls())
			}
		})
	}
}
