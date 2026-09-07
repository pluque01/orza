package catalogrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/domain"
)

func TestConnectionRepositoryRootCRUDAndSelectors(t *testing.T) {
	repository, store := testConnectionRepository(t)
	ctx := context.Background()

	created, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{
		Parent:     app.ItemSelector{Path: "/"},
		Name:       "production",
		Host:       "prod.example.com",
		Username:   "deploy",
		AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatalf("CreateConnection() error = %v", err)
	}
	if created.Connection.ID == "" || created.Connection.Path != "/production" {
		t.Fatalf("created connection = %#v", created.Connection)
	}
	if created.Connection.Port != domain.DefaultSSHPort || created.Connection.Revision != 1 {
		t.Fatalf("created port/revision = %d/%d, want %d/1", created.Connection.Port, created.Connection.Revision, domain.DefaultSSHPort)
	}
	if created.CatalogRevision != 2 {
		t.Fatalf("create catalog revision = %d, want 2", created.CatalogRevision)
	}

	byPath, err := repository.GetConnection(ctx, app.ItemSelector{Path: "/production"})
	if err != nil {
		t.Fatalf("GetConnection(path) error = %v", err)
	}
	byID, err := repository.GetConnection(ctx, app.ItemSelector{ID: created.Connection.ID})
	if err != nil {
		t.Fatalf("GetConnection(ID) error = %v", err)
	}
	if byPath.Connection != byID.Connection || byID.Connection.ID != created.Connection.ID {
		t.Fatalf("path result %#v differs from ID result %#v", byPath.Connection, byID.Connection)
	}

	newName, newHost := "Production", "new.example.com"
	newPort := uint16(65535)
	updated, err := repository.UpdateConnection(ctx, app.UpdateConnectionRequest{
		Connection: app.ItemSelector{ID: created.Connection.ID},
		Expected:   revisionPointer(created.Connection.Revision),
		Name:       &newName,
		Host:       &newHost,
		Port:       &newPort,
	})
	if err != nil {
		t.Fatalf("UpdateConnection() error = %v", err)
	}
	if updated.Connection.ID != created.Connection.ID {
		t.Fatalf("updated ID = %q, want stable ID %q", updated.Connection.ID, created.Connection.ID)
	}
	if updated.Connection.Path != "/Production" || updated.Connection.Host != newHost || updated.Connection.Port != newPort || updated.Connection.Revision != 2 {
		t.Fatalf("updated connection = %#v", updated.Connection)
	}
	if updated.CatalogRevision != 3 {
		t.Fatalf("update catalog revision = %d, want 3", updated.CatalogRevision)
	}

	listed, err := repository.ListConnections(ctx, app.ListConnectionsRequest{Folder: app.ItemSelector{Path: "/"}})
	if err != nil {
		t.Fatalf("ListConnections() error = %v", err)
	}
	if len(listed.Connections) != 1 || listed.Connections[0] != updated.Connection || listed.CatalogRevision != updated.CatalogRevision {
		t.Fatalf("listed = %#v", listed)
	}

	deleted, err := repository.DeleteConnection(ctx, app.DeleteConnectionRequest{
		Connection: app.ItemSelector{Path: "/Production"},
		Expected:   revisionPointer(updated.Connection.Revision),
	})
	if err != nil {
		t.Fatalf("DeleteConnection() error = %v", err)
	}
	if deleted.Deleted != updated.Connection || deleted.CatalogRevision != 4 {
		t.Fatalf("deleted = %#v", deleted)
	}
	if _, err := repository.GetConnection(ctx, app.ItemSelector{ID: created.Connection.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetConnection(deleted) error = %v, want ErrNotFound", err)
	}
	assertCatalogCounts(t, store, 0, 0)
}

func TestConnectionRepositoryPortsAndAuthentication(t *testing.T) {
	tests := []struct {
		name         string
		port         uint16
		auth         app.AuthMethod
		identityFile string
		wantPort     uint16
		wantErr      error
	}{
		{name: "default port", auth: app.AuthMethodAgent, wantPort: 22},
		{name: "lowest port", port: 1, auth: app.AuthMethodAgent, wantPort: 1},
		{name: "highest port", port: 65535, auth: app.AuthMethodAgent, wantPort: 65535},
		{name: "key", auth: app.AuthMethodKey, identityFile: "/home/user/.ssh/id_ed25519", wantPort: 22},
		{name: "password", auth: app.AuthMethodPassword, wantPort: 22},
		{name: "empty host", auth: app.AuthMethodAgent, wantErr: domain.ErrInvalidHost},
		{name: "invalid auth", auth: app.AuthMethod("keyboard-interactive"), wantErr: domain.ErrInvalidAuthMethod},
		{name: "agent with identity", auth: app.AuthMethodAgent, identityFile: "/key", wantErr: domain.ErrInvalidAuthentication},
		{name: "key without identity", auth: app.AuthMethodKey, wantErr: domain.ErrInvalidAuthentication},
		{name: "password with identity", auth: app.AuthMethodPassword, identityFile: "/key", wantErr: domain.ErrInvalidAuthentication},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, _ := testConnectionRepository(t)
			host := "host.example.com"
			if errors.Is(tt.wantErr, domain.ErrInvalidHost) {
				host = ""
			}
			result, err := repository.CreateConnection(context.Background(), app.CreateConnectionRequest{
				Parent:       app.ItemSelector{Path: "/"},
				Name:         fmt.Sprintf("connection-%d", index),
				Host:         host,
				Port:         tt.port,
				AuthMethod:   tt.auth,
				IdentityFile: tt.identityFile,
			})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("CreateConnection() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateConnection() error = %v", err)
			}
			if result.Connection.Port != tt.wantPort || result.Connection.AuthMethod != tt.auth || result.Connection.IdentityFile != tt.identityFile {
				t.Fatalf("connection = %#v", result.Connection)
			}
		})
	}
}

