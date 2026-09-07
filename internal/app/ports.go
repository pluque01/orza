package app

import (
	"context"
	"time"

	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

// ConnectionRepository persists non-secret connection aggregates.
type ConnectionRepository interface {
	CreateConnection(context.Context, CreateConnectionRequest) (ConnectionResult, error)
	GetConnection(context.Context, ItemSelector) (ConnectionResult, error)
	ListConnections(context.Context, ListConnectionsRequest) (ListConnectionsResult, error)
	UpdateConnection(context.Context, UpdateConnectionRequest) (ConnectionResult, error)
	MoveConnection(context.Context, MoveConnectionRequest) (ConnectionResult, error)
	DeleteConnection(context.Context, DeleteConnectionRequest) (DeleteConnectionResult, error)
}

// FolderRepository persists folder aggregates and hierarchical operations.
type FolderRepository interface {
	CreateFolder(context.Context, CreateFolderRequest) (FolderResult, error)
	GetFolder(context.Context, ItemSelector) (FolderResult, error)
	ListChildren(context.Context, ListChildrenRequest) (ListChildrenResult, error)
	RenameFolder(context.Context, RenameFolderRequest) (FolderResult, error)
	MoveFolder(context.Context, MoveFolderRequest) (FolderResult, error)
	DeleteFolder(context.Context, DeleteFolderRequest) (DeleteFolderResult, error)
}

// CredentialOperationRepository persists recoverable credential saga state.
type CredentialOperationRepository interface {
	CreateCredentialOperation(context.Context, CredentialOperation) error
	GetCredentialOperation(context.Context, string) (CredentialOperation, error)
	ListCredentialOperations(context.Context) ([]CredentialOperation, error)
	SetCredentialOperationPhase(context.Context, string, CredentialOperationPhase) error
	DeleteCredentialOperation(context.Context, string) error
}

// TrustedHostRepository persists application-specific host trust decisions.
type TrustedHostRepository interface {
	GetTrustedHost(context.Context, HostEndpoint) (TrustedHost, error)
	TrustHost(context.Context, TrustHostRequest) (TrustedHost, error)
}

// HostTrust checks standard and application-specific host trust sources.
type HostTrust interface {
	CheckHost(context.Context, PresentedHost) (HostTrustResult, error)
}

// CredentialStore is the secure-store boundary used by application services.
type CredentialStore interface {
	credential.CredentialStore
}

// CredentialLifecycle is implemented by CredentialSaga and exposed as a port
// so connection and session use cases can be tested without native stores.
type CredentialLifecycle interface {
	Save(context.Context, Connection, []byte) error
	Replace(context.Context, Connection, []byte) error
	Remove(context.Context, Connection, AuthMethod, string) error
	DeleteConnection(context.Context, Connection) error
	Recover(context.Context) error
}

// Terminal is the local interactive-terminal boundary.
type Terminal interface {
	terminal.Terminal
}

// Clock makes timestamps deterministic in application tests.
type Clock interface {
	Now() time.Time
}

// SSHSessionRunner owns one interactive SSH session for the duration of Run.
type SSHSessionRunner interface {
	Run(context.Context, SSHSessionRequest) (SSHSessionResult, error)
}
