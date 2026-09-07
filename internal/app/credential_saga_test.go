package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/credential"
)

const (
	testConnectionID = NodeID("11111111111111111111111111111111")
	testOldRef       = "22222222222222222222222222222222"
)

func TestCredentialSagaSaveReplaceRemoveAndDelete(t *testing.T) {
	t.Run("save", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, "")
		if err := saga.Save(context.Background(), connection, []byte("saved-password")); err != nil {
			t.Fatal(err)
		}
		updated := readSagaConnection(t, store)
		if updated.CredentialRef == "" || updated.AuthMethod != AuthMethodPassword || updated.Revision != 2 {
			t.Fatalf("updated connection = %#v", updated)
		}
		assertCredential(t, secrets, updated.CredentialRef, "saved-password", true)
		assertNoCredentialOperations(t, store)
	})

	t.Run("replace", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, testOldRef)
		mustSetCredential(t, secrets, testOldRef, "old-password")
		if err := saga.Replace(context.Background(), connection, []byte("new-password")); err != nil {
			t.Fatal(err)
		}
		updated := readSagaConnection(t, store)
		if updated.CredentialRef == testOldRef || updated.CredentialRef == "" {
			t.Fatalf("replacement reference = %q", updated.CredentialRef)
		}
		assertCredential(t, secrets, testOldRef, "", false)
		assertCredential(t, secrets, updated.CredentialRef, "new-password", true)
		assertNoCredentialOperations(t, store)
	})

	t.Run("remove", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, testOldRef)
		mustSetCredential(t, secrets, testOldRef, "old-password")
		if err := saga.Remove(context.Background(), connection, AuthMethodKey, "/keys/id_ed25519"); err != nil {
			t.Fatal(err)
		}
		updated := readSagaConnection(t, store)
		if updated.CredentialRef != "" || updated.AuthMethod != AuthMethodKey || updated.IdentityFile != "/keys/id_ed25519" {
			t.Fatalf("updated connection = %#v", updated)
		}
		assertCredential(t, secrets, testOldRef, "", false)
		assertNoCredentialOperations(t, store)
	})

	t.Run("delete", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, testOldRef)
		mustSetCredential(t, secrets, testOldRef, "old-password")
		if err := saga.DeleteConnection(context.Background(), connection); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := store.DB().QueryRow(`SELECT count(*) FROM nodes WHERE id = ?`, testConnectionID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("connection count = %d", count)
		}
		assertCredential(t, secrets, testOldRef, "", false)
		assertNoCredentialOperations(t, store)
	})
}

func TestCredentialSagaCompensationAndRecovery(t *testing.T) {
	t.Run("save catalog failure removes new secret", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, "")
		want := errors.New("catalog fault")
		saga.repository = &faultSagaRepository{CredentialSagaRepository: saga.repository, applyErr: want}
		if err := saga.Save(context.Background(), connection, []byte("never-orphaned")); !errors.Is(err, want) {
			t.Fatalf("Save() error = %v, want catalog fault", err)
		}
		assertNoCredentialOperations(t, store)
		for _, call := range secrets.Calls() {
			if call.Operation == credential.OperationSet {
				assertCredential(t, secrets, string(call.Key.Reference), "", false)
			}
		}
	})

	t.Run("failed compensation stays tracked and startup completes", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, "")
		faults := &faultSagaRepository{CredentialSagaRepository: saga.repository, applyErr: errors.New("catalog fault")}
		saga.repository = faults
		secrets.SetFault(credential.OperationDelete, errors.New("vault locked"))
		const canary = "tracked-not-persisted-in-sqlite"
		err := saga.Save(context.Background(), connection, []byte(canary))
		if err == nil {
			t.Fatal("Save() error = nil")
		}
		if strings.Contains(err.Error(), canary) {
			t.Fatal("Save() error exposed secret")
		}
		operations := readCredentialOperations(t, store)
		if len(operations) != 1 || operations[0].Phase != CredentialPhaseSecretChanged {
			t.Fatalf("pending operations = %#v", operations)
		}
		assertCatalogHasNoText(t, store, canary)

		faults.applyErr = nil
		secrets.SetFault(credential.OperationDelete, nil)
		if err := saga.Recover(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := saga.Recover(context.Background()); err != nil {
			t.Fatalf("second Recover() error = %v", err)
		}
		updated := readSagaConnection(t, store)
		assertCredential(t, secrets, updated.CredentialRef, canary, true)
		assertNoCredentialOperations(t, store)
	})

	t.Run("remove catalog failure restores old secret", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, testOldRef)
		mustSetCredential(t, secrets, testOldRef, "restore-me")
		want := errors.New("catalog fault")
		saga.repository = &faultSagaRepository{CredentialSagaRepository: saga.repository, applyErr: want}
		if err := saga.Remove(context.Background(), connection, AuthMethodAgent, ""); !errors.Is(err, want) {
			t.Fatalf("Remove() error = %v", err)
		}
		assertCredential(t, secrets, testOldRef, "restore-me", true)
		if got := readSagaConnection(t, store); got.CredentialRef != testOldRef || got.Revision != 1 {
			t.Fatalf("connection after compensation = %#v", got)
		}
		assertNoCredentialOperations(t, store)
	})

	t.Run("ambiguous native delete remains recoverable", func(t *testing.T) {
		saga, store, secrets, connection := testCredentialSaga(t, testOldRef)
		mustSetCredential(t, secrets, testOldRef, "preserve-me")
		ambiguous := &ambiguousDeleteStore{Fake: secrets, fail: true}
		saga.store = ambiguous
		if err := saga.Remove(context.Background(), connection, AuthMethodAgent, ""); err == nil {
			t.Fatal("Remove() error = nil")
		}
		operations := readCredentialOperations(t, store)
		if len(operations) != 1 || operations[0].Phase != CredentialPhasePrepared {
			t.Fatalf("pending operations = %#v", operations)
		}

		ambiguous.mu.Lock()
		ambiguous.fail = false
		ambiguous.mu.Unlock()
		if err := saga.Recover(context.Background()); err != nil {
			t.Fatal(err)
		}
		assertCredential(t, secrets, testOldRef, "preserve-me", true)
		assertNoCredentialOperations(t, store)
	})
}

