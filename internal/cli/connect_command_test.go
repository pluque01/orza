package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

func TestConnectCommandGuardsPresentsTrustAndPropagatesRemoteStatus(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	created, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: "prod", Host: "prod.example", Port: 22,
		Username: "alice", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}

	trusted := &cliTrustedHosts{}
	runner := &cliSessionRunner{run: func(ctx context.Context, request app.SSHSessionRequest) (app.SSHSessionResult, error) {
		if err := request.VerifyHost(ctx, app.PresentedHost{
			Endpoint:     app.HostEndpoint{CanonicalHost: "prod.example", Port: 22},
			KeyAlgorithm: "ssh-ed25519", FingerprintSHA256: "SHA256:test",
		}); err != nil {
			return app.SSHSessionResult{}, err
		}
		status := 42
		return app.SSHSessionResult{State: app.SessionFailed, Outcome: app.SessionOutcomeRemoteFailure, RemoteExitStatus: &status}, errors.New("remote exit")
	}}
	saga, err := app.NewCredentialSaga(&emptyCredentialOperations{}, fixture.store, fixture.scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	connectService, err := app.NewConnectService(app.ConnectOptions{
		Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: saga,
		HostTrust: &cliHostTrust{result: app.HostTrustResult{Status: app.HostTrustUnknown}}, TrustedHosts: trusted,
		Store: fixture.store, Scope: fixture.scope, Runner: runner, Terminal: fixture.local,
	})
	if err != nil {
		t.Fatal(err)
	}

	execute := func(input string, args ...string) commandResult {
		var stdout, stderr bytes.Buffer
		root := NewRoot(RootConfig{Connections: fixture.connections, Connect: connectService, Terminal: fixture.local, Stdin: strings.NewReader(input), Stdout: &stdout, Stderr: &stderr})
		root.SetArgs(args)
		err := root.ExecuteContext(context.Background())
		return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
	}

	jsonResult := execute("once\n", "--json", "connect", string(created.Connection.ID))
	if ExitCode(jsonResult.err) != 42 || runner.calls != 1 {
		t.Fatalf("JSON connect = %v, runner calls = %d", jsonResult.err, runner.calls)
	}
	if strings.Contains(jsonResult.stdout, `"ok"`) || jsonResult.stderr != "" {
		t.Fatalf("JSON connect streams = stdout %q, stderr %q", jsonResult.stdout, jsonResult.stderr)
	}
	fixture.local.SetInteractive(false)
	nonterminal := execute("", "connect", created.Connection.Path)
	if ExitCode(nonterminal.err) != ExitSecurity || runner.calls != 1 {
		t.Fatalf("nonterminal connect = %v, runner calls = %d", nonterminal.err, runner.calls)
	}

	fixture.local.SetInteractive(true)
	connected := execute("once\n", "connect", created.Connection.Path)
	if ExitCode(connected.err) != 42 {
		t.Fatalf("connect error = %v, exit = %d", connected.err, ExitCode(connected.err))
	}
	if !strings.Contains(connected.stdout, "/prod") || !strings.Contains(connected.stdout, "alice@prod.example:22") || !strings.Contains(connected.stdout, "ssh-ed25519") || !strings.Contains(connected.stdout, "SHA256:test") {
		t.Fatalf("connect presentation = %q", connected.stdout)
	}
	if trusted.calls != 0 {
		t.Fatal("trust once was persisted")
	}
}

