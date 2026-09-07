package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

const operationConformanceRuns = 20

type operationConformanceFixture struct {
	model      *Model
	kind       asyncOperationKind
	owner      operationOwnerSurface
	id         uint64
	target     string
	connection app.Connection
	saved      app.Connection
	attempt    app.SSHAttemptTarget
	stable     operationConformanceSurface
}

type operationConformanceSurface struct {
	selectedID app.NodeID
	focus      focusOwner
	screen     screen
	form       *connectionForm
	formValues []string
	formFocus  connectionField
	modal      modalState
	revision   app.CatalogRevision
}

type operationConformanceState struct {
	surface     operationConformanceSurface
	view        string
	status      string
	operationID uint64
	operation   *operationState
}

func TestSC013OperationConformanceMatrix20Runs(t *testing.T) {
	kinds := []struct {
		name string
		kind asyncOperationKind
	}{
		{name: "initial_load", kind: asyncOperationInitialLoad},
		{name: "reload", kind: asyncOperationReload},
		{name: "save", kind: asyncOperationSave},
		{name: "ssh_start", kind: asyncOperationSSHStart},
	}
	outcomes := []string{"success", "failure", "cancel", "quit", "stale", "post_cancel", "confirmed_before_cleanup"}

	for _, kind := range kinds {
		for _, outcome := range outcomes {
			t.Run(kind.name+"/"+outcome, func(t *testing.T) {
				for run := range operationConformanceRuns {
					fixture := newOperationConformanceFixture(t, kind.kind)
					assertOperationConformanceLoading(t, fixture, run)
					assertOperationConformanceMutationSuppression(t, fixture, run)
					assertOperationConformanceOutcome(t, fixture, outcome, run)
				}
			})
		}
	}
}

func TestSC012OperationToConflictConformanceMatrix20Runs(t *testing.T) {
	operations := []struct {
		name string
		kind asyncOperationKind
	}{
		{name: "reload", kind: asyncOperationReload},
		{name: "save", kind: asyncOperationSave},
		{name: "ssh_start", kind: asyncOperationSSHStart},
	}
	conflicts := []struct {
		name string
		kind conflictType
		err  error
	}{
		{name: "missing", kind: conflictTypeMissing, err: app.ErrNotFound},
		{name: "revision_changed", kind: conflictTypeRevisionChanged, err: app.ErrConflict},
	}

	for _, operation := range operations {
		for _, conflict := range conflicts {
			t.Run(operation.name+"/"+conflict.name, func(t *testing.T) {
				for run := range operationConformanceRuns {
					fixture := newOperationConflictFixture(t, operation.kind, conflict.kind)
					assertOperationConformanceLoading(t, fixture, run)
					if got := actionKeys(fixture.model.currentActionDescriptors()); got != "Esc/?/q" {
						t.Fatalf("run %d: conflict escaped operation precedence: %q", run, got)
					}

					cleanupCalls := installOperationCleanupProbe(t, fixture, run)
					beforeCleanup := *cleanupCalls
					_, command := fixture.model.Update(operationConflictMessage(fixture, conflict.err))
					if command != nil {
						t.Fatalf("run %d: conflict completion returned an unexpected command", run)
					}
					if *cleanupCalls != beforeCleanup+1 {
						t.Fatalf("run %d: cleanup calls = %d, want %d", run, *cleanupCalls, beforeCleanup+1)
					}
					assertOperationReleased(t, fixture.model, run)

					published := operationPublishedConflict(fixture.model)
					if published == nil || published.kind != conflict.kind || !published.blocked || published.target.id != fixture.connection.ID || published.target.revision != fixture.connection.Revision {
						t.Fatalf("run %d: published conflict = %#v", run, published)
					}
					if got := actionKeys(fixture.model.currentActionDescriptors()); got != "r/b/Esc/?/q" {
						t.Fatalf("run %d: post-cleanup conflict controls = %q", run, got)
					}
					view := fixture.model.View().Content
					for _, want := range []string{"r Reload", "b Back", "Esc Cancel warning", "? Help", "q Quit"} {
						if !strings.Contains(view, want) {
							t.Fatalf("run %d: conflict frame omitted %q:\n%s", run, want, view)
						}
					}

					stable := captureOperationConformanceSurface(fixture.model)
					_, blocked := fixture.model.Update(keyPress("ctrl+s"))
					if blocked != nil || !reflect.DeepEqual(captureOperationConformanceSurface(fixture.model), stable) {
						t.Fatalf("run %d: conflict allowed persistence or changed its owner surface", run)
					}
					updateModel(fixture.model, keyPress("esc"))
					published = operationPublishedConflict(fixture.model)
					if published == nil || published.detailVisible || !published.blocked || actionKeys(fixture.model.currentActionDescriptors()) != "r/b/Esc/?/q" {
						t.Fatalf("run %d: Cancel warning removed conflict authority: %#v", run, published)
					}
				}
			})
		}
	}
}

