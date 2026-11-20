package catalogrepo

import (
	"context"
	"errors"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/credential"
)

type interleavedDeleteRepository struct {
	*Repository
	beforeDelete func()
}

func (r *interleavedDeleteRepository) DeleteConnection(ctx context.Context, request app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
	r.beforeDelete()
	return r.Repository.DeleteConnection(ctx, request)
}

func TestServiceDeletePreservesConcurrentChanges(t *testing.T) {
	for _, change := range []string{"replace path", "add credential"} {
		t.Run(change, func(t *testing.T) {
			repository, store := testConnectionRepository(t)
			writer := NewRepository(store)
			ctx := context.Background()
			original, err := writer.CreateConnection(ctx, app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "server", Host: "server.example", AuthMethod: app.AuthMethodPassword})
			if err != nil {
				t.Fatal(err)
			}
			secrets := credential.NewFake()
			scope := credential.Scope("test")
			saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), secrets, scope, nil)
			if err != nil {
				t.Fatal(err)
			}
			var replacement app.Connection
			wrapped := &interleavedDeleteRepository{Repository: repository, beforeDelete: func() {
				switch change {
				case "replace path":
					if _, err := writer.DeleteConnection(ctx, app.DeleteConnectionRequest{Connection: app.ItemSelector{ID: original.Connection.ID}}); err != nil {
						t.Fatal(err)
					}
					created, err := writer.CreateConnection(ctx, app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "server", Host: "new.example", AuthMethod: app.AuthMethodPassword})
					if err != nil {
						t.Fatal(err)
					}
					replacement = created.Connection
				case "add credential":
					if err := saga.Save(ctx, original.Connection, []byte("synthetic-secret")); err != nil {
						t.Fatal(err)
					}
				}
			}}
			service, err := app.NewConnectionService(wrapped, saga)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Delete(ctx, app.DeleteConnectionRequest{Connection: app.ItemSelector{Path: "/server"}})
			if change == "replace path" && !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("delete error = %v, want not found", err)
			}
			if change == "add credential" && !errors.Is(err, app.ErrConflict) {
				t.Fatalf("delete error = %v, want conflict", err)
			}
			got, err := writer.GetConnection(ctx, app.ItemSelector{Path: "/server"})
			if err != nil {
				t.Fatal(err)
			}
			if change == "replace path" && got.Connection.ID != replacement.ID {
				t.Fatalf("replacement lost: %#v", got.Connection)
			}
			if change == "add credential" {
				if got.Connection.ID != original.Connection.ID || got.Connection.CredentialRef == "" || got.Connection.Revision <= original.Connection.Revision {
					t.Fatalf("credential update lost: %#v", got.Connection)
				}
				if _, err := secrets.Get(ctx, credential.Key{Scope: scope, Reference: credential.Reference(got.Connection.CredentialRef)}); err != nil {
					t.Fatalf("credential lost: %v", err)
				}
				if _, err := service.Delete(ctx, app.DeleteConnectionRequest{Connection: app.ItemSelector{ID: got.Connection.ID}}); err != nil {
					t.Fatalf("saga delete: %v", err)
				}
				if _, err := secrets.Get(ctx, credential.Key{Scope: scope, Reference: credential.Reference(got.Connection.CredentialRef)}); !errors.Is(err, credential.ErrNotFound) {
					t.Fatalf("saga credential lookup: %v", err)
				}
			}
		})
	}
}

func TestDirectDeleteRejectsRememberedCredential(t *testing.T) {
	repository, store := testConnectionRepository(t)
	ctx := context.Background()
	created, err := repository.CreateConnection(ctx, app.CreateConnectionRequest{Parent: app.ItemSelector{Path: "/"}, Name: "server", Host: "server.example", AuthMethod: app.AuthMethodPassword})
	if err != nil {
		t.Fatal(err)
	}
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), credential.NewFake(), credential.Scope("test"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := saga.Save(ctx, created.Connection, []byte("synthetic-secret")); err != nil {
		t.Fatal(err)
	}
	before, err := repository.GetConnection(ctx, app.ItemSelector{ID: created.Connection.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.DeleteConnection(ctx, app.DeleteConnectionRequest{Connection: app.ItemSelector{ID: created.Connection.ID}, Expected: &before.Connection.Revision})
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("direct delete error = %v, want conflict", err)
	}
	after, err := repository.GetConnection(ctx, app.ItemSelector{ID: created.Connection.ID})
	if err != nil || after != before {
		t.Fatalf("connection after rejected delete = %#v, %v; want %#v", after, err, before)
	}
}
