package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
)

func TestSSHFailurePresentationParity(t *testing.T) {
	const causeCanary = "password=raw-private-cause-canary"
	connections := integrationConnectionService(t)
	created, err := connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: "parity", Host: "captured.example", Port: 2222, AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	connection := created.Connection
	tests := []struct {
		category app.SSHFailureReason
		stage    app.SSHFailureStage
		detail   string
	}{
		{app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out"},
		{app.SSHFailureAuthenticationDenied, app.SSHFailureStageAuthentication, "server rejected available authentication methods"},
		{app.SSHFailureConnectionRefused, app.SSHFailureStageNetworkConnection, "remote endpoint refused connection"},
		{app.SSHFailureHostNotFound, app.SSHFailureStageTargetResolution, "host name could not be resolved"},
		{app.SSHFailureNetworkUnreachable, app.SSHFailureStageNetworkConnection, "network is unreachable"},
		{app.SSHFailureHostTrust, app.SSHFailureStageHostTrust, "changed"},
		{app.SSHFailureCredentialUnavailable, app.SSHFailureStageCredential, "agent"},
		{app.SSHFailureSSHNegotiation, app.SSHFailureStageSSHNegotiation, "handshake"},
		{app.SSHFailureCanceled, app.SSHFailureStageLocalTerminal, "operation canceled"},
		{app.SSHFailureUnexpected, app.SSHFailureStageUnknown, ""},
	}

	// The TUI package exercises the same shared projection for all ten categories
	// without repeatedly starting terminal programs in this integration binary.
	// This matrix covers both CLI encodings against that shared projection.

	for _, test := range tests {
		t.Run(string(test.category), func(t *testing.T) {
			failure := app.NewSSHStartError(test.category, test.stage, test.detail, errors.New(causeCanary)).Presentation()
			readable, readableCalls, readableElapsed := runCLIFailure(t, connections, connection, failure, causeCanary, false)
			structured, structuredCalls, structuredElapsed := runCLIFailure(t, connections, connection, failure, causeCanary, true)

			for _, want := range []string{failure.Summary, string(failure.Category), string(failure.Stage), failure.Recommendation, connection.Path, "captured.example:2222"} {
				if !strings.Contains(readable, want) {
					t.Fatalf("readable CLI omitted %q: %q", want, readable)
				}
			}
			if strings.Contains(readable, causeCanary) {
				t.Fatalf("readable CLI leaked raw cause: %q", readable)
			}

			var envelope struct {
				OK    bool `json:"ok"`
				Error struct {
					Code            cli.ErrorCode        `json:"code"`
					Message         string               `json:"message"`
					Target          string               `json:"target"`
					Endpoint        string               `json:"endpoint"`
					Category        app.SSHFailureReason `json:"category"`
					Stage           app.SSHFailureStage  `json:"stage"`
					Recommendation  string               `json:"recommendation"`
					TechnicalDetail string               `json:"technicalDetail"`
				} `json:"error"`
			}
			decoder := json.NewDecoder(strings.NewReader(structured))
			if err := decoder.Decode(&envelope); err != nil {
				t.Fatalf("decode structured CLI: %v; output %q", err, structured)
			}
			var extra json.RawMessage
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				t.Fatalf("structured CLI emitted more than one object: %q", structured)
			}
			if envelope.OK || envelope.Error.Message != failure.Summary || envelope.Error.Target != connection.Path || envelope.Error.Endpoint != "captured.example:2222" ||
				envelope.Error.Category != failure.Category || envelope.Error.Stage != failure.Stage || envelope.Error.Recommendation != failure.Recommendation || envelope.Error.TechnicalDetail != failure.TechnicalDetail {
				t.Fatalf("structured CLI mismatch: %+v", envelope)
			}
			if envelope.Error.Code != parityErrorCode(test.category) || strings.Contains(structured, causeCanary) {
				t.Fatalf("structured CLI code or safety mismatch: %q", structured)
			}
			if failure.TechnicalDetail != "" && !strings.Contains(readable, "detail: "+failure.TechnicalDetail) {
				t.Fatalf("readable CLI omitted safe detail: %q", readable)
			}
			if len([]rune(failure.TechnicalDetail)) > 256 || len(readable) > 2048 || len(structured) > 2048 {
				t.Fatalf("diagnostic exceeded bounds: detail=%d readable=%d structured=%d", len([]rune(failure.TechnicalDetail)), len(readable), len(structured))
			}

			if readableCalls != 1 || structuredCalls != 1 {
				t.Fatalf("network operations after presentation: readable=%d structured=%d", readableCalls, structuredCalls)
			}
			for name, elapsed := range map[string]time.Duration{"readable CLI": readableElapsed, "structured CLI": structuredElapsed} {
				if elapsed < 0 || elapsed >= time.Second {
					t.Fatalf("%s completion-to-presentation = %v", name, elapsed)
				}
			}
		})
	}
}

