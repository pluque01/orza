package tui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

type sc006Observation struct {
	focus     []string
	selection []string
	details   []string
	target    []string
	error     []string
	actions   []string
}

type sc006FlowCell struct {
	name string
	run  func(*testing.T) int
}

func TestSC006QuickstartPrincipalFlowMatrix80x24NoColorTwentyRuns(t *testing.T) {
	rows := []struct {
		story string
		cells []sc006FlowCell
	}{
		{story: "US1", cells: sc006US1Cells()},
		{story: "US2", cells: sc006US2Cells()},
		{story: "US3", cells: sc006US3Cells()},
		{story: "US4", cells: sc006US4Cells()},
	}
	if got := len(rows); got != 4 {
		t.Fatalf("Principal Flow Matrix rows = %d, want 4", got)
	}

	for _, row := range rows {
		row := row
		t.Run(row.story, func(t *testing.T) {
			if len(row.cells) == 0 {
				t.Fatal("authoritative row has no cells")
			}
			frames := 0
			for _, cell := range row.cells {
				cell := cell
				t.Run(cell.name, func(t *testing.T) {
					observed := cell.run(t)
					if observed == 0 {
						t.Fatal("flow recorded no textual terminal frame")
					}
					frames += observed
				})
			}
			t.Logf("SC-006 Principal Flow Matrix %s: %d cells x %d runs, %d textual completion/cancel frames", row.story, len(row.cells), scConformanceRuns, frames)
		})
	}
}

