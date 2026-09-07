package catalog

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestTrustedHostRepositoryPersistAndReplace(t *testing.T) {
	store, err := Open(testCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository := NewTrustedHostRepository(store)
	ctx := context.Background()

	first := catalogPresentedHost(t, "Example.COM.", 22)
	created, err := repository.TrustHost(ctx, TrustHostRequest{Host: first})
	if err != nil {
		t.Fatalf("TrustHost(create) error = %v", err)
	}
	if created.Revision != 1 || created.CanonicalHost != "example.com" || created.ID == "" {
		t.Fatalf("TrustHost(create) = %+v", created)
	}

	got, err := repository.GetTrustedHost(ctx, HostEndpoint{CanonicalHost: "EXAMPLE.COM", Port: 22})
	if err != nil {
		t.Fatalf("GetTrustedHost() error = %v", err)
	}
	if got.ID != created.ID || got.FingerprintSHA256 != created.FingerprintSHA256 {
		t.Fatalf("GetTrustedHost() = %+v, want ID %q", got, created.ID)
	}

	second := catalogPresentedHost(t, "example.com", 22)
	if _, err := repository.TrustHost(ctx, TrustHostRequest{Host: second}); !errors.Is(err, ErrTrustedHostConflict) {
		t.Fatalf("replacement without revision error = %v, want conflict", err)
	}
	expected := created.Revision
	replaced, err := repository.TrustHost(ctx, TrustHostRequest{Host: second, ExpectedRevision: &expected})
	if err != nil {
		t.Fatalf("TrustHost(replace) error = %v", err)
	}
	if replaced.ID != created.ID || replaced.Revision != 2 || replaced.FingerprintSHA256 == created.FingerprintSHA256 {
		t.Fatalf("TrustHost(replace) = %+v", replaced)
	}
}

func TestTrustedHostRepositoryRejectsConcurrentReplacement(t *testing.T) {
	store, err := Open(testCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository := NewTrustedHostRepository(store)
	ctx := context.Background()

	created, err := repository.TrustHost(ctx, TrustHostRequest{Host: catalogPresentedHost(t, "host", 2222)})
	if err != nil {
		t.Fatal(err)
	}
	observed := created.Revision
	firstReplacement := catalogPresentedHost(t, "host", 2222)
	committed, err := repository.TrustHost(ctx, TrustHostRequest{Host: firstReplacement, ExpectedRevision: &observed})
	if err != nil {
		t.Fatal(err)
	}

	staleReplacement := catalogPresentedHost(t, "host", 2222)
	if _, err := repository.TrustHost(ctx, TrustHostRequest{Host: staleReplacement, ExpectedRevision: &observed}); !errors.Is(err, ErrTrustedHostConflict) {
		t.Fatalf("stale TrustHost() error = %v, want conflict", err)
	}
	got, err := repository.GetTrustedHost(ctx, created.HostEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != committed.Revision || got.FingerprintSHA256 != committed.FingerprintSHA256 {
		t.Fatalf("stale replacement changed record: got %+v, want %+v", got, committed)
	}
}

func TestTrustedHostRepositoryRejectsInconsistentFingerprint(t *testing.T) {
	store, err := Open(testCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	host := catalogPresentedHost(t, "host", 22)
	host.FingerprintSHA256 = "SHA256:not-the-key"

	_, err = NewTrustedHostRepository(store).TrustHost(context.Background(), TrustHostRequest{Host: host})
	if !errors.Is(err, ErrInvalidTrustedHost) {
		t.Fatalf("TrustHost() error = %v, want invalid trusted host", err)
	}
}

func catalogPresentedHost(t *testing.T, host string, port uint16) PresentedHost {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return PresentedHost{
		Endpoint:          HostEndpoint{CanonicalHost: host, Port: port},
		KeyAlgorithm:      key.Type(),
		PublicKey:         key.Marshal(),
		FingerprintSHA256: ssh.FingerprintSHA256(key),
	}
}