func TestSSHStreamsRemainDirectAcrossSuccessAndNetworkInterruption(t *testing.T) {
	const streamBytes = 4 << 20
	for _, test := range []struct {
		name        string
		interrupted bool
		wantBytes   int64
	}{
		{name: "success", wantBytes: streamBytes},
		{name: "network-interruption", interrupted: true, wantBytes: streamBytes / 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := startParityStreamServer(t, streamBytes, test.interrupted)
			host, portText, err := net.SplitHostPort(server.address)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.ParseUint(portText, 10, 16)
			if err != nil {
				t.Fatal(err)
			}
			stdout := &parityCountingWriter{want: 'O'}
			stderr := &parityCountingWriter{want: 'E'}
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			client := sshclient.New(sshclient.Options{Stdin: strings.NewReader(""), Stdout: stdout, Stderr: stderr})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, runErr := client.Run(ctx, app.SSHSessionRequest{
				Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodPassword},
				Terminal:   local,
				VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
				Secret:     func(context.Context, app.SecretRequest) ([]byte, error) { return []byte("stream-password"), nil },
			})
			stdoutBytes, stderrBytes := stdout.bytes.Load(), stderr.bytes.Load()
			if stdout.invalid.Load() || stderr.invalid.Load() {
				t.Fatalf("stream contained invalid bytes: stdout=%v stderr=%v", stdout.invalid.Load(), stderr.invalid.Load())
			}
			if test.interrupted {
				// An abrupt transport close may discard bytes already accepted by the
				// remote writer. Require direct bounded delivery, not impossible TCP
				// completeness after interruption.
				if stdoutBytes == 0 || stderrBytes == 0 || stdoutBytes > test.wantBytes || stderrBytes > test.wantBytes {
					t.Fatalf("interrupted stream delivery stdout=%d stderr=%d, bounds 1..%d", stdoutBytes, stderrBytes, test.wantBytes)
				}
			} else if stdoutBytes != test.wantBytes || stderrBytes != test.wantBytes {
				t.Fatalf("successful stream delivery stdout=%d stderr=%d, want %d each", stdoutBytes, stderrBytes, test.wantBytes)
			}
			if test.interrupted {
				if runErr == nil || result.StartedAt.IsZero() || result.Outcome != app.SessionOutcomeTransportFailure {
					t.Fatalf("interrupted Run() = %+v, %v", result, runErr)
				}
			} else if runErr != nil || result.State != app.SessionSucceeded || result.Outcome != app.SessionOutcomeSuccess {
				t.Fatalf("successful Run() = %+v, %v", result, runErr)
			}
			assertParityTerminalRestored(t, local)
			server.wait(t)
		})
	}
}

