package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/pluque01/orza/internal/catalog"
)

// HostTrustService manages app-owned trust through a connection endpoint.
type HostTrustService struct {
	connections ConnectionRepository
	trusted     TrustedHostRepository
}

func NewHostTrustService(connections ConnectionRepository, trusted TrustedHostRepository) (*HostTrustService, error) {
	if connections == nil || trusted == nil {
		return nil, errors.New("connection and trusted-host repositories are required")
	}
	return &HostTrustService{connections: connections, trusted: trusted}, nil
}

func (s *HostTrustService) ForgetScope(ctx context.Context, selector ItemSelector) (ForgetHostKeyScope, error) {
	target := selectorTarget(selector)
	connection, err := s.connection(ctx, selector)
	if err != nil {
		return ForgetHostKeyScope{}, safeUseCaseError("read host trust", target, err)
	}
	scope := ForgetHostKeyScope{Connection: connection, Endpoint: HostEndpoint{CanonicalHost: connection.Host, Port: connection.Port}}
	trusted, err := s.trusted.GetTrustedHost(ctx, scope.Endpoint)
	if err == nil {
		scope.TrustedHost = &trusted
		return scope, nil
	}
	if errors.Is(err, catalog.ErrTrustedHostNotFound) {
		return scope, nil
	}
	return ForgetHostKeyScope{}, safeUseCaseError("read host trust", target, trustError(err))
}

func (s *HostTrustService) Forget(ctx context.Context, request ForgetHostKeyRequest) (ForgetHostKeyResult, error) {
	target := selectorTarget(request.Connection)
	connection, err := s.connection(ctx, request.Connection)
	if err != nil {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, err)
	}
	endpoint := HostEndpoint{CanonicalHost: connection.Host, Port: connection.Port}
	if request.ExpectedTrustRevision == nil {
		return s.forgetCurrent(ctx, target, endpoint)
	}
	if *request.ExpectedTrustRevision == 0 {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, ErrInvalidRequest)
	}
	revision, err := s.trusted.DeleteTrustedHost(ctx, endpoint, *request.ExpectedTrustRevision)
	if errors.Is(err, catalog.ErrTrustedHostNotFound) {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, ErrConflict)
	}
	if err != nil {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, trustError(err))
	}
	return ForgetHostKeyResult{Endpoint: endpoint, Forgotten: true, CatalogRevision: &revision}, nil
}

func (s *HostTrustService) forgetCurrent(ctx context.Context, target string, endpoint HostEndpoint) (ForgetHostKeyResult, error) {
	trusted, err := s.trusted.GetTrustedHost(ctx, endpoint)
	if errors.Is(err, catalog.ErrTrustedHostNotFound) {
		return ForgetHostKeyResult{Endpoint: endpoint}, nil
	}
	if err != nil {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, trustError(err))
	}
	revision, err := s.trusted.DeleteTrustedHost(ctx, endpoint, trusted.Revision)
	if errors.Is(err, catalog.ErrTrustedHostNotFound) || errors.Is(err, catalog.ErrTrustedHostConflict) {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, ErrConflict)
	}
	if err != nil {
		return ForgetHostKeyResult{}, safeUseCaseError("forget host key", target, trustError(err))
	}
	return ForgetHostKeyResult{Endpoint: endpoint, Forgotten: true, CatalogRevision: &revision}, nil
}

func (s *HostTrustService) connection(ctx context.Context, selector ItemSelector) (Connection, error) {
	if s == nil || s.connections == nil || s.trusted == nil {
		return Connection{}, ErrInvalidRequest
	}
	result, err := s.connections.GetConnection(ctx, selector)
	if err != nil {
		return Connection{}, err
	}
	return result.Connection, nil
}

func trustError(err error) error {
	switch {
	case errors.Is(err, catalog.ErrTrustedHostNotFound):
		return ErrNotFound
	case errors.Is(err, catalog.ErrTrustedHostConflict):
		return ErrConflict
	case errors.Is(err, catalog.ErrInvalidTrustedHost):
		return fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	default:
		return err
	}
}
