package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/domain"
)

var (
	ErrInvalidRequest       = errors.New("invalid application request")
	ErrNotFound             = errors.New("application item not found")
	ErrConflict             = errors.New("application conflict")
	ErrHostRejected         = errors.New("host trust rejected")
	ErrHostRevoked          = errors.New("host key revoked")
	ErrHostNotVerified      = errors.New("host is not verified")
	ErrTrustDecisionMissing = errors.New("host trust decision is required")
	ErrNonInteractive       = errors.New("interactive terminal is required")
)

type ErrorKind string

const (
	ErrorKindInvalid  ErrorKind = "invalid"
	ErrorKindNotFound ErrorKind = "not_found"
	ErrorKindConflict ErrorKind = "conflict"
	ErrorKindCanceled ErrorKind = "canceled"
	ErrorKindSecurity ErrorKind = "security"
	ErrorKindInternal ErrorKind = "internal"
)

// UseCaseError renders only application-controlled text. The cause remains
// available to errors.Is/errors.As but is never included in Error().
type UseCaseError struct {
	Operation string
	Target    string
	Kind      ErrorKind
	Subject   string
	cause     error
}

func (e *UseCaseError) Error() string {
	subject := e.Subject
	if subject == "" {
		subject = "connection"
	}
	message := "operation failed"
	switch e.Kind {
	case ErrorKindInvalid:
		message = "request is invalid"
	case ErrorKindNotFound:
		message = subject + " was not found"
	case ErrorKindConflict:
		message = subject + " changed; reload before retrying"
	case ErrorKindCanceled:
		message = "operation canceled"
	case ErrorKindSecurity:
		message = "secure operation failed"
	}
	if e.Operation != "" {
		message = e.Operation + ": " + message
	}
	if e.Target != "" {
		message += " (target: " + e.Target + ")"
	}
	return message
}

func (e *UseCaseError) Unwrap() error { return e.cause }

func ErrorKindOf(err error) ErrorKind {
	var useCase *UseCaseError
	if errors.As(err, &useCase) {
		return useCase.Kind
	}
	return classifyError(err)
}

type ConnectionService struct {
	repository  ConnectionRepository
	credentials CredentialLifecycle
}

func NewConnectionService(repository ConnectionRepository, credentials CredentialLifecycle) (*ConnectionService, error) {
	if repository == nil || credentials == nil {
		return nil, safeUseCaseError("configure connections", "", ErrInvalidRequest)
	}
	return &ConnectionService{repository: repository, credentials: credentials}, nil
}

func (s *ConnectionService) Create(ctx context.Context, request CreateConnectionRequest) (ConnectionResult, error) {
	if err := contextError(ctx); err != nil {
		return ConnectionResult{}, safeUseCaseError("create connection", request.Parent.Path, err)
	}
	if err := validateCreateConnection(request); err != nil {
		return ConnectionResult{}, safeUseCaseError("create connection", request.Parent.Path, err)
	}

	catalogRequest := request
	catalogRequest.CredentialIntent = CredentialKeep
	catalogRequest.Password = nil
	result, err := s.repository.CreateConnection(ctx, catalogRequest)
	if err != nil {
		return ConnectionResult{}, safeUseCaseError("create connection", request.Parent.Path, err)
	}
	if request.CredentialIntent != CredentialRemember {
		return result, nil
	}

	secret := append([]byte(nil), request.Password...)
	defer wipeSecret(secret)
	if err := s.credentials.Save(ctx, result.Connection, secret); err != nil {
		return ConnectionResult{}, safeUseCaseError("remember connection password", result.Connection.Path, err)
	}
	return s.getAfterCredentialChange(ctx, result.Connection.ID, "read created connection")
}

func (s *ConnectionService) Get(ctx context.Context, selector ItemSelector) (ConnectionResult, error) {
	if err := contextError(ctx); err != nil {
		return ConnectionResult{}, safeUseCaseError("get connection", selectorTarget(selector), err)
	}
	if err := validateSelector(selector); err != nil {
		return ConnectionResult{}, safeUseCaseError("get connection", selectorTarget(selector), err)
	}
	result, err := s.repository.GetConnection(ctx, selector)
	if err != nil {
		return ConnectionResult{}, safeUseCaseError("get connection", selectorTarget(selector), err)
	}
	return result, nil
}