func sc006US1Cells() []sc006FlowCell {
	targets := []struct {
		name string
		id   func(scCatalogFixture) app.NodeID
		path func(scCatalogFixture) string
		kind string
		info string
	}{
		{name: "select_root", id: func(f scCatalogFixture) app.NodeID { return f.root.ID }, path: func(f scCatalogFixture) string { return f.root.Path }, kind: "Kind: Root", info: "Direct connections: 0"},
		{name: "select_populated_folder", id: func(f scCatalogFixture) app.NodeID { return f.direct.ID }, path: func(f scCatalogFixture) string { return f.direct.Path }, kind: "Kind: Folder", info: "direct-connection: direct.example:22"},
		{name: "select_empty_folder", id: func(f scCatalogFixture) app.NodeID { return f.empty.ID }, path: func(f scCatalogFixture) string { return f.empty.Path }, kind: "Kind: Folder", info: detailEmptyConnections},
		{name: "select_connection", id: func(f scCatalogFixture) app.NodeID { return f.directConn.ID }, path: func(f scCatalogFixture) string { return f.directConn.Path }, kind: "Kind: Connection", info: "Endpoint: direct.example:22"},
	}
	cells := make([]sc006FlowCell, 0, len(targets)+1)
	for _, target := range targets {
		target := target
		cells = append(cells, sc006FlowCell{name: target.name, run: func(t *testing.T) int {
			frames := 0
			for run := 1; run <= scConformanceRuns; run++ {
				fixture := newSCCatalogFixture(t)
				model := fixture.model
				sc009Update(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
				selectSCNode(model, target.id(fixture))
				sc006ObserveModel(t, model, target.name, run, sc006Observation{
					focus: []string{"[*] Tree"}, selection: []string{"> "},
					details: []string{target.kind, target.info}, target: []string{"Path: " + target.path(fixture)},
					actions: []string{"r Reload", "? Help", "q Quit"},
				})
				frames++
			}
			return frames
		}})
	}
	cells = append(cells, sc006FlowCell{name: "enter_scroll_leave_details", run: func(t *testing.T) int {
		frames := 0
		for run := 1; run <= scConformanceRuns; run++ {
			fixture := newSCCatalogFixture(t)
			model := fixture.model
			sc009Update(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
			selectSCNode(model, fixture.directConn.ID)
			sc009Update(t, model, keyPress("tab"))
			sc006ObserveModel(t, model, "Details enter", run, sc006Observation{
				focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Kind: Connection"},
				target: []string{"Path: " + fixture.directConn.Path}, actions: []string{"Down/j Scroll down", "Tab/Shift+Tab Tree"},
			})
			frames++
			sc009Update(t, model, keyPress("G"))
			sc006ObserveModel(t, model, "Details scroll", run, sc006Observation{
				focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Endpoint: direct.example:22"},
				target: []string{"direct.example:22"}, actions: []string{"Home/g First row", "End/G Last row"},
			})
			frames++
			sc009Update(t, model, keyPress("tab"))
			sc006ObserveModel(t, model, "Details leave", run, sc006Observation{
				focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Connection"},
				target: []string{"Path: " + fixture.directConn.Path}, actions: []string{"c Connect", "r Reload"},
			})
			frames++
		}
		return frames
	}})
	return cells
}

func sc006US2Cells() []sc006FlowCell {
	type actionCell struct {
		context  string
		name     string
		key      string
		selectID func(scCatalogFixture) app.NodeID
	}
	root := func(f scCatalogFixture) app.NodeID { return f.root.ID }
	folder := func(f scCatalogFixture) app.NodeID { return f.direct.ID }
	connection := func(f scCatalogFixture) app.NodeID { return f.directConn.ID }
	actions := []actionCell{
		{context: "root", name: "new_connection_cancel", key: "n", selectID: root},
		{context: "root", name: "new_folder_cancel", key: "f", selectID: root},
		{context: "root", name: "reload_complete", key: "r", selectID: root},
		{context: "root", name: "help_close", key: "?", selectID: root},
		{context: "root", name: "quit", key: "q", selectID: root},
		{context: "folder", name: "new_connection_cancel", key: "n", selectID: folder},
		{context: "folder", name: "new_folder_cancel", key: "f", selectID: folder},
		{context: "folder", name: "edit_cancel", key: "e", selectID: folder},
		{context: "folder", name: "move_cancel", key: "m", selectID: folder},
		{context: "folder", name: "delete_cancel", key: "d", selectID: folder},
		{context: "folder", name: "reload_complete", key: "r", selectID: folder},
		{context: "folder", name: "help_close", key: "?", selectID: folder},
		{context: "folder", name: "quit", key: "q", selectID: folder},
		{context: "connection", name: "connect_cancel", key: "c", selectID: connection},
		{context: "connection", name: "new_connection_cancel", key: "n", selectID: connection},
		{context: "connection", name: "new_folder_cancel", key: "f", selectID: connection},
		{context: "connection", name: "edit_cancel", key: "e", selectID: connection},
		{context: "connection", name: "move_cancel", key: "m", selectID: connection},
		{context: "connection", name: "delete_cancel", key: "d", selectID: connection},
		{context: "connection", name: "reload_complete", key: "r", selectID: connection},
		{context: "connection", name: "help_close", key: "?", selectID: connection},
		{context: "connection", name: "quit", key: "q", selectID: connection},
	}
	cells := make([]sc006FlowCell, 0, len(actions)+8)
	for _, action := range actions {
		action := action
		cells = append(cells, sc006FlowCell{name: action.context + "/" + action.name, run: func(t *testing.T) int {
			frames := 0
			for run := 1; run <= scConformanceRuns; run++ {
				fixture := newSCCatalogFixture(t)
				model := fixture.model
				sc009Update(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
				selectSCNode(model, action.selectID(fixture))
				selected := model.browser.selectionNode()
				counters := &scActionCounters{}
				model.folders = scFixtureFolders(fixture, counters)
				model.connections = scFixtureConnections(fixture, counters)
				model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
					return app.ConnectResult{}, app.ErrInvalidRequest
				})
				command := sc009Update(t, model, keyPress(action.key))
				switch action.key {
				case "n", "e":
					sc009Update(t, model, keyPress("esc"))
				case "f", "m", "c":
					sc009Update(t, model, keyPress("esc"))
				case "d":
					sc009RunCommand(t, model, command)
					sc009Update(t, model, keyPress("esc"))
				case "r":
					sc009RunCommand(t, model, command)
				case "?":
					sc009Update(t, model, keyPress("?"))
				case "q":
					if command == nil {
						t.Fatalf("run %d: Quit returned no command", run)
					}
					if _, ok := command().(tea.QuitMsg); !ok {
						t.Fatalf("run %d: Quit command returned %T", run, command())
					}
				}
				if selected == nil {
					t.Fatalf("run %d: action began without a selected target", run)
				}
				kind := sc006KindLabel(selected.Kind)
				if action.context == "root" {
					kind = "Root"
				}
				sc006ObserveModel(t, model, action.context+" "+action.name, run, sc006Observation{
					focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: " + kind},
					target: []string{"Path: " + selected.Path}, actions: []string{"r Reload", "? Help", "q Quit"},
				})
				frames++
			}
			return frames
		}})
	}

	for _, kind := range []asyncOperationKind{asyncOperationInitialLoad, asyncOperationReload, asyncOperationSave, asyncOperationSSHStart} {
		kind := kind
		for _, outcome := range []string{"cancel", "quit"} {
			outcome := outcome
			cells = append(cells, sc006FlowCell{name: "operation/" + sc006OperationName(kind) + "/" + outcome, run: func(t *testing.T) int {
				frames := 0
				for run := 1; run <= scConformanceRuns; run++ {
					fixture := newOperationConformanceFixture(t, kind)
					if outcome == "cancel" {
						sc009Update(t, fixture.model, keyPress("esc"))
					} else {
						sc009Update(t, fixture.model, keyPress("q"))
					}
					sc009Update(t, fixture.model, operationCanceledMessage(fixture, fixture.id))
					observation := sc006OperationRestorationObservation(fixture)
					sc006ObserveModel(t, fixture.model, sc006OperationName(kind)+" "+outcome, run, observation)
					frames++
				}
				return frames
			}})
		}
	}
	return cells
}

func sc006US3Cells() []sc006FlowCell {
	return []sc006FlowCell{
		{name: "create_save", run: func(t *testing.T) int { return sc006ConnectionSaveFlow(t, false) }},
		{name: "create_cancel", run: func(t *testing.T) int { return sc006ConnectionCancelFlow(t, false) }},
		{name: "edit_save", run: func(t *testing.T) int { return sc006ConnectionSaveFlow(t, true) }},
		{name: "edit_cancel", run: func(t *testing.T) int { return sc006ConnectionCancelFlow(t, true) }},
		{name: "validation_failure", run: sc006ValidationFailureFlow},
		{name: "persistence_failure", run: sc006PersistenceFailureFlow},
		{name: "conflict_reload", run: func(t *testing.T) int { return sc006ConflictFlow(t, "reload") }},
		{name: "conflict_back", run: func(t *testing.T) int { return sc006ConflictFlow(t, "back") }},
		{name: "conflict_cancel", run: func(t *testing.T) int { return sc006ConflictFlow(t, "cancel") }},
		{name: "dirty_quit_save", run: func(t *testing.T) int { return sc006DirtyQuitFlow(t, "save") }},
		{name: "dirty_quit_discard", run: func(t *testing.T) int { return sc006DirtyQuitFlow(t, "discard") }},
		{name: "dirty_quit_cancel", run: func(t *testing.T) int { return sc006DirtyQuitFlow(t, "cancel") }},
	}
}

func sc006US4Cells() []sc006FlowCell {
	kinds := []modalKind{
		modalKindFolderCreate, modalKindFolderEdit, modalKindMovePicker, modalKindDeleteConnection,
		modalKindDeleteFolder, modalKindConnectConfirmation, modalKindUnsavedChanges, modalKindHelp,
		modalKindOperationError, modalKindSSHFailure,
	}
	cells := make([]sc006FlowCell, 0, 27)
	for _, kind := range kinds {
		kind := kind
		for _, outcome := range []string{"complete", "cancel"} {
			outcome := outcome
			cells = append(cells, sc006FlowCell{name: "modal/" + string(kind) + "/" + outcome, run: func(t *testing.T) int {
				return sc006ModalFlow(t, kind, outcome)
			}})
		}
	}
	cells = append(cells,
		sc006FlowCell{name: "embedded_error", run: func(t *testing.T) int { return sc006EmbeddedModalFlow(t, false) }},
		sc006FlowCell{name: "embedded_conflict", run: func(t *testing.T) int { return sc006EmbeddedModalFlow(t, true) }},
		sc006FlowCell{name: "inline_help_open_close", run: sc006InlineHelpFlow},
	)
	for _, status := range []app.HostTrustStatus{app.HostTrustUnknown, app.HostTrustChanged} {
		status := status
		for _, outcome := range []string{"accept", "cancel"} {
			outcome := outcome
			cells = append(cells, sc006FlowCell{name: "host_trust/" + string(status) + "/" + outcome, run: func(t *testing.T) int {
				return sc006TrustBoundaryFlow(t, status, outcome)
			}})
		}
	}
	return cells
}

func sc006ObserveModel(t *testing.T, model *Model, cell string, run int, observation sc006Observation) {
	t.Helper()
	if model.width != 80 || model.height != 24 || !model.noColor {
		t.Fatalf("%s run %d: frame contract = %dx%d no-color=%v, want 80x24 true", cell, run, model.width, model.height, model.noColor)
	}
	view := model.View().Content
	assertSCNoANSI(t, view)
	groups := []struct {
		name string
		want []string
	}{
		{name: "focus", want: observation.focus},
		{name: "selection", want: observation.selection},
		{name: "Details info", want: observation.details},
		{name: "target", want: observation.target},
		{name: "error", want: observation.error},
		{name: "actions", want: observation.actions},
	}
	assertions := 0
	for _, group := range groups {
		for _, want := range group.want {
			assertions++
			if !strings.Contains(view, want) {
				t.Fatalf("%s run %d: %s observation omitted %q:\n%s", cell, run, group.name, want, view)
			}
		}
	}
	if assertions == 0 {
		t.Fatalf("%s run %d: frame declared no textual observations", cell, run)
	}
}

func sc006KindLabel(kind app.NodeKind) string {
	switch kind {
	case app.NodeKindFolder:
		return "Folder"
	case app.NodeKindConnection:
		return "Connection"
	default:
		return "Root"
	}
}

func sc006OperationName(kind asyncOperationKind) string {
	switch kind {
	case asyncOperationInitialLoad:
		return "initial_load"
	case asyncOperationReload:
		return "reload"
	case asyncOperationSave:
		return "save"
	case asyncOperationSSHStart:
		return "ssh_start"
	default:
		return "unknown"
	}
}

func sc006OperationRestorationObservation(fixture operationConformanceFixture) sc006Observation {
	switch fixture.kind {
	case asyncOperationSave:
		return sc006Observation{focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Edit connection", "saved.test"}, target: []string{fixture.connection.Path}, actions: []string{"Ctrl+S Save", "Esc Cancel"}}
	case asyncOperationSSHStart:
		return sc006Observation{focus: []string{"[*] Connect"}, selection: []string{"> "}, target: []string{"Path: " + fixture.connection.Path, "Endpoint: server.test:22"}, actions: []string{"y Confirm", "Enter/Esc Cancel"}}
	case asyncOperationReload:
		return sc006Observation{focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Kind: Connection"}, target: []string{"Path: " + fixture.connection.Path}, actions: []string{"r Reload", "q Quit"}}
	default:
		return sc006Observation{focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Root"}, target: []string{"Path: /"}, actions: []string{"r Reload", "q Quit"}}
	}
}

func sc006ConnectionSaveFlow(t *testing.T, edit bool) int {
	t.Helper()
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		model, original := sc006SaveModel(t)
		if edit {
			selectSCNode(model, original.ID)
			sc009Update(t, model, keyPress("e"))
			model.form.inputs[fieldHost].SetValue("edited.test")
		} else {
			sc009Update(t, model, keyPress("n"))
			model.form.inputs[fieldName].SetValue("created")
			model.form.inputs[fieldHost].SetValue("created.test")
		}
		reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("ctrl+s")))
		sc009RunCommand(t, model, reload)
		path := "/created"
		host := "created.test"
		if edit {
			path, host = original.Path, "edited.test"
		}
		sc006ObserveModel(t, model, "connection save", run, sc006Observation{
			focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Connection", "Endpoint: " + host + ":22"},
			target: []string{"Path: " + path}, actions: []string{"c Connect", "e Edit", "q Quit"},
		})
		frames++
	}
	return frames
}

func sc006SaveModel(t *testing.T) (*Model, app.Connection) {
	t.Helper()
	original := testConnection("original", syntheticRootID, "/original", 3)
	connections := []app.Connection{original}
	service := ConnectionFuncs{
		ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{Connections: append([]app.Connection(nil), connections...), CatalogRevision: 8}, nil
		},
		CreateFunc: func(_ context.Context, request app.CreateConnectionRequest) (app.ConnectionResult, error) {
			created := testConnection("created", syntheticRootID, "/created", 1)
			created.Host = request.Host
			connections = append(connections, created)
			return app.ConnectionResult{Connection: created, CatalogRevision: 8}, nil
		},
		UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
			updated := connectionWithUpdate(original, request)
			connections = []app.Connection{updated}
			return app.ConnectionResult{Connection: updated, CatalogRevision: 8}, nil
		},
	}
	model := New(Config{Connections: service, Width: 80, Height: 24, NoColor: true})
	sc009RunCommand(t, model, model.Init())
	return model, original
}

