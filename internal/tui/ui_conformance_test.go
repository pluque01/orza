package tui

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const scConformanceRuns = 20

type scCatalogFixture struct {
	model      *Model
	root       app.Folder
	empty      app.Folder
	direct     app.Folder
	nested     app.Folder
	child      app.Folder
	directConn app.Connection
	deepConn   app.Connection
}

type scActionCounters struct {
	create  int
	update  int
	move    int
	delete  int
	connect int
}

func newSCCatalogFixture(t testing.TB) scCatalogFixture {
	t.Helper()
	root := testFolder("sc-root", "", "/", 1)
	empty := testFolder("sc-empty", root.ID, "/01-empty", 1)
	direct := testFolder("sc-direct", root.ID, "/02-direct", 1)
	nested := testFolder("sc-nested", root.ID, "/03-nested", 1)
	child := testFolder("sc-child", nested.ID, "/03-nested/child", 1)
	directConn := testConnection("sc-direct-connection", direct.ID, "/02-direct/direct-connection", 1)
	directConn.Host = "direct.example"
	deepConn := testConnection("sc-deep-connection", child.ID, "/03-nested/child/deep-connection", 1)
	deepConn.Host = "deep.example"

	snapshot := newCatalogSnapshot(root, 7)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{empty, direct, nested}}) ||
		!snapshot.addChildren(direct.ID, app.ListChildrenResult{Connections: []app.Connection{directConn}}) ||
		!snapshot.addChildren(nested.ID, app.ListChildrenResult{Folders: []app.Folder{child}}) ||
		!snapshot.addChildren(child.ID, app.ListChildrenResult{Connections: []app.Connection{deepConn}}) {
		t.Fatal("SC catalog fixture was rejected")
	}

	model := New(Config{Width: 160, Height: 24, NoColor: true})
	model.browser.setSnapshot(snapshot, "")
	for _, folder := range []app.Folder{direct, nested, child} {
		model.browser.expanded[folder.ID] = struct{}{}
	}
	model.browser.rebuildRows()
	model.ownedSelectionID = model.browser.selectedID
	model.syncDetail()
	return scCatalogFixture{model: model, root: root, empty: empty, direct: direct, nested: nested, child: child, directConn: directConn, deepConn: deepConn}
}

func TestSC001SelectionDetailActionSynchronizationTwentyRuns(t *testing.T) {
	type target struct {
		id         app.NodeID
		path       string
		actionKeys string
		want       []string
		absent     []string
	}
	for run := 1; run <= scConformanceRuns; run++ {
		fixture := newSCCatalogFixture(t)
		model := fixture.model
		targets := []target{
			{fixture.root.ID, fixture.root.Path, "n/f/r/?/q", []string{"Kind: Root", "Direct connections: 0", detailEmptyConnections}, []string{"direct.example", "deep.example"}},
			{fixture.empty.ID, fixture.empty.Path, "n/f/e/m/d/r/?/q", []string{"Kind: Folder", "Direct connections: 0", detailEmptyConnections}, []string{"direct.example", "deep.example"}},
			{fixture.direct.ID, fixture.direct.Path, "n/f/e/m/d/r/?/q", []string{"Kind: Folder", "Direct connections: 1", "direct-connection: direct.example:22"}, []string{"deep.example"}},
			{fixture.directConn.ID, fixture.directConn.Path, "c/n/f/e/m/d/r/?/q", []string{"Kind: Connection", "Endpoint: direct.example:22", "Method: agent"}, []string{"deep.example", detailEmptyConnections}},
			{fixture.nested.ID, fixture.nested.Path, "n/f/e/m/d/r/?/q", []string{"Kind: Folder", "Direct connections: 0", detailEmptyConnections}, []string{"deep.example"}},
			{fixture.child.ID, fixture.child.Path, "n/f/e/m/d/r/?/q", []string{"Kind: Folder", "Direct connections: 1", "deep-connection: deep.example:22"}, []string{"direct.example"}},
			{fixture.deepConn.ID, fixture.deepConn.Path, "c/n/f/e/m/d/r/?/q", []string{"Kind: Connection", "Endpoint: deep.example:22", "Method: agent"}, []string{"direct.example", detailEmptyConnections}},
		}

		updateModel(model, keyPress("g"))
		previousDetailLine := ""
		for index, target := range targets {
			if index != 0 {
				updateModel(model, keyPress("j"))
			}
			view := model.View().Content
			if model.browser.selectedID != target.id || model.detailState.targetID != target.id {
				t.Fatalf("run %d target %q: selection/detail = %q/%q", run, target.id, model.browser.selectedID, model.detailState.targetID)
			}
			if got := actionKeys(objectActions(model.actionContext())); got != target.actionKeys {
				t.Fatalf("run %d target %q: actions = %q, want %q", run, target.id, got, target.actionKeys)
			}
			if !strings.Contains(view, "Path: "+target.path) {
				t.Fatalf("run %d target %q: first frame omitted synchronized path %q:\n%s", run, target.id, target.path, view)
			}
			currentDetailLine := "Path: " + target.path
			if !scDetailFrameContains(model, view, currentDetailLine) {
				t.Fatalf("run %d target %q: Details omitted exact line %q", run, target.id, currentDetailLine)
			}
			for _, want := range target.want {
				if !scDetailFrameContains(model, view, want) {
					t.Fatalf("run %d target %q: Details omitted %q", run, target.id, want)
				}
			}
			for _, absent := range target.absent {
				if scDetailFrameContains(model, view, absent) || strings.Contains(strings.Join(model.detailState.content(160), "\n"), absent) {
					t.Fatalf("run %d target %q: Details retained non-contextual data %q", run, target.id, absent)
				}
			}
			if previousDetailLine != "" && scDetailFrameContains(model, view, previousDetailLine) {
				t.Fatalf("run %d target %q: first frame retained previous Details line %q", run, target.id, previousDetailLine)
			}
			previousDetailLine = currentDetailLine
			assertSCNoANSI(t, view)
		}
	}
}

