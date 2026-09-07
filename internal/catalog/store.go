package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	applicationID   = 0x4f525a41 // "ORZA"
	CatalogFileName = "catalog.db"
)

const busyTimeout = 5 * time.Second

var (
	ErrApplicationID = errors.New("file is not an Orza catalog")
	ErrNewerSchema   = errors.New("catalog schema is newer than this application")
	ErrIntegrity     = errors.New("catalog integrity check failed")
)

type Store struct {
	db *sql.DB
}

func DefaultPath() (string, error) {
	dir, err := LocalDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CatalogFileName), nil
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("catalog path is empty")
	}
	securePath, err := prepareCatalogPath(path)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", securePath)
	if err != nil {
		return nil, fmt.Errorf("open catalog: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(0)
	db.SetConnMaxLifetime(0)

	store := &Store{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// CatalogID returns the stable, non-secret identifier used to namespace
// credentials belonging to this catalog.
func (s *Store) CatalogID(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("catalog is not open")
	}
	var id string
	if err := s.db.QueryRowContext(ctx, `SELECT catalog_id FROM catalog_meta WHERE singleton = 1`).Scan(&id); err != nil {
		return "", fmt.Errorf("read catalog ID: %w", err)
	}
	return id, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) CheckIntegrity(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("catalog is not open")
	}
	return checkIntegrity(ctx, s.db)
}

// CheckWritableSchema verifies the file and metadata schema versions on a
// connection after its caller has begun a write transaction.
func (s *Store) CheckWritableSchema(ctx context.Context, connection *sql.Conn) error {
	if s == nil || s.db == nil || connection == nil {
		return errors.New("catalog is not open")
	}
	var fileVersion, metadataVersion int
	if err := connection.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&fileVersion); err != nil {
		return fmt.Errorf("read catalog schema version: %w", err)
	}
	if err := connection.QueryRowContext(ctx, `SELECT schema_version FROM catalog_meta WHERE singleton = 1`).Scan(&metadataVersion); err != nil {
		return fmt.Errorf("read catalog metadata schema version: %w", err)
	}
	if fileVersion > schemaVersion || metadataVersion > schemaVersion {
		return fmt.Errorf("%w: catalog schema version is %d/%d, supported version is %d", ErrNewerSchema, fileVersion, metadataVersion, schemaVersion)
	}
	if fileVersion != schemaVersion || metadataVersion != schemaVersion || fileVersion != metadataVersion {
		return fmt.Errorf("catalog schema version mismatch: file=%d metadata=%d supported=%d", fileVersion, metadataVersion, schemaVersion)
	}
	return nil
}

func (s *Store) initialize(ctx context.Context) error {
	application, err := queryPragmaInt(ctx, s.db, "application_id")
	if err != nil {
		return fmt.Errorf("read catalog application ID: %w", err)
	}
	version, err := queryPragmaInt(ctx, s.db, "user_version")
	if err != nil {
		return fmt.Errorf("read catalog schema version: %w", err)
	}
	if application != 0 && application != applicationID {
		return fmt.Errorf("%w: got %#x", ErrApplicationID, application)
	}
	if application == 0 {
		empty, err := schemaIsEmpty(ctx, s.db)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("%w: missing application ID", ErrApplicationID)
		}
	}
	if version > schemaVersion {
		return fmt.Errorf("%w: catalog is version %d, supported version is %d", ErrNewerSchema, version, schemaVersion)
	}
	if err := checkIntegrity(ctx, s.db); err != nil {
		return err
	}
	if err := configureConnection(ctx, s.db); err != nil {
		return err
	}

	if err := applyMigrations(ctx, s.db, version, catalogMigrations); err != nil {
		return err
	}
	if err := verifySchemaIdentity(ctx, s.db); err != nil {
		return err
	}
	return checkIntegrity(ctx, s.db)
}

func configureConnection(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = DELETE`,
		`PRAGMA synchronous = EXTRA`,
		fmt.Sprintf(`PRAGMA busy_timeout = %d`, busyTimeout.Milliseconds()),
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure catalog (%s): %w", pragma, err)
		}
	}
	return nil
}

func verifySchemaIdentity(ctx context.Context, db *sql.DB) error {
	application, err := queryPragmaInt(ctx, db, "application_id")
	if err != nil {
		return err
	}
	version, err := queryPragmaInt(ctx, db, "user_version")
	if err != nil {
		return err
	}
	if application != applicationID {
		return fmt.Errorf("%w: got %#x", ErrApplicationID, application)
	}
	if version != schemaVersion {
		return fmt.Errorf("catalog schema version is %d, want %d", version, schemaVersion)
	}

	var metadataVersion int
	if err := db.QueryRowContext(ctx, `SELECT schema_version FROM catalog_meta WHERE singleton = 1`).Scan(&metadataVersion); err != nil {
		return fmt.Errorf("read catalog metadata: %w", err)
	}
	if metadataVersion != version {
		return fmt.Errorf("catalog metadata schema version is %d, file schema version is %d", metadataVersion, version)
	}
	return nil
}

func queryPragmaInt(ctx context.Context, db *sql.DB, name string) (int, error) {
	var value int
	if err := db.QueryRowContext(ctx, "PRAGMA "+name).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func schemaIsEmpty(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_schema
		WHERE name NOT LIKE 'sqlite_%'
	`).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect catalog schema: %w", err)
	}
	return count == 0, nil
}

func checkIntegrity(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrIntegrity, err)
	}
	defer rows.Close()

	var failures []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("%w: %v", ErrIntegrity, err)
		}
		if !strings.EqualFold(result, "ok") {
			failures = append(failures, result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrIntegrity, err)
	}
	if len(failures) != 0 {
		return fmt.Errorf("%w: %s", ErrIntegrity, strings.Join(failures, "; "))
	}
	return nil
}