func sc006ConnectionCancelFlow(t *testing.T, edit bool) int {
	t.Helper()
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		fixture := newSCCatalogFixture(t)
		model := fixture.model
		sc009Update(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
		selected, path := fixture.direct.ID, fixture.direct.Path
		key := "n"
		kind := "Folder"
		if edit {
			selected, path, key, kind = fixture.directConn.ID, fixture.directConn.Path, "e", "Connection"
		}
		selectSCNode(model, selected)
		sc009Update(t, model, keyPress(key))
		sc009Update(t, model, keyPress("esc"))
		sc006ObserveModel(t, model, "connection cancel", run, sc006Observation{
			focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: " + kind},
			target: []string{"Path: " + path}, actions: []string{"r Reload", "q Quit"},
		})
		frames++
	}
	return frames
}

func sc006ValidationFailureFlow(t *testing.T) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		model := New(Config{Width: 80, Height: 24, NoColor: true})
		sc009Update(t, model, keyPress("n"))
		model.form.inputs[fieldName].SetValue("bad/name")
		model.form.setFocus(fieldName)
		sc009Update(t, model, keyPress("ctrl+s"))
		sc006ObserveModel(t, model, "validation failure", run, sc006Observation{
			focus: []string{"[*] Details"}, selection: []string{"> [/] /"}, details: []string{">!  Name:", "bad/name"},
			error: []string{"Error:"}, actions: []string{"Ctrl+S Save", "Esc Cancel"},
		})
		frames++
	}
	return frames
}

