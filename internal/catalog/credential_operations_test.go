package catalog

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCredentialOperationRepositoryDurabilityAndTransitions(t *testing.T) {
	store, connectionID := testCredentialCatalog(t, "")
	ctx := context.Background()
	operation := testCredentialOperation(connectionID, "save", "", "22222222222222222222222222222222")

	if err := store.CreateCredentialOperation(ctx, operation); err != nil {
		t.Fatalf("CreateCredentialOperation() error = %v", err)
	}
	got, err := store.GetCredentialOperation(ctx, operation.ID)
	if err != nil {
		t.Fatalf("GetCredentialOperation() error = %v", err)
	}
	if got != operation {
		t.Fatalf("GetCredentialOperation() = %#v, want %#v", got, operation)
	}
	if err := store.CreateCredentialOperation(ctx, testCredentialOperation(connectionID, "save", "", "33333333333333333333333333333333")); !errors.Is(err, ErrCredentialOperationPending) {
		t.Fatalf("second CreateCredentialOperation() error = %v, want pending", err)
	}
	if err := store.SetCredentialOperationPhase(ctx, operation.ID, "catalog_changed"); !errors.Is(err, ErrInvalidCredentialOperation) {
		t.Fatalf("skipped phase error = %v, want invalid operation", err)
	}
	if err := store.SetCredentialOperationPhase(ctx, operation.ID, "secret_changed"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyCredentialOperation(ctx, operation.ID); err != nil {
		t.Fatalf("ApplyCredentialOperation() error = %v", err)
	}

	got, err = store.GetCredentialOperation(ctx, operation.ID)
	if err != nil || got.Phase != "catalog_changed" {
		t.Fatalf("applied operation = %#v, %v", got, err)
	}
	var reference string
	var revision uint64
	if err := store.DB().QueryRow(`
		SELECT c.credential_ref, n.revision
		FROM connections c JOIN nodes n ON n.id = c.node_id
		WHERE c.node_id = ?`, connectionID).Scan(&reference, &revision); err != nil {
		t.Fatal(err)
	}
	if reference != operation.NewRef || revision != 2 {
		t.Fatalf("connection reference/revision = %q/%d", reference, revision)
	}
	if err := store.ApplyCredentialOperation(ctx, operation.ID); err != nil {
		t.Fatalf("idempotent ApplyCredentialOperation() error = %v", err)
	}
}

func TestCredentialOperationDeleteIsAtomic(t *testing.T) {
	const oldRef = "44444444444444444444444444444444"
	store, connectionID := testCredentialCatalog(t, oldRef)
	operation := testCredentialOperation(connectionID, "delete_connection", oldRef, "")
	if err := store.CreateCredentialOperation(context.Background(), operation); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredentialOperationPhase(context.Background(), operation.ID, "secret_changed"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyCredentialOperation(context.Background(), operation.ID); err != nil {
		t.Fatal(err)
	}

	var nodes, operations int
	if err := store.DB().QueryRow(`SELECT count(*) FROM nodes WHERE id = ?`, connectionID).Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`SELECT count(*) FROM credential_operations WHERE id = ?`, operation.ID).Scan(&operations); err != nil {
		t.Fatal(err)
	}
	if nodes != 0 || operations != 0 {
		t.Fatalf("nodes/operations = %d/%d, want 0/0", nodes, operations)
	}
}

func testCredentialCatalog(t *testing.T, credentialRef string) (*Store, string) {
	t.Helper()
	store, err := Open(testCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	const connectionID = "11111111111111111111111111111111"
	var rootID string
	if err := store.DB().QueryRow(`SELECT root_id FROM catalog_meta WHERE singleton = 1`).Scan(&rootID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := store.DB().Exec(`
		INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
		VALUES(?, ?, 'connection', 'server', 1, ?, ?)`, connectionID, rootID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO connections(node_id, host, port, auth_method, credential_ref)
		VALUES(?, 'server.example', 22, 'password', NULLIF(?, ''))`, connectionID, credentialRef); err != nil {
		t.Fatal(err)
	}
	return store, connectionID
}

func testCredentialOperation(connectionID, kind, oldRef, newRef string) CredentialOperation {
	target := ""
	if kind == "save" || kind == "replace" {
		target = "password"
	}
	return CredentialOperation{
		ID:               "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionID:     connectionID,
		ExpectedRevision: 1,
		Kind:             kind,
		OldRef:           oldRef,
		NewRef:           newRef,
		Phase:            "prepared",
		TargetAuthMethod: target,
		CreatedAt:        time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}
}