func TestSC002DisplayedActionTerminalOutcomesAndAliasesTwentyRuns(t *testing.T) {
	type actionCase struct {
		name string
		key  tea.KeyPressMsg
		end  string
	}
	contexts := []struct {
		name      string
		selection func(scCatalogFixture) app.NodeID
		actions   []actionCase
	}{
		{name: "root", selection: func(f scCatalogFixture) app.NodeID { return f.root.ID }, actions: []actionCase{
			{name: "new_connection", key: keyPress("n"), end: "form_cancel"},
			{name: "new_folder", key: keyPress("f"), end: "modal_cancel"},
			{name: "reload", key: keyPress("r"), end: "reload"},
			{name: "help_close_question", key: keyPress("?"), end: "help_question"},
			{name: "help_close_escape", key: keyPress("?"), end: "help_escape"},
			{name: "quit_q", key: keyPress("q"), end: "quit"},
			{name: "quit_ctrl_c", key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), end: "quit"},
		}},
		{name: "folder", selection: func(f scCatalogFixture) app.NodeID { return f.direct.ID }, actions: []actionCase{
			{name: "new_connection", key: keyPress("n"), end: "form_cancel"},
			{name: "new_folder", key: keyPress("f"), end: "modal_cancel"},
			{name: "edit", key: keyPress("e"), end: "modal_cancel"},
			{name: "move", key: keyPress("m"), end: "modal_cancel"},
			{name: "delete_enter_cancel", key: keyPress("d"), end: "delete_enter"},
			{name: "delete_escape_cancel", key: keyPress("d"), end: "delete_escape"},
			{name: "reload", key: keyPress("r"), end: "reload"},
			{name: "help_close_question", key: keyPress("?"), end: "help_question"},
			{name: "help_close_escape", key: keyPress("?"), end: "help_escape"},
			{name: "quit_q", key: keyPress("q"), end: "quit"},
			{name: "quit_ctrl_c", key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), end: "quit"},
		}},
		{name: "connection", selection: func(f scCatalogFixture) app.NodeID { return f.directConn.ID }, actions: []actionCase{
			{name: "connect_enter_cancel", key: keyPress("c"), end: "connect_enter"},
			{name: "connect_escape_cancel", key: keyPress("c"), end: "connect_escape"},
			{name: "new_connection_parent", key: keyPress("n"), end: "form_cancel"},
			{name: "new_folder_parent", key: keyPress("f"), end: "modal_cancel"},
			{name: "edit", key: keyPress("e"), end: "form_cancel"},
			{name: "move", key: keyPress("m"), end: "modal_cancel"},
			{name: "delete_enter_cancel", key: keyPress("d"), end: "delete_enter"},
			{name: "delete_escape_cancel", key: keyPress("d"), end: "delete_escape"},
			{name: "reload", key: keyPress("r"), end: "reload"},
			{name: "help_close_question", key: keyPress("?"), end: "help_question"},
			{name: "help_close_escape", key: keyPress("?"), end: "help_escape"},
			{name: "quit_q", key: keyPress("q"), end: "quit"},
			{name: "quit_ctrl_c", key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), end: "quit"},
		}},
	}

	for _, contextCase := range contexts {
		for _, action := range contextCase.actions {
			t.Run(contextCase.name+"/"+action.name, func(t *testing.T) {
				for run := range scConformanceRuns {
					fixture := newSCCatalogFixture(t)
					model := fixture.model
					selected := contextCase.selection(fixture)
					selectSCNode(model, selected)
					counters := &scActionCounters{}
					model.folders = scFixtureFolders(fixture, counters)
					model.connections = scFixtureConnections(fixture, counters)
					model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
						counters.connect++
						return app.ConnectResult{}, app.ErrInvalidRequest
					})

					command := sc009Update(t, model, action.key)
					switch action.end {
					case "form_cancel":
						if command != nil || model.form == nil {
							t.Fatalf("run %d: form action did not open", run)
						}
						sc009Update(t, model, keyPress("esc"))
					case "modal_cancel":
						if command != nil || !model.modal.isOpen() {
							t.Fatalf("run %d: modal action did not open", run)
						}
						sc009Update(t, model, keyPress("esc"))
					case "delete_enter", "delete_escape":
						sc009RunCommand(t, model, command)
						if !model.modal.isOpen() {
							t.Fatalf("run %d: delete did not reach confirmation", run)
						}
						cancelKey := "enter"
						if action.end == "delete_escape" {
							cancelKey = "esc"
						}
						sc009Update(t, model, keyPress(cancelKey))
					case "connect_enter", "connect_escape":
						cancelKey := "enter"
						if action.end == "connect_escape" {
							cancelKey = "esc"
						}
						sc009Update(t, model, keyPress(cancelKey))
					case "reload":
						sc009RunCommand(t, model, command)
					case "help_question", "help_escape":
						closeKey := "?"
						if action.end == "help_escape" {
							closeKey = "esc"
						}
						sc009Update(t, model, keyPress(closeKey))
					case "quit":
						if command == nil {
							t.Fatalf("run %d: Quit alias returned no command", run)
						}
						if _, ok := command().(tea.QuitMsg); !ok {
							t.Fatalf("run %d: Quit alias command returned %T", run, command())
						}
					}

					if action.end != "quit" && (model.form != nil || model.modal.isOpen() || model.operation != nil || model.browser.selectedID != selected || model.focusOwner != focusOwnerTree) {
						t.Fatalf("run %d: terminal outcome retained owner or changed context: form=%v modal=%v operation=%v selection=%q focus=%v", run, model.form != nil, model.modal.isOpen(), model.operation != nil, model.browser.selectedID, model.focusOwner)
					}
					if counters.create+counters.update+counters.move+counters.delete+counters.connect != 0 {
						t.Fatalf("run %d: cancel/close outcome mutated or started network: %#v", run, counters)
					}
				}
			})
		}
	}
}

