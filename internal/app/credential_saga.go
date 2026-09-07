package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/domain"
)

var (
	ErrCredentialSagaInvalid  = errors.New("invalid credential saga request")
	ErrCredentialUnverified   = errors.New("credential store change could not be verified")
	errCredentialStillPresent = errors.New("credential remains present")
)

// CredentialSagaRepository adds the one atomic catalog mutation required by
// the saga to the existing durable-operation repository contract.
type CredentialSagaRepository interface {
	CredentialOperationRepository
	ApplyCredentialOperation(context.Context, string) error
}

// CatalogCredentialOperationRepository adapts catalog's cycle-free primitive
// records to application types.
type CatalogCredentialOperationRepository struct {
	store *catalog.Store
}

func NewCatalogCredentialOperationRepository(store *catalog.Store) *CatalogCredentialOperationRepository {
	return &CatalogCredentialOperationRepository{store: store}
}

func (r *CatalogCredentialOperationRepository) CreateCredentialOperation(ctx context.Context, operation CredentialOperation) error {
	return r.store.CreateCredentialOperation(ctx, toCatalogCredentialOperation(operation))
}

func (r *CatalogCredentialOperationRepository) GetCredentialOperation(ctx context.Context, id string) (CredentialOperation, error) {
	operation, err := r.store.GetCredentialOperation(ctx, id)
	if err != nil {
		return CredentialOperation{}, err
	}
	return fromCatalogCredentialOperation(operation), nil
}

func (r *CatalogCredentialOperationRepository) ListCredentialOperations(ctx context.Context) ([]CredentialOperation, error) {
	operations, err := r.store.ListCredentialOperations(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]CredentialOperation, len(operations))
	for i := range operations {
		result[i] = fromCatalogCredentialOperation(operations[i])
	}
	return result, nil
}

func (r *CatalogCredentialOperationRepository) SetCredentialOperationPhase(ctx context.Context, id string, phase CredentialOperationPhase) error {
	return r.store.SetCredentialOperationPhase(ctx, id, string(phase))
}

func (r *CatalogCredentialOperationRepository) DeleteCredentialOperation(ctx context.Context, id string) error {
	return r.store.DeleteCredentialOperation(ctx, id)
}

func (r *CatalogCredentialOperationRepository) ApplyCredentialOperation(ctx context.Context, id string) error {
	return r.store.ApplyCredentialOperation(ctx, id)
}

// CredentialSaga coordinates a catalog with one OS credential namespace. The
// caller is responsible for obtaining explicit consent before calling Save or
// Replace; this service has no implicit-consent path.
type CredentialSaga struct {
	repository CredentialSagaRepository
	store      CredentialStore
	scope      credential.Scope
	clock      Clock
}

func NewCredentialSaga(repository CredentialSagaRepository, store CredentialStore, scope credential.Scope, clock Clock) (*CredentialSaga, error) {
	if repository == nil || store == nil || scope == "" {
		return nil, ErrCredentialSagaInvalid
	}
	if clock == nil {
		clock = wallClock{}
	}
	return &CredentialSaga{repository: repository, store: store, scope: scope, clock: clock}, nil
}

// Save persists a first remembered password for a connection.
func (s *CredentialSaga) Save(ctx context.Context, connection Connection, secret []byte) error {
	if connection.CredentialRef != "" {
		return fmt.Errorf("%w: save requires an empty credential reference", ErrCredentialSagaInvalid)
	}
	operation, err := s.newOperation(connection, CredentialOperationSave, "", AuthMethodPassword, "")
	if err != nil {
		return err
	}
	return s.writeCredential(ctx, operation, secret)
}

// Replace writes a new opaque reference before retiring the old credential.
func (s *CredentialSaga) Replace(ctx context.Context, connection Connection, secret []byte) error {
	if connection.CredentialRef == "" {
		return fmt.Errorf("%w: replace requires an existing credential reference", ErrCredentialSagaInvalid)
	}
	operation, err := s.newOperation(connection, CredentialOperationReplace, connection.CredentialRef, AuthMethodPassword, "")
	if err != nil {
		return err
	}
	return s.writeCredential(ctx, operation, secret)
}

