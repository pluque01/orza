package app

import (
	"context"
	"net"
	"strconv"
)

type TunnelMode string

const (
	TunnelLocal   TunnelMode = "local"
	TunnelRemote  TunnelMode = "remote"
	TunnelDynamic TunnelMode = "dynamic"
)

type TunnelEndpoint struct {
	Host string
	Port uint16
}

func (e TunnelEndpoint) String() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(int(e.Port)))
}

type TunnelConfig struct {
	Mode                 TunnelMode
	Listen               TunnelEndpoint
	Destination          TunnelEndpoint
	ExposureAcknowledged bool
}

type TunnelRequest struct {
	Connection     ItemSelector
	Expected       *Revision
	Config         TunnelConfig
	NonInteractive bool
	DecideTrust    TrustDecisionFunc
	ReadSecret     func(context.Context, SecretRequest) ([]byte, error)
}

// TunnelRunRequest is terminal-free. Ready is called only after the listening
// request succeeds; client diagnostics are controlled text, never raw errors.
type TunnelRunRequest struct {
	Connection Connection
	Config     TunnelConfig
	VerifyHost func(context.Context, PresentedHost) error
	Secret     func(context.Context, SecretRequest) ([]byte, error)
	Ready      func()
	Diagnostic func(string)
}

type SSHTunnelRunner interface {
	RunTunnel(context.Context, TunnelRunRequest) error
}

type TunnelState string

const (
	TunnelStarting TunnelState = "Starting"
	TunnelActive   TunnelState = "Active"
	TunnelStopping TunnelState = "Stopping"
	TunnelStopped  TunnelState = "Stopped"
	TunnelFailed   TunnelState = "Failed"
)

type TunnelSnapshot struct {
	ID         uint64
	Attempt    uint64
	Version    uint64
	Connection SSHAttemptTarget
	Config     TunnelConfig
	State      TunnelState
	Scope      string
	Warning    string
	Diagnostic string
}

func (s TunnelSnapshot) Live() bool {
	return s.State == TunnelStarting || s.State == TunnelActive || s.State == TunnelStopping
}