func (s *ConnectionService) List(ctx context.Context, request ListConnectionsRequest) (ListConnectionsResult, error) {
	if err := contextError(ctx); err != nil {
		return ListConnectionsResult{}, safeUseCaseError("list connections", selectorTarget(request.Folder), err)
	}
	if err := validateSelector(request.Folder); err != nil {
		return ListConnectionsResult{}, safeUseCaseError("list connections", selectorTarget(request.Folder), err)
	}
	result, err := s.repository.ListConnections(ctx, request)
	if err != nil {
		return ListConnectionsResult{}, safeUseCaseError("list connections", selectorTarget(request.Folder), err)
	}
	return result, nil
}

func (s *ConnectionService) Update(ctx context.Context, request UpdateConnectionRequest) (ConnectionResult, error) {
	target := selectorTarget(request.Connection)
	if err := contextError(ctx); err != nil {
		return ConnectionResult{}, safeUseCaseError("update connection", target, err)
	}
	if err := validateUpdateShape(request); err != nil {
		return ConnectionResult{}, safeUseCaseError("update connection", target, err)
	}
	currentResult, err := s.repository.GetConnection(ctx, request.Connection)
	if err != nil {
		return ConnectionResult{}, safeUseCaseError("read connection for update", target, err)
	}
	current := currentResult.Connection
	if request.Expected != nil && *request.Expected != current.Revision {
		return ConnectionResult{}, safeUseCaseError("update connection", target, ErrConflict)
	}
	if err := validateUpdatedConnection(current, request); err != nil {
		return ConnectionResult{}, safeUseCaseError("update connection", target, err)
	}

	switch request.CredentialIntent {
	case CredentialRemember:
		return s.updateAndRemember(ctx, current, request)
	case CredentialForget:
		if current.CredentialRef == "" {
			return ConnectionResult{}, safeUseCaseError("forget connection password", target, ErrInvalidRequest)
		}
		return s.removeCredentialAndUpdate(ctx, current, request)
	case CredentialKeep:
		method, _ := updatedAuthentication(current, request)
		if current.CredentialRef != "" && method != AuthMethodPassword {
			return s.removeCredentialAndUpdate(ctx, current, request)
		}
		catalogRequest := sanitizedUpdate(request)
		result, err := s.repository.UpdateConnection(ctx, catalogRequest)
		if err != nil {
			return ConnectionResult{}, safeUseCaseError("update connection", target, err)
		}
		return result, nil
	default:
		return ConnectionResult{}, safeUseCaseError("update connection", target, ErrInvalidRequest)
	}
}

func (s *ConnectionService) Move(ctx context.Context, request MoveConnectionRequest) (ConnectionResult, error) {
	target := selectorTarget(request.Connection)
	if err := contextError(ctx); err != nil {
		return ConnectionResult{}, safeUseCaseError("move connection", target, err)
	}
	if err := validateSelector(request.Connection); err != nil {
		return ConnectionResult{}, safeUseCaseError("move connection", target, err)
	}
	if err := validateSelector(request.Destination); err != nil {
		return ConnectionResult{}, safeUseCaseError("move connection", selectorTarget(request.Destination), err)
	}
	if request.Expected != nil && *request.Expected == 0 || request.ExpectedDestination != nil && *request.ExpectedDestination == 0 {
		return ConnectionResult{}, safeUseCaseError("move connection", target, ErrInvalidRequest)
	}
	if err := validateExpectedPath(request.ExpectedSourcePath); err != nil {
		return ConnectionResult{}, safeUseCaseError("move connection", target, err)
	}
	if err := validateExpectedPath(request.ExpectedDestinationPath); err != nil {
		return ConnectionResult{}, safeUseCaseError("move connection", selectorTarget(request.Destination), err)
	}
	result, err := s.repository.MoveConnection(ctx, request)
	if err != nil {
		return ConnectionResult{}, safeUseCaseError("move connection", target, err)
	}
	return result, nil
}

