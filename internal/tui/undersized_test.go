package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

type us5UndersizedCase struct {
	model    *Model
	security *securityInputState
	secret   string
}

func newUS5UndersizedModel() *Model {
	root := testFolder("root", "", "/", 1)
	connection := testConnection("connection", root.ID, "/production-sensitive-target", 2)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	model.browser.setSnapshot(snapshot, connection.ID)
	model.ownedSelectionID = connection.ID
	model.syncDetail()
	return model
}

func TestUS5UndersizedShellAndExactRestoration20Runs(t *testing.T) {
	tests := []struct {
		name  string
		setup func() us5UndersizedCase
	}{
		{name: "navigation", setup: func() us5UndersizedCase {
			model := newUS5UndersizedModel()
			model.focusOwner = focusOwnerDetail
			model.detailState = model.detailState.withOffset(3)
			return us5UndersizedCase{model: model}
		}},
		{name: "dirty connection form", setup: func() us5UndersizedCase {
			model := newResizeConformanceDirtyForm(t)
			return us5UndersizedCase{model: model, secret: "dirty.production.test"}
		}},
		{name: "confirmation", setup: func() us5UndersizedCase {
			model := newUS5UndersizedModel()
			connection := testConnection("captured", "root", "/captured-confirmation-target", 9)
			target := capturedTargetFromConnection(connection)
			model.openGenericModal(modalKindConnectConfirmation, &target, connectConfirmationPayload{confirmation: newConnectConfirmation(connection)})
			return us5UndersizedCase{model: model, secret: "/captured-confirmation-target"}
		}},
		{name: "operation", setup: func() us5UndersizedCase {
			fixture := newOperationConformanceFixture(t, asyncOperationSSHStart)
			fixture.model.pendingSelection = fixture.connection.ID
			return us5UndersizedCase{model: fixture.model, secret: fixture.connection.Path}
		}},
		{name: "pending synthetic security input", setup: func() us5UndersizedCase {
			model := newUS5UndersizedModel()
			state, _ := newSecurityInputState(securityInputSecret, focusOwnerDetail)
			state = state.withViewport(newViewportState(7))
			return us5UndersizedCase{model: model, security: &state, secret: "synthetic-secret-input"}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := tt.setup()
			for run := 0; run < 20; run++ {
				updateModel(fixture.model, tea.WindowSizeMsg{Width: 40, Height: 12})
				before := snapshotUS5Undersized(fixture)
				beforeGeometry := assertUS5RenderedViewportGeometry(t, fixture.model)

				updateModel(fixture.model, tea.WindowSizeMsg{Width: 39, Height: 11})
				if layout := calculateLayout(fixture.model.width, fixture.model.height, fixture.model.focusedLayoutRegion()); layout.mode != layoutUndersized {
					t.Fatalf("run %d 39x11 mode = %v, want undersized", run+1, layout.mode)
				}
				assertUS5UndersizedOnlyShell(t, fixture.model.View().Content, tt.name, fixture.secret)

				for _, blocked := range []tea.KeyPressMsg{keyPress("j"), keyPress("y"), keyPress("tab")} {
					updateModel(fixture.model, blocked)
				}
				assertUS5UndersizedSnapshot(t, fixture, before, run, "blocked input")

				updateModel(fixture.model, keyPress("?"))
				help := fixture.model.View().Content
				for _, required := range []string{"Help", "40x12", "? Close", "q Quit"} {
					if !strings.Contains(help, required) {
						t.Fatalf("run %d undersized Help omitted %q: %q", run+1, required, help)
					}
				}
				updateModel(fixture.model, keyPress("?"))
				updateModel(fixture.model, tea.WindowSizeMsg{Width: 40, Height: 12})
				assertUS5UndersizedSnapshot(t, fixture, before, run, "recovery")
				if recovered := assertUS5RenderedViewportGeometry(t, fixture.model); !reflect.DeepEqual(recovered, beforeGeometry) {
					t.Fatalf("run %d recovery did not restore viewport geometry before further input\n got: %#v\nwant: %#v", run+1, recovered, beforeGeometry)
				}
			}
			cancelResizeConformanceOperation(fixture.model)
		})
	}
}

func TestUS5UndersizedSafeQuitPreservesDirtyAndOperationProtections(t *testing.T) {
	t.Run("dirty form", func(t *testing.T) {
		for run := range resizeConformanceRuns {
			model := newResizeConformanceDirtyForm(t)
			model.connectionEdit.conflict = nil
			model.connectionEdit.quitAfterSave = false
			model.width, model.height = 39, 11
			form := model.form
			before := snapshotResizeConnectionForm(form)

			_, command := model.Update(keyPress("q"))
			if command != nil || model.form != form || model.modal.kind != modalKindUnsavedChanges || model.focusOwner != focusOwnerModal {
				t.Fatalf("run %d: undersized dirty Quit bypassed unsaved protection: command=%v form=%p modal=%s focus=%v", run+1, command != nil, model.form, model.modal.kind, model.focusOwner)
			}
			if got := snapshotResizeConnectionForm(model.form); !reflect.DeepEqual(got, before) {
				t.Fatalf("run %d: undersized dirty Quit changed form", run+1)
			}
		}
	})

	t.Run("operation", func(t *testing.T) {
		for run := range resizeConformanceRuns {
			fixture := newOperationConformanceFixture(t, asyncOperationReload)
			model := fixture.model
			model.width, model.height = 39, 11
			before := snapshotResizeOperation(model.operation)

			_, command := model.Update(keyPress("q"))
			if command != nil || model.operation == nil || model.operation.phase != asyncPhaseCancelRequested || !model.operation.quitIntent || model.operation.id != before.id || model.operation.retry == nil {
				t.Fatalf("run %d: undersized operation Quit did not wait for cleanup: command=%v operation=%+v", run+1, command != nil, model.operation)
			}
			cancelResizeConformanceOperation(model)
		}
	})
}

type us5UndersizedSnapshot struct {
	model    resizeConformanceSnapshot
	security resizeSecuritySnapshot
}

func snapshotUS5Undersized(fixture us5UndersizedCase) us5UndersizedSnapshot {
	return us5UndersizedSnapshot{
		model:    snapshotResizeConformanceModel(fixture.model),
		security: snapshotResizeSecurity(fixture.security),
	}
}

func assertUS5UndersizedSnapshot(t *testing.T, fixture us5UndersizedCase, want us5UndersizedSnapshot, run int, transition string) {
	t.Helper()
	got := snapshotUS5Undersized(fixture)
	if got.model.operationContext != want.model.operationContext {
		t.Fatalf("run %d %s: operation context identity changed", run+1, transition)
	}
	got.model.operationContext, want.model.operationContext = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("run %d %s changed state\n got: %#v\nwant: %#v", run+1, transition, got, want)
	}
}

func assertUS5UndersizedOnlyShell(t *testing.T, view, name, secret string) {
	t.Helper()
	for _, required := range []string{"Terminal too small", "Required minimum: 40x12", "? Help", "q Quit"} {
		if !strings.Contains(view, required) {
			t.Fatalf("%s undersized shell omitted %q: %q", name, required, view)
		}
	}
	for _, forbidden := range []string{"Tree", "Details", "Actions", "Connect", "Confirm", "Save", "Reload", secret} {
		if forbidden != "" && strings.Contains(view, forbidden) {
			t.Fatalf("%s undersized shell exposed %q instead of minimum/Help/safe Quit only: %q", name, forbidden, view)
		}
	}
}