func TestCredentialSagaRecoversEveryDurablePhase(t *testing.T) {
	tests := []struct {
		name       string
		phase      CredentialOperationPhase
		newPresent bool
		oldPresent bool
	}{
		{name: "prepared before set aborts", phase: CredentialPhasePrepared},
		{name: "prepared after set resumes", phase: CredentialPhasePrepared, newPresent: true},
		{name: "secret changed resumes", phase: CredentialPhaseSecretChanged, newPresent: true},
		{name: "catalog changed finalizes", phase: CredentialPhaseCatalogChanged, newPresent: true},
		{name: "cleanup pending finalizes", phase: CredentialPhaseCleanupPending, newPresent: true, oldPresent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saga, store, secrets, connection := testCredentialSaga(t, testOldRef)
			operation := CredentialOperation{
				ID:               "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ConnectionID:     connection.ID,
				ExpectedRevision: connection.Revision,
				Kind:             CredentialOperationReplace,
				OldRef:           testOldRef,
				NewRef:           "33333333333333333333333333333333",
				Phase:            CredentialPhasePrepared,
				TargetAuthMethod: AuthMethodPassword,
				CreatedAt:        time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
			}
			repository := saga.repository.(*CatalogCredentialOperationRepository)
			if err := repository.CreateCredentialOperation(context.Background(), operation); err != nil {
				t.Fatal(err)
			}
			if tt.newPresent {
				mustSetCredential(t, secrets, operation.NewRef, "new-password")
			}
			if tt.oldPresent {
				mustSetCredential(t, secrets, operation.OldRef, "old-password")
			}
			if tt.phase != CredentialPhasePrepared {
				if err := repository.SetCredentialOperationPhase(context.Background(), operation.ID, CredentialPhaseSecretChanged); err != nil {
					t.Fatal(err)
				}
				if tt.phase == CredentialPhaseCatalogChanged || tt.phase == CredentialPhaseCleanupPending {
					if err := repository.ApplyCredentialOperation(context.Background(), operation.ID); err != nil {
						t.Fatal(err)
					}
					if tt.phase == CredentialPhaseCleanupPending {
						if err := repository.SetCredentialOperationPhase(context.Background(), operation.ID, tt.phase); err != nil {
							t.Fatal(err)
						}
					}
				}
			}

			if err := saga.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := saga.Recover(context.Background()); err != nil {
				t.Fatalf("idempotent Recover() error = %v", err)
			}
			assertNoCredentialOperations(t, store)
			if !tt.newPresent {
				if got := readSagaConnection(t, store); got.CredentialRef != testOldRef {
					t.Fatalf("aborted connection = %#v", got)
				}
				return
			}
			updated := readSagaConnection(t, store)
			if updated.CredentialRef != operation.NewRef {
				t.Fatalf("recovered reference = %q", updated.CredentialRef)
			}
			assertCredential(t, secrets, operation.NewRef, "new-password", true)
			assertCredential(t, secrets, operation.OldRef, "", false)
		})
	}
}

func TestCredentialSagaConcurrentSaveDoesNotLeaveOrphans(t *testing.T) {
	saga, store, secrets, connection := testCredentialSaga(t, "")
	var group sync.WaitGroup
	errorsSeen := make(chan error, 16)
	for i := range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsSeen <- saga.Save(context.Background(), connection, []byte(fmt.Sprintf("password-%d", i)))
		}()
	}
	group.Wait()
	close(errorsSeen)
	var successes int
	for err := range errorsSeen {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful saves = %d, want 1", successes)
	}
	assertNoCredentialOperations(t, store)
	active := readSagaConnection(t, store).CredentialRef
	for _, call := range secrets.Calls() {
		if call.Operation == credential.OperationSet && string(call.Key.Reference) != active {
			assertCredential(t, secrets, string(call.Key.Reference), "", false)
		}
	}
}