func TestSC002DisplayedNavigationAndFormAliasesTwentyRuns(t *testing.T) {
	treeCases := []struct {
		name  string
		start func(scCatalogFixture) app.NodeID
		keys  []tea.KeyPressMsg
		want  func(scCatalogFixture) app.NodeID
	}{
		{name: "up", start: func(f scCatalogFixture) app.NodeID { return f.direct.ID }, keys: []tea.KeyPressMsg{keyPress("k"), tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})}, want: func(f scCatalogFixture) app.NodeID { return f.empty.ID }},
		{name: "down", start: func(f scCatalogFixture) app.NodeID { return f.empty.ID }, keys: []tea.KeyPressMsg{keyPress("j"), tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})}, want: func(f scCatalogFixture) app.NodeID { return f.direct.ID }},
		{name: "home", start: func(f scCatalogFixture) app.NodeID { return f.deepConn.ID }, keys: []tea.KeyPressMsg{keyPress("g"), tea.KeyPressMsg(tea.Key{Code: tea.KeyHome})}, want: func(f scCatalogFixture) app.NodeID { return f.root.ID }},
		{name: "end", start: func(f scCatalogFixture) app.NodeID { return f.root.ID }, keys: []tea.KeyPressMsg{keyPress("G"), tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd})}, want: func(f scCatalogFixture) app.NodeID { return f.deepConn.ID }},
		{name: "left_parent", start: func(f scCatalogFixture) app.NodeID { return f.directConn.ID }, keys: []tea.KeyPressMsg{keyPress("h"), tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft})}, want: func(f scCatalogFixture) app.NodeID { return f.direct.ID }},
		{name: "right_child", start: func(f scCatalogFixture) app.NodeID { return f.direct.ID }, keys: []tea.KeyPressMsg{keyPress("l"), tea.KeyPressMsg(tea.Key{Code: tea.KeyRight})}, want: func(f scCatalogFixture) app.NodeID { return f.directConn.ID }},
	}
	for _, test := range treeCases {
		for alias, pressed := range test.keys {
			t.Run("tree/"+test.name+"/alias_"+string(rune('1'+alias)), func(t *testing.T) {
				for run := range scConformanceRuns {
					fixture := newSCCatalogFixture(t)
					selectSCNode(fixture.model, test.start(fixture))
					if !scNavigationDescriptorDisplayed(fixture.model, pressed) {
						t.Fatalf("run %d: navigation alias %q was not displayed", run, pressed.String())
					}
					sc009Update(t, fixture.model, pressed)
					want := test.want(fixture)
					if fixture.model.browser.selectedID != want || fixture.model.detailState.targetID != want || fixture.model.focusOwner != focusOwnerTree {
						t.Fatalf("run %d: alias %q selection/detail/focus=%q/%q/%v, want %q", run, pressed.String(), fixture.model.browser.selectedID, fixture.model.detailState.targetID, fixture.model.focusOwner, want)
					}
				}
			})
		}
	}

	for _, toggle := range []tea.KeyPressMsg{keyPress("enter"), keyPress(" ")} {
		t.Run("tree/toggle/"+toggle.String(), func(t *testing.T) {
			for run := range scConformanceRuns {
				fixture := newSCCatalogFixture(t)
				selectSCNode(fixture.model, fixture.nested.ID)
				if !scNavigationDescriptorDisplayed(fixture.model, toggle) {
					t.Fatalf("run %d: toggle alias %q was not displayed", run, toggle.String())
				}
				sc009Update(t, fixture.model, toggle)
				if _, expanded := fixture.model.browser.expanded[fixture.nested.ID]; expanded || fixture.model.browser.selectedID != fixture.nested.ID {
					t.Fatalf("run %d: toggle alias %q did not collapse selected folder", run, toggle.String())
				}
			}
		})
	}

	for _, pressed := range []tea.KeyPressMsg{keyPress("tab"), keyPress("shift+tab")} {
		t.Run("tree/focus_details/"+pressed.String(), func(t *testing.T) {
			for run := range scConformanceRuns {
				fixture := newSCCatalogFixture(t)
				selectSCNode(fixture.model, fixture.directConn.ID)
				if !scNavigationDescriptorDisplayed(fixture.model, pressed) {
					t.Fatalf("run %d: focus alias %q was not displayed", run, pressed.String())
				}
				sc009Update(t, fixture.model, pressed)
				if fixture.model.focusOwner != focusOwnerDetail || fixture.model.browser.selectedID != fixture.directConn.ID {
					t.Fatalf("run %d: focus alias %q changed target or missed Details", run, pressed.String())
				}
			}
		})
	}

	detailScroll := []struct {
		name string
		keys []tea.KeyPressMsg
	}{
		{name: "up", keys: []tea.KeyPressMsg{keyPress("k"), tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})}},
		{name: "down", keys: []tea.KeyPressMsg{keyPress("j"), tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})}},
		{name: "home", keys: []tea.KeyPressMsg{keyPress("g"), tea.KeyPressMsg(tea.Key{Code: tea.KeyHome})}},
		{name: "end", keys: []tea.KeyPressMsg{keyPress("G"), tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd})}},
	}
	for _, test := range detailScroll {
		for alias, pressed := range test.keys {
			t.Run("details/"+test.name+"/alias_"+string(rune('1'+alias)), func(t *testing.T) {
				for run := range scConformanceRuns {
					fixture := newSCCatalogFixture(t)
					selectSCNode(fixture.model, fixture.directConn.ID)
					sc009Update(t, fixture.model, keyPress("tab"))
					if !scNavigationDescriptorDisplayed(fixture.model, pressed) {
						t.Fatalf("run %d: detail alias %q was not displayed", run, pressed.String())
					}
					selected := fixture.model.browser.selectedID
					sc009Update(t, fixture.model, pressed)
					if fixture.model.browser.selectedID != selected || fixture.model.detailState.targetID != selected || fixture.model.focusOwner != focusOwnerDetail {
						t.Fatalf("run %d: detail alias %q changed target/focus", run, pressed.String())
					}
				}
			})
		}
	}

	for _, pressed := range []tea.KeyPressMsg{keyPress("tab"), keyPress("shift+tab")} {
		t.Run("details/focus_tree/"+pressed.String(), func(t *testing.T) {
			for run := range scConformanceRuns {
				fixture := newSCCatalogFixture(t)
				selectSCNode(fixture.model, fixture.directConn.ID)
				sc009Update(t, fixture.model, keyPress("tab"))
				sc009Update(t, fixture.model, pressed)
				if fixture.model.focusOwner != focusOwnerTree || fixture.model.browser.selectedID != fixture.directConn.ID {
					t.Fatalf("run %d: Details focus alias %q changed target or missed Tree", run, pressed.String())
				}
			}
		})
	}

	for _, test := range []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		{name: "f1_help", key: tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}), want: "help"},
		{name: "ctrl_c_quit", key: tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), want: "quit"},
		{name: "q_text", key: keyPress("q"), want: "text"},
		{name: "question_text", key: keyPress("?"), want: "text"},
	} {
		t.Run("form/"+test.name, func(t *testing.T) {
			for run := range scConformanceRuns {
				fixture := newSCCatalogFixture(t)
				selectSCNode(fixture.model, fixture.directConn.ID)
				sc009Update(t, fixture.model, keyPress("e"))
				form := fixture.model.form
				beforeText := form.inputs[fieldName].Value()
				command := sc009Update(t, fixture.model, test.key)
				if test.want == "help" {
					if command != nil || fixture.model.modal.kind != modalKindHelp || fixture.model.form != form || fixture.model.focusOwner != focusOwnerModal || fixture.model.modal.openedFrom != focusOwnerConnectionForm {
						t.Fatalf("run %d: F1 did not open standalone form Help", run)
					}
					sc009Update(t, fixture.model, keyPress("esc"))
					if fixture.model.modal.isOpen() || fixture.model.form != form || fixture.model.focusOwner != focusOwnerConnectionForm {
						t.Fatalf("run %d: Esc did not restore form from Help", run)
					}
				} else if test.want == "quit" && command == nil {
					t.Fatalf("run %d: clean form Ctrl+C did not quit", run)
				} else if test.want == "text" {
					if command != nil || fixture.model.form != form || fixture.model.modal.isOpen() || form.inputs[fieldName].Value() != beforeText+test.key.String() {
						t.Fatalf("run %d: printable command %q escaped focused text field", run, test.key.String())
					}
				}
			}
		})
	}
}