func newOperationConformanceFixture(t *testing.T, kind asyncOperationKind) operationConformanceFixture {
	t.Helper()
	connection := testConnection("server", syntheticRootID, "/server", 4)
	connection.Host = "server.test"
	saved := connection
	saved.Host = "saved.test"
	saved.Revision++
	fixture := operationConformanceFixture{kind: kind, connection: connection, saved: saved}

	switch kind {
	case asyncOperationInitialLoad:
		fixture.model = New(Config{Width: 80, Height: 24, NoColor: true})
		fixture.owner = operationOwnerRoot
		fixture.target = "catalog"
		if command := fixture.model.Init(); command == nil {
			t.Fatal("initial load did not return a command")
		}
	case asyncOperationReload:
		fixture.model = loadedModel(t, []app.Connection{connection}, true)
		updateModel(fixture.model, keyPress("l"))
		updateModel(fixture.model, keyPress("tab"))
		fixture.owner = operationOwnerRoot
		fixture.target = "catalog"
		if _, command := fixture.model.Update(keyPress("r")); command == nil {
			t.Fatal("reload did not return a command")
		}
	case asyncOperationSave:
		fixture.model = loadedModel(t, []app.Connection{connection}, true)
		updateModel(fixture.model, keyPress("l"))
		updateModel(fixture.model, keyPress("e"))
		fixture.model.form.inputs[fieldHost].SetValue(saved.Host)
		fixture.model.form.setFocus(fieldHost)
		fixture.owner = operationOwnerForm
		fixture.target = connection.Path
		if _, command := fixture.model.Update(keyPress("ctrl+s")); command == nil {
			t.Fatal("save did not return a command")
		}
	case asyncOperationSSHStart:
		fixture.model = loadedModel(t, []app.Connection{connection}, true)
		updateModel(fixture.model, keyPress("l"))
		updateModel(fixture.model, keyPress("c"))
		fixture.owner = operationOwnerModal
		fixture.target = "server.test:22"
		fixture.attempt = app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
		if _, command := fixture.model.Update(keyPress("y")); command == nil {
			t.Fatal("SSH start did not return a command")
		}
	default:
		t.Fatalf("unsupported operation kind %d", kind)
	}

	if fixture.model.operation == nil {
		t.Fatal("operation did not retain ownership")
	}
	fixture.id = fixture.model.operation.id
	fixture.stable = captureOperationConformanceSurface(fixture.model)
	return fixture
}

func newOperationConflictFixture(t *testing.T, kind asyncOperationKind, conflictKind conflictType) operationConformanceFixture {
	t.Helper()
	fixture := newOperationConformanceFixture(t, kind)
	if kind != asyncOperationReload {
		return fixture
	}

	connection := fixture.connection
	model := loadedModel(t, []app.Connection{connection}, true)
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("e"))
	model.form.inputs[fieldHost].SetValue("pending.test")
	model.form.setFocus(fieldHost)
	model.installFormConflict(conflictKind)
	if _, command := model.Update(keyPress("r")); command == nil {
		t.Fatal("conflict reload did not return a command")
	}
	fixture.model = model
	fixture.owner = operationOwnerForm
	fixture.target = connection.Path
	fixture.id = model.operation.id
	fixture.stable = captureOperationConformanceSurface(model)
	return fixture
}

