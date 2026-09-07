package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/crypto/ssh"
)

func TestSecurityRegressions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDirectory := filepath.Join(home, ".ssh")
	if err := os.Mkdir(sshDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sshDirectory, "config")
	configCanary := []byte("Host existing\n    HostName existing.example\n")
	if err := os.WriteFile(configPath, configCanary, 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Errorf("read SSH config after security tests: %v", err)
			return
		}
		info, err := os.Stat(configPath)
		if err != nil {
			t.Errorf("stat SSH config after security tests: %v", err)
			return
		}
		if !bytes.Equal(got, configCanary) || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Errorf("~/.ssh/config changed: contents=%q mode=%04o", got, info.Mode().Perm())
		}
	}()

	t.Run("human and JSON errors redact causes", func(t *testing.T) {
		const secret = "password=redaction-canary"
		managementErr := cli.NewError(cli.CodeSecurity, "host trust or authentication failed", "/prod", errors.New(secret))
		for _, jsonOutput := range []bool{false, true} {
			var output bytes.Buffer
			if err := cli.WriteError(&output, jsonOutput, managementErr); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), secret) {
				t.Fatalf("error output leaked cause: %q", output.String())
			}
			if jsonOutput && !json.Valid(output.Bytes()) {
				t.Fatalf("error output is not valid JSON: %q", output.String())
			}
		}
	})

	t.Run("adversarial causes stay behind every presentation and persistence boundary", func(t *testing.T) {
		canaries := []string{
			"password-canary-T043",
			"passphrase-canary-T043",
			"private-key-canary-T043",
			"credential-ref-canary-T043",
			"server-text-canary-T043",
			"ansi-canary-T043",
			"control-canary-T043",
			"bidi-canary-T043",
			"oversized-canary-T043",
		}
		payloads := []string{
			"password=" + canaries[0],
			"passphrase=" + canaries[1],
			"-----BEGIN OPENSSH PRIVATE KEY-----\n" + canaries[2],
			"credential_ref=" + canaries[3],
			"SSH server said: " + canaries[4],
			"\x1b[31m" + canaries[5] + "\x1b[0m",
			"before\x00\r\n" + canaries[6],
			"\u202e" + canaries[7] + "\u2066",
			strings.Repeat(canaries[8], 1024),
		}
		causes := make([]error, 0, len(payloads))
		for index, payload := range payloads {
			causes = append(causes, fmt.Errorf("source %d: %w", index, errors.New(payload)))
		}
		joinedCause := fmt.Errorf("outer transport wrapper: %w", errors.Join(causes...))
		unsafeDetail := strings.Join(payloads, " | ")
		startupErr := app.NewSSHStartError(app.SSHFailureUnexpected, app.SSHFailureStageUnknown, unsafeDetail, joinedCause)
		failure := startupErr.Presentation()
		assertNoSecurityCanary(t, "safe startup error", startupErr.Error(), canaries)
		assertNoSecurityCanary(t, "diagnostic", fmt.Sprintf("%+v", failure), canaries)
		if failure.TechnicalDetail != "" {
			t.Fatalf("uncontrolled diagnostic detail survived: %q", failure.TechnicalDetail)
		}

		directory := t.TempDir()
		if err := os.Chmod(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		catalogPath := filepath.Join(directory, catalog.CatalogFileName)
		store, err := catalog.Open(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		repository := catalogrepo.NewRepository(store)
		secrets := credential.NewFake()
		scope := credential.Scope("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
		saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), secrets, scope, nil)
		if err != nil {
			t.Fatal(err)
		}
		connections, err := app.NewConnectionService(repository, saga)
		if err != nil {
			t.Fatal(err)
		}
		created, err := connections.Create(context.Background(), app.CreateConnectionRequest{
			Parent: app.ItemSelector{Path: "/"}, Name: "parity", Host: "captured.example", Port: 2222, AuthMethod: app.AuthMethodAgent,
		})
		if err != nil {
			t.Fatal(err)
		}
		baselineCatalog, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatal(err)
		}

		local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
		connect, err := app.NewConnectService(app.ConnectOptions{
			Connections: repository, Credentials: saga, HostTrust: knownHostTrust{}, TrustedHosts: noOpTrustedHosts{},
			Store: secrets, Scope: scope, Runner: securityFailureRunner{err: startupErr}, Terminal: local,
		})
		if err != nil {
			t.Fatal(err)
		}

		var logs bytes.Buffer
		previousLogWriter := log.Writer()
		log.SetOutput(&logs)
		defer log.SetOutput(previousLogWriter)

		directResult, directErr := connect.Connect(context.Background(), app.ConnectRequest{Connection: app.ItemSelector{ID: created.Connection.ID}})
		if directErr == nil || directResult.Session.Failure == nil {
			t.Fatalf("direct connect did not return a diagnostic failure: result=%+v err=%v", directResult, directErr)
		}
		assertNoSecurityCanary(t, "returned error", directErr.Error(), canaries)
		diagnosticJSON, err := json.Marshal(directResult.Session.Failure)
		if err != nil {
			t.Fatal(err)
		}
		assertNoSecurityCanary(t, "returned diagnostic", string(diagnosticJSON), canaries)

		for _, jsonOutput := range []bool{false, true} {
			name := "human CLI"
			args := []string{"connect", created.Connection.Path}
			if jsonOutput {
				name = "JSON CLI"
				args = append([]string{"--json"}, args...)
			}
			var stdout, stderr bytes.Buffer
			root := cli.NewRoot(cli.RootConfig{
				Connections: connections, Connect: connect, Terminal: local, Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
			})
			root.SetArgs(args)
			commandErr := root.ExecuteContext(context.Background())
			if commandErr == nil {
				t.Fatalf("%s connect unexpectedly succeeded", name)
			}
			assertNoSecurityCanary(t, name+" returned error", commandErr.Error(), canaries)
			if err := cli.WriteError(&stderr, jsonOutput, commandErr); err != nil {
				t.Fatal(err)
			}
			output := stdout.String() + stderr.String()
			assertNoSecurityCanary(t, name, output, canaries)
			if jsonOutput && !json.Valid(stderr.Bytes()) {
				t.Fatalf("JSON CLI output is invalid: %q", stderr.String())
			}
		}

		assertNoSecurityCanary(t, "logs", logs.String(), canaries)
		persistedCatalog, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		assertNoSecurityCanary(t, "catalog", string(persistedCatalog), canaries)
		if !bytes.Equal(persistedCatalog, baselineCatalog) {
			t.Fatal("presenting SSH diagnostics changed the catalog")
		}
	})

	t.Run("leading dash host remains data", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "host-was-executed")
		unsafeHost := "-oProxyCommand=touch " + marker
		connections := integrationConnectionService(t)
		var stdout bytes.Buffer
		root := cli.NewRoot(cli.RootConfig{
			Version: "security", Connections: connections, Terminal: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}),
			Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: io.Discard,
		})
		root.SetArgs([]string{"connection", "create", "/unsafe-host", "--host", unsafeHost, "--user", "tester", "--auth", "agent"})
		if err := root.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("create leading-dash host: %v", err)
		}
		created, err := connections.Get(context.Background(), app.ItemSelector{Path: "/unsafe-host"})
		if err != nil || created.Connection.Host != unsafeHost {
			t.Fatalf("stored host = %q, %v", created.Connection.Host, err)
		}

		var dialed string
		dialErr := errors.New("stop after safe dial boundary")
		client := sshclient.New(sshclient.Options{DialContext: func(_ context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" {
				t.Fatalf("network = %q", network)
			}
			dialed = address
			return nil, dialErr
		}})
		local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
		created.Connection.Username = "tester"
		_, runErr := client.Run(context.Background(), app.SSHSessionRequest{
			Connection: created.Connection, Terminal: local,
			VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
		})
		if !errors.Is(runErr, dialErr) {
			t.Fatalf("Run() error = %v", runErr)
		}
		if want := net.JoinHostPort(unsafeHost, "22"); dialed != want {
			t.Fatalf("dial address = %q, want %q", dialed, want)
		}
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("host input was executed: %v", err)
		}
	})

	t.Run("Unix private keys require restrictive mode", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix mode bits are not available")
		}
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		block, err := ssh.MarshalPrivateKey(privateKey, "security-test")
		if err != nil {
			t.Fatal(err)
		}
		keyBytes := pem.EncodeToMemory(block)
		keyPath := filepath.Join(sshDirectory, "id_security")
		if err := os.WriteFile(keyPath, keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		connection := app.Connection{AuthMethod: app.AuthMethodKey, IdentityFile: keyPath}
		auth, err := sshclient.BuildAuthMethod(context.Background(), connection, nil)
		if err != nil {
			t.Fatalf("0600 private key rejected: %v", err)
		}
		_ = auth.Close()
		if err := os.Chmod(keyPath, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := sshclient.BuildAuthMethod(context.Background(), connection, nil); !errors.Is(err, sshclient.ErrPrivateKeyPermissions) {
			t.Fatalf("0644 private key error = %v, want ErrPrivateKeyPermissions", err)
		}
	})

	t.Run("host trust precedes authentication", func(t *testing.T) {
		server := startSSHServer(t, "unused-password", 0)
		host, portText, err := net.SplitHostPort(server.address)
		if err != nil {
			t.Fatal(err)
		}
		port, err := strconv.ParseUint(portText, 10, 16)
		if err != nil {
			t.Fatal(err)
		}
		secretCalls := 0
		rejected := errors.New("host rejected before authentication")
		client := sshclient.New(sshclient.Options{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
		_, runErr := client.Run(context.Background(), app.SSHSessionRequest{
			Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodPassword},
			Terminal:   terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}),
			VerifyHost: func(context.Context, app.PresentedHost) error { return rejected },
			Secret: func(context.Context, app.SecretRequest) ([]byte, error) {
				secretCalls++
				return []byte("must-not-be-requested"), nil
			},
		})
		if !errors.Is(runErr, rejected) || secretCalls != 0 {
			t.Fatalf("Run() error = %v, secret calls = %d", runErr, secretCalls)
		}
	})

	t.Run("session output is streamed to a bounded sink", func(t *testing.T) {
		const outputBytes = 2 << 20
		server := startStreamingSSHServer(t, "stream-password", outputBytes)
		host, portText, err := net.SplitHostPort(server.address)
		if err != nil {
			t.Fatal(err)
		}
		port, err := strconv.ParseUint(portText, 10, 16)
		if err != nil {
			t.Fatal(err)
		}
		output := &boundedCountingWriter{limit: 4096}
		client := sshclient.New(sshclient.Options{Stdin: strings.NewReader(""), Stdout: output, Stderr: io.Discard})
		result, runErr := client.Run(context.Background(), app.SSHSessionRequest{
			Connection: app.Connection{Host: host, Port: uint16(port), Username: "tester", AuthMethod: app.AuthMethodPassword},
			Terminal:   terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}),
			VerifyHost: func(context.Context, app.PresentedHost) error { return nil },
			Secret:     func(context.Context, app.SecretRequest) ([]byte, error) { return []byte("stream-password"), nil },
		})
		if runErr != nil || result.Outcome != app.SessionOutcomeSuccess {
			t.Fatalf("Run() = %#v, %v", result, runErr)
		}
		if output.total != outputBytes || len(output.retained) != output.limit {
			t.Fatalf("streamed output total=%d retained=%d", output.total, len(output.retained))
		}
	})
}