func TestSC002DisplayedAndAbsentActionDispatchTwentyRuns(t *testing.T) {
	type dispatchCase struct {
		id     actionID
		key    string
		assert func(*testing.T, *Model, tea.Cmd)
	}
	commandStarted := func(t *testing.T, model *Model, command tea.Cmd) {
		t.Helper()
		if command == nil || model.operation == nil {
			t.Fatalf("action did not start its deferred operation: command=%v operation=%#v", command != nil, model.operation)
		}
	}
	quitStarted := func(t *testing.T, _ *Model, command tea.Cmd) {
		t.Helper()
		if command == nil {
			t.Fatal("Quit did not return a command")
		}
		if _, ok := command().(tea.QuitMsg); !ok {
			t.Fatalf("Quit command returned %T", command())
		}
	}
	formOpened := func(t *testing.T, model *Model, command tea.Cmd) {
		t.Helper()
		if command != nil || model.screen != screenConnectionForm || model.form == nil || model.focusOwner != focusOwnerConnectionForm {
			t.Fatalf("connection form dispatch = command %v screen %v form %p focus %v", command != nil, model.screen, model.form, model.focusOwner)
		}
	}
	modalOpened := func(kind modalKind) func(*testing.T, *Model, tea.Cmd) {
		return func(t *testing.T, model *Model, command tea.Cmd) {
			t.Helper()
			if command != nil || model.modal.kind != kind || model.focusOwner != focusOwnerModal {
				t.Fatalf("modal dispatch = command %v kind %v focus %v, want %v", command != nil, model.modal.kind, model.focusOwner, kind)
			}
		}
	}

	contexts := []struct {
		name      string
		selection func(scCatalogFixture) app.NodeID
		shown     []dispatchCase
		absent    []dispatchCase
	}{
		{
			name: "root", selection: func(f scCatalogFixture) app.NodeID { return f.root.ID },
			shown: []dispatchCase{
				{actionNewConnection, "n", formOpened}, {actionNewFolder, "f", modalOpened(modalKindFolderCreate)},
				{actionReload, "r", commandStarted}, {actionHelp, "?", modalOpened(modalKindHelp)}, {actionQuit, "q", quitStarted},
			},
			absent: []dispatchCase{{actionConnect, "c", nil}, {actionEdit, "e", nil}, {actionMove, "m", nil}, {actionDelete, "d", nil}},
		},
		{
			name: "folder", selection: func(f scCatalogFixture) app.NodeID { return f.direct.ID },
			shown: []dispatchCase{
				{actionNewConnection, "n", formOpened}, {actionNewFolder, "f", modalOpened(modalKindFolderCreate)},
				{actionEdit, "e", modalOpened(modalKindFolderEdit)}, {actionMove, "m", modalOpened(modalKindMovePicker)},
				{actionDelete, "d", commandStarted}, {actionReload, "r", commandStarted},
				{actionHelp, "?", modalOpened(modalKindHelp)}, {actionQuit, "q", quitStarted},
			},
			absent: []dispatchCase{{actionConnect, "c", nil}},
		},
		{
			name: "connection", selection: func(f scCatalogFixture) app.NodeID { return f.directConn.ID },
			shown: []dispatchCase{
				{actionConnect, "c", modalOpened(modalKindConnectConfirmation)}, {actionNewConnection, "n", formOpened},
				{actionNewFolder, "f", modalOpened(modalKindFolderCreate)}, {actionEdit, "e", formOpened},
				{actionMove, "m", modalOpened(modalKindMovePicker)}, {actionDelete, "d", commandStarted},
				{actionReload, "r", commandStarted}, {actionHelp, "?", modalOpened(modalKindHelp)}, {actionQuit, "q", quitStarted},
			},
		},
	}

	for _, context := range contexts {
		for run := 1; run <= scConformanceRuns; run++ {
			for _, action := range context.shown {
				fixture := newSCCatalogFixture(t)
				selectSCNode(fixture.model, context.selection(fixture))
				view := fixture.model.View().Content
				descriptor, present := scDescriptor(action.id, objectActions(fixture.model.actionContext()))
				if !present || !strings.Contains(view, descriptor.text()) {
					t.Fatalf("%s run %d: displayed action %s missing from inventory/frame", context.name, run, action.id)
				}
				_, command := fixture.model.Update(keyPress(action.key))
				action.assert(t, fixture.model, command)
			}

			for _, action := range context.absent {
				fixture := newSCCatalogFixture(t)
				selectSCNode(fixture.model, context.selection(fixture))
				before := scInteractionSnapshot(fixture.model)
				descriptor, known := scKnownObjectDescriptor(action.id)
				if _, present := scDescriptor(action.id, objectActions(fixture.model.actionContext())); present || known && strings.Contains(fixture.model.View().Content, descriptor.text()) {
					t.Fatalf("%s run %d: absent action %s was displayed", context.name, run, action.id)
				}
				_, command := fixture.model.Update(keyPress(action.key))
				if command != nil || !reflect.DeepEqual(scInteractionSnapshot(fixture.model), before) {
					t.Fatalf("%s run %d: absent action %s changed state", context.name, run, action.id)
				}
			}
		}
	}
}