func TestConnectionDetailsAuthenticationCombinations(t *testing.T) {
	credentialID := "11111111111111111111111111111111"
	tests := []struct {
		name       string
		auth       domain.AuthMethod
		identity   string
		credential string
		wantErr    bool
	}{
		{name: "agent", auth: domain.AuthMethodAgent},
		{name: "key", auth: domain.AuthMethodKey, identity: "/key"},
		{name: "password prompted", auth: domain.AuthMethodPassword},
		{name: "password remembered", auth: domain.AuthMethodPassword, credential: credentialID},
		{name: "agent identity", auth: domain.AuthMethodAgent, identity: "/key", wantErr: true},
		{name: "agent credential", auth: domain.AuthMethodAgent, credential: credentialID, wantErr: true},
		{name: "key missing identity", auth: domain.AuthMethodKey, wantErr: true},
		{name: "key credential", auth: domain.AuthMethodKey, identity: "/key", credential: credentialID, wantErr: true},
		{name: "password identity", auth: domain.AuthMethodPassword, identity: "/key", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details, err := domain.NewConnectionDetails("host", 0, "user", tt.auth, tt.identity, tt.credential)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidAuthentication) {
					t.Fatalf("NewConnectionDetails() error = %v, want ErrInvalidAuthentication", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewConnectionDetails() error = %v", err)
			}
			if details.Port() != domain.DefaultSSHPort || details.CredentialRef() != tt.credential {
				t.Fatalf("details port/credential = %d/%q", details.Port(), details.CredentialRef())
			}
		})
	}
}

