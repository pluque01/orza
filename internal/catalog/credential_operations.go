package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCredentialOperationNotFound = errors.New("credential operation not found")
	ErrCredentialOperationPending  = errors.New("credential operation already pending")
	ErrCredentialOperationConflict = errors.New("credential operation conflicts with connection")
	ErrInvalidCredentialOperation  = errors.New("invalid credential operation")
)

// CredentialOperation is the catalog representation of non-secret saga data.
// It deliberately uses primitive fields so catalog does not depend on app.
type CredentialOperation struct {
	ID                 string
	ConnectionID       string
	ExpectedRevision   uint64
	Kind               string
	OldRef             string
	NewRef             string
	Phase              string
	TargetAuthMethod   string
	TargetIdentityFile string
	CreatedAt          time.Time
}

func (s *Store) CreateCredentialOperation(ctx context.Context, operation CredentialOperation) error {
	if s == nil || s.db == nil {
		return errors.New("catalog is not open")
	}
	if err := validateCredentialOperation(operation); err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO credential_operations(
			id, connection_id, expected_revision, operation, old_ref, new_ref,
			phase, target_auth_method, target_identity_file, created_at
		)
		SELECT ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), ?
		WHERE NOT EXISTS (
			SELECT 1 FROM credential_operations WHERE connection_id = ?
		)`,
		operation.ID, operation.ConnectionID, operation.ExpectedRevision, operation.Kind,
		operation.OldRef, operation.NewRef, operation.Phase, operation.TargetAuthMethod,
		operation.TargetIdentityFile, operation.CreatedAt.UTC().Format(time.RFC3339Nano),
		operation.ConnectionID,
	)
	if err != nil {
		return fmt.Errorf("create credential operation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read created credential operation count: %w", err)
	}
	if rows == 0 {
		return ErrCredentialOperationPending
	}
	return nil
}

func (s *Store) GetCredentialOperation(ctx context.Context, id string) (CredentialOperation, error) {
	if s == nil || s.db == nil {
		return CredentialOperation{}, errors.New("catalog is not open")
	}
	operation, err := scanCredentialOperation(s.db.QueryRowContext(ctx, credentialOperationSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return CredentialOperation{}, ErrCredentialOperationNotFound
	}
	if err != nil {
		return CredentialOperation{}, fmt.Errorf("get credential operation: %w", err)
	}
	return operation, nil
}

func (s *Store) ListCredentialOperations(ctx context.Context) ([]CredentialOperation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("catalog is not open")
	}
	rows, err := s.db.QueryContext(ctx, credentialOperationSelect+` ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list credential operations: %w", err)
	}
	defer rows.Close()

	operations := make([]CredentialOperation, 0)
	for rows.Next() {
		operation, err := scanCredentialOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan credential operation: %w", err)
		}
		operations = append(operations, operation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list credential operations: %w", err)
	}
	return operations, nil
}