func TestSC003CreateEditConditionalFormTraversalTwentyRuns(t *testing.T) {
	authCases := []struct {
		method app.AuthMethod
		order  []connectionField
	}{
		{app.AuthMethodAgent, []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldSave}},
		{app.AuthMethodKey, []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldIdentity, fieldSave}},
		{app.AuthMethodPassword, []connectionField{fieldName, fieldHost, fieldPort, fieldUsername, fieldAuth, fieldRemember, fieldSave}},
	}
	flows := []struct {
		name      string
		selection func(scCatalogFixture) app.NodeID
		key       string
	}{
		{"create", func(f scCatalogFixture) app.NodeID { return f.direct.ID }, "n"},
		{"edit", func(f scCatalogFixture) app.NodeID { return f.directConn.ID }, "e"},
	}

	for _, flow := range flows {
		for _, auth := range authCases {
			t.Run(flow.name+"/"+string(auth.method), func(t *testing.T) {
				for run := 1; run <= scConformanceRuns; run++ {
					fixture := newSCCatalogFixture(t)
					model := fixture.model
					selected := flow.selection(fixture)
					selectSCNode(model, selected)
					updateModel(model, keyPress(flow.key))
					if model.form == nil || model.form.focusedField() != fieldName || model.modal.isOpen() {
						t.Fatalf("run %d: %s %s did not begin in Details/Name", run, flow.name, auth.method)
					}
					model.form.inputs[fieldAuth].SetValue(string(auth.method))
					model.form.baseline[fieldAuth] = string(auth.method)
					model.form.syncDependencies()

					forward := []connectionField{model.form.focusedField()}
					assertSCFocusedFormField(t, model, forward[0])
					for range len(auth.order) - 1 {
						updateModel(model, keyPress("tab"))
						forward = append(forward, model.form.focusedField())
						assertSCFocusedFormField(t, model, model.form.focusedField())
					}
					if !slices.Equal(forward, auth.order) {
						t.Fatalf("run %d: forward order = %v, want %v", run, forward, auth.order)
					}

					wantReverse := append([]connectionField(nil), auth.order...)
					slices.Reverse(wantReverse)
					for _, reverseKey := range []tea.KeyPressMsg{keyPress("shift+tab"), tea.KeyPressMsg(tea.Key{Code: tea.KeyF2})} {
						model.form.setFocus(fieldSave)
						reverse := []connectionField{model.form.focusedField()}
						for range len(auth.order) - 1 {
							updateModel(model, reverseKey)
							reverse = append(reverse, model.form.focusedField())
							assertSCFocusedFormField(t, model, model.form.focusedField())
						}
						if !slices.Equal(reverse, wantReverse) {
							t.Fatalf("run %d: reverse alias %q order = %v, want %v", run, reverseKey.String(), reverse, wantReverse)
						}
					}

					updateModel(model, keyPress("esc"))
					if model.screen != screenBrowser || model.form != nil || model.modal.isOpen() || model.browser.selectedID != selected || model.browser.revision != 7 {
						t.Fatalf("run %d: cancel did not restore unchanged %s context", run, flow.name)
					}
				}
			})
		}
	}
}

func TestSC003SaveFailuresPreserveAllFormValuesTwentyRuns(t *testing.T) {
	failures := []struct {
		name string
		err  error
	}{
		{name: "persistence", err: errors.New("controlled persistence failure")},
		{name: "conflict", err: app.ErrConflict},
	}
	for _, flow := range []string{"create", "edit"} {
		for _, failure := range failures {
			t.Run(flow+"/"+failure.name, func(t *testing.T) {
				for run := 1; run <= scConformanceRuns; run++ {
					fixture := newSCCatalogFixture(t)
					model := fixture.model
					model.connections = ConnectionFuncs{
						CreateFunc: func(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
							return app.ConnectionResult{}, failure.err
						},
						UpdateFunc: func(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
							return app.ConnectionResult{}, failure.err
						},
					}
					if flow == "create" {
						selectSCNode(model, fixture.direct.ID)
						updateModel(model, keyPress("n"))
						model.form.inputs[fieldName].SetValue("created")
					} else {
						selectSCNode(model, fixture.directConn.ID)
						updateModel(model, keyPress("e"))
						model.form.inputs[fieldName].SetValue("edited")
					}
					model.form.inputs[fieldHost].SetValue("preserved.example")
					model.form.inputs[fieldPort].SetValue("2202")
					model.form.inputs[fieldUsername].SetValue("preserved-user")
					model.form.inputs[fieldAuth].SetValue(string(app.AuthMethodAgent))
					model.form.inputs[fieldIdentity].SetValue("/hidden-but-preserved/key")
					model.form.remember = false
					model.form.setFocus(fieldHost)
					before := scAllFormValuesSnapshot(model)

					command := sc009Update(t, model, keyPress("ctrl+s"))
					if command == nil {
						t.Fatalf("run %d: %s Save did not start", run, flow)
					}
					sc009RunCommand(t, model, command)
					if got := scAllFormValuesSnapshot(model); !reflect.DeepEqual(got, before) {
						t.Fatalf("run %d: %s %s failure changed form values\n got: %#v\nwant: %#v", run, flow, failure.name, got, before)
					}
					if model.modal.isOpen() || model.operation != nil || model.focusOwner != focusOwnerConnectionForm {
						t.Fatalf("run %d: %s %s failure changed form ownership", run, flow, failure.name)
					}
					if failure.err == app.ErrConflict && flow == "edit" {
						if model.connectionEdit.conflict == nil || !strings.Contains(model.View().Content, "Save blocked") {
							t.Fatalf("run %d: %s conflict did not remain inline", run, flow)
						}
					} else if model.form.formError == "" || !strings.Contains(model.View().Content, "Error:") {
						t.Fatalf("run %d: %s persistence failure did not remain inline", run, flow)
					}
				}
			})
		}
	}

	for _, flow := range []string{"create", "edit"} {
		t.Run(flow+"/validation", func(t *testing.T) {
			for run := 1; run <= scConformanceRuns; run++ {
				fixture := newSCCatalogFixture(t)
				model := fixture.model
				if flow == "create" {
					selectSCNode(model, fixture.direct.ID)
					updateModel(model, keyPress("n"))
					model.form.inputs[fieldName].SetValue("created")
				} else {
					selectSCNode(model, fixture.directConn.ID)
					updateModel(model, keyPress("e"))
					model.form.inputs[fieldName].SetValue("edited")
				}
				model.form.inputs[fieldHost].SetValue("preserved.example")
				model.form.inputs[fieldPort].SetValue("0")
				model.form.inputs[fieldUsername].SetValue("preserved-user")
				model.form.inputs[fieldAuth].SetValue(string(app.AuthMethodKey))
				model.form.inputs[fieldIdentity].SetValue("/preserved/key")
				model.form.setFocus(fieldPort)
				before := scAllFormValuesSnapshot(model)
				if command := sc009Update(t, model, keyPress("ctrl+s")); command != nil {
					t.Fatalf("run %d: invalid %s Save started persistence", run, flow)
				}
				if got := scAllFormValuesSnapshot(model); !reflect.DeepEqual(got, before) || model.form.errors[fieldPort] == "" {
					t.Fatalf("run %d: invalid %s Save changed values or omitted field error", run, flow)
				}
			}
		})
	}
}