func (s *ConnectionService) DeleteScope(ctx context.Context, selector ItemSelector) (ConnectionDeleteScope, error) {
	result, err := s.Get(ctx, selector)
	if err != nil {
		return ConnectionDeleteScope{}, err
	}
	connection := result.Connection
	return ConnectionDeleteScope{
		ID:                    connection.ID,
		Path:                  connection.Path,
		Name:                  connection.Name,
		Host:                  connection.Host,
		Username:              connection.Username,
		Revision:              connection.Revision,
		HasRememberedPassword: connection.CredentialRef != "",
	}, nil
}

func (s *ConnectionService) Delete(ctx context.Context, request DeleteConnectionRequest) (DeleteConnectionResult, error) {
	target := selectorTarget(request.Connection)
	if err := contextError(ctx); err != nil {
		return DeleteConnectionResult{}, safeUseCaseError("delete connection", target, err)
	}
	if err := validateSelector(request.Connection); err != nil || request.Expected != nil && *request.Expected == 0 {
		return DeleteConnectionResult{}, safeUseCaseError("delete connection", target, errors.Join(ErrInvalidRequest, err))
	}
	currentResult, err := s.repository.GetConnection(ctx, request.Connection)
	if err != nil {
		return DeleteConnectionResult{}, safeUseCaseError("read connection for delete", target, err)
	}
	current := currentResult.Connection
	if request.Expected != nil && *request.Expected != current.Revision {
		return DeleteConnectionResult{}, safeUseCaseError("delete connection", target, ErrConflict)
	}
	if current.CredentialRef == "" {
		result, err := s.repository.DeleteConnection(ctx, request)
		if err != nil {
			return DeleteConnectionResult{}, safeUseCaseError("delete connection", target, err)
		}
		return result, nil
	}
	if err := s.credentials.DeleteConnection(ctx, current); err != nil {
		return DeleteConnectionResult{}, safeUseCaseError("delete connection and password", target, err)
	}
	var catalogRevision CatalogRevision
	if current.ParentID != "" {
		listed, listErr := s.repository.ListConnections(ctx, ListConnectionsRequest{Folder: ItemSelector{ID: current.ParentID}})
		if listErr == nil {
			catalogRevision = listed.CatalogRevision
		}
	}
	return DeleteConnectionResult{
		Deleted:         current,
		CatalogRevision: catalogRevision,
	}, nil
}

func (s *ConnectionService) updateAndRemember(ctx context.Context, current Connection, request UpdateConnectionRequest) (ConnectionResult, error) {
	target := selectorTarget(request.Connection)
	secret := append([]byte(nil), request.Password...)
	defer wipeSecret(secret)

	catalogRequest := sanitizedUpdate(request)
	if hasCatalogUpdate(catalogRequest) {
		result, err := s.repository.UpdateConnection(ctx, catalogRequest)
		if err != nil {
			return ConnectionResult{}, safeUseCaseError("update connection", target, err)
		}
		current = result.Connection
	}
	var err error
	if current.CredentialRef == "" {
		err = s.credentials.Save(ctx, current, secret)
	} else {
		err = s.credentials.Replace(ctx, current, secret)
	}
	if err != nil {
		return ConnectionResult{}, safeUseCaseError("remember connection password", target, err)
	}
	return s.getAfterCredentialChange(ctx, current.ID, "read updated connection")
}

