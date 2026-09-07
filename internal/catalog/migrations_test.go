package catalog

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInitialMigrationCreatesSchema(t *testing.T) {
	store, err := Open(testCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	wantTables := []string{"catalog_meta", "nodes", "connections", "trusted_hosts", "credential_operations"}
	for _, table := range wantTables {
		var count int
		if err := store.DB().QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %q count = %d, want 1", table, count)
		}
	}
	wantIndexes := []string{
		"nodes_parent_name_uq",
		"nodes_single_root_uq",
		"nodes_parent_idx",
		"nodes_kind_idx",
		"trusted_hosts_destination_uq",
		"credential_operations_connection_idx",
		"credential_operations_phase_idx",
	}
	for _, index := range wantIndexes {
		var count int
		if err := store.DB().QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("index %q count = %d, want 1", index, count)
		}
	}

	var rootID, catalogID string
	var roots, revision int
	if err := store.DB().QueryRow(`SELECT root_id, catalog_id, catalog_revision FROM catalog_meta WHERE singleton = 1`).Scan(&rootID, &catalogID, &revision); err != nil {
		t.Fatal(err)
	}
	if len(rootID) != 32 || len(catalogID) != 32 || rootID == catalogID || revision != 1 {
		t.Fatalf("invalid initial metadata: root=%q catalog=%q revision=%d", rootID, catalogID, revision)
	}
	if err := store.DB().QueryRow(`SELECT count(*) FROM nodes WHERE id = ? AND parent_id IS NULL AND kind = 'folder'`, rootID).Scan(&roots); err != nil {
		t.Fatal(err)
	}
	if roots != 1 {
		t.Fatalf("root nodes = %d, want 1", roots)
	}

	assertConstraint(t, store.DB(), `UPDATE nodes SET name = 'renamed-root' WHERE id = ?`, rootID)
	assertConstraint(t, store.DB(), `INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
		VALUES('11111111111111111111111111111111', ?, 'folder', 'bad/name', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, rootID)
	if _, err := store.DB().Exec(`INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
		VALUES('11111111111111111111111111111111', ?, 'connection', 'server', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, rootID); err != nil {
		t.Fatalf("insert connection node: %v", err)
	}
	assertConstraint(t, store.DB(), `INSERT INTO connections(node_id, host, port, auth_method, identity_file)
		VALUES(?, 'host', 22, 'agent', '/secret/key')`, "11111111111111111111111111111111")
	assertConstraint(t, store.DB(), `INSERT INTO trusted_hosts(id, canonical_host, port, key_algorithm, public_key, fingerprint_sha256, revision, accepted_at)
		VALUES('22222222222222222222222222222222', 'host', 70000, 'ssh-ed25519', x'01', 'SHA256:x', 1, '2026-01-01T00:00:00Z')`)
}

func TestMigrationRollsBackOnFailure(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	migrations := []migration{{version: 1, name: "broken", sql: `CREATE TABLE should_rollback(id INTEGER); SELECT * FROM missing_table;`}}
	err = applyMigrations(context.Background(), db, 0, migrations)
	if err == nil {
		t.Fatal("applyMigrations() succeeded")
	}

	var count, version int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name = 'should_rollback'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if count != 0 || version != 0 {
		t.Fatalf("failed migration persisted table count=%d version=%d", count, version)
	}
}

func TestMigrationsMustBeContiguous(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = applyMigrations(context.Background(), db, 0, []migration{{version: 2, name: "gap", sql: `SELECT 1`}})
	if !errors.Is(err, ErrMigrationSequence) {
		t.Fatalf("applyMigrations() error = %v, want ErrMigrationSequence", err)
	}
}

func assertConstraint(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err == nil {
		t.Fatalf("constraint accepted query %q", query)
	}
}
