package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestSSHClientAgainstInProcessServerRestoresTerminalAndReturnsRemoteStatus(t *testing.T) {
	server := startSSHServer(t, "test-password", 23)
	host, portText, err := net.SplitHostPort(server.address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	local := terminal.NewFake(terminal.Size{Columns: 90, Rows: 28})
	local.SetRaw(false)
	var output bytesBuffer
	client := sshclient.New(sshclient.Options{Stdin: &bytesBuffer{}, Stdout: &output, Stderr: &output})
	verified := false
	result, runErr := client.Run(context.Background(), app.SSHSessionRequest{
		Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodPassword},
		Terminal:   local,
		VerifyHost: func(_ context.Context, presented app.PresentedHost) error {
			verified = presented.FingerprintSHA256 != "" && presented.KeyAlgorithm == server.signer.PublicKey().Type()
			return nil
		},
		Secret: func(context.Context, app.SecretRequest) ([]byte, error) {
			if !verified {
				t.Fatal("password requested before host verification")
			}
			return []byte("test-password"), nil
		},
	})
	var remote interface{ Status() int }
	if !errors.As(runErr, &remote) || remote.Status() != 23 {
		t.Fatalf("Run() result = %#v, error = %v", result, runErr)
	}
	if result.RemoteExitStatus == nil || *result.RemoteExitStatus != 23 || result.Outcome != app.SessionOutcomeRemoteFailure {
		t.Fatalf("session result = %#v", result)
	}
	if !verified || output.String() != "server-output\n" {
		t.Fatalf("verified = %v, output = %q", verified, output.String())
	}
	if local.CurrentState().Raw() {
		t.Fatal("terminal remained raw after remote exit")
	}
	if !hasTerminalOperation(local.Calls(), terminal.OperationMakeRaw) || !hasTerminalOperation(local.Calls(), terminal.OperationRestore) {
		t.Fatalf("terminal calls = %#v", local.Calls())
	}
}

func TestSSHClientAgainstInProcessServerClassifiesRejectedPassword(t *testing.T) {
	server := startSSHServer(t, "accepted-password", 0)
	host, portText, err := net.SplitHostPort(server.address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	client := sshclient.New(sshclient.Options{Stdin: &bytesBuffer{}, Stdout: &bytesBuffer{}, Stderr: &bytesBuffer{}})
	result, runErr := client.Run(context.Background(), app.SSHSessionRequest{
		Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodPassword},
		Terminal:   local,
		VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
		Secret:     func(context.Context, app.SecretRequest) ([]byte, error) { return []byte("rejected-password"), nil },
	})
	var failure *app.SSHStartError
	if !errors.As(runErr, &failure) || failure.Reason() != app.SSHFailureAuthenticationDenied || failure.Stage() != app.SSHFailureStageAuthentication {
		t.Fatalf("Run() result = %#v, error = %v", result, runErr)
	}
	if result.StartedAt.IsZero() == false || result.RemoteExitStatus != nil {
		t.Fatalf("rejected session became active: %#v", result)
	}
	if local.CurrentState().Raw() || !hasTerminalOperation(local.Calls(), terminal.OperationRestore) {
		t.Fatalf("terminal not restored: %#v", local.Calls())
	}
	server.waitClosed(t)
}

func TestSSHClientAgainstInProcessServerPreservesLazyPasswordFailures(t *testing.T) {
	tests := []struct {
		name       string
		promptErr  error
		wantReason app.SSHFailureReason
		wantStage  app.SSHFailureStage
	}{
		{"credential unavailable", errors.New("secure store canary"), app.SSHFailureCredentialUnavailable, app.SSHFailureStageCredential},
		{"canceled", context.Canceled, app.SSHFailureCanceled, app.SSHFailureStageAuthentication},
		{"timeout", context.DeadlineExceeded, app.SSHFailureTimeout, app.SSHFailureStageAuthentication},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := startSSHServer(t, "accepted-password", 0)
			host, portText, err := net.SplitHostPort(server.address)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.ParseUint(portText, 10, 16)
			if err != nil {
				t.Fatal(err)
			}
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			client := sshclient.New(sshclient.Options{Stdin: &bytesBuffer{}, Stdout: &bytesBuffer{}, Stderr: &bytesBuffer{}})
			_, runErr := client.Run(context.Background(), app.SSHSessionRequest{
				Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodPassword},
				Terminal:   local,
				VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
				Secret:     func(context.Context, app.SecretRequest) ([]byte, error) { return nil, test.promptErr },
			})
			var failure *app.SSHStartError
			if !errors.As(runErr, &failure) || failure.Reason() != test.wantReason || failure.Stage() != test.wantStage || !errors.Is(runErr, test.promptErr) {
				t.Fatalf("Run() error = %v", runErr)
			}
			if local.CurrentState().Raw() || !hasTerminalOperation(local.Calls(), terminal.OperationRestore) {
				t.Fatalf("terminal not restored: %#v", local.Calls())
			}
			server.waitClosed(t)
		})
	}
}

