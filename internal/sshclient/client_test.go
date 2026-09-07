package sshclient

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestRunVerifiesHostBeforeAuthentication(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	events := []string{}
	recordingLocal := &recordingTerminal{Fake: local, restore: func() { events = append(events, "terminal.restore") }}
	connection := &fakeConn{close: func() error { events = append(events, "connection.close"); return nil }}
	remote := &fakeSession{wait: func() error { return nil }, close: func() error {
		events = append(events, "session.close")
		return nil
	}}
	transport := &fakeTransport{session: remote, close: func() error {
		events = append(events, "client.close")
		return nil
	}}
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	authCloser := closerFunc(func() error { events = append(events, "auth.close"); return nil })
	client := New(Options{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return connection, nil },
		Authenticate: func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			events = append(events, "authenticate")
			return []ssh.AuthMethod{ssh.Password("not-recorded")}, authCloser, nil
		},
		Handshake: func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
			if err := config.HostKeyCallback("ignored", fakeAddr("server:22"), publicKey); err != nil {
				return nil, err
			}
			events = append(events, "verified")
			if _, err := config.AuthCallback(new(ssh.ClientAuthContext)); err != nil {
				return nil, err
			}
			return transport, nil
		},
		Now: func() time.Time { return time.Unix(10, 0) },
	})
	request := testRequest(recordingLocal)
	request.Activate = func(context.Context) error {
		events = append(events, "activate")
		return nil
	}
	request.VerifyHost = func(_ context.Context, host app.PresentedHost) error {
		events = append(events, "verify")
		if host.Endpoint.CanonicalHost != "example.test" || host.Endpoint.Port != 2222 {
			t.Fatalf("endpoint = %+v", host.Endpoint)
		}
		if host.RemoteAddress != "server:22" || host.KeyAlgorithm == "" || host.FingerprintSHA256 == "" || len(host.PublicKey) == 0 {
			t.Fatalf("presented host = %+v", host)
		}
		return nil
	}

	result, err := client.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != app.SessionSucceeded || result.Outcome != app.SessionOutcomeSuccess {
		t.Fatalf("result = %+v", result)
	}
	want := []string{"verify", "verified", "authenticate", "activate", "session.close", "client.close", "connection.close", "auth.close", "terminal.restore"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	assertTerminalRestored(t, local)
}

func TestRunDialCancellationRestoresTerminal(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	ctx, cancel := context.WithCancel(context.Background())
	client := New(Options{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	result, err := client.Run(ctx, testRequest(local))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v", err)
	}
	var failure *app.SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != app.SSHFailureCanceled || failure.Stage() != app.SSHFailureStageNetworkConnection {
		t.Fatalf("Run cancellation = %v", err)
	}
	if result.State != app.SessionCanceled || result.Outcome != app.SessionOutcomeCanceled {
		t.Fatalf("result = %+v", result)
	}
	assertTerminalRestored(t, local)
}

func TestRunAuthenticationFailureClosesConnectionAndRestores(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	events := []string{}
	wantErr := errors.New("credentials rejected")
	connection := &fakeConn{close: func() error { events = append(events, "connection.close"); return nil }}
	client := New(Options{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return connection, nil },
		Authenticate: func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			return nil, closerFunc(func() error { events = append(events, "auth.close"); return nil }), wantErr
		},
		Handshake: func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
			if err := config.HostKeyCallback("", nil, testPublicKey(t)); err != nil {
				return nil, err
			}
			_, err := config.AuthCallback(new(ssh.ClientAuthContext))
			return nil, err
		},
	})
	result, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageAuthentication || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if result.Outcome != app.SessionOutcomeTransportFailure {
		t.Fatalf("result = %+v", result)
	}
	if want := []string{"connection.close", "auth.close"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	assertTerminalRestored(t, local)
}

func TestRunHostRejectionNeverRequestsAuthentication(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	wantErr := errors.New("changed host key")
	authCalled := false
	client := New(Options{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return &fakeConn{}, nil },
		Authenticate: func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			authCalled = true
			return nil, nil, nil
		},
		Handshake: func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
			return nil, config.HostKeyCallback("", nil, testPublicKey(t))
		},
	})
	request := testRequest(local)
	request.VerifyHost = func(context.Context, app.PresentedHost) error { return wantErr }
	_, err := client.Run(context.Background(), request)
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StageHostVerify || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if authCalled {
		t.Fatal("authentication ran before rejected host verification")
	}
	assertTerminalRestored(t, local)
}