// Remove forgets a remembered password and applies the caller-selected final
// authentication configuration.
func (s *CredentialSaga) Remove(ctx context.Context, connection Connection, target AuthMethod, identityFile string) error {
	if connection.CredentialRef == "" || !validCredentialTarget(target, identityFile) {
		return fmt.Errorf("%w: invalid remove target", ErrCredentialSagaInvalid)
	}
	operation, err := s.newOperation(connection, CredentialOperationRemove, connection.CredentialRef, target, identityFile)
	if err != nil {
		return err
	}
	return s.deleteCredential(ctx, operation)
}

// DeleteConnection removes a connection and its remembered credential, if any.
func (s *CredentialSaga) DeleteConnection(ctx context.Context, connection Connection) error {
	operation, err := s.newOperation(connection, CredentialOperationDeleteConnection, connection.CredentialRef, "", "")
	if err != nil {
		return err
	}
	return s.deleteCredential(ctx, operation)
}

// Recover resumes every durable operation. It is safe to call repeatedly and
// never needs secret bytes from the catalog.
func (s *CredentialSaga) Recover(ctx context.Context) error {
	operations, err := s.repository.ListCredentialOperations(ctx)
	if err != nil {
		return fmt.Errorf("list credential operations for recovery: %w", err)
	}
	for _, operation := range operations {
		if err := s.recoverOperation(ctx, operation); err != nil {
			return fmt.Errorf("recover credential operation %s: %w", operation.ID, err)
		}
	}
	return nil
}

func (s *CredentialSaga) newOperation(connection Connection, kind CredentialOperationKind, oldRef string, target AuthMethod, identityFile string) (CredentialOperation, error) {
	if _, err := domain.ParseID(string(connection.ID)); err != nil || connection.Revision == 0 {
		return CredentialOperation{}, fmt.Errorf("%w: connection identity", ErrCredentialSagaInvalid)
	}
	if oldRef != "" {
		if _, err := domain.ParseID(oldRef); err != nil {
			return CredentialOperation{}, fmt.Errorf("%w: credential reference", ErrCredentialSagaInvalid)
		}
	}

	id, err := domain.NewID()
	if err != nil {
		return CredentialOperation{}, fmt.Errorf("generate credential operation ID: %w", err)
	}
	operation := CredentialOperation{
		ID:                 id.String(),
		ConnectionID:       connection.ID,
		ExpectedRevision:   connection.Revision,
		Kind:               kind,
		OldRef:             oldRef,
		Phase:              CredentialPhasePrepared,
		TargetAuthMethod:   target,
		TargetIdentityFile: identityFile,
		CreatedAt:          s.clock.Now().UTC(),
	}
	if kind == CredentialOperationSave || kind == CredentialOperationReplace {
		newReference, err := domain.NewID()
		if err != nil {
			return CredentialOperation{}, fmt.Errorf("generate credential reference: %w", err)
		}
		operation.NewRef = newReference.String()
	}
	return operation, nil
}

func (s *CredentialSaga) writeCredential(ctx context.Context, operation CredentialOperation, secret []byte) error {
	if err := s.repository.CreateCredentialOperation(ctx, operation); err != nil {
		return fmt.Errorf("prepare credential operation: %w", err)
	}
	key := s.key(operation.NewRef)
	setErr := s.store.Set(ctx, key, secret)
	verifyErr := s.verifyValue(ctx, key, secret)
	if verifyErr != nil {
		failure := errors.Join(setErr, verifyErr)
		return s.compensateNewCredential(ctx, operation, failure)
	}
	if err := s.repository.SetCredentialOperationPhase(ctx, operation.ID, CredentialPhaseSecretChanged); err != nil {
		return s.compensateNewCredential(ctx, operation, fmt.Errorf("record stored credential: %w", err))
	}
	operation.Phase = CredentialPhaseSecretChanged

	state, err := s.applyAndInspect(ctx, operation)
	switch state {
	case catalogApplied:
		return s.finishApplied(ctx, operation)
	case catalogCompleted:
		return nil
	case catalogUnchanged:
		return s.compensateNewCredential(ctx, operation, err)
	default:
		return err
	}
}