func sc006PersistenceFailureFlow(t *testing.T) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		model := New(Config{Width: 80, Height: 24, NoColor: true, Connections: ConnectionFuncs{
			CreateFunc: func(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
				return app.ConnectionResult{}, errors.New("controlled persistence failure")
			},
		}})
		sc009Update(t, model, keyPress("n"))
		model.form.inputs[fieldName].SetValue("created")
		model.form.inputs[fieldHost].SetValue("persist.test")
		model.form.setFocus(fieldSave)
		sc009RunCommand(t, model, sc009Update(t, model, keyPress("ctrl+s")))
		sc006ObserveModel(t, model, "persistence failure", run, sc006Observation{
			focus: []string{"[*] Details"}, selection: []string{"> [/] /"}, details: []string{"[ Save connection ]"},
			error: []string{"Error: Save failed safely"}, actions: []string{"Ctrl+S Save", "Esc Cancel"},
		})
		frames++
	}
	return frames
}

func sc006ConflictFlow(t *testing.T, outcome string) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		connection := testConnection("connection", syntheticRootID, "/connection", 4)
		model := loadedModel(t, []app.Connection{connection}, true)
		sc009Update(t, model, keyPress("l"))
		sc009Update(t, model, keyPress("e"))
		model.form.inputs[fieldHost].SetValue("pending.test")
		model.form.setFocus(fieldHost)
		model.installFormConflict(conflictTypeRevisionChanged)
		switch outcome {
		case "reload":
			model.connections = ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 8}, nil
			}}
			sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
			sc006ObserveModel(t, model, "conflict Reload", run, sc006Observation{
				focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Edit connection", "pending.test"},
				target: []string{connection.Path}, actions: []string{"Ctrl+S Save", "Esc Cancel"},
			})
		case "back":
			sc009Update(t, model, keyPress("b"))
			sc006ObserveModel(t, model, "conflict Back", run, sc006Observation{
				focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Root"}, target: []string{"Path: /"}, actions: []string{"r Reload", "q Quit"},
			})
		case "cancel":
			sc009Update(t, model, keyPress("esc"))
			sc006ObserveModel(t, model, "conflict Cancel", run, sc006Observation{
				focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Edit connection", "pending.test"},
				target: []string{connection.Path}, error: []string{"Warning: Save blocked"}, actions: []string{"r Reload", "b Back", "Esc Cancel warning"},
			})
		}
		frames++
	}
	return frames
}