func TestRunPTYFailureClosesInOrderAndRestores(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	events := []string{}
	wantErr := errors.New("pty rejected")
	remote := &fakeSession{requestPty: func(string, int, int, ssh.TerminalModes) error { return wantErr }, close: func() error {
		events = append(events, "session.close")
		return nil
	}}
	transport := &fakeTransport{session: remote, close: func() error { events = append(events, "client.close"); return nil }}
	client := successfulClient(local, transport, &fakeConn{close: func() error { events = append(events, "connection.close"); return nil }})
	_, err := client.Run(context.Background(), testRequest(local))
	var sshErr *Error
	if !errors.As(err, &sshErr) || sshErr.Stage != StagePTY || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if want := []string{"session.close", "client.close", "connection.close"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	assertTerminalRestored(t, local)
}

func TestHandshakeCancellationClosesUnderlyingConnection(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() { _ = serverConnection.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := handshake(ctx, clientConnection, "example.test:22", &ssh.ClientConfig{
		User: "alice",
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return errors.New("unexpected host-key callback")
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("handshake error = %v", err)
	}
	if _, err := clientConnection.Write([]byte("closed")); err == nil {
		t.Fatal("underlying connection remains open")
	}
}

func TestRunNormalizesPreActiveFailuresAndRestoresTerminal(t *testing.T) {
	signalErr := errors.New("process signal")
	tests := []struct {
		name       string
		configure  func(*terminal.Fake, *Options)
		reason     app.SSHFailureReason
		stage      app.SSHFailureStage
		cause      error
		wantClosed bool
	}{
		{"dns", func(_ *terminal.Fake, options *Options) {
			options.DialContext = func(context.Context, string, string) (net.Conn, error) { return nil, &net.DNSError{IsNotFound: true} }
		}, app.SSHFailureHostNotFound, app.SSHFailureStageTargetResolution, nil, false},
		{"deadline", func(_ *terminal.Fake, options *Options) {
			options.DialContext = func(context.Context, string, string) (net.Conn, error) { return nil, context.DeadlineExceeded }
		}, app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, context.DeadlineExceeded, false},
		{"host trust", func(_ *terminal.Fake, options *Options) {
			options.Handshake = callbackHandshake(func(config *ssh.ClientConfig) error { return config.HostKeyCallback("", nil, testPublicKeyNoTest()) })
		}, app.SSHFailureHostTrust, app.SSHFailureStageHostTrust, app.ErrHostRejected, true},
		{"handshake", func(_ *terminal.Fake, options *Options) {
			options.Handshake = func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) {
				return nil, errors.New("server canary")
			}
		}, app.SSHFailureSSHNegotiation, app.SSHFailureStageSSHNegotiation, nil, true},
		{"pty", func(_ *terminal.Fake, options *Options) {
			options.Handshake = transportHandshake(&fakeTransport{session: &fakeSession{requestPty: func(string, int, int, ssh.TerminalModes) error { return errors.New("pty canary") }}})
		}, app.SSHFailureSSHNegotiation, app.SSHFailureStageSessionSetup, nil, true},
		{"shell", func(_ *terminal.Fake, options *Options) {
			options.Handshake = transportHandshake(&fakeTransport{session: &fakeSession{shell: func() error { return errors.New("shell canary") }}})
		}, app.SSHFailureSSHNegotiation, app.SSHFailureStageSessionSetup, nil, true},
		{"terminal", func(local *terminal.Fake, _ *Options) {
			local.SetFault(terminal.OperationSize, errors.New("terminal canary"))
		}, app.SSHFailureUnexpected, app.SSHFailureStageLocalTerminal, nil, false},
		{"signal value", func(_ *terminal.Fake, options *Options) {
			options.DialContext = func(context.Context, string, string) (net.Conn, error) { return nil, signalErr }
		}, app.SSHFailureUnexpected, app.SSHFailureStageNetworkConnection, signalErr, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			local.QueueResize()
			closed := 0
			options := Options{DialContext: func(context.Context, string, string) (net.Conn, error) {
				return &fakeConn{close: func() error { closed++; return nil }}, nil
			}}
			test.configure(local, &options)
			request := testRequest(local)
			if test.name == "host trust" {
				request.VerifyHost = func(context.Context, app.PresentedHost) error { return app.ErrHostRejected }
			}
			_, err := New(options).Run(context.Background(), request)
			var failure *app.SSHStartError
			if !errors.As(err, &failure) || failure.Reason() != test.reason || failure.Stage() != test.stage {
				t.Fatalf("Run error = %v", err)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("Run error does not preserve cause: %v", err)
			}
			if test.wantClosed && closed != 1 {
				t.Fatalf("connection close count = %d", closed)
			}
			assertTerminalRestored(t, local)
		})
	}
}

func TestRunAuthenticationExhaustionIsDeniedButLocalFailureWins(t *testing.T) {
	localFailure := errors.New("local callback canary")
	tests := []struct {
		name      string
		auth      AuthenticationFunc
		want      app.SSHFailureReason
		wantCause error
	}{
		{"remote denial", func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			return []ssh.AuthMethod{ssh.Password("not-recorded")}, nil, nil
		}, app.SSHFailureAuthenticationDenied, nil},
		{"local callback", func(context.Context, app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
			return nil, nil, localFailure
		}, app.SSHFailureCredentialUnavailable, localFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			connection := &fakeConn{}
			client := New(Options{DialContext: func(context.Context, string, string) (net.Conn, error) { return connection, nil }, Authenticate: test.auth, Handshake: func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
				if err := config.HostKeyCallback("", nil, testPublicKey(t)); err != nil {
					return nil, err
				}
				if _, err := config.AuthCallback(new(ssh.ClientAuthContext)); err != nil {
					return nil, err
				}
				_, err := config.AuthCallback(new(ssh.ClientAuthContext))
				return nil, err
			}})
			_, err := client.Run(context.Background(), testRequest(local))
			var failure *app.SSHStartError
			if !errors.As(err, &failure) || failure.Reason() != test.want {
				t.Fatalf("Run error = %v", err)
			}
			if test.wantCause != nil && !errors.Is(err, test.wantCause) {
				t.Fatalf("cause lost: %v", err)
			}
			assertTerminalRestored(t, local)
		})
	}
}