func assertOperationConformanceLoading(t *testing.T, fixture operationConformanceFixture, run int) {
	t.Helper()
	model := fixture.model
	if model.operation == nil || model.operation.id != fixture.id || model.operationID != fixture.id || model.operation.ctx == nil || model.operation.cancel == nil {
		t.Fatalf("run %d: operation ownership = operation %#v last=%d", run, model.operation, model.operationID)
	}
	if model.operation.kind != fixture.kind || model.operation.owner != fixture.owner || !model.operation.matchesResult(fixture.id, fixture.kind) {
		t.Fatalf("run %d: wrong operation owner: %#v", run, model.operation)
	}
	if fixture.target == "catalog" {
		if model.operation.target != nil {
			t.Fatalf("run %d: catalog operation captured target %#v", run, model.operation.target)
		}
	} else if model.operation.target == nil || operationLoadingTarget(*model.operation) != fixture.target {
		t.Fatalf("run %d: captured loading target = %#v, want %q", run, model.operation.target, fixture.target)
	}
	wantStatus := operationLoadingStatus(fixture.kind, fixture.target)
	if !strings.Contains(model.View().Content, wantStatus) {
		t.Fatalf("run %d: loading frame omitted %q", run, wantStatus)
	}
	if got := actionKeys(model.currentActionDescriptors()); got != "Esc/?/q" {
		t.Fatalf("run %d: operation controls = %q", run, got)
	}
	if kind := fixture.kind; kind == asyncOperationInitialLoad && len(model.browser.snapshot.nodes) != 1 {
		t.Fatalf("run %d: initial load did not retain the empty shell", run)
	}
}

func assertOperationConformanceMutationSuppression(t *testing.T, fixture operationConformanceFixture, run int) {
	t.Helper()
	for _, pressed := range []string{"n", "f", "e", "m", "d", "r", "c", "j", "k", "tab", "enter", "y", "b", "x", "ctrl+s"} {
		before := captureOperationConformanceState(fixture.model)
		_, command := fixture.model.Update(keyPress(pressed))
		if command != nil {
			t.Fatalf("run %d: key %q started a mutation during operation", run, pressed)
		}
		if after := captureOperationConformanceState(fixture.model); !reflect.DeepEqual(after, before) {
			t.Fatalf("run %d: key %q changed operation-owned state", run, pressed)
		}
	}
}

