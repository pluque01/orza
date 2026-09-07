//go:build !windows

package sshclient

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestClassifyNetworkErrorUnix(t *testing.T) {
	tests := []struct {
		name   string
		cause  error
		reason app.SSHFailureReason
		stage  app.SSHFailureStage
	}{
		{"dns not found", &net.DNSError{Err: "localized resolver text", Name: "secret.example", IsNotFound: true}, app.SSHFailureHostNotFound, app.SSHFailureStageTargetResolution},
		{"refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, app.SSHFailureConnectionRefused, app.SSHFailureStageNetworkConnection},
		{"network unreachable", &net.OpError{Op: "dial", Err: syscall.ENETUNREACH}, app.SSHFailureNetworkUnreachable, app.SSHFailureStageNetworkConnection},
		{"host unreachable", &net.OpError{Op: "dial", Err: syscall.EHOSTUNREACH}, app.SSHFailureNetworkUnreachable, app.SSHFailureStageNetworkConnection},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifyNetworkError(test.cause)
			if got == nil || got.Reason() != test.reason || got.Stage() != test.stage || !errors.Is(got, test.cause) {
				t.Fatalf("classifyNetworkError() = %#v", got)
			}
		})
	}
}
