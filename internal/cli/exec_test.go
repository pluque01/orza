package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/credential"
)

func TestExecValidatesShapeAndJSON(t *testing.T) {
	for _, args := range [][]string{{"exec", "/work/host"}, {"exec", "/work/host", "--", ""}, {"--json", "exec", "/work/host", "--", "true"}, {"exec", "/work/host", "--timeout", "0s", "--", "true"}} {
		root := NewRoot(RootConfig{Version: "dev", Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
		root.SetArgs(args)
		if got := ExitCode(root.ExecuteContext(context.Background())); got != ExitUsage {
			t.Fatalf("args %v: exit = %d, want %d", args, got, ExitUsage)
		}
	}
}

func TestExecStreamsAndMapsRemoteAndLocalResults(t *testing.T) {
	for _, test := range []struct {
		name   string
		result app.SSHSessionResult
		runErr error
		want   int
	}{
		{name: "remote", result: app.SSHSessionResult{RemoteExitStatus: statusPointer(23)}, want: 23},
		{name: "not found", runErr: app.ErrNotFound, want: ExitTransport},
		{name: "security", runErr: app.NewSSHStartError(app.SSHFailureHostTrust, app.SSHFailureStageHostTrust, "unknown", errors.New("private-secret")), want: ExitSecurity},
		{name: "canceled", runErr: context.Canceled, want: ExitCanceled},
		{name: "transport", runErr: errors.New("private-secret"), want: ExitTransport},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := execTestService(t, test.result, test.runErr)
			var stdout, stderr bytes.Buffer
			root := NewRoot(RootConfig{Version: "dev", Command: service, Stdin: strings.NewReader("input"), Stdout: &stdout, Stderr: &stderr})
			root.SetArgs([]string{"exec", "/server", "--", "printf out | cat"})
			err := root.ExecuteContext(context.Background())
			if got := ExitCode(err); got != test.want {
				t.Fatalf("exit = %d, want %d (%v)", got, test.want, err)
			}
			if strings.Contains(err.Error(), "private-secret") {
				t.Fatalf("unsafe cause leaked: %v", err)
			}
		})
	}
}

func statusPointer(value int) *int { return &value }

func execTestService(t *testing.T, result app.SSHSessionResult, runErr error) *app.CommandService {
	t.Helper()
	connection := app.Connection{Node: app.Node{ID: "0123456789abcdef0123456789abcdef", Path: "/server", Revision: 1}, Host: "server.example", Port: 22, Username: "operator", AuthMethod: app.AuthMethodAgent}
	service, err := app.NewCommandService(app.CommandOptions{Connections: execRepository{connection}, Credentials: execCredentials{}, HostTrust: execTrust{}, TrustedHosts: execTrustedHosts{}, Store: credential.NewFake(), Scope: "catalog", Runner: execRunner{result: result, err: runErr}})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type execRepository struct{ connection app.Connection }

func (r execRepository) GetConnection(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
	return app.ConnectionResult{Connection: r.connection}, nil
}
func (execRepository) CreateConnection(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (execRepository) ListConnections(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
	return app.ListConnectionsResult{}, app.ErrInvalidRequest
}
func (execRepository) UpdateConnection(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (execRepository) MoveConnection(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error) {
	return app.ConnectionResult{}, app.ErrInvalidRequest
}
func (execRepository) DeleteConnection(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
	return app.DeleteConnectionResult{}, app.ErrInvalidRequest
}

type execCredentials struct{}

func (execCredentials) Save(context.Context, app.Connection, []byte) error    { return nil }
func (execCredentials) Replace(context.Context, app.Connection, []byte) error { return nil }
func (execCredentials) Remove(context.Context, app.Connection, app.AuthMethod, string) error {
	return nil
}
func (execCredentials) DeleteConnection(context.Context, app.Connection) error { return nil }
func (execCredentials) Recover(context.Context) error                          { return nil }

type execTrust struct{}

func (execTrust) CheckHost(context.Context, app.PresentedHost) (app.HostTrustResult, error) {
	return app.HostTrustResult{Status: app.HostTrustKnown}, nil
}

type execTrustedHosts struct{}

func (execTrustedHosts) GetTrustedHost(context.Context, app.HostEndpoint) (app.TrustedHost, error) {
	return app.TrustedHost{}, app.ErrNotFound
}
func (execTrustedHosts) TrustHost(context.Context, app.TrustHostRequest) (app.TrustedHost, error) {
	return app.TrustedHost{}, nil
}
func (execTrustedHosts) DeleteTrustedHost(context.Context, app.HostEndpoint, app.Revision) (app.CatalogRevision, error) {
	return 0, nil
}

type execRunner struct {
	result app.SSHSessionResult
	err    error
}

func (r execRunner) RunCommand(context.Context, app.SSHCommandRequest) (app.SSHSessionResult, error) {
	return r.result, r.err
}

func TestExecUnavailableServiceIsCatalogFailure(t *testing.T) {
	root := NewRoot(RootConfig{Version: "dev", Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	root.SetArgs([]string{"exec", "/work/host", "--", "true"})
	if got := ExitCode(root.ExecuteContext(context.Background())); got != ExitCatalog {
		t.Fatalf("exit = %d, want %d", got, ExitCatalog)
	}
}