func assertOperationConformanceOutcome(t *testing.T, fixture operationConformanceFixture, outcome string, run int) {
	t.Helper()
	model := fixture.model
	switch outcome {
	case "success":
		cleanupCalls := installOperationCleanupProbe(t, fixture, run)
		before := *cleanupCalls
		_, command := model.Update(operationSuccessMessage(fixture, fixture.id))
		if *cleanupCalls != before+1 {
			t.Fatalf("run %d: success did not cross cleanup exactly once", run)
		}
		assertOperationCommitReconciled(t, fixture, command, false, run)
	case "failure":
		cleanupCalls := installOperationCleanupProbe(t, fixture, run)
		before := *cleanupCalls
		_, command := model.Update(operationFailureMessage(fixture, fixture.id))
		if command != nil || *cleanupCalls != before+1 {
			t.Fatalf("run %d: failure command=%v cleanup delta=%d", run, command != nil, *cleanupCalls-before)
		}
		assertOperationReleased(t, model, run)
		if model.browser.selectedID != fixture.stable.selectedID {
			t.Fatalf("run %d: failure changed selection", run)
		}
		if fixture.kind == asyncOperationSave {
			if model.form != fixture.stable.form || model.form.focusedField() != fixture.stable.formFocus || model.form.formError == "" {
				t.Fatalf("run %d: Save failure did not preserve focused form", run)
			}
		} else if !model.modal.isOpen() || model.focusOwner != focusOwnerModal || fixture.kind != asyncOperationSSHStart && model.modal.openedFrom != fixture.stable.focus || fixture.kind == asyncOperationSSHStart && model.modal.openedFrom != fixture.stable.modal.openedFrom {
			t.Fatalf("run %d: failure did not preserve opener context: %#v", run, model.modal)
		}
		assertOperationDuplicateInert(t, fixture, run)
	case "cancel":
		cleanupCalls := installOperationCleanupProbe(t, fixture, run)
		updateModel(model, keyPress("esc"))
		if model.operation == nil || model.operation.phase != asyncPhaseCancelRequested || *cleanupCalls != 1 {
			t.Fatalf("run %d: Cancel released before cleanup: %#v", run, model.operation)
		}
		_, command := model.Update(operationCanceledMessage(fixture, fixture.id))
		if command != nil || *cleanupCalls != 2 {
			t.Fatalf("run %d: Cancel completion command=%v cleanup=%d", run, command != nil, *cleanupCalls)
		}
		assertOperationRestored(t, fixture, run)
	case "quit":
		cleanupCalls := installOperationCleanupProbe(t, fixture, run)
		_, premature := model.Update(keyPress("q"))
		if premature != nil || model.operation == nil || !model.operation.quitIntent || *cleanupCalls != 1 {
			t.Fatalf("run %d: Quit did not wait for cleanup", run)
		}
		_, command := model.Update(operationCanceledMessage(fixture, fixture.id))
		if command == nil || *cleanupCalls != 2 {
			t.Fatalf("run %d: Quit was not released after cleanup", run)
		}
		assertOperationRestored(t, fixture, run)
	case "stale":
		before := captureOperationConformanceState(model)
		_, command := model.Update(operationSuccessMessage(fixture, fixture.id+100))
		if command != nil || !reflect.DeepEqual(captureOperationConformanceState(model), before) {
			t.Fatalf("run %d: stale owner result changed state", run)
		}
		updateModel(model, operationCanceledMessage(fixture, fixture.id))
		assertOperationRestored(t, fixture, run)
	case "post_cancel":
		cleanupCalls := installOperationCleanupProbe(t, fixture, run)
		updateModel(model, keyPress("esc"))
		before := captureOperationConformanceState(model)
		_, command := model.Update(operationSuccessMessage(fixture, fixture.id+1))
		if command != nil || !reflect.DeepEqual(captureOperationConformanceState(model), before) || *cleanupCalls != 1 {
			t.Fatalf("run %d: post-cancel non-owner result changed state", run)
		}
		updateModel(model, operationCanceledMessage(fixture, fixture.id))
		if *cleanupCalls != 2 {
			t.Fatalf("run %d: post-cancel cleanup calls = %d", run, *cleanupCalls)
		}
		assertOperationRestored(t, fixture, run)
	case "confirmed_before_cleanup":
		cleanupCalls := installOperationCleanupProbe(t, fixture, run)
		_, premature := model.Update(keyPress("q"))
		if premature != nil || *cleanupCalls != 1 {
			t.Fatalf("run %d: confirmed-result Quit did not wait", run)
		}
		_, command := model.Update(operationSuccessMessage(fixture, fixture.id))
		if *cleanupCalls != 2 {
			t.Fatalf("run %d: confirmed result did not clean owner", run)
		}
		assertOperationCommitReconciled(t, fixture, command, true, run)
	default:
		t.Fatalf("unknown outcome %q", outcome)
	}
}