func TestConnectionRepositoryUsesCaseSensitiveSharedNames(t *testing.T) {
	repository, store := testConnectionRepository(t)
	insertTestFolder(t, store, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Prod", rootID(t, store))

	lower, err := repository.CreateConnection(context.Background(), app.CreateConnectionRequest{
		Parent:     app.ItemSelector{Path: "/"},
		Name:       "prod",
		Host:       "lower.example.com",
		AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatalf("create case-distinct connection: %v", err)
	}
	if lower.Connection.Path != "/prod" {
		t.Fatalf("path = %q, want /prod", lower.Connection.Path)
	}

	_, err = repository.CreateConnection(context.Background(), app.CreateConnectionRequest{
		Parent:     app.ItemSelector{Path: "/"},
		Name:       "Prod",
		Host:       "duplicate.example.com",
		AuthMethod: app.AuthMethodAgent,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("create shared-name duplicate error = %v, want ErrConflict", err)
	}
}

func TestConnectionRepositoryMovePreservesIDAndResolvesNewPath(t *testing.T) {
	repository, store := testConnectionRepository(t)
	root := rootID(t, store)
	insertTestFolder(t, store, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "destination", root)

	created := mustCreateConnection(t, repository, "server")
	moved, err := repository.MoveConnection(context.Background(), app.MoveConnectionRequest{
		Connection:  app.ItemSelector{Path: "/server"},
		Destination: app.ItemSelector{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Expected:    revisionPointer(created.Connection.Revision),
	})
	if err != nil {
		t.Fatalf("MoveConnection() error = %v", err)
	}
	if moved.Connection.ID != created.Connection.ID || moved.Connection.Path != "/destination/server" || moved.Connection.Revision != 2 {
		t.Fatalf("moved connection = %#v", moved.Connection)
	}
	if _, err := repository.GetConnection(context.Background(), app.ItemSelector{Path: "/server"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old path error = %v, want ErrNotFound", err)
	}
	if got, err := repository.GetConnection(context.Background(), app.ItemSelector{Path: "/destination/server"}); err != nil || got.Connection.ID != created.Connection.ID {
		t.Fatalf("new path result = %#v, error = %v", got, err)
	}
}

func TestConnectionRepositoryExpectedRevisionConflicts(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*Repository, app.Connection) error
	}{
		{
			name: "update",
			mutate: func(repository *Repository, connection app.Connection) error {
				host := "stale.example.com"
				_, err := repository.UpdateConnection(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: revisionPointer(1), Host: &host})
				return err
			},
		},
		{
			name: "move",
			mutate: func(repository *Repository, connection app.Connection) error {
				_, err := repository.MoveConnection(context.Background(), app.MoveConnectionRequest{Connection: app.ItemSelector{ID: connection.ID}, Destination: app.ItemSelector{Path: "/destination"}, Expected: revisionPointer(1)})
				return err
			},
		},
		{
			name: "delete",
			mutate: func(repository *Repository, connection app.Connection) error {
				_, err := repository.DeleteConnection(context.Background(), app.DeleteConnectionRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: revisionPointer(1)})
				return err
			},
		},
	}

	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			repository, store := testConnectionRepository(t)
			insertTestFolder(t, store, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "destination", rootID(t, store))
			created := mustCreateConnection(t, repository, "server")
			host := "fresh.example.com"
			fresh, err := repository.UpdateConnection(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: created.Connection.ID}, Expected: revisionPointer(1), Host: &host})
			if err != nil {
				t.Fatal(err)
			}
			before := catalogRevision(t, store)
			if err := tt.mutate(repository, fresh.Connection); !errors.Is(err, ErrConflict) {
				t.Fatalf("mutation error = %v, want ErrConflict", err)
			}
			if after := catalogRevision(t, store); after != before {
				t.Fatalf("catalog revision after conflict = %d, want %d", after, before)
			}
			got, err := repository.GetConnection(context.Background(), app.ItemSelector{ID: created.Connection.ID})
			if err != nil || got.Connection != fresh.Connection {
				t.Fatalf("connection after conflict = %#v, error = %v", got.Connection, err)
			}
		})
	}
}

func TestConnectionRepositoryUnrelatedChangesDoNotConflict(t *testing.T) {
	repository, _ := testConnectionRepository(t)
	first := mustCreateConnection(t, repository, "first")
	second := mustCreateConnection(t, repository, "second")

	secondHost := "second-new.example.com"
	if _, err := repository.UpdateConnection(context.Background(), app.UpdateConnectionRequest{
		Connection: app.ItemSelector{ID: second.Connection.ID}, Expected: revisionPointer(second.Connection.Revision), Host: &secondHost,
	}); err != nil {
		t.Fatal(err)
	}
	firstHost := "first-new.example.com"
	updated, err := repository.UpdateConnection(context.Background(), app.UpdateConnectionRequest{
		Connection: app.ItemSelector{ID: first.Connection.ID}, Expected: revisionPointer(first.Connection.Revision), Host: &firstHost,
	})
	if err != nil {
		t.Fatalf("UpdateConnection() after unrelated change error = %v", err)
	}
	if updated.Connection.Host != firstHost || updated.Connection.Revision != 2 {
		t.Fatalf("updated first = %#v", updated.Connection)
	}
}