func (s *CredentialSaga) deleteCredential(ctx context.Context, operation CredentialOperation) error {
	if err := s.repository.CreateCredentialOperation(ctx, operation); err != nil {
		return fmt.Errorf("prepare credential operation: %w", err)
	}

	var oldSecret []byte
	if operation.OldRef != "" {
		var err error
		oldSecret, err = s.store.Get(ctx, s.key(operation.OldRef))
		if err != nil {
			cleanupErr := s.repository.DeleteCredentialOperation(ctx, operation.ID)
			return errors.Join(fmt.Errorf("read credential before delete: %w", err), cleanupErr)
		}
		defer wipeSecret(oldSecret)
		if err := s.deleteAndVerify(ctx, s.key(operation.OldRef)); err != nil {
			if errors.Is(err, errCredentialStillPresent) {
				cleanupErr := s.repository.DeleteCredentialOperation(ctx, operation.ID)
				return errors.Join(err, cleanupErr)
			}
			// An ambiguous native result must remain durable for recovery.
			return err
		}
	}

	if err := s.repository.SetCredentialOperationPhase(ctx, operation.ID, CredentialPhaseSecretChanged); err != nil {
		return s.compensateOldCredential(ctx, operation, oldSecret, fmt.Errorf("record deleted credential: %w", err))
	}
	operation.Phase = CredentialPhaseSecretChanged
	state, err := s.applyAndInspect(ctx, operation)
	switch state {
	case catalogApplied:
		return s.finishApplied(ctx, operation)
	case catalogCompleted:
		return nil
	case catalogUnchanged:
		return s.compensateOldCredential(ctx, operation, oldSecret, err)
	default:
		return err
	}
}

type catalogApplyState uint8

const (
	catalogUnknown catalogApplyState = iota
	catalogUnchanged
	catalogApplied
	catalogCompleted
)

func (s *CredentialSaga) applyAndInspect(ctx context.Context, operation CredentialOperation) (catalogApplyState, error) {
	applyErr := s.repository.ApplyCredentialOperation(ctx, operation.ID)
	if applyErr == nil {
		if operation.Kind == CredentialOperationDeleteConnection {
			return catalogCompleted, nil
		}
		return catalogApplied, nil
	}

	current, inspectErr := s.repository.GetCredentialOperation(ctx, operation.ID)
	if inspectErr == nil {
		switch current.Phase {
		case CredentialPhaseCatalogChanged, CredentialPhaseCleanupPending:
			return catalogApplied, nil
		case CredentialPhasePrepared, CredentialPhaseSecretChanged:
			return catalogUnchanged, applyErr
		default:
			return catalogUnknown, errors.Join(applyErr, ErrCredentialUnverified)
		}
	}
	if isCredentialOperationNotFound(inspectErr) {
		// The only normal removal during Apply is the atomic connection delete.
		// Another recovery worker may also have completed any idempotent operation.
		return catalogCompleted, nil
	}
	return catalogUnknown, errors.Join(applyErr, fmt.Errorf("inspect catalog after apply failure: %w", inspectErr))
}

func (s *CredentialSaga) finishApplied(ctx context.Context, operation CredentialOperation) error {
	if operation.Kind != CredentialOperationReplace {
		if operation.Kind == CredentialOperationDeleteConnection {
			return nil
		}
		return s.repository.DeleteCredentialOperation(ctx, operation.ID)
	}
	if err := s.repository.SetCredentialOperationPhase(ctx, operation.ID, CredentialPhaseCleanupPending); err != nil {
		return fmt.Errorf("record old credential cleanup: %w", err)
	}
	if err := s.deleteAndVerify(ctx, s.key(operation.OldRef)); err != nil {
		return fmt.Errorf("clean up replaced credential: %w", err)
	}
	return s.repository.DeleteCredentialOperation(ctx, operation.ID)
}

