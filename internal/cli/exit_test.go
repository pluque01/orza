package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/domain"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", want: ExitSuccess},
		{name: "usage", err: NewError(CodeUsage, "bad usage", "", nil), want: ExitUsage},
		{name: "validation", err: domain.ErrInvalidName, want: ExitUsage},
		{name: "not found", err: NewError(CodeNotFound, "not found", "", nil), want: ExitNotFound},
		{name: "conflict", err: NewError(CodeConflict, "changed", "", nil), want: ExitConflict},
		{name: "canceled", err: context.Canceled, want: ExitCanceled},
		{name: "security", err: credential.ErrUnavailable, want: ExitSecurity},
		{name: "catalog", err: catalog.ErrIntegrity, want: ExitCatalog},
		{name: "transport", err: NewError(CodeTransport, "connection failed", "", nil), want: ExitTransport},
		{name: "unknown", err: errors.New("unexpected"), want: ExitInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestStartupDiagnosticCodeAndExitMappings(t *testing.T) {
	tests := []struct {
		category app.SSHFailureReason
		code     ErrorCode
		exit     int
	}{
		{app.SSHFailureTimeout, CodeTransport, ExitTransport},
		{app.SSHFailureAuthenticationDenied, CodeSecurity, ExitSecurity},
		{app.SSHFailureConnectionRefused, CodeTransport, ExitTransport},
		{app.SSHFailureHostNotFound, CodeTransport, ExitTransport},
		{app.SSHFailureNetworkUnreachable, CodeTransport, ExitTransport},
		{app.SSHFailureHostTrust, CodeSecurity, ExitSecurity},
		{app.SSHFailureCredentialUnavailable, CodeSecurity, ExitSecurity},
		{app.SSHFailureSSHNegotiation, CodeTransport, ExitTransport},
		{app.SSHFailureCanceled, CodeCanceled, ExitCanceled},
		{app.SSHFailureUnexpected, CodeTransport, ExitTransport},
	}
	for _, test := range tests {
		t.Run(string(test.category), func(t *testing.T) {
			diagnostic := app.NewSSHStartError(test.category, stageForCategory(test.category), detailForCategory(test.category), nil).Presentation()
			err := newStartupError(codeForStartupDiagnostic(diagnostic), "failed", "/prod", "prod.example:22", diagnostic, context.Canceled)
			code, _, _ := safeError(err)
			if code != test.code || ExitCode(err) != test.exit {
				t.Fatalf("code/exit = %s/%d, want %s/%d", code, ExitCode(err), test.code, test.exit)
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatal("startup error did not preserve its cause chain")
			}
			var management *ManagementError
			if !errors.As(err, &management) {
				t.Fatal("startup error did not preserve ManagementError errors.As")
			}
		})
	}
}

func stageForCategory(category app.SSHFailureReason) app.SSHFailureStage {
	switch category {
	case app.SSHFailureAuthenticationDenied:
		return app.SSHFailureStageAuthentication
	case app.SSHFailureHostNotFound:
		return app.SSHFailureStageTargetResolution
	case app.SSHFailureHostTrust:
		return app.SSHFailureStageHostTrust
	case app.SSHFailureCredentialUnavailable:
		return app.SSHFailureStageCredential
	case app.SSHFailureSSHNegotiation:
		return app.SSHFailureStageSSHNegotiation
	default:
		return app.SSHFailureStageNetworkConnection
	}
}

func detailForCategory(category app.SSHFailureReason) string {
	switch category {
	case app.SSHFailureTimeout:
		return "operation timed out"
	case app.SSHFailureAuthenticationDenied:
		return "server rejected available authentication methods"
	case app.SSHFailureConnectionRefused:
		return "remote endpoint refused connection"
	case app.SSHFailureHostNotFound:
		return "host name could not be resolved"
	case app.SSHFailureNetworkUnreachable:
		return "network is unreachable"
	case app.SSHFailureHostTrust:
		return "rejected"
	case app.SSHFailureCredentialUnavailable:
		return "agent"
	case app.SSHFailureSSHNegotiation:
		return "handshake"
	case app.SSHFailureCanceled:
		return "operation canceled"
	default:
		return ""
	}
}

func TestRemoteExitCode(t *testing.T) {
	for _, status := range []int{1, 42, 255} {
		err := NewRemoteExitError(status)
		if got := ExitCode(err); got != status {
			t.Fatalf("ExitCode(NewRemoteExitError(%d)) = %d", status, got)
		}
	}

	if got := ExitCode(NewRemoteExitError(256)); got != ExitTransport {
		t.Fatalf("invalid remote status mapped to %d, want %d", got, ExitTransport)
	}
}