func sc006DirtyQuitFlow(t *testing.T, outcome string) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		var persisted app.Connection
		model, connection := dirtyEditModel(t, nil)
		if outcome == "save" {
			model.connections = ConnectionFuncs{
				UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
					persisted = connectionWithUpdate(connection, request)
					return app.ConnectionResult{Connection: persisted, CatalogRevision: 8}, nil
				},
				ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
					return app.ListConnectionsResult{Connections: []app.Connection{persisted}, CatalogRevision: 8}, nil
				},
			}
		}
		sc009Update(t, model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
		switch outcome {
		case "save":
			reconcile := sc009RunCommand(t, model, sc009Update(t, model, keyPress("s")))
			quit := sc009RunCommand(t, model, reconcile)
			if quit == nil {
				t.Fatalf("run %d: dirty Save did not defer Quit through reconciliation", run)
			}
			sc006ObserveModel(t, model, "dirty Quit Save", run, sc006Observation{
				focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Connection", "Endpoint: pending.test:22"},
				target: []string{"Path: " + connection.Path}, actions: []string{"c Connect", "q Quit"},
			})
		case "discard":
			command := sc009Update(t, model, keyPress("d"))
			if command == nil {
				t.Fatalf("run %d: dirty Discard did not return Quit", run)
			}
			sc006ObserveModel(t, model, "dirty Quit Discard", run, sc006Observation{
				focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Connection"},
				target: []string{"Path: " + connection.Path}, actions: []string{"c Connect", "q Quit"},
			})
		case "cancel":
			sc009Update(t, model, keyPress("esc"))
			sc006ObserveModel(t, model, "dirty Quit Cancel", run, sc006Observation{
				focus: []string{"[*] Details"}, selection: []string{"> "}, details: []string{"Edit connection", "pending.test"},
				target: []string{connection.Path}, actions: []string{"Ctrl+S Save", "Esc Cancel"},
			})
		}
		frames++
	}
	return frames
}