func assertNoSecurityCanary(t *testing.T, surface, output string, canaries []string) {
	t.Helper()
	for _, canary := range canaries {
		if strings.Contains(output, canary) {
			t.Fatalf("%s leaked %q", surface, canary)
		}
	}
}

type securityFailureRunner struct{ err error }

func (r securityFailureRunner) Run(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error) {
	return app.SSHSessionResult{}, r.err
}

func startStreamingSSHServer(t *testing.T, password string, outputBytes int) sshTestServer {
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
					_, _ = io.CopyN(channel, repeatingByteReader('x'), int64(outputBytes))
					payload := make([]byte, 4)
					binary.BigEndian.PutUint32(payload, 0)
					_, _ = channel.SendRequest("exit-status", false, payload)
					_ = channel.Close()
					_ = serverConnection.Wait()
					return
				default:
					_ = request.Reply(false, nil)
				}
			}
		}
	}()
	server := sshTestServer{address: listener.Addr().String(), signer: signer}
	server.close = func() { _ = listener.Close(); wait.Wait() }
	t.Cleanup(server.close)
	return server
}

type repeatingByteReader byte

func (r repeatingByteReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = byte(r)
	}
	return len(buffer), nil
}

type boundedCountingWriter struct {
	limit    int
	total    int
	retained []byte
}

func (w *boundedCountingWriter) Write(value []byte) (int, error) {
	w.total += len(value)
	remaining := w.limit - len(w.retained)
	if remaining > len(value) {
		remaining = len(value)
	}
	if remaining > 0 {
		w.retained = append(w.retained, value[:remaining]...)
	}
	return len(value), nil
}