func (s *ConnectionService) removeCredentialAndUpdate(ctx context.Context, current Connection, request UpdateConnectionRequest) (ConnectionResult, error) {
	target := selectorTarget(request.Connection)
	method, identity := updatedAuthentication(current, request)
	if err := s.credentials.Remove(ctx, current, method, identity); err != nil {
		return ConnectionResult{}, safeUseCaseError("forget connection password", target, err)
	}
	refreshed, err := s.getAfterCredentialChange(ctx, current.ID, "read connection after forgetting password")
	if err != nil {
		return ConnectionResult{}, err
	}

	remainder := sanitizedUpdate(request)
	remainder.Connection = ItemSelector{ID: current.ID}
	remainder.Expected = revisionPointerValue(refreshed.Connection.Revision)
	remainder.AuthMethod = nil
	remainder.IdentityFile = nil
	if !hasCatalogUpdate(remainder) {
		return refreshed, nil
	}
	result, repositoryErr := s.repository.UpdateConnection(ctx, remainder)
	if repositoryErr != nil {
		return ConnectionResult{}, safeUseCaseError("update connection", target, repositoryErr)
	}
	return result, nil
}

func (s *ConnectionService) getAfterCredentialChange(ctx context.Context, id NodeID, operation string) (ConnectionResult, error) {
	result, err := s.repository.GetConnection(ctx, ItemSelector{ID: id})
	if err != nil {
		return ConnectionResult{}, safeUseCaseError(operation, string(id), err)
	}
	return result, nil
}

func validateCreateConnection(request CreateConnectionRequest) error {
	if err := validateSelector(request.Parent); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	if request.ExpectedParent != nil && *request.ExpectedParent == 0 {
		return ErrInvalidRequest
	}
	if err := validateExpectedPath(request.ExpectedParentPath); err != nil {
		return err
	}
	if _, err := domain.NewName(request.Name); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	if _, err := connectionDetails(request.Host, request.Port, request.Username, request.AuthMethod, request.IdentityFile, ""); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	switch request.CredentialIntent {
	case CredentialKeep:
		if request.Password != nil {
			return ErrInvalidRequest
		}
	case CredentialRemember:
		if request.AuthMethod != AuthMethodPassword || request.Password == nil {
			return ErrInvalidRequest
		}
	default:
		return ErrInvalidRequest
	}
	return nil
}

func validateUpdateShape(request UpdateConnectionRequest) error {
	if err := validateSelector(request.Connection); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	if request.Expected != nil && *request.Expected == 0 {
		return ErrInvalidRequest
	}
	if request.Name == nil && request.Host == nil && request.Port == nil && request.Username == nil && request.AuthMethod == nil && request.IdentityFile == nil && request.CredentialIntent == CredentialKeep {
		return ErrInvalidRequest
	}
	if request.Name != nil {
		if _, err := domain.NewName(*request.Name); err != nil {
			return errors.Join(ErrInvalidRequest, err)
		}
	}
	if request.Port != nil && *request.Port == 0 {
		return errors.Join(ErrInvalidRequest, domain.ErrInvalidPort)
	}
	if request.CredentialIntent == CredentialRemember && request.Password == nil || request.CredentialIntent != CredentialRemember && request.Password != nil {
		return ErrInvalidRequest
	}
	if request.CredentialIntent != CredentialKeep && request.CredentialIntent != CredentialRemember && request.CredentialIntent != CredentialForget {
		return ErrInvalidRequest
	}
	return nil
}

func validateUpdatedConnection(current Connection, request UpdateConnectionRequest) error {
	name := current.Name
	if request.Name != nil {
		name = *request.Name
	}
	if _, err := domain.NewName(name); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	host := current.Host
	if request.Host != nil {
		host = *request.Host
	}
	port := current.Port
	if request.Port != nil {
		port = *request.Port
	}
	username := current.Username
	if request.Username != nil {
		username = *request.Username
	}
	method, identity := updatedAuthentication(current, request)
	reference := current.CredentialRef
	if request.CredentialIntent == CredentialForget || method != AuthMethodPassword {
		reference = ""
	}
	if request.CredentialIntent == CredentialRemember {
		if method != AuthMethodPassword || identity != "" {
			return ErrInvalidRequest
		}
	}
	if _, err := connectionDetails(host, port, username, method, identity, reference); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	return nil
}

func updatedAuthentication(current Connection, request UpdateConnectionRequest) (AuthMethod, string) {
	method := current.AuthMethod
	if request.AuthMethod != nil {
		method = *request.AuthMethod
	}
	identity := current.IdentityFile
	if request.IdentityFile != nil {
		identity = *request.IdentityFile
	}
	return method, identity
}

