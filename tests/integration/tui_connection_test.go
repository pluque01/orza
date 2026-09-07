package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
)

func TestTUIConnectionCRUDAndStaleRecovery(t *testing.T) {
	connections := integrationConnectionService(t)
	model := tui.New(tui.Config{Connections: connections, Width: 80, Height: 24, NoColor: true})
	command := model.Init()
	updateTUI(t, model, command())

	updateTUI(t, model, tuiKey("n"))
	typeText(t, model, "integration")
	updateTUI(t, model, tuiKey("tab"))
	typeText(t, model, "example.test")
	updateTUI(t, model, tuiKey("tab"))
	updateTUI(t, model, tuiKey("tab"))
	typeText(t, model, "tester")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))

	listed, err := connections.List(context.Background(), app.ListConnectionsRequest{Folder: app.ItemSelector{Path: "/"}})
	if err != nil || len(listed.Connections) != 1 {
		t.Fatalf("created connections = %+v, %v", listed.Connections, err)
	}
	created := listed.Connections[0]

	updateTUI(t, model, tuiKey("e"))
	updateTUI(t, model, tuiKey("tab"))
	updateTUI(t, model, tuiKey("ctrl+a"))
	updateTUI(t, model, tuiKey("ctrl+k"))
	typeText(t, model, "edited.test")
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
	updated, err := connections.Get(context.Background(), app.ItemSelector{ID: created.ID})
	if err != nil || updated.Connection.Host != "edited.test" {
		t.Fatalf("updated connection = %+v, %v", updated.Connection, err)
	}

	updateTUI(t, model, tuiKey("e"))
	updateTUI(t, model, tuiKey("tab"))
	updateTUI(t, model, tuiKey("ctrl+a"))
	updateTUI(t, model, tuiKey("ctrl+k"))
	typeText(t, model, "stale.test")
	external := "external.test"
	revision := updated.Connection.Revision
	if _, err := connections.Update(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: created.ID}, Expected: &revision, Host: &external}); err != nil {
		t.Fatal(err)
	}
	_, command = model.Update(tuiKey("ctrl+s"))
	if command == nil {
		t.Fatal("stale save did not start")
	}
	updateTUI(t, model, command())
	if view := model.View().Content; !strings.Contains(view, "Save blocked") || !strings.Contains(view, "Reload") || !strings.Contains(view, created.Path) {
		t.Fatalf("stale conflict view = %q", view)
	}
	current, err := connections.Get(context.Background(), app.ItemSelector{ID: created.ID})
	if err != nil || current.Connection.Host != external {
		t.Fatalf("external update was overwritten: %+v, %v", current.Connection, err)
	}

	updateTUI(t, model, tuiKey("b"))
	updateTUI(t, model, tuiKey("l"))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	if view := model.View().Content; !strings.Contains(view, "Operation: delete connection") || !strings.Contains(view, "never overwritten") || !strings.Contains(view, "r Reload") {
		t.Fatalf("stale delete view = %q", view)
	}
	if current, err = connections.Get(context.Background(), app.ItemSelector{ID: created.ID}); err != nil || current.Connection.Host != external {
		t.Fatalf("stale delete changed connection: %+v, %v", current.Connection, err)
	}
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("r")))
	assertTreeSelectionPath(t, model, created.Path)
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("d")))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("y")))
	if _, err := connections.Get(context.Background(), app.ItemSelector{ID: created.ID}); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("deleted connection lookup = %v", err)
	}
}

func TestTUIInitialRootAndRootConnectionCreateDoNotReportFalseConflicts(t *testing.T) {
	folders, connections := integrationTreeServices(t)
	model := tui.New(tui.Config{Folders: folders, Connections: connections, Width: 80, Height: 24, NoColor: true})
	executeTUICommand(t, model, model.Init())

	if view := model.View().Content; strings.Contains(view, "TARGET NO LONGER EXISTS") {
		t.Errorf("initial root was reported missing:\n%s", view)
	}

	updateTUI(t, model, tuiKey("n"))
	typeText(t, model, "root-connection")
	updateTUI(t, model, tuiKey("tab"))
	typeText(t, model, "root.test")
	command := updateTUI(t, model, tuiKey("ctrl+s"))
	if command == nil {
		t.Fatalf("root connection Save was blocked:\n%s", model.View().Content)
	}
	executeTUICommand(t, model, command)
	if view := model.View().Content; strings.Contains(view, "Save blocked") {
		t.Fatalf("root connection create reported a false conflict:\n%s", view)
	}
	if _, err := connections.Get(context.Background(), app.ItemSelector{Path: "/root-connection"}); err != nil {
		t.Fatalf("root connection was not created: %v", err)
	}

	updateTUI(t, model, tuiKey("n"))
	typeText(t, model, "root-connection")
	updateTUI(t, model, tuiKey("tab"))
	typeText(t, model, "other.test")
	if command := updateTUI(t, model, tuiKey("ctrl+s")); command != nil {
		executeTUICommand(t, model, command)
	}
	view := model.View().Content
	if !strings.Contains(view, "name already exists in this folder") || strings.Contains(view, "captured connection changed") {
		t.Fatalf("duplicate name did not produce a field error:\n%s", view)
	}
}

