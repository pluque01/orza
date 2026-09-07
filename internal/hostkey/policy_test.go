package hostkey

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestPolicyClassifiesKnownUnknownAndChanged(t *testing.T) {
	knownKey := testPublicKey(t)
	otherKey := testPublicKey(t)
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{"example.com"}, knownKey) + "\n"
	if err := os.WriteFile(knownHosts, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := NewPolicy(&fakeTrustedHosts{}, knownHosts)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		host string
		key  ssh.PublicKey
		want app.HostTrustStatus
	}{
		{name: "known", host: "example.com", key: knownKey, want: app.HostTrustKnown},
		{name: "unknown", host: "new.example.com", key: otherKey, want: app.HostTrustUnknown},
		{name: "changed", host: "example.com", key: otherKey, want: app.HostTrustChanged},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			presented := mustPresentedHost(t, test.host, 22, test.key)
			result, err := policy.CheckHost(context.Background(), presented)
			if err != nil {
				t.Fatalf("CheckHost() error = %v", err)
			}
			if result.Status != test.want {
				t.Fatalf("CheckHost() status = %q, want %q", result.Status, test.want)
			}
		})
	}
}

func TestPolicyRevocationCannotBeOverridden(t *testing.T) {
	key := testPublicKey(t)
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	line := "@revoked " + knownhosts.Line([]string{"example.com"}, key) + "\n"
	if err := os.WriteFile(knownHosts, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &fakeTrustedHosts{trusted: trustedFromKey("example.com", 22, key)}
	policy, err := NewPolicy(repository, knownHosts)
	if err != nil {
		t.Fatal(err)
	}
	presented := mustPresentedHost(t, "example.com", 22, key)

	result, err := policy.CheckHost(context.Background(), presented)
	if err != nil || result.Status != app.HostTrustRevoked {
		t.Fatalf("CheckHost() = %+v, %v, want revoked", result, err)
	}
	if _, err := policy.DecideHost(context.Background(), presented, TrustOnce, nil); !errors.Is(err, ErrHostRevoked) {
		t.Fatalf("DecideHost(once) error = %v, want revoked", err)
	}
	if _, err := policy.DecideHost(context.Background(), presented, TrustPersist, nil); !errors.Is(err, ErrHostRevoked) {
		t.Fatalf("DecideHost(persist) error = %v, want revoked", err)
	}
	if repository.writes != 0 {
		t.Fatalf("revoked decision performed %d writes", repository.writes)
	}
}

func TestPolicyTrustOnceDoesNotPersist(t *testing.T) {
	repository := &fakeTrustedHosts{}
	policy := &Policy{repository: repository}
	presented := mustPresentedHost(t, "new.example.com", 22, testPublicKey(t))

	result, err := policy.DecideHost(context.Background(), presented, TrustOnce, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != app.HostTrustUnknown || result.Persisted != nil || repository.writes != 0 {
		t.Fatalf("DecideHost(once) = %+v, writes = %d", result, repository.writes)
	}
}

func TestPolicyPersistedTrustIsKnown(t *testing.T) {
	repository := &fakeTrustedHosts{}
	policy := &Policy{repository: repository}
	presented := mustPresentedHost(t, "new.example.com", 2222, testPublicKey(t))

	decision, err := policy.DecideHost(context.Background(), presented, TrustPersist, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Persisted == nil || repository.writes != 1 {
		t.Fatalf("DecideHost(persist) = %+v, writes = %d", decision, repository.writes)
	}
	checked, err := policy.CheckHost(context.Background(), presented)
	if err != nil || checked.Status != app.HostTrustKnown || checked.Known == nil {
		t.Fatalf("CheckHost() = %+v, %v, want persisted known host", checked, err)
	}
}

func TestPolicyPropagatesConcurrentReplacementConflict(t *testing.T) {
	oldKey := testPublicKey(t)
	newKey := testPublicKey(t)
	repository := &fakeTrustedHosts{
		trusted:  trustedFromKey("example.com", 22, oldKey),
		trustErr: catalog.ErrTrustedHostConflict,
	}
	policy := &Policy{repository: repository}
	presented := mustPresentedHost(t, "example.com", 22, newKey)
	staleRevision := app.Revision(1)

	_, err := policy.DecideHost(context.Background(), presented, TrustPersist, &staleRevision)
	if !errors.Is(err, catalog.ErrTrustedHostConflict) {
		t.Fatalf("DecideHost() error = %v, want concurrent replacement conflict", err)
	}
	if repository.writes != 1 {
		t.Fatalf("concurrent replacement writes = %d, want one rejected attempt", repository.writes)
	}
}

func TestPolicyRejectsInconsistentPresentedMetadata(t *testing.T) {
	policy := &Policy{repository: &fakeTrustedHosts{}}
	presented := mustPresentedHost(t, "host", 22, testPublicKey(t))
	presented.FingerprintSHA256 = "SHA256:forged"
	if _, err := policy.CheckHost(context.Background(), presented); !errors.Is(err, ErrInvalidHostKey) {
		t.Fatalf("CheckHost() error = %v, want invalid host key", err)
	}
}

type fakeTrustedHosts struct {
	trusted  *app.TrustedHost
	writes   int
	trustErr error
}

func (f *fakeTrustedHosts) GetTrustedHost(_ context.Context, endpoint app.HostEndpoint) (app.TrustedHost, error) {
	if f.trusted == nil || f.trusted.HostEndpoint != endpoint {
		return app.TrustedHost{}, sql.ErrNoRows
	}
	return *f.trusted, nil
}

func (f *fakeTrustedHosts) TrustHost(_ context.Context, request app.TrustHostRequest) (app.TrustedHost, error) {
	f.writes++
	if f.trustErr != nil {
		return app.TrustedHost{}, f.trustErr
	}
	trusted := app.TrustedHost{
		ID:                "11111111111111111111111111111111",
		HostEndpoint:      request.Host.Endpoint,
		KeyAlgorithm:      request.Host.KeyAlgorithm,
		PublicKey:         append([]byte(nil), request.Host.PublicKey...),
		FingerprintSHA256: request.Host.FingerprintSHA256,
		Revision:          1,
		AcceptedAt:        time.Now(),
	}
	f.trusted = &trusted
	return trusted, nil
}

func trustedFromKey(host string, port uint16, key ssh.PublicKey) *app.TrustedHost {
	return &app.TrustedHost{
		ID:                "11111111111111111111111111111111",
		HostEndpoint:      app.HostEndpoint{CanonicalHost: host, Port: port},
		KeyAlgorithm:      key.Type(),
		PublicKey:         key.Marshal(),
		FingerprintSHA256: ssh.FingerprintSHA256(key),
		Revision:          1,
		AcceptedAt:        time.Now(),
	}
}

func mustPresentedHost(t *testing.T, host string, port uint16, key ssh.PublicKey) app.PresentedHost {
	t.Helper()
	presented, err := NewPresentedHost(app.HostEndpoint{CanonicalHost: host, Port: port}, "192.0.2.10:22", key)
	if err != nil {
		t.Fatal(err)
	}
	return presented
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return key
}
