//go:build !windows

package sshclient

import (
	"context"
	"errors"
	"net"
	"syscall"

	"github.com/pluque01/orza/internal/app"
)

func classifyNetworkError(cause error) *app.SSHStartError {
	var dns *net.DNSError
	if errors.As(cause, &dns) {
		if dns.IsTimeout || errors.Is(cause, context.DeadlineExceeded) {
			return app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageTargetResolution, "operation timed out", cause)
		}
		if dns.IsNotFound {
			return app.NewSSHStartError(app.SSHFailureHostNotFound, app.SSHFailureStageTargetResolution, "host name could not be resolved", cause)
		}
	}
	if errors.Is(cause, context.DeadlineExceeded) || isNetworkTimeout(cause) {
		return app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", cause)
	}
	if errors.Is(cause, syscall.ECONNREFUSED) {
		return app.NewSSHStartError(app.SSHFailureConnectionRefused, app.SSHFailureStageNetworkConnection, "remote endpoint refused connection", cause)
	}
	if errors.Is(cause, syscall.ENETUNREACH) || errors.Is(cause, syscall.EHOSTUNREACH) {
		return app.NewSSHStartError(app.SSHFailureNetworkUnreachable, app.SSHFailureStageNetworkConnection, "network is unreachable", cause)
	}
	return nil
}

func isNetworkTimeout(cause error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(cause, &timeout) && timeout.Timeout()
}
