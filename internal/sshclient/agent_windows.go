//go:build windows

package sshclient

import (
	"context"
	"fmt"

	winio "github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh/agent"
)

const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

func OpenAgent(ctx context.Context) (*AgentClient, error) {
	connection, err := winio.DialPipeContext(ctx, openSSHAgentPipe)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	return newAgentClient(connection, agent.NewClient(connection)), nil
}