func TestSSHTimeoutAndCancellationRestoreTerminal(t *testing.T) {
	for _, test := range []struct {
		name       string
		timeout    bool
		wantErr    error
		wantReason app.SSHFailureReason
		wantState  app.SessionState
	}{
		{name: "timeout", timeout: true, wantErr: context.DeadlineExceeded, wantReason: app.SSHFailureTimeout, wantState: app.SessionFailed},
		{name: "cancellation", wantErr: context.Canceled, wantReason: app.SSHFailureCanceled, wantState: app.SessionCanceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			entered := make(chan struct{})
			client := sshclient.New(sshclient.Options{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				close(entered)
				<-ctx.Done()
				return nil, ctx.Err()
			}})
			ctx := context.Background()
			var cancel context.CancelFunc
			if test.timeout {
				ctx, cancel = context.WithTimeout(ctx, 25*time.Millisecond)
			} else {
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					<-entered
					cancel()
				}()
			}
			defer cancel()
			result, runErr := client.Run(ctx, app.SSHSessionRequest{
				Connection: app.Connection{Host: "injected.example", Port: 22, Username: "tester", AuthMethod: app.AuthMethodAgent},
				Terminal:   local,
				VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
			})
			var failure *app.SSHStartError
			if !errors.Is(runErr, test.wantErr) || !errors.As(runErr, &failure) || failure.Reason() != test.wantReason || result.State != test.wantState {
				t.Fatalf("Run() = %+v, %v", result, runErr)
			}
			assertParityTerminalRestored(t, local)
		})
	}
}

func runCLIFailure(t *testing.T, connections *app.ConnectionService, connection app.Connection, failure app.SSHFailurePresentation, causeCanary string, jsonOutput bool) (string, int, time.Duration) {
	t.Helper()
	runner := &parityFailureRunner{failure: failure, cause: errors.New(causeCanary)}
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	connect, err := app.NewConnectService(app.ConnectOptions{
		Connections: parityConnectionRepository{connections}, Credentials: parityCredentialLifecycle{},
		HostTrust: knownHostTrust{}, TrustedHosts: noOpTrustedHosts{}, Store: credential.NewFake(),
		Scope: credential.Scope("dddddddddddddddddddddddddddddddd"), Runner: runner, Terminal: local,
	})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	root := cli.NewRoot(cli.RootConfig{Connections: connections, Connect: connect, Terminal: local, Stdout: &stdout, Stderr: &stderr})
	args := []string{"connect", connection.Path}
	if jsonOutput {
		args = append([]string{"--json"}, args...)
	}
	root.SetArgs(args)
	commandErr := root.ExecuteContext(context.Background())
	if commandErr == nil {
		t.Fatal("CLI connect unexpectedly succeeded")
	}
	if err := cli.WriteError(&stderr, jsonOutput, commandErr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), causeCanary) || strings.Contains(stdout.String(), `"ok"`) {
		t.Fatalf("CLI stdout crossed output or safety boundary: %q", stdout.String())
	}
	return stderr.String(), runner.calls, time.Since(runner.completed)
}

type parityConnectionRepository struct{ service *app.ConnectionService }

