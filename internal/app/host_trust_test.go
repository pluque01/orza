package app

import (
	"context"
	"errors"
	"testing"

	"github.com/pluque01/orza/internal/catalog"
)

func TestHostTrustServiceForgetsConnectionEndpoint(t *testing.T) {
	connection := Connection{Node: Node{ID: "connection", Path: "/connection"}, Host: "host.example", Port: 2222}
	trusted := TrustedHost{HostEndpoint: HostEndpoint{CanonicalHost: "host.example", Port: 2222}, Revision: 3}
	repository := &hostTrustConnectionRepository{connection: connection}
	hosts := &hostTrustRepository{trusted: &trusted}
	service, err := NewHostTrustService(repository, hosts)
	if err != nil {
		t.Fatal(err)
	}

	scope, err := service.ForgetScope(context.Background(), ItemSelector{ID: connection.ID})
	if err != nil || scope.TrustedHost == nil || scope.Endpoint != trusted.HostEndpoint {
		t.Fatalf("ForgetScope() = %+v, %v", scope, err)
	}
	result, err := service.Forget(context.Background(), scope.Request())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Forgotten || hosts.deleted != trusted.Revision || result.CatalogRevision == nil {
		t.Fatalf("Forget() = %+v, deleted revision %d", result, hosts.deleted)
	}
}

func TestHostTrustServiceForgetIsIdempotentButPinnedScopeConflicts(t *testing.T) {
	connection := Connection{Node: Node{ID: "connection", Path: "/connection"}, Host: "host.example", Port: 22}
	service, err := NewHostTrustService(&hostTrustConnectionRepository{connection: connection}, &hostTrustRepository{getErr: catalog.ErrTrustedHostNotFound})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Forget(context.Background(), ForgetHostKeyRequest{Connection: ItemSelector{ID: connection.ID}})
	if err != nil || result.Forgotten || result.Endpoint.Port != 22 {
		t.Fatalf("idempotent Forget() = %+v, %v", result, err)
	}
	stale := Revision(1)
	if _, err := service.Forget(context.Background(), ForgetHostKeyRequest{Connection: ItemSelector{ID: connection.ID}, ExpectedTrustRevision: &stale}); !errors.Is(err, ErrConflict) {
		t.Fatalf("pinned Forget() error = %v, want conflict", err)
	}
}

type hostTrustConnectionRepository struct{ connection Connection }

func (r *hostTrustConnectionRepository) GetConnection(context.Context, ItemSelector) (ConnectionResult, error) {
	return ConnectionResult{Connection: r.connection}, nil
}
func (*hostTrustConnectionRepository) CreateConnection(context.Context, CreateConnectionRequest) (ConnectionResult, error) {
	return ConnectionResult{}, ErrInvalidRequest
}
func (*hostTrustConnectionRepository) ListConnections(context.Context, ListConnectionsRequest) (ListConnectionsResult, error) {
	return ListConnectionsResult{}, ErrInvalidRequest
}
func (*hostTrustConnectionRepository) UpdateConnection(context.Context, UpdateConnectionRequest) (ConnectionResult, error) {
	return ConnectionResult{}, ErrInvalidRequest
}
func (*hostTrustConnectionRepository) MoveConnection(context.Context, MoveConnectionRequest) (ConnectionResult, error) {
	return ConnectionResult{}, ErrInvalidRequest
}
func (*hostTrustConnectionRepository) DeleteConnection(context.Context, DeleteConnectionRequest) (DeleteConnectionResult, error) {
	return DeleteConnectionResult{}, ErrInvalidRequest
}

type hostTrustRepository struct {
	trusted *TrustedHost
	getErr  error
	deleted Revision
}

func (r *hostTrustRepository) GetTrustedHost(context.Context, HostEndpoint) (TrustedHost, error) {
	if r.getErr != nil {
		return TrustedHost{}, r.getErr
	}
	if r.trusted == nil {
		return TrustedHost{}, catalog.ErrTrustedHostNotFound
	}
	return *r.trusted, nil
}
func (*hostTrustRepository) TrustHost(context.Context, TrustHostRequest) (TrustedHost, error) {
	return TrustedHost{}, ErrInvalidRequest
}
func (r *hostTrustRepository) DeleteTrustedHost(_ context.Context, _ HostEndpoint, revision Revision) (CatalogRevision, error) {
	if r.trusted == nil {
		return 0, catalog.ErrTrustedHostNotFound
	}
	if r.trusted.Revision != revision {
		return 0, catalog.ErrTrustedHostConflict
	}
	r.deleted = revision
	r.trusted = nil
	value := CatalogRevision(3)
	return value, nil
}