func TestJSONConnectReturnsCapturedStartupDiagnostic(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	created, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: "prod", Host: "prod.example", Port: 2222, Username: "alice", AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &cliSessionRunner{run: func(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error) {
		return app.SSHSessionResult{}, app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", errors.New("password=canary"))
	}}
	saga, _ := app.NewCredentialSaga(&emptyCredentialOperations{}, fixture.store, fixture.scope, nil)
	connectService, err := app.NewConnectService(app.ConnectOptions{
		Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: saga,
		HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store,
		Scope: fixture.scope, Runner: runner, Terminal: fixture.local,
	})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	root := NewRoot(RootConfig{Connections: fixture.connections, Connect: connectService, Terminal: fixture.local, Stdout: &stdout, Stderr: &stderr})
	root.SetArgs([]string{"--json", "connect", created.Connection.Path})
	commandErr := root.ExecuteContext(context.Background())
	if commandErr == nil {
		t.Fatal("connect succeeded")
	}
	if writeErr := WriteError(&stderr, true, commandErr); writeErr != nil {
		t.Fatal(writeErr)
	}
	if got := stderr.String(); !strings.Contains(got, `"endpoint":"prod.example:2222"`) || !strings.Contains(got, `"category":"timeout"`) || strings.Contains(got, "alice") || strings.Contains(got, "canary") {
		t.Fatalf("structured diagnostic = %q", got)
	}
	if strings.Contains(stdout.String(), `"ok"`) || !strings.Contains(stdout.String(), "Connecting /prod") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestConnectStartupFallbackDiagnosticsHaveHumanJSONParity(t *testing.T) {
	const canary = "startup-password-canary"
	tests := []struct {
		name           string
		credentials    app.CredentialLifecycle
		runner         app.SSHSessionRunner
		wantStage      app.SSHFailureStage
		wantCategory   app.SSHFailureReason
		recommendation string
	}{
		{
			name:        "credential recovery",
			credentials: &cliCredentialLifecycle{err: errors.New(canary)},
			runner: &cliSessionRunner{run: func(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error) {
				return app.SSHSessionResult{}, nil
			}},
			wantStage:      app.SSHFailureStageCredential,
			wantCategory:   app.SSHFailureUnexpected,
			recommendation: "Check the configuration or retry.",
		},
		{
			name:        "incomplete runner result",
			credentials: &cliCredentialLifecycle{},
			runner: &cliSessionRunner{run: func(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error) {
				return app.SSHSessionResult{Failure: &app.SSHFailurePresentation{TechnicalDetail: canary}}, nil
			}},
			wantStage:      app.SSHFailureStageUnknown,
			wantCategory:   app.SSHFailureUnexpected,
			recommendation: "Check the configuration or retry.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newConnectionCLIFixture(t)
			created, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{
				Parent: app.ItemSelector{Path: "/"}, Name: "prod", Host: "prod.example", Port: 2222, AuthMethod: app.AuthMethodAgent,
			})
			if err != nil {
				t.Fatal(err)
			}
			connectService, err := app.NewConnectService(app.ConnectOptions{
				Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: test.credentials,
				HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store,
				Scope: fixture.scope, Runner: test.runner, Terminal: fixture.local,
			})
			if err != nil {
				t.Fatal(err)
			}

			run := func(jsonOutput bool) string {
				var stdout, rendered bytes.Buffer
				root := NewRoot(RootConfig{Connections: fixture.connections, Connect: connectService, Terminal: fixture.local, Stdout: &stdout, Stderr: &bytes.Buffer{}})
				args := []string{"connect", created.Connection.Path}
				if jsonOutput {
					args = append([]string{"--json"}, args...)
				}
				root.SetArgs(args)
				commandErr := root.ExecuteContext(context.Background())
				if commandErr == nil {
					t.Fatal("connect succeeded")
				}
				if writeErr := WriteError(&rendered, jsonOutput, commandErr); writeErr != nil {
					t.Fatal(writeErr)
				}
				if strings.Contains(rendered.String(), canary) {
					t.Fatalf("cause canary leaked: %q", rendered.String())
				}
				return rendered.String()
			}

			human := run(false)
			var jsonResult errorEnvelope
			jsonOutput := run(true)
			if err := json.Unmarshal([]byte(jsonOutput), &jsonResult); err != nil {
				t.Fatalf("decode JSON diagnostic %q: %v", jsonOutput, err)
			}
			wantEndpoint := "prod.example:2222"
			for _, want := range []string{
				"endpoint: " + wantEndpoint,
				"category: " + string(test.wantCategory),
				"stage: " + string(test.wantStage),
				"recommendation: " + test.recommendation,
			} {
				if !strings.Contains(human, want) {
					t.Fatalf("human diagnostic missing %q: %q", want, human)
				}
			}
			got := jsonResult.Error
			if got.Endpoint != wantEndpoint || got.Category != test.wantCategory || got.Stage != test.wantStage || got.Recommendation != test.recommendation {
				t.Fatalf("JSON diagnostic does not match human contract: human=%q JSON=%s", human, jsonOutput)
			}
		})
	}
}

