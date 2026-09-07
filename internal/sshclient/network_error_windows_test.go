//go:build windows

package sshclient

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestClassifyNetworkErrorWindows(t *testing.T) {
	tests := []struct {
		name   string
		cause  error
		reason app.SSHFailureReason
	}{
		{"refused", &net.OpError{Op: "dial", Err: syscall.Errno(10061)}, app.SSHFailureConnectionRefused},
		{"network unreachable", &net.OpError{Op: "dial", Err: syscall.Errno(10051)}, app.SSHFailureNetworkUnreachable},
		{"host unreachable", &net.OpError{Op: "dial", Err: syscall.Errno(10065)}, app.SSHFailureNetworkUnreachable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifyNetworkError(test.cause)
			if got == nil || got.Reason() != test.reason || got.Stage() != app.SSHFailureStageNetworkConnection || !errors.Is(got, test.cause) {
				t.Fatalf("classifyNetworkError() = %#v", got)
			}
		})
	}
}