func TestSC003SuccessfulCreateEditTerminalOutcomesTwentyRuns(t *testing.T) {
	for _, flow := range []string{"create", "edit"} {
		t.Run(flow, func(t *testing.T) {
			for run := range scConformanceRuns {
				original := testConnection("original", syntheticRootID, "/original", 3)
				connections := []app.Connection{original}
				mutations := 0
				service := ConnectionFuncs{
					ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
						return app.ListConnectionsResult{Connections: append([]app.Connection(nil), connections...), CatalogRevision: app.CatalogRevision(8 + mutations)}, nil
					},
					CreateFunc: func(_ context.Context, request app.CreateConnectionRequest) (app.ConnectionResult, error) {
						mutations++
						if request.Parent.Path != "/" || request.Name != "created" || request.Host != "created.test" {
							t.Fatalf("run %d: create request = %#v", run, request)
						}
						created := testConnection("created", syntheticRootID, "/created", 1)
						created.Host = request.Host
						connections = append(connections, created)
						return app.ConnectionResult{Connection: created, CatalogRevision: 9}, nil
					},
					UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
						mutations++
						if request.Connection.ID != original.ID || request.Expected == nil || *request.Expected != original.Revision || request.Host == nil || *request.Host != "edited.test" {
							t.Fatalf("run %d: update request = %#v", run, request)
						}
						updated := original
						updated.Host = *request.Host
						updated.Revision++
						connections = []app.Connection{updated}
						return app.ConnectionResult{Connection: updated, CatalogRevision: 9}, nil
					},
				}
				model := New(Config{Connections: service, Width: 80, Height: 24, NoColor: true})
				sc009RunCommand(t, model, model.Init())
				if flow == "create" {
					sc009Update(t, model, keyPress("n"))
					model.form.inputs[fieldName].SetValue("created")
					model.form.inputs[fieldHost].SetValue("created.test")
				} else {
					selectSCNode(model, original.ID)
					sc009Update(t, model, keyPress("e"))
					model.form.inputs[fieldHost].SetValue("edited.test")
				}

				reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("ctrl+s")))
				sc009RunCommand(t, model, reload)
				wantID, wantHost := app.NodeID("created"), "created.test"
				if flow == "edit" {
					wantID, wantHost = original.ID, "edited.test"
				}
				selected := model.browser.selection()
				if mutations != 1 || model.form != nil || model.modal.isOpen() || model.operation != nil || model.browser.selectedID != wantID || model.detailState.targetID != wantID || selected == nil || selected.Host != wantHost || model.focusOwner != focusOwnerTree {
					t.Fatalf("run %d: %s terminal result mutations=%d selection=%q detail=%q host=%v form=%v modal=%v operation=%v focus=%v", run, flow, mutations, model.browser.selectedID, model.detailState.targetID, selected, model.form != nil, model.modal.isOpen(), model.operation != nil, model.focusOwner)
				}
				assertSCNoANSI(t, model.View().Content)
			}
		})
	}
}

func TestSC003EnterActivatesFocusedSaveTwentyRuns(t *testing.T) {
	for run := range scConformanceRuns {
		connection := testConnection("connection", syntheticRootID, "/connection", 3)
		current := connection
		updates := 0
		model := New(Config{Width: 80, Height: 24, NoColor: true, Connections: ConnectionFuncs{
			ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				return app.ListConnectionsResult{Connections: []app.Connection{current}, CatalogRevision: app.CatalogRevision(8 + updates)}, nil
			},
			UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
				updates++
				if request.Connection.ID != connection.ID || request.Host == nil || *request.Host != "enter-save.test" {
					t.Fatalf("run %d: Enter Save request = %#v", run, request)
				}
				current.Host = *request.Host
				current.Revision++
				return app.ConnectionResult{Connection: current, CatalogRevision: 9}, nil
			},
		}})
		sc009RunCommand(t, model, model.Init())
		selectSCNode(model, connection.ID)
		sc009Update(t, model, keyPress("e"))
		model.form.inputs[fieldHost].SetValue("enter-save.test")
		model.form.setFocus(fieldSave)
		reload := sc009RunCommand(t, model, sc009Update(t, model, keyPress("enter")))
		sc009RunCommand(t, model, reload)
		selected := model.browser.selection()
		if updates != 1 || selected == nil || selected.ID != connection.ID || selected.Host != "enter-save.test" || model.form != nil || model.operation != nil || model.focusOwner != focusOwnerTree {
			t.Fatalf("run %d: Enter Save terminal result updates=%d selected=%#v form=%v operation=%v focus=%v", run, updates, selected, model.form != nil, model.operation != nil, model.focusOwner)
		}
	}
}

func TestSC004SC005ExactGeometryAndResizePreservationTwentyRuns(t *testing.T) {
	type stateCase struct {
		name  string
		setup func(scCatalogFixture) *Model
	}
	states := []stateCase{
		{"tree", func(f scCatalogFixture) *Model { selectSCNode(f.model, f.deepConn.ID); return f.model }},
		{"details", func(f scCatalogFixture) *Model {
			selectSCNode(f.model, f.direct.ID)
			updateModel(f.model, keyPress("tab"))
			return f.model
		}},
	}
	for _, flow := range []struct {
		name      string
		selection func(scCatalogFixture) app.NodeID
		key       string
	}{
		{"create", func(f scCatalogFixture) app.NodeID { return f.direct.ID }, "n"},
		{"edit", func(f scCatalogFixture) app.NodeID { return f.directConn.ID }, "e"},
	} {
		for _, method := range []app.AuthMethod{app.AuthMethodAgent, app.AuthMethodKey, app.AuthMethodPassword} {
			flow, method := flow, method
			states = append(states, stateCase{flow.name + "-" + string(method), func(f scCatalogFixture) *Model {
				selectSCNode(f.model, flow.selection(f))
				updateModel(f.model, keyPress(flow.key))
				f.model.form.inputs[fieldAuth].SetValue(string(method))
				f.model.form.baseline[fieldAuth] = string(method)
				f.model.form.syncDependencies()
				last := f.model.form.visibleFields()[len(f.model.form.visibleFields())-1]
				f.model.form.setFocus(last)
				return f.model
			}})
		}
	}

	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			model := state.setup(newSCCatalogFixture(t))
			preserved := scInteractionSnapshot(model)
			for run := 1; run <= scConformanceRuns; run++ {
				for _, size := range us5ContractSizes {
					updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
					layout := calculateLayout(size.width, size.height, model.focusedLayoutRegion())
					want := scExpectedLayout(size.width, size.height, model.focusedLayoutRegion())
					if layout != want {
						t.Fatalf("run %d %s: layout = %#v, want %#v", run, size.name, layout, want)
					}
					assertLayoutInvariants(t, layout)
					view := model.View().Content
					assertUS5FrameBounded(t, view, size.width, size.height)
					for _, title := range []string{"Tree", "Details", "Actions"} {
						if !strings.Contains(view, title) {
							t.Fatalf("run %d %s omitted %s", run, size.name, title)
						}
					}
					if model.screen == screenConnectionForm {
						assertSCFocusedFormField(t, model, model.form.focusedField())
						for _, safety := range []string{"Esc Cancel", "Ctrl+C Quit", "F1 Help", "Ctrl+S Save"} {
							if !strings.Contains(view, safety) {
								t.Fatalf("run %d %s omitted form priority %q", run, size.name, safety)
							}
						}
					} else if !strings.Contains(view, "> ") || !strings.Contains(view, "r Reload") || !strings.Contains(view, "q Quit") || !strings.Contains(view, "? Help") {
						t.Fatalf("run %d %s omitted selected row or browser safety controls", run, size.name)
					}
					if got := scInteractionSnapshot(model); !reflect.DeepEqual(got, preserved) {
						t.Fatalf("run %d %s changed interaction state", run, size.name)
					}
				}
			}

			for run := 1; run <= scConformanceRuns; run++ {
				for _, width := range []int{40, 60, 79, 80, 100, 160, 80, 79, 40} {
					updateModel(model, tea.WindowSizeMsg{Width: width, Height: 24})
					_ = model.View().Content
					if got := scInteractionSnapshot(model); !reflect.DeepEqual(got, preserved) {
						t.Fatalf("resize run %d at %dx24 changed interaction state", run, width)
					}
				}
			}
			for run := 1; run <= scConformanceRuns; run++ {
				for _, size := range []us5Size{{"80x12", 80, 12}, {"80x24", 80, 24}, {"100x20", 100, 20}, {"100x30", 100, 30}} {
					updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
					_ = model.View().Content
					if got := scInteractionSnapshot(model); !reflect.DeepEqual(got, preserved) {
						t.Fatalf("wide-short run %d at %s changed interaction state", run, size.name)
					}
				}
			}
		})
	}
}