func (s *CredentialSaga) compensateNewCredential(ctx context.Context, operation CredentialOperation, cause error) error {
	compensationErr := s.deleteAndVerify(ctx, s.key(operation.NewRef))
	if compensationErr != nil {
		return errors.Join(cause, fmt.Errorf("compensate new credential: %w", compensationErr))
	}
	cleanupErr := s.repository.DeleteCredentialOperation(ctx, operation.ID)
	return errors.Join(cause, cleanupErr)
}

func (s *CredentialSaga) compensateOldCredential(ctx context.Context, operation CredentialOperation, secret []byte, cause error) error {
	if operation.OldRef == "" {
		cleanupErr := s.repository.DeleteCredentialOperation(ctx, operation.ID)
		return errors.Join(cause, cleanupErr)
	}
	setErr := s.store.Set(ctx, s.key(operation.OldRef), secret)
	verifyErr := s.verifyValue(ctx, s.key(operation.OldRef), secret)
	if verifyErr != nil {
		return errors.Join(cause, setErr, fmt.Errorf("compensate old credential: %w", verifyErr))
	}
	cleanupErr := s.repository.DeleteCredentialOperation(ctx, operation.ID)
	return errors.Join(cause, cleanupErr)
}

func (s *CredentialSaga) recoverOperation(ctx context.Context, operation CredentialOperation) error {
	switch operation.Phase {
	case CredentialPhasePrepared:
		if err := s.recoverPrepared(ctx, &operation); err != nil {
			return err
		}
		if operation.Phase == CredentialPhasePrepared {
			return nil
		}
		fallthrough
	case CredentialPhaseSecretChanged:
		ready, err := s.recoverySecretState(ctx, operation)
		if err != nil || !ready {
			return err
		}
		state, err := s.applyAndInspect(ctx, operation)
		switch state {
		case catalogApplied:
			return s.finishApplied(ctx, operation)
		case catalogCompleted:
			return nil
		default:
			return err
		}
	case CredentialPhaseCatalogChanged, CredentialPhaseCleanupPending:
		if err := s.verifyAppliedRecoveryState(ctx, operation); err != nil {
			return err
		}
		return s.finishApplied(ctx, operation)
	default:
		return fmt.Errorf("%w: unknown recovery phase", ErrCredentialSagaInvalid)
	}
}

func (s *CredentialSaga) recoverPrepared(ctx context.Context, operation *CredentialOperation) error {
	if operation.Kind == CredentialOperationSave || operation.Kind == CredentialOperationReplace {
		secret, err := s.store.Get(ctx, s.key(operation.NewRef))
		wipeSecret(secret)
		if errors.Is(err, credential.ErrNotFound) {
			return s.repository.DeleteCredentialOperation(ctx, operation.ID)
		}
		if err != nil {
			return fmt.Errorf("inspect prepared credential: %w", err)
		}
	} else if operation.OldRef != "" {
		secret, err := s.store.Get(ctx, s.key(operation.OldRef))
		wipeSecret(secret)
		if err == nil {
			// Deletion did not complete, so preserve the old aggregate.
			return s.repository.DeleteCredentialOperation(ctx, operation.ID)
		}
		if !errors.Is(err, credential.ErrNotFound) {
			return fmt.Errorf("inspect prepared credential deletion: %w", err)
		}
	}
	if err := s.repository.SetCredentialOperationPhase(ctx, operation.ID, CredentialPhaseSecretChanged); err != nil {
		return err
	}
	operation.Phase = CredentialPhaseSecretChanged
	return nil
}

func (s *CredentialSaga) recoverySecretState(ctx context.Context, operation CredentialOperation) (bool, error) {
	var reference string
	wantPresent := operation.Kind == CredentialOperationSave || operation.Kind == CredentialOperationReplace
	if wantPresent {
		reference = operation.NewRef
	} else {
		reference = operation.OldRef
		if reference == "" {
			return true, nil
		}
	}
	secret, err := s.store.Get(ctx, s.key(reference))
	wipeSecret(secret)
	if wantPresent && err == nil || !wantPresent && errors.Is(err, credential.ErrNotFound) {
		return true, nil
	}
	if wantPresent && errors.Is(err, credential.ErrNotFound) || !wantPresent && err == nil {
		// The catalog is still unchanged in this phase, so abandoning is safe.
		return false, s.repository.DeleteCredentialOperation(ctx, operation.ID)
	}
	return false, fmt.Errorf("verify recovered credential state: %w", err)
}