func assertOperationCommitReconciled(t *testing.T, fixture operationConformanceFixture, command tea.Cmd, wantQuit bool, run int) {
	t.Helper()
	model := fixture.model
	switch fixture.kind {
	case asyncOperationInitialLoad, asyncOperationReload:
		assertOperationReleased(t, model, run)
		if model.browser.revision != 41 || !model.browser.hasNode(fixture.connection.ID) {
			t.Fatalf("run %d: accepted snapshot was not reconciled", run)
		}
		if wantQuit != (command != nil) {
			t.Fatalf("run %d: reload quit command=%v, want %v", run, command != nil, wantQuit)
		}
	case asyncOperationSave:
		if command == nil || model.operation == nil || model.operation.kind != asyncOperationReload || model.operation.id <= fixture.id || model.form != nil {
			t.Fatalf("run %d: Save commit did not enter reconciliation: operation=%#v", run, model.operation)
		}
		if wantQuit != model.operation.quitIntent {
			t.Fatalf("run %d: reconciliation quit intent=%v, want %v", run, model.operation.quitIntent, wantQuit)
		}
		reconcileID := model.operation.id
		_, quit := model.Update(operationResultMsg{id: reconcileID, kind: operationReload, list: app.ListConnectionsResult{Connections: []app.Connection{fixture.saved}, CatalogRevision: 41}})
		assertOperationReleased(t, model, run)
		selected := model.browser.selection()
		if model.browser.selectedID != fixture.saved.ID || model.browser.revision != 41 || selected == nil || selected.Revision != fixture.saved.Revision || selected.Host != fixture.saved.Host {
			t.Fatalf("run %d: post-commit Save result was not reconciled", run)
		}
		if wantQuit != (quit != nil) {
			t.Fatalf("run %d: Save reconciliation quit command=%v, want %v", run, quit != nil, wantQuit)
		}
	case asyncOperationSSHStart:
		assertOperationReleased(t, model, run)
		if model.sessionResult.Session.StartedAt.IsZero() || command == nil {
			t.Fatalf("run %d: active SSH commit was not retained before exit", run)
		}
	}
	assertOperationDuplicateInert(t, fixture, run)
}

func assertOperationRestored(t *testing.T, fixture operationConformanceFixture, run int) {
	t.Helper()
	assertOperationReleased(t, fixture.model, run)
	if got := captureOperationConformanceSurface(fixture.model); !reflect.DeepEqual(got, fixture.stable) {
		t.Fatalf("run %d: cleanup did not restore stable selection/focus/form/modal\n got: %#v\nwant: %#v", run, got, fixture.stable)
	}
	assertOperationDuplicateInert(t, fixture, run)
}

func assertOperationReleased(t *testing.T, model *Model, run int) {
	t.Helper()
	if model.operation != nil {
		t.Fatalf("run %d: operation resources remain after cleanup: operation=%#v", run, model.operation)
	}
}

func assertOperationDuplicateInert(t *testing.T, fixture operationConformanceFixture, run int) {
	t.Helper()
	before := captureOperationConformanceState(fixture.model)
	_, command := fixture.model.Update(operationSuccessMessage(fixture, fixture.id))
	if command != nil || !reflect.DeepEqual(captureOperationConformanceState(fixture.model), before) {
		t.Fatalf("run %d: duplicate completion changed the stable state", run)
	}
}

func installOperationCleanupProbe(t *testing.T, fixture operationConformanceFixture, run int) *int {
	t.Helper()
	model := fixture.model
	original := model.operation.cancel
	calls := new(int)
	model.operation.cancel = func() {
		*calls = *calls + 1
		if model.operation == nil || model.operation.id != fixture.id {
			t.Fatalf("run %d: owner was released before cleanup callback", run)
		}
		if got := actionKeys(model.currentActionDescriptors()); got != "Esc/?/q" {
			t.Fatalf("run %d: controls during cleanup = %q", run, got)
		}
		if got := captureOperationConformanceSurface(model); !reflect.DeepEqual(got, fixture.stable) {
			t.Fatalf("run %d: owner surface changed before cleanup", run)
		}
		if original != nil {
			original()
		}
	}
	return calls
}