func TestSC006NoColorPrincipalBrowserAndConnectionFormFlowsTwentyRuns(t *testing.T) {
	for run := 1; run <= scConformanceRuns; run++ {
		fixture := newSCCatalogFixture(t)
		model := fixture.model
		updateModel(model, tea.WindowSizeMsg{Width: 80, Height: 24})
		for _, id := range []app.NodeID{fixture.root.ID, fixture.empty.ID, fixture.direct.ID, fixture.directConn.ID, fixture.nested.ID, fixture.deepConn.ID} {
			selectSCNode(model, id)
			view := model.View().Content
			assertSCNoANSI(t, view)
			for _, cue := range []string{"[*] Tree", "[ ] Details", "[ ] Actions", "> ", "Path:"} {
				if !strings.Contains(view, cue) {
					t.Fatalf("run %d node %q omitted no-color cue %q", run, id, cue)
				}
			}
		}

		selectSCNode(model, fixture.direct.ID)
		updateModel(model, keyPress("tab"))
		view := model.View().Content
		assertSCNoANSI(t, view)
		if !strings.Contains(view, "[*] Details") || !strings.Contains(view, "[ ] Tree") {
			t.Fatalf("run %d: Details focus lacked textual ownership", run)
		}
		updateModel(model, keyPress("tab"))

		updateModel(model, keyPress("n"))
		view = model.View().Content
		assertSCNoANSI(t, view)
		if !strings.Contains(view, "[*] Details") || !strings.Contains(view, ">   Name:") || !strings.Contains(view, "  * [ Save connection ]") {
			t.Fatalf("run %d: create form omitted focus/primary cues:\n%s", run, view)
		}
		updateModel(model, keyPress("ctrl+s"))
		view = model.View().Content
		assertSCNoANSI(t, view)
		if !strings.Contains(view, "Error:") || !strings.Contains(view, "!") {
			t.Fatalf("run %d: validation flow omitted textual error/invalid cues", run)
		}

		fixture = newSCCatalogFixture(t)
		model = fixture.model
		updateModel(model, tea.WindowSizeMsg{Width: 80, Height: 24})
		selectSCNode(model, fixture.direct.ID)
		updateModel(model, keyPress("n"))
		updateModel(model, keyPress("esc"))
		if model.screen != screenBrowser || model.browser.selectedID != fixture.direct.ID {
			t.Fatalf("run %d: clean create cancel did not restore browser", run)
		}
		selectSCNode(model, fixture.directConn.ID)
		updateModel(model, keyPress("e"))
		if model.modal.isOpen() || model.screen != screenConnectionForm {
			t.Fatalf("run %d: edit did not remain in Details", run)
		}
		assertSCNoANSI(t, model.View().Content)
		updateModel(model, keyPress("esc"))
		if model.screen != screenBrowser || model.browser.selectedID != fixture.directConn.ID {
			t.Fatalf("run %d: clean edit cancel did not restore browser", run)
		}
	}
}

type scStateSnapshot struct {
	selection    app.NodeID
	detailTarget app.NodeID
	treeOffset   int
	detailOffset int
	focus        focusOwner
	screen       screen
	formFocus    connectionField
	formValues   []string
	formOffset   int
	expanded     []app.NodeID
}

type scFormValuesSnapshot struct {
	values      [fieldIdentity + 1]string
	remember    bool
	focus       connectionField
	target      capturedTarget
	selection   app.NodeID
	destination *app.Folder
	original    *app.Connection
}

func scAllFormValuesSnapshot(model *Model) scFormValuesSnapshot {
	snapshot := scFormValuesSnapshot{selection: model.browser.selectedID}
	if model.form == nil {
		return snapshot
	}
	for field := fieldName; field <= fieldIdentity; field++ {
		snapshot.values[field] = model.form.inputs[field].Value()
	}
	snapshot.remember = model.form.remember
	snapshot.focus = model.form.focusedField()
	if model.connectionEdit != nil {
		snapshot.target = model.connectionEdit.target.clone()
	}
	if model.form.destination != nil {
		copy := *model.form.destination
		snapshot.destination = &copy
	}
	if model.form.original != nil {
		copy := *model.form.original
		snapshot.original = &copy
	}
	return snapshot
}

func scInteractionSnapshot(model *Model) scStateSnapshot {
	expanded := make([]app.NodeID, 0, len(model.browser.expanded))
	for id := range model.browser.expanded {
		expanded = append(expanded, id)
	}
	slices.Sort(expanded)
	snapshot := scStateSnapshot{
		selection: model.browser.selectedID, detailTarget: model.detailState.targetID,
		treeOffset: model.browser.viewport.logicalOffset, detailOffset: model.detailState.viewport.logicalOffset,
		focus: model.focusOwner, screen: model.screen, expanded: expanded,
	}
	if model.form != nil {
		snapshot.formFocus = model.form.focusedField()
		snapshot.formOffset = model.form.viewport.logicalOffset
		snapshot.formValues = make([]string, fieldIdentity+1)
		for field := fieldName; field <= fieldIdentity; field++ {
			snapshot.formValues[field] = model.form.inputs[field].Value()
		}
	}
	return snapshot
}

func scDescriptor(id actionID, descriptors []actionDescriptor) (actionDescriptor, bool) {
	for _, descriptor := range descriptors {
		if descriptor.id == id {
			return descriptor, true
		}
	}
	return actionDescriptor{}, false
}