type sc006ModalFixture struct {
	model       *Model
	root        app.Folder
	folder      app.Folder
	destination app.Folder
	connection  app.Connection
}

func newSC006ModalFixture(t *testing.T, kind modalKind) sc006ModalFixture {
	t.Helper()
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 3)
	destination := testFolder("destination", root.ID, "/destination", 2)
	connection := testConnection("connection", folder.ID, "/folder/connection", 4)
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	sc009Snapshot(model, root, []app.Folder{folder, destination}, []app.Connection{connection}, connection.ID)
	model.focusOwner = focusOwnerDetail
	var target *capturedTarget
	var payload any
	switch kind {
	case modalKindFolderCreate:
		form := newFolderForm(nil, app.ItemSelector{ID: folder.ID})
		form.setDestination(folder)
		form.input.SetValue("created")
		captured := capturedTargetFromFolder(folder)
		target, payload = &captured, folderCreatePayload{form: form}
	case modalKindFolderEdit:
		form := newFolderForm(&folder, app.ItemSelector{})
		form.input.SetValue("renamed")
		captured := capturedTargetFromFolder(folder)
		target, payload = &captured, folderEditPayload{form: form}
	case modalKindMovePicker:
		captured := capturedTargetFromConnection(connection)
		target, payload = &captured, movePickerPayload{picker: newMovePicker(connection.Node, []app.Folder{destination})}
	case modalKindDeleteConnection:
		captured := capturedTargetFromConnection(connection)
		scope := app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision}
		target, payload = &captured, deleteConnectionPayload{confirmation: newDeleteConfirmation(scope)}
	case modalKindDeleteFolder:
		captured := capturedTargetFromFolder(folder)
		scope := app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1, Snapshot: []app.NodeRevision{{ID: folder.ID, Revision: folder.Revision}}}
		target, payload = &captured, deleteFolderPayload{confirmation: newFolderDeleteConfirmation(scope)}
	case modalKindConnectConfirmation:
		captured := capturedTargetFromConnection(connection)
		target, payload = &captured, connectConfirmationPayload{confirmation: newConnectConfirmation(connection)}
	case modalKindUnsavedChanges:
		payload = unsavedChangesPayload{intent: unsavedIntentQuit, target: connection.Path}
	case modalKindHelp:
		payload = helpPayload{lines: []string{"Up Move up", "Down Move down", "Esc Close"}}
	case modalKindOperationError:
		payload = operationErrorPayload{modal: newErrorModal("move catalog item", connection.Path, app.ErrInvalidRequest)}
	case modalKindSSHFailure:
		failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()
		payload = sshFailurePayload{modal: newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, failure)}
	}
	if !model.openGenericModal(kind, target, payload) {
		t.Fatalf("open %s", kind)
	}
	return sc006ModalFixture{model: model, root: root, folder: folder, destination: destination, connection: connection}
}

func sc006ModalFlow(t *testing.T, kind modalKind, outcome string) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		fixture := newSC006ModalFixture(t, kind)
		model := fixture.model
		initial := sc006ModalObservation(fixture, kind)
		sc006ObserveModel(t, model, string(kind)+" open", run, initial)
		frames++
		if outcome == "cancel" {
			sc009Update(t, model, keyPress("esc"))
		} else {
			sc006CompleteModal(t, fixture, kind)
		}
		sc006ObserveModel(t, model, string(kind)+" "+outcome, run, sc006BrowserObservation(model))
		frames++
	}
	return frames
}

func sc006ModalObservation(fixture sc006ModalFixture, kind modalKind) sc006Observation {
	observation := sc006Observation{focus: []string{"[*] " + modalTitle(kind)}, selection: []string{"> "}}
	switch kind {
	case modalKindFolderCreate:
		observation.target, observation.actions = []string{"Target: " + fixture.folder.Path}, []string{"Save", "Cancel"}
	case modalKindFolderEdit:
		observation.target, observation.actions = []string{"Target: " + fixture.folder.Path, "ID/revision: folder/3"}, []string{"Save", "Cancel"}
	case modalKindMovePicker:
		observation.target, observation.actions = []string{"Source: " + fixture.connection.Path, fixture.destination.Path}, []string{"Enter Move", "Esc Cancel"}
	case modalKindDeleteConnection:
		observation.target, observation.actions = []string{"Path: " + fixture.connection.Path, "Target: (default)@host.test"}, []string{"y Confirm", "Enter/Esc Cancel"}
	case modalKindDeleteFolder:
		observation.target, observation.actions = []string{"Path: " + fixture.folder.Path, "ID/revision: folder/3"}, []string{"y Confirm", "Enter/Esc Cancel"}
	case modalKindConnectConfirmation:
		observation.target, observation.actions = []string{"Path: " + fixture.connection.Path, "Endpoint: host.test:22"}, []string{"y Confirm", "Enter/Esc Cancel"}
	case modalKindUnsavedChanges:
		observation.target, observation.actions = []string{"Target: " + fixture.connection.Path}, []string{"s Save", "d Discard", "Esc Cancel"}
	case modalKindHelp:
		observation.details, observation.actions = []string{"Up Move up", "Down Move down"}, []string{"?/Esc Close"}
	case modalKindOperationError:
		observation.target, observation.error, observation.actions = []string{"Target: " + fixture.connection.Path}, []string{"Recoverable operation error", "Cause:"}, []string{"b/Esc Back", "q Quit"}
	case modalKindSSHFailure:
		observation.target, observation.error, observation.actions = []string{"Path: " + fixture.connection.Path, "Endpoint: host.test:22"}, []string{"SSH startup failed", "Category: timeout"}, []string{"r Retry", "e Edit", "b/Esc Back", "q Quit"}
	}
	return observation
}

