//go:build !windows

package sshclient

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/agent"
)

func OpenAgent(ctx context.Context) (*AgentClient, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if strings.TrimSpace(socket) == "" {
		return nil, ErrAgentUnavailable
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	return newAgentClient(connection, agent.NewClient(connection)), nil
}
