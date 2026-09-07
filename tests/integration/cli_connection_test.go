package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

func TestCLIConnectionLifecycleWithInjectedSSHRunner(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := catalog.Open(filepath.Join(directory, catalog.CatalogFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository := catalogrepo.NewRepository(store)
	credentialStore := credential.NewFake()
	scope := credential.Scope("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), credentialStore, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := app.NewConnectionService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	local := terminal.NewFake(terminal.Size{Columns: 100, Rows: 30})
	runner := &integrationRunner{run: func(ctx context.Context, request app.SSHSessionRequest) (app.SSHSessionResult, error) {
		if err := request.VerifyHost(ctx, app.PresentedHost{
			Endpoint:     app.HostEndpoint{CanonicalHost: request.Connection.Host, Port: request.Connection.Port},
			KeyAlgorithm: "ssh-ed25519", FingerprintSHA256: "SHA256:integration",
		}); err != nil {
			return app.SSHSessionResult{}, err
		}
		return app.SSHSessionResult{State: app.SessionSucceeded, Outcome: app.SessionOutcomeSuccess}, nil
	}}
	connectService, err := app.NewConnectService(app.ConnectOptions{
		Connections: repository, Credentials: saga, HostTrust: knownHostTrust{},
		TrustedHosts: noOpTrustedHosts{}, Store: credentialStore, Scope: scope,
		Runner: runner, Terminal: local,
	})
	if err != nil {
		t.Fatal(err)
	}

	execute := func(input string, args ...string) (string, error) {
		var output bytes.Buffer
		root := cli.NewRoot(cli.RootConfig{
			Version: "integration", Connections: connections, Connect: connectService,
			Terminal: local, Stdin: strings.NewReader(input), Stdout: &output, Stderr: &bytes.Buffer{},
		})
		root.SetArgs(args)
		err := root.ExecuteContext(context.Background())
		return output.String(), err
	}

	createdJSON, err := execute("", "--json", "connection", "create", "/integration", "--host", "127.0.0.1", "--user", "tester", "--auth", "agent")
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Data struct {
			ID       string `json:"id"`
			Revision uint64 `json:"revision"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(createdJSON), &created); err != nil || created.Data.ID == "" || created.Data.Revision != 1 {
		t.Fatalf("create response = %q, %v", createdJSON, err)
	}
	if _, err := execute("", "connection", "update", created.Data.ID, "--host", "localhost", "--if-revision", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute("", "connection", "update", created.Data.ID, "--host", "stale", "--if-revision", "1"); cli.ExitCode(err) != cli.ExitConflict {
		t.Fatalf("stale update = %v, exit %d", err, cli.ExitCode(err))
	}
	if _, err := execute("", "connect", created.Data.ID); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d", runner.calls)
	}
	if _, err := execute("", "connection", "delete", created.Data.ID, "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := connections.Get(context.Background(), app.ItemSelector{ID: app.NodeID(created.Data.ID)}); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("deleted connection lookup = %v", err)
	}
}

type integrationRunner struct {
	calls int
	run   func(context.Context, app.SSHSessionRequest) (app.SSHSessionResult, error)
}

func (r *integrationRunner) Run(ctx context.Context, request app.SSHSessionRequest) (app.SSHSessionResult, error) {
	r.calls++
	return r.run(ctx, request)
}

type knownHostTrust struct{}

func (knownHostTrust) CheckHost(context.Context, app.PresentedHost) (app.HostTrustResult, error) {
	return app.HostTrustResult{Status: app.HostTrustKnown}, nil
}

type noOpTrustedHosts struct{}

func (noOpTrustedHosts) GetTrustedHost(context.Context, app.HostEndpoint) (app.TrustedHost, error) {
	return app.TrustedHost{}, app.ErrNotFound
}
func (noOpTrustedHosts) TrustHost(context.Context, app.TrustHostRequest) (app.TrustedHost, error) {
	return app.TrustedHost{}, nil
}
