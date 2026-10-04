package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/pluque01/orza/internal/credential"
)

func TestCredentialSagaRecoveryUnavailablePreservesPendingOperation(t *testing.T) {
	type recoveryCase struct {
		name        string
		kind        CredentialOperationKind
		phase       CredentialOperationPhase
		getFault    bool
		deleteFault bool
		calls       []credential.Operation
	}
	var cases []recoveryCase
	for _, kind := range []CredentialOperationKind{CredentialOperationSave, CredentialOperationReplace, CredentialOperationRemove, CredentialOperationDeleteConnection} {
		for _, phase := range []CredentialOperationPhase{CredentialPhasePrepared, CredentialPhaseSecretChanged} {
			cases = append(cases, recoveryCase{name: string(kind) + "/" + string(phase) + "/read", kind: kind, phase: phase, getFault: true, calls: []credential.Operation{credential.OperationGet}})
		}
	}
	for _, kind := range []CredentialOperationKind{CredentialOperationSave, CredentialOperationReplace} {
		cases = append(cases, recoveryCase{name: string(kind) + "/catalog_changed/verify", kind: kind, phase: CredentialPhaseCatalogChanged, getFault: true, calls: []credential.Operation{credential.OperationGet}})
	}
	cases = append(cases,
		recoveryCase{name: "replace/cleanup_pending/verify_active", kind: CredentialOperationReplace, phase: CredentialPhaseCleanupPending, getFault: true, calls: []credential.Operation{credential.OperationGet}},
		recoveryCase{name: "replace/cleanup_pending/delete", kind: CredentialOperationReplace, phase: CredentialPhaseCleanupPending, deleteFault: true, calls: []credential.Operation{credential.OperationGet, credential.OperationDelete, credential.OperationGet}},
		recoveryCase{name: "remove/catalog_changed/delete", kind: CredentialOperationRemove, phase: CredentialPhaseCatalogChanged, deleteFault: true, calls: []credential.Operation{credential.OperationDelete, credential.OperationGet}},
		recoveryCase{name: "remove/catalog_changed/verify_deleted", kind: CredentialOperationRemove, phase: CredentialPhaseCatalogChanged, getFault: true, calls: []credential.Operation{credential.OperationDelete, credential.OperationGet}},
		recoveryCase{name: "remove/catalog_changed/delete_and_verify_locked", kind: CredentialOperationRemove, phase: CredentialPhaseCatalogChanged, getFault: true, deleteFault: true, calls: []credential.Operation{credential.OperationDelete, credential.OperationGet}},
	)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			oldRef := testOldRef
			if tt.kind == CredentialOperationSave {
				oldRef = ""
			}
			saga, catalog, secrets, connection := testCredentialSaga(t, oldRef)
			operation := CredentialOperation{
				ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConnectionID: connection.ID,
				ExpectedRevision: connection.Revision, Kind: tt.kind, OldRef: oldRef,
				Phase: CredentialPhasePrepared, CreatedAt: sagaTestClock{}.Now(),
			}
			switch tt.kind {
			case CredentialOperationSave, CredentialOperationReplace:
				operation.NewRef = "33333333333333333333333333333333"
				operation.TargetAuthMethod = AuthMethodPassword
			case CredentialOperationRemove:
				operation.TargetAuthMethod = AuthMethodAgent
			}
			if err := saga.repository.CreateCredentialOperation(ctx, operation); err != nil {
				t.Fatal(err)
			}
			if tt.phase != CredentialPhasePrepared {
				if err := saga.repository.SetCredentialOperationPhase(ctx, operation.ID, CredentialPhaseSecretChanged); err != nil {
					t.Fatal(err)
				}
				if tt.phase == CredentialPhaseCatalogChanged || tt.phase == CredentialPhaseCleanupPending {
					if err := saga.repository.ApplyCredentialOperation(ctx, operation.ID); err != nil {
						t.Fatal(err)
					}
					if tt.phase == CredentialPhaseCleanupPending {
						if err := saga.repository.SetCredentialOperationPhase(ctx, operation.ID, tt.phase); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			// Include both physically absent and present credentials behind an
			// unavailable result. Neither can be inferred by the recovery owner.
			if tt.kind == CredentialOperationReplace || operation.NewRef != "" && tt.phase == CredentialPhaseCatalogChanged {
				mustSetCredential(t, secrets, operation.NewRef, "new-recovery-canary")
			}
			if tt.kind == CredentialOperationReplace || tt.deleteFault {
				mustSetCredential(t, secrets, operation.OldRef, "old-recovery-canary")
			}
			pending := readCredentialOperations(t, catalog)
			before := readSagaConnection(t, catalog)
			if len(pending) != 1 || pending[0].Phase != tt.phase {
				t.Fatalf("initial pending operations = %#v", pending)
			}
			unavailable := fmt.Errorf("locked backend: %w", credential.ErrUnavailable)
			if tt.getFault {
				secrets.SetFault(credential.OperationGet, unavailable)
			}
			if tt.deleteFault {
				secrets.SetFault(credential.OperationDelete, unavailable)
			}
			for range 2 {
				start := len(secrets.Calls())
				err := saga.Recover(ctx)
				if !errors.Is(err, credential.ErrUnavailable) || errors.Is(err, credential.ErrNotFound) {
					t.Fatalf("Recover() = %v, want unavailable, not absent", err)
				}
				if got := readCredentialOperations(t, catalog); !reflect.DeepEqual(got, pending) {
					t.Fatalf("unavailable recovery advanced or discarded pending state: got=%#v want=%#v", got, pending)
				}
				if got := readSagaConnection(t, catalog); got != before {
					t.Fatalf("unavailable recovery changed catalog: got=%#v want=%#v", got, before)
				}
				calls := secrets.Calls()[start:]
				if len(calls) != len(tt.calls) {
					t.Fatalf("backend calls = %#v, want %v; no retries or fallback allowed", calls, tt.calls)
				}
				for i, call := range calls {
					ref := operation.OldRef
					if operation.NewRef != "" && i == 0 {
						ref = operation.NewRef
					}
					if call.Operation != tt.calls[i] || call.Key != saga.key(ref) {
						t.Fatalf("backend call %d = %#v, want %s for captured reference", i, call, tt.calls[i])
					}
				}
				if tt.deleteFault {
					assertCredential(t, secrets, operation.OldRef, "old-recovery-canary", true)
				}
			}
			// Pending work remains recoverable once the same backend is available.
			secrets.SetFault(credential.OperationGet, nil)
			secrets.SetFault(credential.OperationDelete, nil)
			if err := saga.Recover(ctx); err != nil {
				t.Fatalf("Recover() after backend availability restored = %v", err)
			}
			assertNoCredentialOperations(t, catalog)
		})
	}
}