func TestTUIConnectionFormDetailsCancelAndSelectionTwentyRuns(t *testing.T) {
	for run := range 20 {
		connections := integrationConnectionService(t)
		model := tui.New(tui.Config{Connections: connections, Width: 80, Height: 24, NoColor: true})
		updateTUI(t, model, model.Init()())

		updateTUI(t, model, tuiKey("n"))
		view := model.View().Content
		for _, want := range []string{"Tree", "Details", "New connection", "Actions"} {
			if !strings.Contains(view, want) {
				t.Fatalf("run %d: create form omitted %q: %q", run, want, view)
			}
		}
		typeText(t, model, "discarded")
		updateTUI(t, model, tuiKey("esc"))
		if !strings.Contains(model.View().Content, "s Save") || !strings.Contains(model.View().Content, "d Discard") || !strings.Contains(model.View().Content, "Esc Cancel") {
			t.Fatalf("run %d: dirty Cancel prompt omitted choices", run)
		}
		updateTUI(t, model, tuiKey("d"))
		if !strings.Contains(model.View().Content, "> [/] /") {
			t.Fatalf("run %d: cancel did not restore root selection", run)
		}

		updateTUI(t, model, tuiKey("n"))
		typeText(t, model, "created")
		updateTUI(t, model, tuiKey("tab"))
		typeText(t, model, "created.test")
		executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
		listed, err := connections.List(context.Background(), app.ListConnectionsRequest{Folder: app.ItemSelector{Path: "/"}})
		if err != nil || len(listed.Connections) != 1 {
			t.Fatalf("run %d: created connections = %+v, %v", run, listed.Connections, err)
		}
		created := listed.Connections[0]
		assertTreeSelectionPath(t, model, created.Path)

		updateTUI(t, model, tuiKey("e"))
		if view := model.View().Content; !strings.Contains(view, "Tree") || !strings.Contains(view, "Edit connection") || !strings.Contains(view, created.Path) {
			t.Fatalf("run %d: edit did not remain in Details: %q", run, view)
		}
		updateTUI(t, model, tuiKey("tab"))
		updateTUI(t, model, tuiKey("ctrl+a"))
		updateTUI(t, model, tuiKey("ctrl+k"))
		typeText(t, model, "edited.test")
		executeTUICommand(t, model, updateTUI(t, model, tuiKey("ctrl+s")))
		updated, err := connections.Get(context.Background(), app.ItemSelector{ID: created.ID})
		if err != nil || updated.Connection.Host != "edited.test" {
			t.Fatalf("run %d: updated connection = %+v, %v", run, updated.Connection, err)
		}
		assertTreeSelectionPath(t, model, updated.Connection.Path)
	}
}

func TestTUISessionFailureRecoveryHandoffAndTerminalRestoration(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "11111111111111111111111111111111", Name: "prod", Path: "/prod", Revision: 1}, Host: "prod.test", Port: 22, AuthMethod: app.AuthMethodAgent}
	list := func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
		return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 1}, nil
	}

	t.Run("active session hands off and returns status", func(t *testing.T) {
		local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
		writer := newSignalingWriter("[ssh] prod", "Endpoint: prod.test:22", "Connect to SSH target?")
		services := tui.ConnectionFuncs{ListFunc: list}
		status := 23
		input := pipeInput(t, []readerStage{
			{wait: writer.signal("[ssh] prod"), data: "j"},
			{wait: writer.signal("Endpoint: prod.test:22"), data: "c"},
			{wait: writer.signal("Connect to SSH target?"), data: "y"},
		})
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := tui.Run(ctx, tui.Config{
			Connections: services,
			Connect: tui.ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
				return app.ConnectResult{Connection: connection, Session: app.SSHSessionResult{State: app.SessionSucceeded, Outcome: app.SessionOutcomeRemoteFailure, StartedAt: time.Unix(1, 0), RemoteExitStatus: &status}}, nil
			}),
			Terminal: local, Stdin: input, Stdout: writer, NoColor: true,
		})
		if err != nil || result.RemoteExitStatus == nil || *result.RemoteExitStatus != status {
			t.Fatalf("Run() = %+v, %v; output = %q", result, err, writer.String())
		}
		assertConnectConfirmationOutput(t, writer.String(), connection)
		assertTerminalRestored(t, local)
	})
}