func (r parityConnectionRepository) GetConnection(ctx context.Context, selector app.ItemSelector) (app.ConnectionResult, error) {
	return r.service.Get(ctx, selector)
}
func (parityConnectionRepository) CreateConnection(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (parityConnectionRepository) ListConnections(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
	return app.ListConnectionsResult{}, app.ErrInvalidRequest
}
func (parityConnectionRepository) UpdateConnection(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (parityConnectionRepository) MoveConnection(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (parityConnectionRepository) DeleteConnection(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
	return app.DeleteConnectionResult{}, app.ErrInvalidRequest
}

type parityCredentialLifecycle struct{}

func (parityCredentialLifecycle) Save(context.Context, app.Connection, []byte) error    { return nil }
func (parityCredentialLifecycle) Replace(context.Context, app.Connection, []byte) error { return nil }
func (parityCredentialLifecycle) Remove(context.Context, app.Connection, app.AuthMethod, string) error {
	return nil
}
func (parityCredentialLifecycle) DeleteConnection(context.Context, app.Connection) error { return nil }
func (parityCredentialLifecycle) Recover(context.Context) error                          { return nil }

type parityFailureRunner struct {
	failure   app.SSHFailurePresentation
	cause     error
	calls     int
	completed time.Time
}

func (r *parityFailureRunner) Run(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error) {
	r.calls++
	r.completed = time.Now()
	return app.SSHSessionResult{}, app.NewSSHStartError(r.failure.Category, r.failure.Stage, r.failure.TechnicalDetail, r.cause)
}

func parityErrorCode(category app.SSHFailureReason) cli.ErrorCode {
	switch category {
	case app.SSHFailureCanceled:
		return cli.CodeCanceled
	case app.SSHFailureAuthenticationDenied, app.SSHFailureHostTrust, app.SSHFailureCredentialUnavailable:
		return cli.CodeSecurity
	default:
		return cli.CodeTransport
	}
}

type parityStreamServer struct {
	address string
	done    <-chan struct{}
	close   func()
}

func startParityStreamServer(t *testing.T, streamBytes int, interrupted bool) parityStreamServer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	configuration := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != "tester" || string(password) != "stream-password" {
			return nil, errors.New("authentication rejected")
		}
		return nil, nil
	}}
	configuration.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		serverConnection, channels, requests, handshakeErr := ssh.NewServerConn(connection, configuration)
		if handshakeErr != nil {
			return
		}
		defer serverConnection.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			if incoming.ChannelType() != "session" {
				_ = incoming.Reject(ssh.UnknownChannelType, "session required")
				continue
			}
			channel, channelRequests, channelErr := incoming.Accept()
			if channelErr != nil {
				return
			}
			for request := range channelRequests {
				switch request.Type {
				case "pty-req":
					_ = request.Reply(true, nil)
				case "shell":
					_ = request.Reply(true, nil)
					amount := streamBytes
					if interrupted {
						amount /= 2
					}
					if writeParityStream(channel, 'O', amount) != nil || writeParityStream(channel.Stderr(), 'E', amount) != nil {
						return
					}
					if interrupted {
						_ = connection.Close()
						return
					}
					payload := make([]byte, 4)
					binary.BigEndian.PutUint32(payload, 0)
					_, _ = channel.SendRequest("exit-status", false, payload)
					_ = channel.Close()
					// Keep the transport alive until the client drains both channel
					// streams and closes it during deterministic cleanup.
					_ = serverConnection.Wait()
					return
				default:
					_ = request.Reply(false, nil)
				}
			}
		}
	}()
	server := parityStreamServer{address: listener.Addr().String(), done: done}
	server.close = func() { _ = listener.Close() }
	t.Cleanup(server.close)
	return server
}

func writeParityStream(writer io.Writer, value byte, amount int) error {
	chunk := bytes.Repeat([]byte{value}, 32<<10)
	for amount > 0 {
		size := min(amount, len(chunk))
		if _, err := writer.Write(chunk[:size]); err != nil {
			return err
		}
		amount -= size
	}
	return nil
}

func (s parityStreamServer) wait(t *testing.T) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream server did not release its connection")
	}
}

type parityCountingWriter struct {
	want    byte
	bytes   atomic.Int64
	invalid atomic.Bool
}

func (w *parityCountingWriter) Write(value []byte) (int, error) {
	for _, current := range value {
		if current != w.want {
			w.invalid.Store(true)
			break
		}
	}
	w.bytes.Add(int64(len(value)))
	return len(value), nil
}

func assertParityTerminalRestored(t *testing.T, local *terminal.Fake) {
	t.Helper()
	calls := local.Calls()
	restores := 0
	for _, call := range calls {
		if call.Operation == terminal.OperationRestore {
			restores++
		}
	}
	if local.CurrentState().Raw() || restores != 1 || len(calls) == 0 || calls[len(calls)-1].Operation != terminal.OperationRestore {
		t.Fatalf("terminal cleanup calls = %#v", calls)
	}
}