func sc006BrowserObservation(model *Model) sc006Observation {
	selected := model.browser.selectionNode()
	if selected == nil {
		return sc006Observation{focus: []string{"[*] Tree"}, selection: []string{"> "}, details: []string{"Kind: Root"}, target: []string{"Path: /"}, actions: []string{"r Reload", "q Quit"}}
	}
	kind := sc006KindLabel(selected.Kind)
	if selected.ID == model.browser.snapshot.rootID {
		kind = "Root"
	}
	focus := "[*] Tree"
	if model.focusOwner == focusOwnerDetail {
		focus = "[*] Details"
	}
	actions := []string{"r Reload", "q Quit"}
	if selected.Kind == app.NodeKindConnection {
		actions = append(actions, "c Connect")
	}
	return sc006Observation{
		focus: []string{focus}, selection: []string{"> "}, details: []string{"Kind: " + kind},
		target: []string{"Path: " + selected.Path}, actions: actions,
	}
}

func sc006CompleteModal(t *testing.T, fixture sc006ModalFixture, kind modalKind) {
	t.Helper()
	model := fixture.model
	switch kind {
	case modalKindFolderCreate:
		sc009Update(t, model, keyPress("enter"))
		sc006FinishOperation(t, model, operationResultMsg{id: model.operation.id, kind: operationFolderCreate, folder: testFolder("created", fixture.folder.ID, "/folder/created", 1)})
	case modalKindFolderEdit:
		sc009Update(t, model, keyPress("enter"))
		renamed := fixture.folder
		renamed.Name, renamed.Revision = "renamed", renamed.Revision+1
		sc006FinishOperation(t, model, operationResultMsg{id: model.operation.id, kind: operationFolderRename, folder: renamed})
	case modalKindMovePicker:
		sc009Update(t, model, keyPress("enter"))
		moved := fixture.connection
		moved.ParentID, moved.Path = fixture.destination.ID, "/destination/connection"
		sc006FinishOperation(t, model, operationResultMsg{id: model.operation.id, kind: operationMove, connection: app.ConnectionResult{Connection: moved}})
	case modalKindDeleteConnection:
		sc009Update(t, model, keyPress("y"))
		sc006FinishOperation(t, model, operationResultMsg{id: model.operation.id, kind: operationDelete, deleted: app.DeleteConnectionResult{Deleted: fixture.connection}})
	case modalKindDeleteFolder:
		sc009Update(t, model, keyPress("y"))
		sc006FinishOperation(t, model, operationResultMsg{id: model.operation.id, kind: operationFolderDelete})
	case modalKindConnectConfirmation:
		sc009Update(t, model, keyPress("y"))
		id := model.operation.id
		attempt := app.SSHAttemptTarget{ID: fixture.connection.ID, Revision: fixture.connection.Revision, Path: fixture.connection.Path, Host: fixture.connection.Host, Port: fixture.connection.Port}
		sc009Update(t, model, sessionFinishedMsg{id: id, attempt: attempt, result: app.ConnectResult{Connection: fixture.connection, Attempt: attempt, Session: app.SSHSessionResult{State: app.SessionSucceeded, StartedAt: time.Unix(1, 0)}}})
	case modalKindUnsavedChanges:
		sc009Update(t, model, keyPress("d"))
	case modalKindHelp:
		sc009Update(t, model, keyPress("?"))
	case modalKindOperationError:
		sc009Update(t, model, keyPress("b"))
	case modalKindSSHFailure:
		sc009Update(t, model, keyPress("b"))
	}
}