func connectionDetails(host string, port uint16, username string, method AuthMethod, identity, reference string) (domain.ConnectionDetails, error) {
	domainMethod, err := domain.ParseAuthMethod(string(method))
	if err != nil {
		return domain.ConnectionDetails{}, err
	}
	return domain.NewConnectionDetails(host, uint64(port), username, domainMethod, identity, reference)
}

func sanitizedUpdate(request UpdateConnectionRequest) UpdateConnectionRequest {
	request.CredentialIntent = CredentialKeep
	request.Password = nil
	return request
}

func hasCatalogUpdate(request UpdateConnectionRequest) bool {
	return request.Name != nil || request.Host != nil || request.Port != nil || request.Username != nil || request.AuthMethod != nil || request.IdentityFile != nil
}

func validateSelector(selector ItemSelector) error {
	if (selector.ID == "") == (selector.Path == "") {
		return ErrInvalidRequest
	}
	if selector.ID != "" {
		_, err := domain.ParseID(string(selector.ID))
		return err
	}
	_, err := domain.ParseLogicalPath(selector.Path)
	return err
}

func validateExpectedPath(path string) error {
	if path == "" {
		return nil
	}
	if _, err := domain.ParseLogicalPath(path); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidRequest
	}
	return ctx.Err()
}

func selectorTarget(selector ItemSelector) string {
	if (selector.ID == "") == (selector.Path == "") {
		return ""
	}
	if selector.Path != "" {
		path, err := domain.ParseLogicalPath(selector.Path)
		if err == nil {
			return path.String()
		}
		return ""
	}
	if _, err := domain.ParseID(string(selector.ID)); err != nil {
		return ""
	}
	return string(selector.ID)
}

func revisionPointerValue(revision Revision) *Revision { return &revision }

func safeUseCaseError(operation, target string, cause error) error {
	if cause == nil {
		cause = ErrInvalidRequest
	}
	return &UseCaseError{Operation: operation, Target: target, Kind: classifyError(cause), cause: cause}
}

func safeFolderUseCaseError(operation, target string, cause error) error {
	if cause == nil {
		cause = ErrInvalidRequest
	}
	return &UseCaseError{Operation: operation, Target: target, Subject: "folder", Kind: classifyError(cause), cause: cause}
}

func classifyError(err error) ErrorKind {
	if err == nil {
		return ErrorKindInternal
	}
	var useCase *UseCaseError
	if errors.As(err, &useCase) {
		return useCase.Kind
	}
	var validation *domain.ValidationError
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ErrorKindCanceled
	case errors.Is(err, ErrInvalidRequest), errors.As(err, &validation):
		return ErrorKindInvalid
	case errors.Is(err, domain.ErrInvalidID), errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrInvalidPath), errors.Is(err, domain.ErrInvalidRevision), errors.Is(err, domain.ErrInvalidHost), errors.Is(err, domain.ErrInvalidPort), errors.Is(err, domain.ErrInvalidUsername), errors.Is(err, domain.ErrInvalidAuthMethod), errors.Is(err, domain.ErrInvalidAuthentication), errors.Is(err, domain.ErrInvalidIdentityFile), errors.Is(err, domain.ErrInvalidCredentialRef):
		return ErrorKindInvalid
	case errors.Is(err, ErrNotFound):
		return ErrorKindNotFound
	case errors.Is(err, ErrConflict):
		return ErrorKindConflict
	case errors.Is(err, ErrHostRejected), errors.Is(err, ErrHostRevoked), errors.Is(err, ErrHostNotVerified), errors.Is(err, ErrTrustDecisionMissing), errors.Is(err, ErrNonInteractive), errors.Is(err, credential.ErrNotFound), errors.Is(err, credential.ErrUnavailable):
		return ErrorKindSecurity
	default:
		return ErrorKindInternal
	}
}

func (e *UseCaseError) Format(state fmt.State, verb rune) {
	_, _ = fmt.Fprint(state, e.Error())
}

var _ CredentialLifecycle = (*CredentialSaga)(nil)