func TestConnectionRepositoryRollsBackTransactionalFailure(t *testing.T) {
	repository, store := testConnectionRepository(t)
	created := mustCreateConnection(t, repository, "server")
	if _, err := store.DB().Exec(`
		CREATE TRIGGER fail_connection_update
		BEFORE UPDATE ON connections
		BEGIN
			SELECT RAISE(ABORT, 'injected failure');
		END
	`); err != nil {
		t.Fatal(err)
	}
	beforeRevision := catalogRevision(t, store)

	name, host := "renamed", "changed.example.com"
	_, err := repository.UpdateConnection(context.Background(), app.UpdateConnectionRequest{
		Connection: app.ItemSelector{ID: created.Connection.ID}, Expected: revisionPointer(created.Connection.Revision), Name: &name, Host: &host,
	})
	if err == nil {
		t.Fatal("UpdateConnection() error = nil, want injected failure")
	}
	if after := catalogRevision(t, store); after != beforeRevision {
		t.Fatalf("catalog revision after rollback = %d, want %d", after, beforeRevision)
	}
	got, getErr := repository.GetConnection(context.Background(), app.ItemSelector{ID: created.Connection.ID})
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got.Connection != created.Connection {
		t.Fatalf("connection after rollback = %#v, want %#v", got.Connection, created.Connection)
	}
}

func TestConnectionRepositoryRejectsInvalidSelectorsAndKinds(t *testing.T) {
	repository, store := testConnectionRepository(t)
	insertTestFolder(t, store, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "folder", rootID(t, store))

	selectors := []app.ItemSelector{
		{},
		{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Path: "/folder"},
		{ID: "not-an-id"},
		{Path: "relative"},
	}
	for _, selector := range selectors {
		if _, err := repository.GetConnection(context.Background(), selector); err == nil {
			t.Errorf("GetConnection(%#v) error = nil", selector)
		}
	}
	if _, err := repository.GetConnection(context.Background(), app.ItemSelector{Path: "/folder"}); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("GetConnection(folder) error = %v, want ErrWrongKind", err)
	}
}

func testConnectionRepository(t *testing.T) (*Repository, *catalog.Store) {
	t.Helper()
	store, err := catalog.Open(testCatalogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewRepository(store), store
}

func mustCreateConnection(t *testing.T, repository *Repository, name string) app.ConnectionResult {
	t.Helper()
	result, err := repository.CreateConnection(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: name, Host: name + ".example.com", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func revisionPointer(revision app.Revision) *app.Revision {
	return &revision
}

func rootID(t *testing.T, store *catalog.Store) string {
	t.Helper()
	var id string
	if err := store.DB().QueryRow(`SELECT root_id FROM catalog_meta WHERE singleton = 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestFolder(t *testing.T, store *catalog.Store, id, name, parentID string) {
	t.Helper()
	if _, err := store.DB().Exec(`
		INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
		VALUES(?, ?, 'folder', ?, 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`, id, parentID, name); err != nil {
		t.Fatal(err)
	}
}

func catalogRevision(t *testing.T, store *catalog.Store) app.CatalogRevision {
	t.Helper()
	var revision app.CatalogRevision
	if err := store.DB().QueryRow(`SELECT catalog_revision FROM catalog_meta WHERE singleton = 1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func assertCatalogCounts(t *testing.T, store *catalog.Store, wantNodes, wantConnections int) {
	t.Helper()
	var nodes, connections int
	if err := store.DB().QueryRow(`SELECT count(*) FROM nodes WHERE kind = 'connection'`).Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`SELECT count(*) FROM connections`).Scan(&connections); err != nil {
		t.Fatal(err)
	}
	if nodes != wantNodes || connections != wantConnections {
		t.Fatalf("connection nodes/details = %d/%d, want %d/%d", nodes, connections, wantNodes, wantConnections)
	}
}

func testCatalogPath(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, catalog.CatalogFileName)
}