func TestRunPreservesLazyAgentSigningFailureAcrossAuthCallback(t *testing.T) {
	wantErr := errors.New("agent signing canary")
	_, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	algorithm, ok := base.(ssh.AlgorithmSigner)
	if !ok {
		t.Fatal("test signer does not support algorithm selection")
	}
	signer := &failingAlgorithmSigner{AlgorithmSigner: algorithm, err: wantErr}
	host, port := startPublicKeyAuthServer(t)
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	client := New(Options{Authenticate: func(ctx context.Context, request app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
		result, err := buildAuthMethod(ctx, request.Connection, request.Secret, func(context.Context) (*AgentClient, error) {
			return newAgentClient(nil, &signerAgent{signer: signer}), nil
		}, nil)
		if err != nil {
			return nil, nil, err
		}
		return []ssh.AuthMethod{result.Method}, result, nil
	}})
	request := testRequest(local)
	request.Connection.Host = host
	request.Connection.Port = port
	request.Connection.AuthMethod = app.AuthMethodAgent

	_, err = client.Run(context.Background(), request)
	var failure *app.SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != app.SSHFailureCredentialUnavailable || failure.Stage() != app.SSHFailureStageCredential || !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v", err)
	}
	if signer.signCalls+signer.algorithmCalls == 0 {
		t.Fatal("SSH authentication did not attempt to sign")
	}
	assertTerminalRestored(t, local)
}