func TestTUIConnectConfirmationCancelStartsZeroNetwork(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "11111111111111111111111111111111", Name: "prod", Path: "/prod", Revision: 1}, Host: "prod.test", Port: 22, AuthMethod: app.AuthMethodAgent}
	connectCalls := 0
	model := tui.New(tui.Config{
		Connections: tui.ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 1}, nil
		}},
		Connect: tui.ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
			connectCalls++
			return app.ConnectResult{}, nil
		}),
		Width: 80, Height: 24, NoColor: true,
	})
	updateTUI(t, model, model.Init()())
	updateTUI(t, model, tuiKey("j"))
	updateTUI(t, model, tuiKey("c"))
	if view := model.View().Content; !strings.Contains(view, "Connect to SSH target?") || !strings.Contains(view, connection.Path) {
		t.Fatalf("connect confirmation = %q", view)
	}
	updateTUI(t, model, tuiKey("enter"))
	if connectCalls != 0 {
		t.Fatalf("cancel started %d network calls", connectCalls)
	}
	assertTreeSelectionPath(t, model, connection.Path)
}

func assertConnectConfirmationOutput(t *testing.T, output string, connection app.Connection) {
	t.Helper()
	for _, value := range []string{"Connect to SSH target?", "Path: " + connection.Path, "Endpoint: " + connection.Host + ":22"} {
		if !strings.Contains(output, value) {
			t.Fatalf("connect confirmation omitted %q: %q", value, output)
		}
	}
}

func integrationConnectionService(t *testing.T) *app.ConnectionService {
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
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(store), credential.NewFake(), credential.Scope("cccccccccccccccccccccccccccccccc"), nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewConnectionService(catalogrepo.NewRepository(store), saga)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func updateTUI(t *testing.T, model *tui.Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	updated, command := model.Update(msg)
	if updated != model {
		t.Fatal("root model pointer changed")
	}
	return command
}

func executeTUICommand(t *testing.T, model *tui.Model, command tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatalf("expected operation command; view:\n%s", model.View().Content)
	}
	command = updateTUI(t, model, command())
	if command != nil {
		updateTUI(t, model, command())
	}
}

func typeText(t *testing.T, model *tui.Model, value string) {
	t.Helper()
	for _, character := range value {
		updateTUI(t, model, tea.KeyPressMsg(tea.Key{Code: character, Text: string(character)}))
	}
}

func tuiKey(value string) tea.KeyPressMsg {
	switch value {
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	case "ctrl+s":
		return tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl})
	case "ctrl+a":
		return tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl})
	case "ctrl+k":
		return tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl})
	case "f1":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyF1})
	default:
		return tea.KeyPressMsg(tea.Key{Code: []rune(value)[0], Text: value})
	}
}

type readerStage struct {
	wait <-chan struct{}
	data string
}

func pipeInput(t *testing.T, stages []readerStage) io.Reader {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	go func() {
		defer writer.Close()
		for _, stage := range stages {
			if stage.wait != nil {
				<-stage.wait
			}
			_, _ = io.WriteString(writer, stage.data)
		}
	}()
	return reader
}

type signalingWriter struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	signals map[string]chan struct{}
	seen    map[string]bool
}

func newSignalingWriter(needles ...string) *signalingWriter {
	w := &signalingWriter{signals: make(map[string]chan struct{}, len(needles)), seen: make(map[string]bool, len(needles))}
	for _, needle := range needles {
		w.signals[needle] = make(chan struct{})
	}
	return w
}

func (w *signalingWriter) signal(needle string) <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.signals[needle]
}

func (w *signalingWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buffer.Write(value)
	for needle, signal := range w.signals {
		if !w.seen[needle] && strings.Contains(w.buffer.String(), needle) {
			close(signal)
			w.seen[needle] = true
		}
	}
	return n, err
}

func (w *signalingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func assertTerminalRestored(t *testing.T, local *terminal.Fake) {
	t.Helper()
	calls := local.Calls()
	if len(calls) < 2 || calls[len(calls)-1].Operation != terminal.OperationRestore {
		t.Fatalf("terminal calls = %+v, want final restore", calls)
	}
}
