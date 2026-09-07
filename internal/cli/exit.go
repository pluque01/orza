package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/domain"
)

const (
	ExitSuccess   = 0
	ExitInternal  = 1
	ExitUsage     = 2
	ExitNotFound  = 3
	ExitConflict  = 4
	ExitCanceled  = 5
	ExitSecurity  = 6
	ExitCatalog   = 7
	ExitTransport = 10
)

// ErrorCode is the stable machine-readable classification in JSON failures.
type ErrorCode string

const (
	CodeUsage     ErrorCode = "invalid_usage"
	CodeNotFound  ErrorCode = "not_found"
	CodeConflict  ErrorCode = "conflict"
	CodeCanceled  ErrorCode = "canceled"
	CodeSecurity  ErrorCode = "security_failure"
	CodeCatalog   ErrorCode = "catalog_unavailable"
	CodeTransport ErrorCode = "transport_failure"
	CodeInternal  ErrorCode = "internal"
)

// ManagementError separates safe display fields from an optional internal cause.
type ManagementError struct {
	code       ErrorCode
	message    string
	target     string
	endpoint   string
	diagnostic *app.SSHFailurePresentation
	cause      error
}

func newStartupError(code ErrorCode, message, target, endpoint string, diagnostic app.SSHFailurePresentation, cause error) error {
	return &ManagementError{
		code:       code,
		message:    message,
		target:     target,
		endpoint:   endpoint,
		diagnostic: &diagnostic,
		cause:      cause,
	}
}

func codeForStartupDiagnostic(diagnostic app.SSHFailurePresentation) ErrorCode {
	switch diagnostic.Category {
	case app.SSHFailureCanceled:
		return CodeCanceled
	case app.SSHFailureAuthenticationDenied, app.SSHFailureHostTrust, app.SSHFailureCredentialUnavailable:
		return CodeSecurity
	default:
		return CodeTransport
	}
}

// NewError creates an error safe to render. message and target must not contain secrets.
func NewError(code ErrorCode, message, target string, cause error) error {
	if message == "" {
		message = defaultMessage(code)
	}
	return &ManagementError{code: code, message: message, target: target, cause: cause}
}

func (e *ManagementError) Error() string {
	if e.target == "" {
		return e.message
	}
	return fmt.Sprintf("%s (target: %s)", e.message, e.target)
}

func (e *ManagementError) Unwrap() error {
	return e.cause
}

// RemoteExitError propagates a valid remote process status without classifying it
// as a local management failure.
type RemoteExitError struct {
	status int
}

func NewRemoteExitError(status int) error {
	return &RemoteExitError{status: status}
}

func (e *RemoteExitError) Error() string {
	return fmt.Sprintf("remote session exited with status %d", e.status)
}

// ExitCode maps command failures onto the stable CLI contract.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var remote *RemoteExitError
	if errors.As(err, &remote) {
		if remote.status >= 1 && remote.status <= 255 {
			return remote.status
		}
		return ExitTransport
	}

	var management *ManagementError
	if errors.As(err, &management) {
		return exitCodeForErrorCode(management.code)
	}

	var validation *domain.ValidationError
	switch {
	case errors.Is(err, context.Canceled):
		return ExitCanceled
	case errors.As(err, &validation),
		errors.Is(err, domain.ErrInvalidID),
		errors.Is(err, domain.ErrInvalidName),
		errors.Is(err, domain.ErrInvalidNodeKind),
		errors.Is(err, domain.ErrInvalidRevision),
		errors.Is(err, domain.ErrRevisionOverflow),
		errors.Is(err, domain.ErrInvalidPath):
		return ExitUsage
	case errors.Is(err, credential.ErrUnavailable), errors.Is(err, credential.ErrNotFound):
		return ExitSecurity
	case errors.Is(err, catalog.ErrApplicationID),
		errors.Is(err, catalog.ErrNewerSchema),
		errors.Is(err, catalog.ErrIntegrity):
		return ExitCatalog
	default:
		return ExitInternal
	}
}

func exitCodeForErrorCode(code ErrorCode) int {
	switch code {
	case CodeUsage:
		return ExitUsage
	case CodeNotFound:
		return ExitNotFound
	case CodeConflict:
		return ExitConflict
	case CodeCanceled:
		return ExitCanceled
	case CodeSecurity:
		return ExitSecurity
	case CodeCatalog:
		return ExitCatalog
	case CodeTransport:
		return ExitTransport
	default:
		return ExitInternal
	}
}

func defaultMessage(code ErrorCode) string {
	switch code {
	case CodeUsage:
		return "invalid command usage"
	case CodeNotFound:
		return "item not found"
	case CodeConflict:
		return "the item changed; reload before retrying"
	case CodeCanceled:
		return "operation canceled"
	case CodeSecurity:
		return "secure operation failed"
	case CodeCatalog:
		return "catalog is unavailable; verify its location and permissions"
	case CodeTransport:
		return "SSH transport failed"
	default:
		return "operation failed"
	}
}

func safeError(err error) (ErrorCode, string, string) {
	var remote *RemoteExitError
	if errors.As(err, &remote) {
		return CodeTransport, remote.Error(), ""
	}
	var management *ManagementError
	if errors.As(err, &management) {
		return management.code, management.message, management.target
	}

	code := CodeInternal
	switch ExitCode(err) {
	case ExitUsage:
		code = CodeUsage
	case ExitNotFound:
		code = CodeNotFound
	case ExitConflict:
		code = CodeConflict
	case ExitCanceled:
		code = CodeCanceled
	case ExitSecurity:
		code = CodeSecurity
	case ExitCatalog:
		code = CodeCatalog
	case ExitTransport:
		code = CodeTransport
	}
	return code, defaultMessage(code), ""
}

func startupErrorFields(err error) (string, *app.SSHFailurePresentation) {
	var management *ManagementError
	if !errors.As(err, &management) || management.diagnostic == nil {
		return "", nil
	}
	diagnostic := *management.diagnostic
	return management.endpoint, &diagnostic
}