func (s *Store) SetCredentialOperationPhase(ctx context.Context, id, phase string) error {
	if s == nil || s.db == nil {
		return errors.New("catalog is not open")
	}
	if !validCredentialPhase(phase) {
		return fmt.Errorf("%w: phase", ErrInvalidCredentialOperation)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE credential_operations SET phase = ?
		WHERE id = ? AND (
			phase = ?
			OR (phase = 'prepared' AND ? = 'secret_changed')
			OR (phase = 'secret_changed' AND ? = 'catalog_changed')
			OR (phase = 'catalog_changed' AND ? = 'cleanup_pending')
		)`, phase, id, phase, phase, phase, phase)
	if err != nil {
		return fmt.Errorf("set credential operation phase: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated credential operation count: %w", err)
	}
	if rows != 0 {
		return nil
	}
	if _, err := s.GetCredentialOperation(ctx, id); err != nil {
		return err
	}
	return fmt.Errorf("%w: phase transition", ErrInvalidCredentialOperation)
}

func (s *Store) DeleteCredentialOperation(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return errors.New("catalog is not open")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM credential_operations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete credential operation: %w", err)
	}
	return nil
}

// ApplyCredentialOperation atomically changes the connection and records the
// catalog phase. delete_connection removes both rows in the same transaction.
func (s *Store) ApplyCredentialOperation(ctx context.Context, id string) (err error) {
	if s == nil || s.db == nil {
		return errors.New("catalog is not open")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin credential catalog change: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	operation, err := scanCredentialOperation(tx.QueryRowContext(ctx, credentialOperationSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCredentialOperationNotFound
	}
	if err != nil {
		return fmt.Errorf("read credential operation for apply: %w", err)
	}
	if operation.Phase == "catalog_changed" || operation.Phase == "cleanup_pending" {
		return tx.Commit()
	}
	if operation.Phase != "secret_changed" {
		return fmt.Errorf("%w: operation is in phase %q", ErrInvalidCredentialOperation, operation.Phase)
	}

	var revision uint64
	var currentRef sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT n.revision, c.credential_ref
		FROM nodes n JOIN connections c ON c.node_id = n.id
		WHERE n.id = ?`, operation.ConnectionID).Scan(&revision, &currentRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCredentialOperationConflict
		}
		return fmt.Errorf("read connection for credential operation: %w", err)
	}
	if revision != operation.ExpectedRevision || nullableString(currentRef) != operation.OldRef {
		return ErrCredentialOperationConflict
	}

	switch operation.Kind {
	case "save", "replace":
		if _, err := tx.ExecContext(ctx, `
			UPDATE connections
			SET auth_method = 'password', identity_file = NULL, credential_ref = ?
			WHERE node_id = ?`, operation.NewRef, operation.ConnectionID); err != nil {
			return fmt.Errorf("set connection credential reference: %w", err)
		}
	case "remove":
		if _, err := tx.ExecContext(ctx, `
			UPDATE connections
			SET auth_method = ?, identity_file = NULLIF(?, ''), credential_ref = NULL
			WHERE node_id = ?`, operation.TargetAuthMethod, operation.TargetIdentityFile, operation.ConnectionID); err != nil {
			return fmt.Errorf("clear connection credential reference: %w", err)
		}
	case "delete_connection":
		if _, err := tx.ExecContext(ctx, `DELETE FROM credential_operations WHERE id = ?`, id); err != nil {
			return fmt.Errorf("remove credential operation before connection delete: %w", err)
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE id = ? AND revision = ?`, operation.ConnectionID, operation.ExpectedRevision)
		if err != nil {
			return fmt.Errorf("delete connection for credential operation: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			return ErrCredentialOperationConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE catalog_meta SET catalog_revision = catalog_revision + 1 WHERE singleton = 1`); err != nil {
			return fmt.Errorf("advance catalog revision: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit connection delete credential operation: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("%w: kind", ErrInvalidCredentialOperation)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE nodes SET revision = revision + 1, updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), operation.ConnectionID); err != nil {
		return fmt.Errorf("advance connection revision: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE catalog_meta SET catalog_revision = catalog_revision + 1 WHERE singleton = 1`); err != nil {
		return fmt.Errorf("advance catalog revision: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE credential_operations SET phase = 'catalog_changed' WHERE id = ?`, id); err != nil {
		return fmt.Errorf("record credential catalog change: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit credential catalog change: %w", err)
	}
	return nil
}

const credentialOperationSelect = `
	SELECT id, connection_id, expected_revision, operation,
	       old_ref, new_ref, phase, target_auth_method, target_identity_file, created_at
	FROM credential_operations`

type credentialOperationScanner interface {
	Scan(...any) error
}

func scanCredentialOperation(scanner credentialOperationScanner) (CredentialOperation, error) {
	var operation CredentialOperation
	var oldRef, newRef, targetAuth, targetIdentity sql.NullString
	var createdAt string
	err := scanner.Scan(
		&operation.ID, &operation.ConnectionID, &operation.ExpectedRevision, &operation.Kind,
		&oldRef, &newRef, &operation.Phase, &targetAuth, &targetIdentity, &createdAt,
	)
	if err != nil {
		return CredentialOperation{}, err
	}
	operation.OldRef = nullableString(oldRef)
	operation.NewRef = nullableString(newRef)
	operation.TargetAuthMethod = nullableString(targetAuth)
	operation.TargetIdentityFile = nullableString(targetIdentity)
	operation.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return CredentialOperation{}, fmt.Errorf("parse credential operation timestamp: %w", err)
	}
	return operation, nil
}

func nullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func validateCredentialOperation(operation CredentialOperation) error {
	const maxSQLiteInteger = uint64(1<<63 - 1)
	if !canonicalID(operation.ID) || !canonicalID(operation.ConnectionID) || operation.ExpectedRevision == 0 || operation.ExpectedRevision > maxSQLiteInteger || operation.Phase != "prepared" || operation.CreatedAt.IsZero() {
		return ErrInvalidCredentialOperation
	}
	if operation.OldRef != "" && !canonicalID(operation.OldRef) || operation.NewRef != "" && !canonicalID(operation.NewRef) {
		return ErrInvalidCredentialOperation
	}
	switch operation.Kind {
	case "save":
		if operation.OldRef != "" || operation.NewRef == "" || operation.TargetAuthMethod != "password" || operation.TargetIdentityFile != "" {
			return ErrInvalidCredentialOperation
		}
	case "replace":
		if operation.OldRef == "" || operation.NewRef == "" || operation.OldRef == operation.NewRef || operation.TargetAuthMethod != "password" || operation.TargetIdentityFile != "" {
			return ErrInvalidCredentialOperation
		}
	case "remove":
		if operation.OldRef == "" || operation.NewRef != "" || !validAuthTarget(operation.TargetAuthMethod, operation.TargetIdentityFile) {
			return ErrInvalidCredentialOperation
		}
	case "delete_connection":
		if operation.NewRef != "" || operation.TargetAuthMethod != "" || operation.TargetIdentityFile != "" {
			return ErrInvalidCredentialOperation
		}
	default:
		return ErrInvalidCredentialOperation
	}
	return nil
}

func validAuthTarget(method, identity string) bool {
	switch method {
	case "agent", "password":
		return identity == ""
	case "key":
		return identity != ""
	default:
		return false
	}
}

func validCredentialPhase(phase string) bool {
	switch phase {
	case "prepared", "secret_changed", "catalog_changed", "cleanup_pending":
		return true
	default:
		return false
	}
}

func canonicalID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
