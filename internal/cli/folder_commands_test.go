package cli

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestFolderCommandsCRUDSelectorsJSONAndConflicts(t *testing.T) {
	fixture := newFolderCLIFixture(t)

	for _, path := range []string{"/clients", "/clients/acme", "/clients/acme/prod"} {
		result := fixture.execute(t, "--json", "folder", "create", path)
		if result.err != nil || !strings.Contains(result.stdout, `"kind":"folder"`) || !strings.Contains(result.stdout, `"path":"`+path+`"`) {
			t.Fatalf("create %s = %q, %v", path, result.stdout, result.err)
		}
	}

	created := fixture.execute(t, "--json", "folder", "show", "/clients/acme/prod")
	var shown struct {
		Data struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Revision uint64 `json:"revision"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &shown); err != nil {
		t.Fatalf("show JSON = %q: %v", created.stdout, err)
	}
	if created.err != nil || shown.Data.ID == "" || shown.Data.Path != "/clients/acme/prod" || shown.Data.Revision != 1 {
		t.Fatalf("show = %#v, %v", shown, created.err)
	}
	byID := fixture.execute(t, "folder", "show", shown.Data.ID)
	if byID.err != nil || !strings.Contains(byID.stdout, "/clients/acme/prod") {
		t.Fatalf("show ID = %q, %v", byID.stdout, byID.err)
	}

	if result := fixture.execute(t, "connection", "create", "/clients/acme/prod/web", "--host", "web.test", "--auth", "agent"); result.err != nil {
		t.Fatal(result.err)
	}
	listed := fixture.execute(t, "--json", "folder", "list", "/clients/acme/prod")
	if listed.err != nil || !strings.Contains(listed.stdout, `"kind":"connection"`) || !strings.Contains(listed.stdout, `"path":"/clients/acme/prod/web"`) {
		t.Fatalf("list = %q, %v", listed.stdout, listed.err)
	}

	renamed := fixture.execute(t, "folder", "rename", shown.Data.ID, "production", "--if-revision", "1")
	if renamed.err != nil || !strings.Contains(renamed.stdout, "/clients/acme/production") || !strings.Contains(renamed.stdout, "revision 2") {
		t.Fatalf("rename = %q, %v", renamed.stdout, renamed.err)
	}
	stale := fixture.execute(t, "folder", "move", shown.Data.ID, "/", "--if-revision", "1")
	if ExitCode(stale.err) != ExitConflict {
		t.Fatalf("stale move = %v, exit %d", stale.err, ExitCode(stale.err))
	}
	moved := fixture.execute(t, "folder", "move", shown.Data.ID, "/", "--if-revision", "2")
	if moved.err != nil || !strings.Contains(moved.stdout, "/production") || !strings.Contains(moved.stdout, "revision 3") {
		t.Fatalf("move = %q, %v", moved.stdout, moved.err)
	}
}

func TestFolderDeleteRequiresRecursiveConfirmationAndProtectsRoot(t *testing.T) {
	fixture := newFolderCLIFixture(t)
	for _, path := range []string{"/tree", "/tree/branch"} {
		if result := fixture.execute(t, "folder", "create", path); result.err != nil {
			t.Fatal(result.err)
		}
	}
	if result := fixture.execute(t, "connection", "create", "/tree/branch/leaf", "--host", "leaf.test", "--auth", "agent"); result.err != nil {
		t.Fatal(result.err)
	}

	if result := fixture.execute(t, "folder", "delete", "/tree", "--yes"); ExitCode(result.err) != ExitConflict {
		t.Fatalf("nonrecursive delete = %v, exit %d", result.err, ExitCode(result.err))
	}
	fixture.local.SetInteractive(false)
	if result := fixture.execute(t, "folder", "delete", "/tree", "--recursive"); ExitCode(result.err) != ExitSecurity {
		t.Fatalf("noninteractive confirmation = %v, exit %d", result.err, ExitCode(result.err))
	}
	fixture.local.SetInteractive(true)
	canceled := fixture.executeWithInput(t, "n\n", "folder", "delete", "/tree", "--recursive")
	if ExitCode(canceled.err) != ExitCanceled || !strings.Contains(canceled.stdout, "2 folders, 1 connection, 0 remembered credentials") {
		t.Fatalf("cancel = %q, %v", canceled.stdout, canceled.err)
	}
	deleted := fixture.execute(t, "--json", "folder", "delete", "/tree", "--recursive", "--yes")
	if deleted.err != nil || !strings.Contains(deleted.stdout, `"foldersDeleted":2`) || !strings.Contains(deleted.stdout, `"connectionsDeleted":1`) {
		t.Fatalf("recursive delete = %q, %v", deleted.stdout, deleted.err)
	}

	for _, args := range [][]string{
		{"folder", "rename", "/", "root"},
		{"folder", "move", "/", "/"},
		{"folder", "delete", "/", "--recursive", "--yes"},
	} {
		result := fixture.execute(t, args...)
		if ExitCode(result.err) != ExitConflict {
			t.Fatalf("root operation %v = %v, exit %d", args, result.err, ExitCode(result.err))
		}
	}
}

type folderCLIFixture struct {
	folders     *app.FolderService
	connections *app.ConnectionService
	local       *terminal.Fake
}

func newFolderCLIFixture(t *testing.T) *folderCLIFixture {
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
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), credential.NewFake(), credential.Scope("dddddddddddddddddddddddddddddddd"), nil)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := app.NewConnectionService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	folders, err := app.NewFolderService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	return &folderCLIFixture{folders: folders, connections: connections, local: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})}
}

func (f *folderCLIFixture) execute(t *testing.T, args ...string) commandResult {
	t.Helper()
	return f.executeWithInput(t, "", args...)
}

func (f *folderCLIFixture) executeWithInput(t *testing.T, input string, args ...string) commandResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := NewRoot(RootConfig{Version: "test", Folders: f.folders, Connections: f.connections, Terminal: f.local, Stdin: strings.NewReader(input), Stdout: &stdout, Stderr: &stderr})
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}
