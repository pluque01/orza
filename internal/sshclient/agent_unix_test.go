//go:build !windows

package sshclient

import (
	"context"
	"errors"
	"testing"
)

func TestOpenAgentRequiresSSHAuthSock(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	client, err := OpenAgent(context.Background())
	if client != nil {
		_ = client.Close()
	}
	if !errors.Is(err, ErrAgentUnavailable) {
		t.Fatalf("OpenAgent() error = %v, want unavailable", err)
	}
}