func TestRunDistinguishesEmptyAgentFromRejectedAgentIdentity(t *testing.T) {
	_, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		agent      agent.Agent
		wantReason app.SSHFailureReason
		wantStage  app.SSHFailureStage
		wantDetail string
	}{
		{"empty agent", agent.NewKeyring(), app.SSHFailureCredentialUnavailable, app.SSHFailureStageCredential, "agent"},
		{"rejected identity", &signerAgent{signer: signer}, app.SSHFailureAuthenticationDenied, app.SSHFailureStageAuthentication, "server rejected available authentication methods"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host, port := startRejectingPublicKeyAuthServer(t)
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			closed := &trackingCloser{}
			client := New(Options{Authenticate: func(ctx context.Context, request app.SSHSessionRequest) ([]ssh.AuthMethod, io.Closer, error) {
				result, err := buildAuthMethod(ctx, request.Connection, request.Secret, func(context.Context) (*AgentClient, error) {
					return newAgentClient(closed, test.agent), nil
				}, nil)
				if err != nil {
					return nil, nil, err
				}
				return []ssh.AuthMethod{result.Method}, result, nil
			}})
			request := testRequest(local)
			request.Connection.Host = host
			request.Connection.Port = port
			request.Connection.AuthMethod = app.AuthMethodAgent

			_, runErr := client.Run(context.Background(), request)
			var failure *app.SSHStartError
			if !errors.As(runErr, &failure) || failure.Reason() != test.wantReason || failure.Stage() != test.wantStage || failure.Presentation().TechnicalDetail != test.wantDetail {
				t.Fatalf("Run() error = %v", runErr)
			}
			if closed.calls != 1 {
				t.Fatalf("agent transport close calls = %d, want 1", closed.calls)
			}
			assertTerminalRestored(t, local)
		})
	}
}

func TestRunPrimaryClassificationSurvivesJoinedCleanupFailure(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	primary := errors.New("handshake primary canary")
	cleanup := errors.New("cleanup secondary canary")
	connection := &fakeConn{close: func() error { return cleanup }}
	client := New(Options{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return connection, nil },
		Handshake: func(context.Context, net.Conn, string, *ssh.ClientConfig) (sshTransport, error) {
			return nil, primary
		},
	})
	_, err := client.Run(context.Background(), testRequest(local))
	var failure *app.SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != app.SSHFailureSSHNegotiation || failure.Stage() != app.SSHFailureStageSSHNegotiation || !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("Run error = %v", err)
	}
	if rendered := err.Error(); strings.Contains(rendered, "primary canary") || strings.Contains(rendered, "secondary canary") {
		t.Fatalf("joined error exposed raw causes: %q", rendered)
	}
	assertTerminalRestored(t, local)
}

func TestErrorUsesOnlyControlledStageText(t *testing.T) {
	const canary = "private-stage-or-cause-canary"
	err := &Error{Stage: Stage(canary), Err: errors.New(canary)}
	if rendered := err.Error(); rendered != "ssh unknown failed" || strings.Contains(rendered, canary) {
		t.Fatalf("Error() = %q", rendered)
	}
	if !errors.Is(err, err.Err) {
		t.Fatal("safe error no longer unwraps its cause")
	}
}

func callbackHandshake(callback func(*ssh.ClientConfig) error) handshakeFunc {
	return func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
		return nil, callback(config)
	}
}

func transportHandshake(transport sshTransport) handshakeFunc {
	return func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
		if err := config.HostKeyCallback("", nil, testPublicKeyNoTest()); err != nil {
			return nil, err
		}
		return transport, nil
	}
}

func successfulClient(_ app.Terminal, transport sshTransport, connection net.Conn) *Client {
	return New(Options{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return connection, nil },
		Handshake: func(_ context.Context, _ net.Conn, _ string, config *ssh.ClientConfig) (sshTransport, error) {
			if err := config.HostKeyCallback("", nil, testPublicKeyNoTest()); err != nil {
				return nil, err
			}
			return transport, nil
		},
	})
}

func testRequest(local app.Terminal) app.SSHSessionRequest {
	return app.SSHSessionRequest{
		Connection: app.Connection{Host: "example.test", Port: 2222, Username: "alice"},
		VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
		Terminal:   local,
	}
}