func TestConnectPinsCapturedIDAndRevisionBeforeNetwork(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	created, err := fixture.connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: "prod", Host: "old.example", Port: 22, AuthMethod: app.AuthMethodAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &cliSessionRunner{run: func(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error) {
		t.Fatal("network runner was called for a stale captured revision")
		return app.SSHSessionResult{}, nil
	}}
	saga, _ := app.NewCredentialSaga(&emptyCredentialOperations{}, fixture.store, fixture.scope, nil)
	connectService, err := app.NewConnectService(app.ConnectOptions{
		Connections: &connectionRepositoryFromService{service: fixture.connections}, Credentials: saga,
		HostTrust: &cliHostTrust{}, TrustedHosts: &cliTrustedHosts{}, Store: fixture.store,
		Scope: fixture.scope, Runner: runner, Terminal: fixture.local,
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout := &callbackWriter{callback: func() {
		host := "new.example"
		expected := created.Connection.Revision
		if _, updateErr := fixture.connections.Update(context.Background(), app.UpdateConnectionRequest{
			Connection: app.ItemSelector{ID: created.Connection.ID}, Expected: &expected, Host: &host,
		}); updateErr != nil {
			t.Fatal(updateErr)
		}
	}}
	root := NewRoot(RootConfig{Connections: fixture.connections, Connect: connectService, Terminal: fixture.local, Stdout: stdout, Stderr: &bytes.Buffer{}})
	root.SetArgs([]string{"--json", "connect", created.Connection.Path})
	commandErr := root.ExecuteContext(context.Background())
	if ExitCode(commandErr) != ExitConflict || runner.calls != 0 {
		t.Fatalf("connect error/runner calls = %v/%d", commandErr, runner.calls)
	}
	var rendered bytes.Buffer
	if writeErr := WriteError(&rendered, true, commandErr); writeErr != nil {
		t.Fatal(writeErr)
	}
	if strings.Contains(rendered.String(), "category") || strings.Contains(rendered.String(), "endpoint") {
		t.Fatalf("pre-network conflict received startup fields: %q", rendered.String())
	}
}

type callbackWriter struct {
	bytes.Buffer
	callback func()
	wrote    bool
}

func (w *callbackWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if !w.wrote {
		w.wrote = true
		w.callback()
	}
	return n, err
}

type cliSessionRunner struct {
	calls int
	run   func(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error)
}

func (r *cliSessionRunner) Run(ctx context.Context, request app.SSHSessionRequest) (app.SSHSessionResult, error) {
	r.calls++
	return r.run(ctx, request)
}

type cliHostTrust struct{ result app.HostTrustResult }

func (h *cliHostTrust) CheckHost(context.Context, app.PresentedHost) (app.HostTrustResult, error) {
	return h.result, nil
}

type cliTrustedHosts struct{ calls int }

func (r *cliTrustedHosts) GetTrustedHost(context.Context, app.HostEndpoint) (app.TrustedHost, error) {
	return app.TrustedHost{}, app.ErrNotFound
}
func (r *cliTrustedHosts) TrustHost(_ context.Context, request app.TrustHostRequest) (app.TrustedHost, error) {
	r.calls++
	return app.TrustedHost{HostEndpoint: request.Host.Endpoint, Revision: 1}, nil
}

// ConnectService needs the repository port. This adapter deliberately delegates
// through ConnectionService so command tests do not bypass application behavior.
type connectionRepositoryFromService struct{ service *app.ConnectionService }

func (r *connectionRepositoryFromService) GetConnection(ctx context.Context, selector app.ItemSelector) (app.ConnectionResult, error) {
	return r.service.Get(ctx, selector)
}
func (r *connectionRepositoryFromService) CreateConnection(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (r *connectionRepositoryFromService) ListConnections(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
	return app.ListConnectionsResult{}, app.ErrInvalidRequest
}
func (r *connectionRepositoryFromService) UpdateConnection(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (r *connectionRepositoryFromService) MoveConnection(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (r *connectionRepositoryFromService) DeleteConnection(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
	return app.DeleteConnectionResult{}, app.ErrInvalidRequest
}

type emptyCredentialOperations struct{}

type cliCredentialLifecycle struct{ err error }

func (c *cliCredentialLifecycle) Save(context.Context, app.Connection, []byte) error { return c.err }
func (c *cliCredentialLifecycle) Replace(context.Context, app.Connection, []byte) error {
	return c.err
}
func (c *cliCredentialLifecycle) Remove(context.Context, app.Connection, app.AuthMethod, string) error {
	return c.err
}
func (c *cliCredentialLifecycle) DeleteConnection(context.Context, app.Connection) error {
	return c.err
}
func (c *cliCredentialLifecycle) Recover(context.Context) error { return c.err }

func (*emptyCredentialOperations) CreateCredentialOperation(context.Context, app.CredentialOperation) error {
	return nil
}
func (*emptyCredentialOperations) GetCredentialOperation(context.Context, string) (app.CredentialOperation, error) {
	return app.CredentialOperation{}, app.ErrNotFound
}
func (*emptyCredentialOperations) ListCredentialOperations(context.Context) ([]app.CredentialOperation, error) {
	return nil, nil
}
func (*emptyCredentialOperations) SetCredentialOperationPhase(context.Context, string, app.CredentialOperationPhase) error {
	return nil
}
func (*emptyCredentialOperations) DeleteCredentialOperation(context.Context, string) error {
	return nil
}
func (*emptyCredentialOperations) ApplyCredentialOperation(context.Context, string) error { return nil }

var _ app.CredentialStore = (*credential.Fake)(nil)
var _ app.Terminal = (*terminal.Fake)(nil)
