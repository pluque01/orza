package app

import (
	"context"
	"time"
)

// NodeID is a catalog node's canonical, stable identifier.
type NodeID string

// Revision is an optimistic-lock version. Persisted revisions start at one.
type Revision uint64

// CatalogRevision identifies a committed catalog transaction.
type CatalogRevision uint64

// ItemSelector identifies exactly one node by ID or absolute logical path.
type ItemSelector struct {
	ID   NodeID
	Path string
}

type NodeKind string

const (
	NodeKindFolder     NodeKind = "folder"
	NodeKindConnection NodeKind = "connection"
)

// Node contains the fields shared by non-secret catalog projections.
type Node struct {
	ID        NodeID
	ParentID  NodeID
	Kind      NodeKind
	Name      string
	Path      string
	Revision  Revision
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AuthMethod string

const (
	AuthMethodAgent    AuthMethod = "agent"
	AuthMethodKey      AuthMethod = "key"
	AuthMethodPassword AuthMethod = "password"
)

// Connection is a non-secret catalog projection.
type Connection struct {
	Node
	Host          string
	Port          uint16
	Username      string
	AuthMethod    AuthMethod
	IdentityFile  string
	CredentialRef string
}

type Folder struct {
	Node
}

type CreateConnectionRequest struct {
	Parent             ItemSelector
	ExpectedParent     *Revision
	ExpectedParentPath string
	Name               string
	Host               string
	Port               uint16
	Username           string
	AuthMethod         AuthMethod
	IdentityFile       string
	CredentialIntent   CredentialIntent
	// Password is accepted only when CredentialIntent is CredentialRemember.
	// Services strip it before crossing the catalog repository boundary. The
	// caller retains ownership of this slice and should clear it after use.
	Password []byte
}

// UpdateConnectionRequest uses nil for unchanged fields. Empty pointed-to
// strings explicitly clear optional fields.
type UpdateConnectionRequest struct {
	Connection       ItemSelector
	Expected         *Revision
	Name             *string
	Host             *string
	Port             *uint16
	Username         *string
	AuthMethod       *AuthMethod
	IdentityFile     *string
	CredentialIntent CredentialIntent
	// Password is transient input for CredentialRemember and is never returned.
	// The caller retains ownership of this slice and should clear it after use.
	Password []byte
}

type MoveConnectionRequest struct {
	Connection              ItemSelector
	Destination             ItemSelector
	Expected                *Revision
	ExpectedDestination     *Revision
	ExpectedSourcePath      string
	ExpectedDestinationPath string
}

type DeleteConnectionRequest struct {
	Connection ItemSelector
	Expected   *Revision
}

type ListConnectionsRequest struct {
	Folder ItemSelector
}

type ConnectionResult struct {
	Connection      Connection
	CatalogRevision CatalogRevision
}

type ListConnectionsResult struct {
	Connections     []Connection
	CatalogRevision CatalogRevision
}

type DeleteConnectionResult struct {
	Deleted         Connection
	CatalogRevision CatalogRevision
}

// CredentialIntent describes an explicit remembered-password change. Its zero
// value preserves the current credential state.
type CredentialIntent string

const (
	CredentialKeep     CredentialIntent = ""
	CredentialRemember CredentialIntent = "remember"
	CredentialForget   CredentialIntent = "forget"
)

// ConnectionDeleteScope is safe to display for confirmation and pins the
// exact connection revision that was shown to the user.
type ConnectionDeleteScope struct {
	ID                    NodeID
	Path                  string
	Name                  string
	Host                  string
	Username              string
	Revision              Revision
	HasRememberedPassword bool
}

// Request returns the compare-and-swap deletion authorized by this scope.
func (scope ConnectionDeleteScope) Request() DeleteConnectionRequest {
	expected := scope.Revision
	return DeleteConnectionRequest{
		Connection: ItemSelector{ID: scope.ID},
		Expected:   &expected,
	}
}

type CreateFolderRequest struct {
	Parent             ItemSelector
	ExpectedParent     *Revision
	ExpectedParentPath string
	Name               string
}

type RenameFolderRequest struct {
	Folder   ItemSelector
	Name     string
	Expected *Revision
}

type MoveFolderRequest struct {
	Folder                  ItemSelector
	Destination             ItemSelector
	Expected                *Revision
	ExpectedDestination     *Revision
	ExpectedSourcePath      string
	ExpectedDestinationPath string
}

type DeleteFolderRequest struct {
	Folder    ItemSelector
	Expected  *Revision
	Recursive bool
	Snapshot  []NodeRevision
}

type ListChildrenRequest struct {
	Folder ItemSelector
}

type NodeRevision struct {
	ID       NodeID
	Revision Revision
}

// FolderDeleteScope is the complete, non-secret subtree shown for destructive
// confirmation. Snapshot pins both membership and every observed revision.
type FolderDeleteScope struct {
	ID                    NodeID
	Path                  string
	Name                  string
	Revision              Revision
	Folders               int
	Connections           int
	RememberedCredentials int
	Snapshot              []NodeRevision
}

// Request returns a deletion request with its own snapshot copy so adapters
// cannot accidentally mutate the scope after presenting it.
func (scope FolderDeleteScope) Request(recursive bool) DeleteFolderRequest {
	expected := scope.Revision
	return DeleteFolderRequest{
		Folder:    ItemSelector{ID: scope.ID},
		Expected:  &expected,
		Recursive: recursive,
		Snapshot:  append([]NodeRevision(nil), scope.Snapshot...),
	}
}

type FolderResult struct {
	Folder          Folder
	CatalogRevision CatalogRevision
}

type ListChildrenResult struct {
	Folders         []Folder
	Connections     []Connection
	CatalogRevision CatalogRevision
}

type DeleteFolderResult struct {
	FoldersDeleted     int
	ConnectionsDeleted int
	CatalogRevision    CatalogRevision
}

// HostEndpoint identifies the host identity namespace used by SSH.
type HostEndpoint struct {
	CanonicalHost string
	Port          uint16
}

// PresentedHost contains public host-key information safe to display.
type PresentedHost struct {
	Endpoint          HostEndpoint
	RemoteAddress     string
	KeyAlgorithm      string
	PublicKey         []byte
	FingerprintSHA256 string
}

type TrustedHost struct {
	ID string
	HostEndpoint
	KeyAlgorithm      string
	PublicKey         []byte
	FingerprintSHA256 string
	Revision          Revision
	AcceptedAt        time.Time
}

type HostTrustStatus string

const (
	HostTrustKnown   HostTrustStatus = "known"
	HostTrustUnknown HostTrustStatus = "unknown"
	HostTrustChanged HostTrustStatus = "changed"
	HostTrustRevoked HostTrustStatus = "revoked"
)

type HostTrustResult struct {
	Status HostTrustStatus
	Known  *TrustedHost
}

type TrustHostRequest struct {
	Host             PresentedHost
	ExpectedRevision *Revision
}

type TrustDecision string

const (
	TrustReject  TrustDecision = "reject"
	TrustOnce    TrustDecision = "once"
	TrustPersist TrustDecision = "persist"
)

// TrustDecisionPrompt contains only public host identity information.
type TrustDecisionPrompt struct {
	Host   PresentedHost
	Status HostTrustStatus
	Known  *TrustedHost
}

type TrustDecisionFunc func(context.Context, TrustDecisionPrompt) (TrustDecision, error)

type CredentialOperationKind string

const (
	CredentialOperationSave             CredentialOperationKind = "save"
	CredentialOperationReplace          CredentialOperationKind = "replace"
	CredentialOperationRemove           CredentialOperationKind = "remove"
	CredentialOperationDeleteConnection CredentialOperationKind = "delete_connection"
)

type CredentialOperationPhase string

const (
	CredentialPhasePrepared       CredentialOperationPhase = "prepared"
	CredentialPhaseSecretChanged  CredentialOperationPhase = "secret_changed"
	CredentialPhaseCatalogChanged CredentialOperationPhase = "catalog_changed"
	CredentialPhaseCleanupPending CredentialOperationPhase = "cleanup_pending"
)

// CredentialOperation contains durable saga metadata and never secret bytes.
type CredentialOperation struct {
	ID                 string
	ConnectionID       NodeID
	ExpectedRevision   Revision
	Kind               CredentialOperationKind
	OldRef             string
	NewRef             string
	Phase              CredentialOperationPhase
	TargetAuthMethod   AuthMethod
	TargetIdentityFile string
	CreatedAt          time.Time
}

type SessionState string

const (
	SessionIdle           SessionState = "idle"
	SessionResolving      SessionState = "resolving"
	SessionVerifyingHost  SessionState = "verifying_host"
	SessionAuthenticating SessionState = "authenticating"
	SessionOpeningPTY     SessionState = "opening_pty"
	SessionActive         SessionState = "active"
	SessionClosing        SessionState = "closing"
	SessionSucceeded      SessionState = "succeeded"
	SessionFailed         SessionState = "failed"
	SessionCanceled       SessionState = "canceled"
)

type SessionOutcome string

const (
	SessionOutcomeSuccess          SessionOutcome = "success"
	SessionOutcomeRemoteFailure    SessionOutcome = "remote_failure"
	SessionOutcomeTransportFailure SessionOutcome = "transport_failure"
	SessionOutcomeCanceled         SessionOutcome = "canceled"
)

type SecretKind string

const (
	SecretPassword   SecretKind = "password"
	SecretPassphrase SecretKind = "passphrase"
)

type SecretRequest struct {
	Kind       SecretKind
	Prompt     string
	Credential string
}

// SSHSessionRequest supplies callbacks so host verification occurs before an
// adapter requests authentication secrets. Returned secret slices are ephemeral.
type SSHSessionRequest struct {
	Connection Connection
	VerifyHost func(context.Context, PresentedHost) error
	Secret     func(context.Context, SecretRequest) ([]byte, error)
	// Activate blocks immediately before the interactive session takes ownership
	// of the local terminal. Presentations use it to release their input reader.
	Activate func(context.Context) error
	Terminal Terminal
}

type SSHSessionResult struct {
	State            SessionState
	Outcome          SessionOutcome
	StartedAt        time.Time
	RemoteExitStatus *int
	Failure          *SSHFailurePresentation
}

// SSHFailurePresentation is the cause-free diagnostic shared by all user
// interfaces. Its strings are controlled by the application package.
type SSHFailurePresentation struct {
	Category        SSHFailureReason
	Stage           SSHFailureStage
	Summary         string
	Recommendation  string
	TechnicalDetail string
}

// SSHAttemptTarget captures the public catalog identity used for one attempt.
// It intentionally excludes usernames and all credential-related fields.
type SSHAttemptTarget struct {
	ID       NodeID
	Revision Revision
	Path     string
	Host     string
	Port     uint16
}

type ConnectRequest struct {
	Connection  ItemSelector
	Expected    *Revision
	DecideTrust TrustDecisionFunc
	// ReadSecret is an optional presentation boundary for terminal-owned,
	// ephemeral authentication input. Returned bytes must not be retained.
	ReadSecret func(context.Context, SecretRequest) ([]byte, error)
	// Activate is called immediately before direct remote terminal streaming.
	Activate func(context.Context) error
}

// ConnectResult is a secret-free snapshot of the selected connection and its
// terminal session result.
type ConnectResult struct {
	Connection Connection
	Attempt    SSHAttemptTarget
	Session    SSHSessionResult
}