func assertTerminalRestored(t *testing.T, local *terminal.Fake) {
	t.Helper()
	calls := local.Calls()
	if len(calls) < 2 || calls[0].Operation != terminal.OperationCapture || calls[len(calls)-1].Operation != terminal.OperationRestore {
		t.Fatalf("terminal calls = %v", calls)
	}
	if local.CurrentState().Raw() {
		t.Fatal("terminal remains raw")
	}
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	key := testPublicKeyNoTest()
	if key == nil {
		t.Fatal("could not construct public key")
	}
	return key
}

func testPublicKeyNoTest() ssh.PublicKey {
	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil
	}
	key, _ := ssh.NewPublicKey(public)
	return key
}

func startPublicKeyAuthServer(t *testing.T) (string, uint16) {
	return startPublicKeyAuthServerWithCallback(t, func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) { return nil, nil })
}

func startRejectingPublicKeyAuthServer(t *testing.T) (string, uint16) {
	return startPublicKeyAuthServerWithCallback(t, func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
		return nil, errors.New("public key rejected")
	})
}

func startPublicKeyAuthServerWithCallback(t *testing.T, callback func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error)) (string, uint16) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PublicKeyCallback: callback}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		server, _, _, err := ssh.NewServerConn(connection, config)
		if err == nil {
			_ = server.Close()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SSH authentication server did not stop")
		}
	})
	address := listener.Addr().(*net.TCPAddr)
	return address.IP.String(), uint16(address.Port)
}

type signerAgent struct {
	agent.Agent
	signer ssh.Signer
}

func (a *signerAgent) Signers() ([]ssh.Signer, error) { return []ssh.Signer{a.signer}, nil }

type failingAlgorithmSigner struct {
	ssh.AlgorithmSigner
	err            error
	signCalls      int
	algorithmCalls int
}

func (s *failingAlgorithmSigner) Sign(io.Reader, []byte) (*ssh.Signature, error) {
	s.signCalls++
	return nil, s.err
}

func (s *failingAlgorithmSigner) SignWithAlgorithm(io.Reader, []byte, string) (*ssh.Signature, error) {
	s.algorithmCalls++
	return nil, s.err
}

type fakeTransport struct {
	session    remoteSession
	err        error
	newSession func() (remoteSession, error)
	close      func() error
}

func (t *fakeTransport) NewSession() (remoteSession, error) {
	if t.newSession != nil {
		return t.newSession()
	}
	return t.session, t.err
}
func (t *fakeTransport) Close() error {
	if t.close != nil {
		return t.close()
	}
	return nil
}

type fakeSession struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	requestPty func(string, int, int, ssh.TerminalModes) error
	shell      func() error
	wait       func() error
	resize     func(int, int) error
	close      func() error
}

func (s *fakeSession) SetIO(stdin io.Reader, stdout, stderr io.Writer) {
	s.stdin, s.stdout, s.stderr = stdin, stdout, stderr
}
func (s *fakeSession) RequestPty(name string, rows, columns int, modes ssh.TerminalModes) error {
	if s.requestPty != nil {
		return s.requestPty(name, rows, columns, modes)
	}
	return nil
}
func (s *fakeSession) Shell() error {
	if s.shell != nil {
		return s.shell()
	}
	return nil
}
func (s *fakeSession) Wait() error {
	if s.wait != nil {
		return s.wait()
	}
	return nil
}
func (s *fakeSession) WindowChange(rows, columns int) error {
	if s.resize != nil {
		return s.resize(rows, columns)
	}
	return nil
}
func (s *fakeSession) Close() error {
	if s.close != nil {
		return s.close()
	}
	return nil
}

type fakeConn struct{ close func() error }

func (c *fakeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *fakeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *fakeConn) LocalAddr() net.Addr              { return fakeAddr("local") }
func (c *fakeConn) RemoteAddr() net.Addr             { return fakeAddr("remote") }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }
func (c *fakeConn) Close() error {
	if c.close != nil {
		return c.close()
	}
	return nil
}

type fakeAddr string

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return string(a) }

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

type recordingTerminal struct {
	*terminal.Fake
	restore func()
}

func (t *recordingTerminal) Restore(ctx context.Context, state terminal.State) error {
	if t.restore != nil {
		t.restore()
	}
	return t.Fake.Restore(ctx, state)
}