func (s *CredentialSaga) verifyAppliedRecoveryState(ctx context.Context, operation CredentialOperation) error {
	switch operation.Kind {
	case CredentialOperationSave, CredentialOperationReplace:
		secret, err := s.store.Get(ctx, s.key(operation.NewRef))
		wipeSecret(secret)
		if err != nil {
			return fmt.Errorf("verify active recovered credential: %w", err)
		}
	case CredentialOperationRemove:
		if err := s.deleteAndVerify(ctx, s.key(operation.OldRef)); err != nil {
			return fmt.Errorf("verify removed recovered credential: %w", err)
		}
	}
	return nil
}

func (s *CredentialSaga) verifyValue(ctx context.Context, key credential.Key, expected []byte) error {
	actual, err := s.store.Get(ctx, key)
	if err != nil {
		return errors.Join(ErrCredentialUnverified, err)
	}
	defer wipeSecret(actual)
	if !bytes.Equal(actual, expected) {
		return ErrCredentialUnverified
	}
	return nil
}

func (s *CredentialSaga) deleteAndVerify(ctx context.Context, key credential.Key) error {
	deleteErr := s.store.Delete(ctx, key)
	secret, getErr := s.store.Get(ctx, key)
	wipeSecret(secret)
	if errors.Is(getErr, credential.ErrNotFound) {
		return nil
	}
	if getErr == nil {
		return errors.Join(deleteErr, ErrCredentialUnverified, errCredentialStillPresent)
	}
	return errors.Join(deleteErr, ErrCredentialUnverified, getErr)
}

func (s *CredentialSaga) key(reference string) credential.Key {
	return credential.Key{Scope: s.scope, Reference: credential.Reference(reference)}
}

func validCredentialTarget(method AuthMethod, identityFile string) bool {
	switch method {
	case AuthMethodAgent, AuthMethodPassword:
		return identityFile == ""
	case AuthMethodKey:
		return identityFile != ""
	default:
		return false
	}
}

func isCredentialOperationNotFound(err error) bool {
	return errors.Is(err, catalog.ErrCredentialOperationNotFound) || errors.Is(err, sql.ErrNoRows)
}

func toCatalogCredentialOperation(operation CredentialOperation) catalog.CredentialOperation {
	return catalog.CredentialOperation{
		ID:                 operation.ID,
		ConnectionID:       string(operation.ConnectionID),
		ExpectedRevision:   uint64(operation.ExpectedRevision),
		Kind:               string(operation.Kind),
		OldRef:             operation.OldRef,
		NewRef:             operation.NewRef,
		Phase:              string(operation.Phase),
		TargetAuthMethod:   string(operation.TargetAuthMethod),
		TargetIdentityFile: operation.TargetIdentityFile,
		CreatedAt:          operation.CreatedAt,
	}
}

func fromCatalogCredentialOperation(operation catalog.CredentialOperation) CredentialOperation {
	return CredentialOperation{
		ID:                 operation.ID,
		ConnectionID:       NodeID(operation.ConnectionID),
		ExpectedRevision:   Revision(operation.ExpectedRevision),
		Kind:               CredentialOperationKind(operation.Kind),
		OldRef:             operation.OldRef,
		NewRef:             operation.NewRef,
		Phase:              CredentialOperationPhase(operation.Phase),
		TargetAuthMethod:   AuthMethod(operation.TargetAuthMethod),
		TargetIdentityFile: operation.TargetIdentityFile,
		CreatedAt:          operation.CreatedAt,
	}
}

func wipeSecret(secret []byte) {
	for i := range secret {
		secret[i] = 0
	}
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

var _ CredentialSagaRepository = (*CatalogCredentialOperationRepository)(nil)