type faultSagaRepository struct {
	CredentialSagaRepository
	mu       sync.Mutex
	applyErr error
}

type ambiguousDeleteStore struct {
	*credential.Fake
	mu          sync.Mutex
	fail        bool
	afterDelete bool
}

func (s *ambiguousDeleteStore) Delete(ctx context.Context, key credential.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		s.afterDelete = true
		return credential.ErrUnavailable
	}
	return s.Fake.Delete(ctx, key)
}

func (s *ambiguousDeleteStore) Get(ctx context.Context, key credential.Key) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail && s.afterDelete {
		return nil, credential.ErrUnavailable
	}
	return s.Fake.Get(ctx, key)
}

func (r *faultSagaRepository) ApplyCredentialOperation(ctx context.Context, id string) error {
	r.mu.Lock()
	err := r.applyErr
	r.mu.Unlock()
	if err != nil {
		return err
	}
	return r.CredentialSagaRepository.ApplyCredentialOperation(ctx, id)
}

type sagaTestClock struct{}

func (sagaTestClock) Now() time.Time {
	return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
}

func testCredentialSaga(t *testing.T, reference string) (*CredentialSaga, *catalog.Store, *credential.Fake, Connection) {
	t.Helper()
	path := catalogPathForSagaTest(t)
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var rootID string
	if err := store.DB().QueryRow(`SELECT root_id FROM catalog_meta WHERE singleton = 1`).Scan(&rootID); err != nil {
		t.Fatal(err)
	}
	now := sagaTestClock{}.Now().Format(time.RFC3339Nano)
	if _, err := store.DB().Exec(`
		INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
		VALUES(?, ?, 'connection', 'server', 1, ?, ?)`, testConnectionID, rootID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO connections(node_id, host, port, auth_method, credential_ref)
		VALUES(?, 'server.example', 22, 'password', NULLIF(?, ''))`, testConnectionID, reference); err != nil {
		t.Fatal(err)
	}
	secrets := credential.NewFake()
	repository := NewCatalogCredentialOperationRepository(store)
	saga, err := NewCredentialSaga(repository, secrets, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", sagaTestClock{})
	if err != nil {
		t.Fatal(err)
	}
	connection := readSagaConnection(t, store)
	return saga, store, secrets, connection
}

func readSagaConnection(t *testing.T, store *catalog.Store) Connection {
	t.Helper()
	var connection Connection
	var reference, identityFile *string
	connection.ID = testConnectionID
	if err := store.DB().QueryRow(`
		SELECT n.revision, c.auth_method, c.identity_file, c.credential_ref
		FROM nodes n JOIN connections c ON c.node_id = n.id
		WHERE n.id = ?`, testConnectionID).Scan(&connection.Revision, &connection.AuthMethod, &identityFile, &reference); err != nil {
		t.Fatal(err)
	}
	if identityFile != nil {
		connection.IdentityFile = *identityFile
	}
	if reference != nil {
		connection.CredentialRef = *reference
	}
	return connection
}

func readCredentialOperations(t *testing.T, store *catalog.Store) []CredentialOperation {
	t.Helper()
	operations, err := NewCatalogCredentialOperationRepository(store).ListCredentialOperations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return operations
}

func assertNoCredentialOperations(t *testing.T, store *catalog.Store) {
	t.Helper()
	if operations := readCredentialOperations(t, store); len(operations) != 0 {
		t.Fatalf("pending credential operations = %#v", operations)
	}
}

func mustSetCredential(t *testing.T, store *credential.Fake, reference, secret string) {
	t.Helper()
	if err := store.Set(context.Background(), credential.Key{Scope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reference: credential.Reference(reference)}, []byte(secret)); err != nil {
		t.Fatal(err)
	}
}

func assertCredential(t *testing.T, store *credential.Fake, reference, want string, exists bool) {
	t.Helper()
	got, ok := store.Lookup(credential.Key{Scope: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reference: credential.Reference(reference)})
	if ok != exists || exists && string(got) != want {
		t.Fatalf("credential %q = %q, %t; want %q, %t", reference, got, ok, want, exists)
	}
}

func assertCatalogHasNoText(t *testing.T, store *catalog.Store, text string) {
	t.Helper()
	var count int
	if err := store.DB().QueryRow(`
		SELECT count(*) FROM credential_operations
		WHERE id LIKE '%' || ? || '%'
		   OR connection_id LIKE '%' || ? || '%'
		   OR coalesce(old_ref, '') LIKE '%' || ? || '%'
		   OR coalesce(new_ref, '') LIKE '%' || ? || '%'
		   OR coalesce(target_identity_file, '') LIKE '%' || ? || '%'`, text, text, text, text, text).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("catalog contains secret canary")
	}
}