func scKnownObjectDescriptor(id actionID) (actionDescriptor, bool) {
	for _, inventory := range [][]actionDescriptor{rootActionDescriptors, folderActionDescriptors, connectionActionDescriptors} {
		if descriptor, found := scDescriptor(id, inventory); found {
			return descriptor, true
		}
	}
	return actionDescriptor{}, false
}

func scNavigationDescriptorDisplayed(model *Model, pressed tea.KeyPressMsg) bool {
	descriptors := focusedNavigationActions(model.actionContext())
	descriptor, ok := actionForKey(pressed, descriptors, model.keys)
	return ok && strings.Contains(model.View().Content, descriptor.text())
}

func selectSCNode(model *Model, id app.NodeID) {
	model.browser.selectedID = id
	model.browser.expandAncestors(id)
	model.browser.rebuildRows()
	model.ownedSelectionID = id
	model.syncDetail()
}

func scFixtureFolders(fixture scCatalogFixture, counters *scActionCounters) FolderFuncs {
	byID := map[app.NodeID]app.Folder{
		fixture.root.ID: fixture.root, fixture.empty.ID: fixture.empty, fixture.direct.ID: fixture.direct,
		fixture.nested.ID: fixture.nested, fixture.child.ID: fixture.child,
	}
	return FolderFuncs{
		GetFunc: func(_ context.Context, selector app.ItemSelector) (app.FolderResult, error) {
			if selector.Path == "/" {
				return app.FolderResult{Folder: fixture.root, CatalogRevision: 8}, nil
			}
			folder, ok := byID[selector.ID]
			if !ok {
				return app.FolderResult{}, app.ErrNotFound
			}
			return app.FolderResult{Folder: folder, CatalogRevision: 8}, nil
		},
		ListFunc: func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
			result := app.ListChildrenResult{CatalogRevision: 8}
			switch request.Folder.ID {
			case fixture.root.ID:
				result.Folders = []app.Folder{fixture.empty, fixture.direct, fixture.nested}
			case fixture.direct.ID:
				result.Connections = []app.Connection{fixture.directConn}
			case fixture.nested.ID:
				result.Folders = []app.Folder{fixture.child}
			case fixture.child.ID:
				result.Connections = []app.Connection{fixture.deepConn}
			}
			return result, nil
		},
		CreateFunc: func(context.Context, app.CreateFolderRequest) (app.FolderResult, error) {
			counters.create++
			return app.FolderResult{}, app.ErrInvalidRequest
		},
		RenameFunc: func(context.Context, app.RenameFolderRequest) (app.FolderResult, error) {
			counters.update++
			return app.FolderResult{}, app.ErrInvalidRequest
		},
		MoveFunc: func(context.Context, app.MoveFolderRequest) (app.FolderResult, error) {
			counters.move++
			return app.FolderResult{}, app.ErrInvalidRequest
		},
		DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.FolderDeleteScope, error) {
			folder, ok := byID[selector.ID]
			if !ok {
				return app.FolderDeleteScope{}, app.ErrNotFound
			}
			return app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1, Snapshot: []app.NodeRevision{{ID: folder.ID, Revision: folder.Revision}}}, nil
		},
		DeleteFunc: func(context.Context, app.DeleteFolderRequest) (app.DeleteFolderResult, error) {
			counters.delete++
			return app.DeleteFolderResult{}, app.ErrInvalidRequest
		},
	}
}

func scFixtureConnections(fixture scCatalogFixture, counters *scActionCounters) ConnectionFuncs {
	byID := map[app.NodeID]app.Connection{fixture.directConn.ID: fixture.directConn, fixture.deepConn.ID: fixture.deepConn}
	return ConnectionFuncs{
		CreateFunc: func(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error) {
			counters.create++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		UpdateFunc: func(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
			counters.update++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		MoveFunc: func(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error) {
			counters.move++
			return app.ConnectionResult{}, app.ErrInvalidRequest
		},
		DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
			connection, ok := byID[selector.ID]
			if !ok {
				return app.ConnectionDeleteScope{}, app.ErrNotFound
			}
			return app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision}, nil
		},
		DeleteFunc: func(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
			counters.delete++
			return app.DeleteConnectionResult{}, app.ErrInvalidRequest
		},
	}
}

func assertSCFocusedFormField(t testing.TB, model *Model, field connectionField) {
	t.Helper()
	view := model.View().Content
	if model.form == nil || model.form.focusedField() != field || !strings.Contains(view, ">") {
		t.Fatalf("focused form field %v is not visible", field)
	}
	label := formLabels[field]
	if field == fieldSave {
		label = "Save connection"
	}
	if !strings.Contains(view, label) {
		t.Fatalf("focused form field %v omitted label %q:\n%s", field, label, view)
	}
}

func assertSCNoANSI(t testing.TB, view string) {
	t.Helper()
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("no-color frame contains ANSI: %q", view)
	}
}

func scDetailFrameContains(model *Model, view, exact string) bool {
	layout := calculateLayout(model.width, model.height, model.focusedLayoutRegion())
	lines := strings.Split(view, "\n")
	for row := layout.details.y + 1; row < layout.details.bottom()-1 && row < len(lines); row++ {
		line := ansi.Cut(lines[row], layout.details.x, layout.details.right())
		line = strings.TrimPrefix(line, "│ ")
		line = strings.TrimSuffix(line, " │")
		if strings.TrimRight(line, " ") == exact {
			return true
		}
	}
	return false
}

func scExpectedLayout(width, height int, focus layoutRegion) layoutState {
	state := layoutState{width: width, height: height}
	if width < minimumLayoutWidth || height < minimumLayoutHeight {
		state.mode = layoutUndersized
		return state
	}
	state.reduced = width < wideLayoutWidth || height < completeLayoutHeight
	baseHeight := height - actionsOuterHeight
	state.actions = layoutRect{x: 0, y: baseHeight, width: width, height: actionsOuterHeight}
	if width >= wideLayoutWidth {
		state.mode = layoutWide
		baseWidth := width - wideGutterWidth
		treeWidth := baseWidth * 40 / 100
		state.tree = layoutRect{x: 0, y: 0, width: treeWidth, height: baseHeight}
		state.details = layoutRect{x: treeWidth + wideGutterWidth, y: 0, width: width - treeWidth - wideGutterWidth, height: baseHeight}
		return state
	}
	state.mode = layoutStacked
	treeHeight := baseHeight / 2
	if baseHeight%2 != 0 && focus != regionDetails {
		treeHeight++
	}
	state.tree = layoutRect{x: 0, y: 0, width: width, height: treeHeight}
	state.details = layoutRect{x: 0, y: treeHeight, width: width, height: baseHeight - treeHeight}
	return state
}