func TestSSHClientAgainstInProcessServerDistinguishesEmptyAgentFromRejectedIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the in-process agent uses SSH_AUTH_SOCK")
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		privateKey any
		wantReason app.SSHFailureReason
		wantStage  app.SSHFailureStage
		wantDetail string
	}{
		{"empty agent", nil, app.SSHFailureCredentialUnavailable, app.SSHFailureStageCredential, "agent"},
		{"rejected identity", private, app.SSHFailureAuthenticationDenied, app.SSHFailureStageAuthentication, "server rejected available authentication methods"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			startSSHAgent(t, test.privateKey)
			server := startRejectingPublicKeySSHServer(t)
			host, portText, err := net.SplitHostPort(server.address)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.ParseUint(portText, 10, 16)
			if err != nil {
				t.Fatal(err)
			}
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			client := sshclient.New(sshclient.Options{Stdin: &bytesBuffer{}, Stdout: &bytesBuffer{}, Stderr: &bytesBuffer{}})
			result, runErr := client.Run(context.Background(), app.SSHSessionRequest{
				Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodAgent},
				Terminal:   local,
				VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
			})
			var failure *app.SSHStartError
			if !errors.As(runErr, &failure) || failure.Reason() != test.wantReason || failure.Stage() != test.wantStage || failure.Presentation().TechnicalDetail != test.wantDetail {
				t.Fatalf("Run() result = %#v, error = %v", result, runErr)
			}
			if result.StartedAt.IsZero() == false || result.RemoteExitStatus != nil {
				t.Fatalf("rejected session became active: %#v", result)
			}
			if local.CurrentState().Raw() || !hasTerminalOperation(local.Calls(), terminal.OperationRestore) {
				t.Fatalf("terminal not restored: %#v", local.Calls())
			}
			server.waitClosed(t)
			if test.privateKey != nil && server.publicKeyAttempts.Load() == 0 {
				t.Fatal("server did not reject the available agent identity")
			}
		})
	}
}

type sshTestServer struct {
	address           string
	signer            ssh.Signer
	close             func()
	closed            <-chan struct{}
	publicKeyAttempts *atomic.Int32
}

func startSSHServer(t *testing.T, password string, status uint32) sshTestServer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	configuration := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, secret []byte) (*ssh.Permissions, error) {
		if metadata.User() != "tester" || string(secret) != password {
			return nil, errors.New("authentication rejected")
		}
		return nil, nil
	}}
	configuration.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		serverConnection, channels, requests, err := ssh.NewServerConn(connection, configuration)
		if err != nil {
			return
		}
		defer serverConnection.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			if incoming.ChannelType() != "session" {
				_ = incoming.Reject(ssh.UnknownChannelType, "session required")
				continue
			}
			channel, channelRequests, err := incoming.Accept()
			if err != nil {
				return
			}
			for request := range channelRequests {
				switch request.Type {
				case "pty-req":
					_ = request.Reply(true, nil)
				case "shell":
					_ = request.Reply(true, nil)
					_, _ = io.WriteString(channel, "server-output\n")
					payload := make([]byte, 4)
					binary.BigEndian.PutUint32(payload, status)
					_, _ = channel.SendRequest("exit-status", false, payload)
					_ = channel.Close()
					return
				default:
					_ = request.Reply(false, nil)
				}
			}
		}
	}()
	closed := make(chan struct{})
	go func() { wait.Wait(); close(closed) }()
	server := sshTestServer{address: listener.Addr().String(), signer: signer, closed: closed}
	server.close = func() { _ = listener.Close(); wait.Wait() }
	t.Cleanup(server.close)
	return server
}

func startRejectingPublicKeySSHServer(t *testing.T) sshTestServer {
	t.Helper()
	attempts := &atomic.Int32{}
	return startSSHServerWithConfig(t, &ssh.ServerConfig{PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
		attempts.Add(1)
		return nil, errors.New("authentication rejected")
	}}, attempts)
}

func startSSHServerWithConfig(t *testing.T, configuration *ssh.ServerConfig, publicKeyAttempts *atomic.Int32) sshTestServer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	configuration.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		serverConnection, _, _, err := ssh.NewServerConn(connection, configuration)
		if err == nil {
			_ = serverConnection.Close()
		}
	}()
	closed := make(chan struct{})
	go func() { wait.Wait(); close(closed) }()
	server := sshTestServer{address: listener.Addr().String(), signer: signer, closed: closed, publicKeyAttempts: publicKeyAttempts}
	server.close = func() { _ = listener.Close(); wait.Wait() }
	t.Cleanup(server.close)
	return server
}

func startSSHAgent(t *testing.T, privateKey any) {
	t.Helper()
	keyring := agent.NewKeyring()
	if privateKey != nil {
		if err := keyring.Add(agent.AddedKey{PrivateKey: privateKey}); err != nil {
			t.Fatal(err)
		}
	}
	directory, err := os.MkdirTemp("/tmp", "orza-agent-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socket := filepath.Join(directory, "agent.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_AUTH_SOCK", socket)
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_ = agent.ServeAgent(keyring, connection)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SSH agent connection was not cleaned up")
		}
	})
}

func (s sshTestServer) waitClosed(t *testing.T) {
	t.Helper()
	select {
	case <-s.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("SSH server connection was not cleaned up")
	}
}

type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Read([]byte) (int, error) { return 0, io.EOF }
func (b *bytesBuffer) Write(value []byte) (int, error) {
	b.data = append(b.data, value...)
	return len(value), nil
}
func (b *bytesBuffer) String() string { return string(b.data) }

func hasTerminalOperation(calls []terminal.Call, operation terminal.Operation) bool {
	for _, call := range calls {
		if call.Operation == operation {
			return true
		}
	}
	return false
}