func sc006FinishOperation(t *testing.T, model *Model, result operationResultMsg) {
	t.Helper()
	sc009Update(t, model, result)
	if model.operation != nil && model.operation.kind == asyncOperationReload {
		snapshot := model.browser.snapshot
		sc009Update(t, model, operationResultMsg{id: model.operation.id, kind: operationReload, snapshot: &snapshot})
	}
}

func sc006EmbeddedModalFlow(t *testing.T, conflict bool) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		fixture := newSC006ModalFixture(t, modalKindMovePicker)
		model := fixture.model
		if conflict {
			state, ok := newConflictState(conflictTypeRevisionChanged, capturedTargetFromConnection(fixture.connection), conflictOwnerModal)
			if !ok {
				t.Fatal("embedded conflict setup failed")
			}
			model.modal.conflict = &state
			sc006ObserveModel(t, model, "embedded conflict", run, sc006Observation{
				focus: []string{"[*] Move"}, selection: []string{"> "}, target: []string{"Target: " + fixture.connection.Path, "Source: " + fixture.connection.Path},
				error: []string{"Warning: captured target is no longer current"}, actions: []string{"r Reload", "b Back", "Esc Cancel warning"},
			})
		} else {
			model.modal.recoverableError = "controlled embedded failure"
			sc006ObserveModel(t, model, "embedded error", run, sc006Observation{
				focus: []string{"[*] Move"}, selection: []string{"> "}, target: []string{"Source: " + fixture.connection.Path, fixture.destination.Path},
				error: []string{"Error: controlled embedded failure"}, actions: []string{"Enter Move", "Esc Cancel"},
			})
		}
		frames++
	}
	return frames
}

func sc006InlineHelpFlow(t *testing.T) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		fixture := newSC006ModalFixture(t, modalKindMovePicker)
		model := fixture.model
		sc009Update(t, model, keyPress("?"))
		sc006ObserveModel(t, model, "inline Help open", run, sc006Observation{
			focus: []string{"[*] Move"}, selection: []string{"> "}, details: []string{"Help"}, target: []string{"Move"}, actions: []string{"?/Esc Close"},
		})
		frames++
		sc009Update(t, model, keyPress("esc"))
		sc006ObserveModel(t, model, "inline Help close", run, sc006ModalObservation(fixture, modalKindMovePicker))
		frames++
	}
	return frames
}

func sc006TrustBoundaryFlow(t *testing.T, status app.HostTrustStatus, outcome string) int {
	frames := 0
	for run := 1; run <= scConformanceRuns; run++ {
		prompt := app.TrustDecisionPrompt{
			Status: status,
			Host: app.PresentedHost{
				Endpoint:          app.HostEndpoint{CanonicalHost: "server.test", Port: 22},
				RemoteAddress:     "192.0.2.10:22",
				KeyAlgorithm:      "ssh-ed25519",
				FingerprintSHA256: "SHA256:presented-public-fingerprint",
			},
		}
		if status == app.HostTrustChanged {
			prompt.Known = &app.TrustedHost{FingerprintSHA256: "SHA256:known-public-fingerprint"}
		}
		input, want := "once\n", app.TrustOnce
		if outcome == "cancel" {
			input, want = "\n", app.TrustReject
		}
		var output bytes.Buffer
		decision, err := decideTrust(context.Background(), strings.NewReader(input), &output, prompt, true)
		if err != nil || decision != want {
			t.Fatalf("run %d %s/%s: decision/error = %q/%v, want %q/nil", run, status, outcome, decision, err, want)
		}
		view := output.String()
		assertSCNoANSI(t, view)
		wantText := []string{
			"Verify host identity", "Host: server.test:22", "Remote address: 192.0.2.10:22",
			"Algorithm: ssh-ed25519", "SHA-256 fingerprint: SHA256:presented-public-fingerprint",
			"> Reject (default)    Trust once    Trust and persist", "Decision [reject/once/persist] (reject):",
		}
		if status == app.HostTrustChanged {
			wantText = append(wantText, "Known fingerprint: SHA256:known-public-fingerprint", "WARNING: changed key; this may indicate a possible attack.")
		}
		for _, wantText := range wantText {
			if !strings.Contains(view, wantText) {
				t.Fatalf("run %d %s/%s: trust boundary omitted %q:\n%s", run, status, outcome, wantText, view)
			}
		}
		for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
			if width := ansi.StringWidth(line); width > 80 {
				t.Fatalf("run %d %s/%s: trust line width %d > 80: %q", run, status, outcome, width, line)
			}
		}
		if lines := strings.Count(view, "\n") + 1; lines > 24 {
			t.Fatalf("run %d %s/%s: trust frame height %d > 24", run, status, outcome, lines)
		}
		frames++
	}
	return frames
}
