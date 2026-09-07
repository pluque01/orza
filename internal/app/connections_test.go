package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConnectionServiceCRUDAndCredentialLifecycle(t *testing.T) {
	repository := newServiceRepository()
	credentials := &fakeCredentialLifecycle{repository: repository}
	service, err := NewConnectionService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}

	secret := []byte("remember-me")
	expectedParent := Revision(9)
	created, err := service.Create(context.Background(), CreateConnectionRequest{
		Parent:             ItemSelector{Path: "/"},
		ExpectedParent:     &expectedParent,
		ExpectedParentPath: "/",
		Name:               "server",
		Host:               "server.example",
		AuthMethod:         AuthMethodPassword,
		CredentialIntent:   CredentialRemember,
		Password:           secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.saved != 1 || created.Connection.CredentialRef == "" {
		t.Fatalf("created = %#v, save calls = %d", created.Connection, credentials.saved)
	}
	if repository.created.Password != nil || repository.created.CredentialIntent != CredentialKeep {
		t.Fatal("secret-bearing fields crossed the repository boundary")
	}
	if repository.created.ExpectedParent == nil || *repository.created.ExpectedParent != expectedParent || repository.created.ExpectedParentPath != "/" {
		t.Fatal("expected parent revision did not cross the repository boundary")
	}

	got, err := service.Get(context.Background(), ItemSelector{ID: created.Connection.ID})
	if err != nil || got.Connection.ID != created.Connection.ID {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	listed, err := service.List(context.Background(), ListConnectionsRequest{Folder: ItemSelector{Path: "/"}})
	if err != nil || len(listed.Connections) != 1 {
		t.Fatalf("List() = %#v, %v", listed, err)
	}

	replacement := []byte("replacement")
	updated, err := service.Update(context.Background(), UpdateConnectionRequest{
		Connection:       ItemSelector{ID: created.Connection.ID},
		Expected:         revisionRef(created.Connection.Revision),
		CredentialIntent: CredentialRemember,
		Password:         replacement,
	})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.replaced != 1 || updated.Connection.CredentialRef == created.Connection.CredentialRef {
		t.Fatalf("updated = %#v, replace calls = %d", updated.Connection, credentials.replaced)
	}

	destination := ItemSelector{Path: "/archive"}
	expectedDestination := Revision(8)
	moved, err := service.Move(context.Background(), MoveConnectionRequest{
		Connection:              ItemSelector{ID: updated.Connection.ID},
		Destination:             destination,
		Expected:                revisionRef(updated.Connection.Revision),
		ExpectedDestination:     &expectedDestination,
		ExpectedSourcePath:      updated.Connection.Path,
		ExpectedDestinationPath: "/archive",
	})
	if err != nil || moved.Connection.ParentID == updated.Connection.ParentID {
		t.Fatalf("Move() = %#v, %v", moved, err)
	}
	if repository.moved.ExpectedDestination == nil || *repository.moved.ExpectedDestination != expectedDestination || repository.moved.ExpectedSourcePath != updated.Connection.Path || repository.moved.ExpectedDestinationPath != "/archive" {
		t.Fatal("expected destination revision did not cross the repository boundary")
	}

	scope, err := service.DeleteScope(context.Background(), ItemSelector{ID: moved.Connection.ID})
	if err != nil {
		t.Fatal(err)
	}
	if scope.ID != moved.Connection.ID || scope.Revision != moved.Connection.Revision || !scope.HasRememberedPassword {
		t.Fatalf("delete scope = %#v", scope)
	}
	deleted, err := service.Delete(context.Background(), scope.Request())
	if err != nil {
		t.Fatal(err)
	}
	if credentials.deleted != 1 || deleted.Deleted.ID != moved.Connection.ID {
		t.Fatalf("deleted = %#v, saga delete calls = %d", deleted, credentials.deleted)
	}
}

func TestConnectionServiceRemovesRememberedPasswordOnAuthChange(t *testing.T) {
	repository := newServiceRepository()
	repository.connection.AuthMethod = AuthMethodPassword
	repository.connection.CredentialRef = serviceCredentialRef
	credentials := &fakeCredentialLifecycle{repository: repository}
	service, err := NewConnectionService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}

	method := AuthMethodKey
	identity := "/keys/id_ed25519"
	result, err := service.Update(context.Background(), UpdateConnectionRequest{
		Connection:   ItemSelector{ID: repository.connection.ID},
		Expected:     revisionRef(repository.connection.Revision),
		AuthMethod:   &method,
		IdentityFile: &identity,
	})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.removed != 1 || result.Connection.AuthMethod != AuthMethodKey || result.Connection.CredentialRef != "" {
		t.Fatalf("result = %#v, remove calls = %d", result.Connection, credentials.removed)
	}
}

func TestConnectionServiceValidationConflictCancellationAndRedaction(t *testing.T) {
	repository := newServiceRepository()
	credentials := &fakeCredentialLifecycle{repository: repository}
	service, err := NewConnectionService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), CreateConnectionRequest{
		Parent:           ItemSelector{Path: "/"},
		Name:             "server",
		Host:             "server.example",
		AuthMethod:       AuthMethodAgent,
		CredentialIntent: CredentialRemember,
		Password:         []byte("validation-canary"),
	})
	if !errors.Is(err, ErrInvalidRequest) || strings.Contains(err.Error(), "validation-canary") {
		t.Fatalf("validation error = %v", err)
	}
	if repository.createCalls != 0 {
		t.Fatal("invalid request reached repository")
	}

	repository.updateErr = ErrConflict
	host := "new.example"
	_, err = service.Update(context.Background(), UpdateConnectionRequest{
		Connection: ItemSelector{ID: repository.connection.ID},
		Host:       &host,
	})
	if !errors.Is(err, ErrConflict) || ErrorKindOf(err) != ErrorKindConflict {
		t.Fatalf("conflict error = %v (%s)", err, ErrorKindOf(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.Get(ctx, ItemSelector{ID: repository.connection.ID})
	if !errors.Is(err, context.Canceled) || ErrorKindOf(err) != ErrorKindCanceled {
		t.Fatalf("cancellation error = %v (%s)", err, ErrorKindOf(err))
	}

	const canary = "repository-secret-canary"
	repository.getErr = errors.New(canary)
	_, err = service.Get(context.Background(), ItemSelector{ID: repository.connection.ID})
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("unsafe repository error = %v", err)
	}
}

func TestConnectionServiceRememberedCreateConflictDoesNotPersistCredential(t *testing.T) {
	repository := newServiceRepository()
	repository.createErr = ErrConflict
	credentials := &fakeCredentialLifecycle{repository: repository}
	service, err := NewConnectionService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}
	expectedParent := Revision(7)
	_, err = service.Create(context.Background(), CreateConnectionRequest{
		Parent: ItemSelector{Path: "/destination"}, ExpectedParent: &expectedParent, ExpectedParentPath: "/destination",
		Name: "server", Host: "server.example", AuthMethod: AuthMethodPassword,
		CredentialIntent: CredentialRemember, Password: []byte("not-persisted"),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() error = %v, want conflict", err)
	}
	if credentials.saved != 0 {
		t.Fatalf("credential save calls = %d, want 0", credentials.saved)
	}
	if repository.created.ExpectedParent == nil || *repository.created.ExpectedParent != expectedParent || repository.created.ExpectedParentPath != "/destination" || repository.created.Password != nil || repository.created.CredentialIntent != CredentialKeep {
		t.Fatalf("repository request = %#v", repository.created)
	}
}

const (
	serviceConnectionID  = NodeID("11111111111111111111111111111111")
	serviceCredentialRef = "22222222222222222222222222222222"
)

type serviceRepository struct {
	connection  Connection
	created     CreateConnectionRequest
	moved       MoveConnectionRequest
	createCalls int
	createErr   error
	getErr      error
	updateErr   error
}

func newServiceRepository() *serviceRepository {
	return &serviceRepository{connection: Connection{
		Node: Node{ID: serviceConnectionID, ParentID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: NodeKindConnection, Name: "server", Path: "/server", Revision: 1},
		Host: "server.example", Port: 22, Username: "operator", AuthMethod: AuthMethodAgent,
	}}
}

func (r *serviceRepository) CreateConnection(_ context.Context, request CreateConnectionRequest) (ConnectionResult, error) {
	r.createCalls++
	r.created = request
	if r.createErr != nil {
		return ConnectionResult{}, r.createErr
	}
	r.connection.Name = request.Name
	r.connection.Host = request.Host
	r.connection.Port = request.Port
	if r.connection.Port == 0 {
		r.connection.Port = 22
	}
	r.connection.Username = request.Username
	r.connection.AuthMethod = request.AuthMethod
	r.connection.IdentityFile = request.IdentityFile
	return ConnectionResult{Connection: r.connection, CatalogRevision: 2}, nil
}

func (r *serviceRepository) GetConnection(context.Context, ItemSelector) (ConnectionResult, error) {
	if r.getErr != nil {
		return ConnectionResult{}, r.getErr
	}
	return ConnectionResult{Connection: r.connection, CatalogRevision: CatalogRevision(r.connection.Revision + 1)}, nil
}

func (r *serviceRepository) ListConnections(context.Context, ListConnectionsRequest) (ListConnectionsResult, error) {
	return ListConnectionsResult{Connections: []Connection{r.connection}, CatalogRevision: 2}, nil
}

func (r *serviceRepository) UpdateConnection(_ context.Context, request UpdateConnectionRequest) (ConnectionResult, error) {
	if r.updateErr != nil {
		return ConnectionResult{}, r.updateErr
	}
	if request.Name != nil {
		r.connection.Name = *request.Name
	}
	if request.Host != nil {
		r.connection.Host = *request.Host
	}
	if request.Port != nil {
		r.connection.Port = *request.Port
	}
	if request.Username != nil {
		r.connection.Username = *request.Username
	}
	if request.AuthMethod != nil {
		r.connection.AuthMethod = *request.AuthMethod
	}
	if request.IdentityFile != nil {
		r.connection.IdentityFile = *request.IdentityFile
	}
	r.connection.Revision++
	return ConnectionResult{Connection: r.connection, CatalogRevision: CatalogRevision(r.connection.Revision + 1)}, nil
}

func (r *serviceRepository) MoveConnection(_ context.Context, request MoveConnectionRequest) (ConnectionResult, error) {
	r.moved = request
	r.connection.ParentID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	r.connection.Path = "/archive/" + r.connection.Name
	r.connection.Revision++
	return ConnectionResult{Connection: r.connection, CatalogRevision: CatalogRevision(r.connection.Revision + 1)}, nil
}

func (r *serviceRepository) DeleteConnection(context.Context, DeleteConnectionRequest) (DeleteConnectionResult, error) {
	deleted := r.connection
	r.connection = Connection{}
	return DeleteConnectionResult{Deleted: deleted, CatalogRevision: CatalogRevision(deleted.Revision + 2)}, nil
}

type fakeCredentialLifecycle struct {
	repository *serviceRepository
	saved      int
	replaced   int
	removed    int
	deleted    int
	recovered  int
	err        error
}

func (f *fakeCredentialLifecycle) Save(context.Context, Connection, []byte) error {
	f.saved++
	if f.err != nil {
		return f.err
	}
	f.repository.connection.CredentialRef = serviceCredentialRef
	f.repository.connection.AuthMethod = AuthMethodPassword
	f.repository.connection.Revision++
	return nil
}

func (f *fakeCredentialLifecycle) Replace(context.Context, Connection, []byte) error {
	f.replaced++
	if f.err != nil {
		return f.err
	}
	f.repository.connection.CredentialRef = "33333333333333333333333333333333"
	f.repository.connection.Revision++
	return nil
}

func (f *fakeCredentialLifecycle) Remove(_ context.Context, _ Connection, target AuthMethod, identity string) error {
	f.removed++
	if f.err != nil {
		return f.err
	}
	f.repository.connection.CredentialRef = ""
	f.repository.connection.AuthMethod = target
	f.repository.connection.IdentityFile = identity
	f.repository.connection.Revision++
	return nil
}

func (f *fakeCredentialLifecycle) DeleteConnection(context.Context, Connection) error {
	f.deleted++
	if f.err != nil {
		return f.err
	}
	f.repository.connection = Connection{}
	return nil
}

func (f *fakeCredentialLifecycle) Recover(context.Context) error {
	f.recovered++
	return f.err
}

func revisionRef(revision Revision) *Revision { return &revision }
