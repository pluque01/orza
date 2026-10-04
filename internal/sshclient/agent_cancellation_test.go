package sshclient

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type blockedAgent struct {
	agent.Agent
	started, closed chan struct{}
	once            sync.Once
}

type blockedSigningAgent struct {
	agent.Agent
	public          ssh.PublicKey
	started, closed chan struct{}
	once            sync.Once
}
type blockedAgentSigner struct{ a *blockedSigningAgent }

func (a *blockedSigningAgent) Signers() ([]ssh.Signer, error) {
	return []ssh.Signer{&blockedAgentSigner{a}}, nil
}
func (s *blockedAgentSigner) PublicKey() ssh.PublicKey { return s.a.public }
func (s *blockedAgentSigner) Sign(io.Reader, []byte) (*ssh.Signature, error) {
	close(s.a.started)
	<-s.a.closed
	return nil, net.ErrClosed
}
func (a *blockedSigningAgent) Close() error { a.once.Do(func() { close(a.closed) }); return nil }

func TestTunnelRootCancellationDuringAgentSigning(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		config := &ssh.ServerConfig{PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) { return nil, nil }}
		config.AddHostKey(key)
		server, _, _, err := ssh.NewServerConn(raw, config)
		if err == nil {
			_ = server.Close()
		}
	}()
	a := &blockedSigningAgent{public: key.PublicKey(), started: make(chan struct{}), closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := New(Options{Authenticate: func(ctx context.Context, req app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
		result, err := buildAuthMethod(ctx, req.Connection, req.Secret, func(context.Context) (*AgentClient, error) { return newAgentClient(a, a), nil }, nil)
		if err != nil {
			return nil, nil, err
		}
		return []ssh.AuthMethod{result.Method}, result, nil
	}})
	req := tunnelRequest(app.TunnelLocal)
	req.Connection.Host = "127.0.0.1"
	req.Connection.Port = uint16(listener.Addr().(*net.TCPAddr).Port)
	req.Connection.AuthMethod = app.AuthMethodAgent
	req.Ready = func() { t.Error("ready during canceled authentication") }
	done := make(chan error, 1)
	go func() { done <- client.RunTunnel(ctx, req) }()
	forwardingDoubleAwait(t, a.started)
	cancel()
	tunnelResult(t, done)
	forwardingDoubleAwait(t, a.closed)
	forwardingDoubleAwait(t, serverDone)
}

func (a *blockedAgent) Signers() ([]ssh.Signer, error) {
	close(a.started)
	<-a.closed
	return nil, errors.New("agent disconnected")
}
func (a *blockedAgent) Close() error { a.once.Do(func() { close(a.closed) }); return nil }
func TestAgentCancellationDuringSignerList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &blockedAgent{started: make(chan struct{}), closed: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := buildAuthMethod(ctx, app.Connection{AuthMethod: app.AuthMethodAgent}, nil, func(context.Context) (*AgentClient, error) { return newAgentClient(a, a), nil }, nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("result=%v", err)
		}
	}()
	forwardingDoubleAwait(t, a.started)
	cancel()
	forwardingDoubleAwait(t, done)
	forwardingDoubleAwait(t, a.closed)
}