func operationSuccessMessage(fixture operationConformanceFixture, id uint64) tea.Msg {
	switch fixture.kind {
	case asyncOperationInitialLoad, asyncOperationReload:
		return operationResultMsg{id: id, kind: operationReload, list: app.ListConnectionsResult{Connections: []app.Connection{fixture.connection}, CatalogRevision: 41}}
	case asyncOperationSave:
		return operationResultMsg{id: id, kind: operationUpdate, connection: app.ConnectionResult{Connection: fixture.saved, CatalogRevision: 41}}
	case asyncOperationSSHStart:
		return sessionFinishedMsg{id: id, attempt: fixture.attempt, result: app.ConnectResult{Connection: fixture.connection, Attempt: fixture.attempt, Session: app.SSHSessionResult{State: app.SessionSucceeded, StartedAt: time.Unix(100, 0)}}}
	default:
		panic("unsupported operation kind")
	}
}

func operationFailureMessage(fixture operationConformanceFixture, id uint64) tea.Msg {
	err := errors.New("controlled operation failure")
	switch fixture.kind {
	case asyncOperationInitialLoad, asyncOperationReload:
		return operationResultMsg{id: id, kind: operationReload, err: err}
	case asyncOperationSave:
		return operationResultMsg{id: id, kind: operationUpdate, err: err}
	case asyncOperationSSHStart:
		return sessionFinishedMsg{id: id, attempt: fixture.attempt, result: app.ConnectResult{Attempt: fixture.attempt}, err: err}
	default:
		panic("unsupported operation kind")
	}
}

func operationCanceledMessage(fixture operationConformanceFixture, id uint64) tea.Msg {
	switch fixture.kind {
	case asyncOperationInitialLoad, asyncOperationReload:
		return operationResultMsg{id: id, kind: operationReload, err: context.Canceled}
	case asyncOperationSave:
		return operationResultMsg{id: id, kind: operationUpdate, err: context.Canceled}
	case asyncOperationSSHStart:
		return sessionFinishedMsg{id: id, attempt: fixture.attempt, result: app.ConnectResult{Attempt: fixture.attempt}, err: context.Canceled}
	default:
		panic("unsupported operation kind")
	}
}

func operationConflictMessage(fixture operationConformanceFixture, err error) tea.Msg {
	switch fixture.kind {
	case asyncOperationReload:
		return operationResultMsg{id: fixture.id, kind: operationReload, err: err}
	case asyncOperationSave:
		return operationResultMsg{id: fixture.id, kind: operationUpdate, err: err}
	case asyncOperationSSHStart:
		return sessionFinishedMsg{id: fixture.id, attempt: fixture.attempt, result: app.ConnectResult{Attempt: fixture.attempt}, err: err}
	default:
		panic("operation kind cannot publish a target conflict")
	}
}

func operationPublishedConflict(model *Model) *conflictState {
	if model.connectionEdit != nil && model.connectionEdit.conflict != nil {
		return model.connectionEdit.conflict
	}
	return model.modal.conflict
}

func operationLoadingTarget(operation operationState) string {
	if operation.target == nil {
		return "catalog"
	}
	if operation.kind == asyncOperationSSHStart && operation.target.endpointOrScope != "" {
		return operation.target.endpointOrScope
	}
	return operation.target.path
}

func captureOperationConformanceSurface(model *Model) operationConformanceSurface {
	surface := operationConformanceSurface{
		selectedID: model.browser.selectedID,
		focus:      model.focusOwner,
		screen:     model.screen,
		form:       model.form,
		modal:      model.modal,
		revision:   model.browser.revision,
	}
	if model.form != nil {
		surface.formFocus = model.form.focusedField()
		surface.formValues = make([]string, fieldCount)
		for field := fieldName; field < fieldCount; field++ {
			surface.formValues[field] = model.form.inputs[field].Value()
		}
	}
	return surface
}

func captureOperationConformanceState(model *Model) operationConformanceState {
	state := operationConformanceState{
		surface: captureOperationConformanceSurface(model),
		view:    model.View().Content, status: model.status, operationID: model.operationID,
	}
	if model.operation != nil {
		copy := *model.operation
		copy.ctx = nil
		copy.cancel = nil
		if copy.target != nil {
			target := copy.target.clone()
			copy.target = &target
		}
		state.operation = &copy
	}
	return state
}
