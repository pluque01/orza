package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenCreatesCatalogWithRequiredSettings(t *testing.T) {
	if applicationID != 0x4f525a41 {
		t.Fatalf("applicationID = %#x, want ORZA", applicationID)
	}

	path := testCatalogPath(t)

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if got := store.DB().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}

	checks := []struct {
		pragma string
		want   string
	}{
		{"foreign_keys", "1"},
		{"journal_mode", "delete"},
		{"synchronous", "3"},
		{"busy_timeout", fmt.Sprint(busyTimeout.Milliseconds())},
		{"application_id", fmt.Sprint(applicationID)},
		{"user_version", fmt.Sprint(schemaVersion)},
	}
	for _, check := range checks {
		var got string
		if err := store.DB().QueryRow("PRAGMA " + check.pragma).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", check.pragma, err)
		}
		if !strings.EqualFold(got, check.want) {
			t.Errorf("PRAGMA %s = %q, want %q", check.pragma, got, check.want)
		}
	}

	var metaVersion int
	if err := store.DB().QueryRow(`SELECT schema_version FROM catalog_meta WHERE singleton = 1`).Scan(&metaVersion); err != nil {
		t.Fatalf("read catalog metadata: %v", err)
	}
	if metaVersion != schemaVersion {
		t.Errorf("catalog_meta.schema_version = %d, want %d", metaVersion, schemaVersion)
	}

	if err := store.CheckIntegrity(context.Background()); err != nil {
		t.Errorf("CheckIntegrity() error = %v", err)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	path := testCatalogPath(t)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA application_id = %d; PRAGMA user_version = %d`, applicationID, schemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if store != nil {
		_ = store.Close()
	}
	if !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("Open() error = %v, want ErrNewerSchema", err)
	}
}

func TestOpenRejectsWrongApplicationID(t *testing.T) {
	path := testCatalogPath(t)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA application_id = 1234`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	store, err := Open(path)
	if store != nil {
		_ = store.Close()
	}
	if !errors.Is(err, ErrApplicationID) {
		t.Fatalf("Open() error = %v, want ErrApplicationID", err)
	}
}

func TestOpenRejectsMalformedCatalog(t *testing.T) {
	path := testCatalogPath(t)
	if err := os.WriteFile(path, []byte("not a sqlite catalog"), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if store != nil {
		_ = store.Close()
	}
	if err == nil {
		t.Fatal("Open() succeeded for malformed catalog")
	}
}

func TestOpenRejectsFailedIntegrityCheck(t *testing.T) {
	path := testCatalogPath(t)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`PRAGMA writable_schema = ON; UPDATE sqlite_schema SET rootpage = 2147483647 WHERE name = 'nodes'; PRAGMA writable_schema = OFF`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if store != nil {
		_ = store.Close()
	}
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Open() error = %v, want ErrIntegrity", err)
	}
}

func TestOpenRecoversHotJournal(t *testing.T) {
	if os.Getenv("ORZA_HOT_JOURNAL_HELPER") == "1" {
		writeHotJournal(t)
		return
	}

	path := testCatalogPath(t)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestOpenRecoversHotJournal$")
	cmd.Env = append(os.Environ(), "ORZA_HOT_JOURNAL_HELPER=1", "ORZA_HOT_JOURNAL_PATH="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("journal helper: %v\n%s", err, output)
	}
	if _, err := os.Stat(path + "-journal"); err != nil {
		t.Fatalf("hot journal was not created: %v", err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatalf("Open() with hot journal: %v", err)
	}
	defer store.Close()
	var revision int
	if err := store.DB().QueryRow(`SELECT catalog_revision FROM catalog_meta WHERE singleton = 1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if revision != 1 {
		t.Fatalf("catalog_revision = %d, want rolled back value 1", revision)
	}
}

func writeHotJournal(t *testing.T) {
	path := os.Getenv("ORZA_HOT_JOURNAL_PATH")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode = DELETE; PRAGMA synchronous = EXTRA; BEGIN IMMEDIATE; UPDATE catalog_meta SET catalog_revision = 99`); err != nil {
		t.Fatal(err)
	}
	// Do not close the connection: process termination makes the rollback journal hot.
	os.Exit(0)
}

func testCatalogPath(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, CatalogFileName)
}
