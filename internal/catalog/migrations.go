package catalog

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
)

const schemaVersion = 1

var ErrMigrationSequence = errors.New("catalog migrations are not contiguous")

//go:embed migrations/001_initial.sql
var initialMigration string

type migration struct {
	version int
	name    string
	sql     string
}

var catalogMigrations = []migration{
	{version: 1, name: "initial", sql: initialMigration},
}

func applyMigrations(ctx context.Context, db *sql.DB, currentVersion int, migrations []migration) error {
	for i, migration := range migrations {
		if migration.version != i+1 {
			return fmt.Errorf("%w: migration %q has version %d, want %d", ErrMigrationSequence, migration.name, migration.version, i+1)
		}
	}

	for _, migration := range migrations {
		if migration.version <= currentVersion {
			continue
		}
		if migration.version != currentVersion+1 {
			return fmt.Errorf("%w: migration %q follows version %d", ErrMigrationSequence, migration.name, currentVersion)
		}
		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
		currentVersion = migration.version
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration migration) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for migration %q: %w", migration.name, err)
	}
	defer conn.Close()

	if _, err = conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin migration %q: %w", migration.name, err)
	}
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	if _, err = conn.ExecContext(ctx, migration.sql); err != nil {
		return fmt.Errorf("apply migration %q: %w", migration.name, err)
	}
	if _, err = conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA application_id = %d`, applicationID)); err != nil {
		return fmt.Errorf("set application ID in migration %q: %w", migration.name, err)
	}
	if _, err = conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, migration.version)); err != nil {
		return fmt.Errorf("set schema version in migration %q: %w", migration.name, err)
	}
	if _, err = conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("commit migration %q: %w", migration.name, err)
	}
	return nil
}
