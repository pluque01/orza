package cli

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
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

func TestConnectionCommandsCRUDAndJSONRedaction(t *testing.T) {
	fixture := newConnectionCLIFixture(t)

	created := fixture.execute(t, "--json", "connection", "create", "/prod", "--host", "prod.example", "--user", "alice", "--auth", "agent")
	if created.err != nil {
		t.Fatal(created.err)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			ID            string `json:"id"`
			Kind          string `json:"kind"`
			Path          string `json:"path"`
			Revision      uint64 `json:"revision"`
			CredentialRef string `json:"credentialRef"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &envelope); err != nil {
		t.Fatalf("create JSON = %q: %v", created.stdout, err)
	}
	if !envelope.OK || envelope.Data.ID == "" || envelope.Data.Kind != "connection" || envelope.Data.Path != "/prod" || envelope.Data.Revision != 1 {
		t.Fatalf("create response = %#v", envelope)
	}
	if envelope.Data.CredentialRef != "" || strings.Contains(created.stdout, "password") {
		t.Fatalf("create response exposed credential data: %s", created.stdout)
	}

	listed := fixture.execute(t, "connection", "list")
	if listed.err != nil || !strings.Contains(listed.stdout, envelope.Data.ID) || !strings.Contains(listed.stdout, "/prod") {
		t.Fatalf("list = stdout %q, error %v", listed.stdout, listed.err)
	}
	shown := fixture.execute(t, "--json", "connection", "show", envelope.Data.ID)
	if shown.err != nil || !strings.Contains(shown.stdout, `"path":"/prod"`) || strings.Contains(shown.stdout, "credentialRef") {
		t.Fatalf("show = stdout %q, error %v", shown.stdout, shown.err)
	}

	updated := fixture.execute(t, "--json", "connection", "update", "/prod", "--host", "new.example", "--if-revision", "1")
	if updated.err != nil || !strings.Contains(updated.stdout, `"revision":2`) {
		t.Fatalf("update = stdout %q, error %v", updated.stdout, updated.err)
	}
	stale := fixture.execute(t, "connection", "update", "/prod", "--host", "stale.example", "--if-revision", "1")
	if ExitCode(stale.err) != ExitConflict {
		t.Fatalf("stale update error = %v, exit = %d", stale.err, ExitCode(stale.err))
	}

	moved := fixture.execute(t, "connection", "move", "/prod", "/", "--if-revision", "2")
	if moved.err != nil || !strings.Contains(moved.stdout, "revision 3") {
		t.Fatalf("move = stdout %q, error %v", moved.stdout, moved.err)
	}
	deleted := fixture.execute(t, "--json", "connection", "delete", envelope.Data.ID, "--if-revision", "3", "--yes")
	if deleted.err != nil || !strings.Contains(deleted.stdout, `"deleted":true`) {
		t.Fatalf("delete = stdout %q, error %v", deleted.stdout, deleted.err)
	}
	missing := fixture.execute(t, "connection", "show", "/prod")
	if ExitCode(missing.err) != ExitNotFound {
		t.Fatalf("missing show error = %v, exit = %d", missing.err, ExitCode(missing.err))
	}
}

func TestConnectionCommandValidationAndExactSelectors(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	tests := []struct {
		name string
		args []string
	}{
		{name: "agent rejects identity", args: []string{"connection", "create", "/bad", "--host", "host", "--auth", "agent", "--identity-file", "id"}},
		{name: "key requires identity", args: []string{"connection", "create", "/bad", "--host", "host", "--auth", "key"}},
		{name: "password rejects identity", args: []string{"connection", "create", "/bad", "--host", "host", "--auth", "password", "--identity-file", "id"}},
		{name: "remember requires password", args: []string{"connection", "create", "/bad", "--host", "host", "--auth", "agent", "--remember-password"}},
		{name: "relative path is treated as exact ID", args: []string{"connection", "show", "prod"}},
		{name: "uppercase ID is invalid", args: []string{"connection", "show", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}},
		{name: "update requires change", args: []string{"connection", "update", "/missing"}},
		{name: "exclusive user flags", args: []string{"connection", "update", "/missing", "--user", "me", "--clear-user"}},
		{name: "exclusive password flags", args: []string{"connection", "update", "/missing", "--remember-password", "--forget-password"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := fixture.execute(t, test.args...)
			if ExitCode(result.err) != ExitUsage {
				t.Fatalf("error = %v, exit = %d", result.err, ExitCode(result.err))
			}
		})
	}
}

func TestRememberPasswordRequiresInteractiveExplicitConsentAndDeleteCleansIt(t *testing.T) {
	fixture := newConnectionCLIFixture(t)
	fixture.local.SetInteractive(false)
	noninteractive := fixture.execute(t, "connection", "create", "/prod", "--host", "host", "--auth", "password", "--remember-password")
	if ExitCode(noninteractive.err) != ExitSecurity {
		t.Fatalf("noninteractive remember error = %v, exit = %d", noninteractive.err, ExitCode(noninteractive.err))
	}

	fixture.local.SetInteractive(true)
	fixture.local.QueueSecret([]byte("top-secret"), nil)
	created := fixture.executeWithInput(t, "y\n", "--json", "connection", "create", "/prod", "--host", "host", "--auth", "password", "--remember-password")
	if created.err != nil {
		t.Fatal(created.err)
	}
	connection, err := fixture.connections.Get(context.Background(), app.ItemSelector{Path: "/prod"})
	if err != nil || connection.Connection.CredentialRef == "" {
		t.Fatalf("remembered connection = %#v, %v", connection, err)
	}
	key := credential.Key{Scope: fixture.scope, Reference: credential.Reference(connection.Connection.CredentialRef)}
	if secret, ok := fixture.store.Lookup(key); !ok || string(secret) != "top-secret" {
		t.Fatalf("stored credential missing")
	}
	if strings.Contains(created.stdout, "top-secret") || strings.Contains(created.stdout, connection.Connection.CredentialRef) {
		t.Fatalf("create output leaked secret metadata: %q", created.stdout)
	}

	canceled := fixture.executeWithInput(t, "n\n", "connection", "delete", "/prod")
	if ExitCode(canceled.err) != ExitCanceled {
		t.Fatalf("delete cancel = %v", canceled.err)
	}
	deleted := fixture.execute(t, "connection", "delete", "/prod", "--yes")
	if deleted.err != nil {
		t.Fatal(deleted.err)
	}
	if _, ok := fixture.store.Lookup(key); ok {
		t.Fatal("credential remains after connection deletion")
	}
}

type connectionCLIFixture struct {
	connections *app.ConnectionService
	store       *credential.Fake
	local       *terminal.Fake
	scope       credential.Scope
}

func newConnectionCLIFixture(t *testing.T) *connectionCLIFixture {
	t.Helper()
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
	credentials := credential.NewFake()
	scope := credential.Scope("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), credentials, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := app.NewConnectionService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	return &connectionCLIFixture{connections: connections, store: credentials, local: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}), scope: scope}
}

type commandResult struct {
	stdout string
	stderr string
	err    error
}

func (f *connectionCLIFixture) execute(t *testing.T, args ...string) commandResult {
	t.Helper()
	return f.executeWithInput(t, "", args...)
}

func (f *connectionCLIFixture) executeWithInput(t *testing.T, input string, args ...string) commandResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := NewRoot(RootConfig{Version: "test", Connections: f.connections, Terminal: f.local, Stdin: strings.NewReader(input), Stdout: &stdout, Stderr: &stderr})
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func TestConnectionErrorsNeverRenderInternalCauses(t *testing.T) {
	const secret = "database-password-canary"
	err := commandError(errors.New(secret), "/prod")
	var output bytes.Buffer
	if writeErr := WriteError(&output, true, err); writeErr != nil {
		t.Fatal(writeErr)
	}
	if strings.Contains(output.String(), secret) {
		t.Fatalf("error leaked cause: %q", output.String())
	}
}
