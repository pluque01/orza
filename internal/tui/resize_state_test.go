package tui

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

type us5ModalPayload struct {
	value   string
	control int
}

type us5OpaqueFixture struct {
	model    *Model
	modal    modalState
	conflict conflictState
	security securityInputState
	intent   string
}

type us5OpaqueSnapshot struct {
	selection        app.NodeID
	expanded         []app.NodeID
	treeOffset       int
	detailTarget     app.NodeID
	detailOffset     int
	focus            focusOwner
	formValues       []string
	formFocus        connectionField
	formOffset       int
	formError        string
	modal            modalState
	conflict         conflictState
	operation        operationState
	pendingSelection app.NodeID
	security         securityInputState
	intent           string
}

type us5RenderedViewportSnapshot struct {
	name      string
	render    int
	visible   int
	content   int
	available int
	active    int
	bar       scrollbarGeometry
}

func newUS5OpaqueFixture(t *testing.T) *us5OpaqueFixture {
	t.Helper()
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/team", 3)
	connection := testConnection("connection", folder.ID, "/team/prod", 7)
	snapshot := newCatalogSnapshot(root, 11)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}}) ||
		!snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
		t.Fatal("fixture snapshot setup failed")
	}

	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(snapshot, connection.ID)
	model.browser.expanded[folder.ID] = struct{}{}
	model.browser.rebuildRows()
	model.browser.viewport = newViewportState(5)
	model.detailState, _ = newDetailState(snapshot, connection.ID)
	model.detailState = model.detailState.withOffset(4)
	model.ownedSelectionID = connection.ID
	model.focusOwner = focusOwnerConnectionForm
	model.screen = screenConnectionForm
	model.form = validConnectionForm()
	model.form.inputs[fieldName].SetValue("opaque-用户")
	model.form.setFocus(fieldHost)
	model.form.viewport = newViewportState(6)
	model.form.formError = "retained save error"
	model.pendingSelection = connection.ID

	target := capturedTarget{id: connection.ID, revision: connection.Revision, kind: app.NodeKindConnection, path: connection.Path, endpointOrScope: "host.test:22", ancestorIDs: []app.NodeID{folder.ID, root.ID}}
	conflict, ok := newConflictState(conflictTypeRevisionChanged, target, conflictOwnerForm)
	if !ok {
		t.Fatal("conflict fixture setup failed")
	}
	model.operationID = 40
	_, _, ok = model.beginOperationWith(asyncOperationSave, &target, operationOwnerForm)
	if !ok {
		t.Fatal("operation fixture setup failed")
	}

	registry, err := registerModalPayload[us5ModalPayload](modalRegistry{}, modalKindHelp)
	if err != nil {
		t.Fatal(err)
	}
	modal, err := (modalState{}).open(registry, modalOpenRequest{
		kind: modalKindHelp, openedFrom: focusOwnerConnectionForm,
		target: &target, payload: us5ModalPayload{value: "opaque panel", control: 3},
		viewport: newViewportState(8), conflict: &conflict,
	})
	if err != nil {
		t.Fatal(err)
	}
	security, ok := newSecurityInputState(securityInputSecret, focusOwnerConnectionForm)
	if !ok {
		t.Fatal("security input fixture setup failed")
	}
	security = security.withViewport(newViewportState(9))

	return &us5OpaqueFixture{
		model: model, modal: modal, conflict: conflict, security: security,
		intent: "quit-after-save:pending",
	}
}

func (fixture *us5OpaqueFixture) snapshot() us5OpaqueSnapshot {
	expanded := make([]app.NodeID, 0, len(fixture.model.browser.expanded))
	for id := range fixture.model.browser.expanded {
		expanded = append(expanded, id)
	}
	sort.Slice(expanded, func(i, j int) bool { return expanded[i] < expanded[j] })
	values := make([]string, fieldCount)
	for field := fieldName; field < fieldCount; field++ {
		if field <= fieldIdentity {
			values[field] = fixture.model.form.inputs[field].Value()
		}
	}
	operation := operationState{}
	if fixture.model.operation != nil {
		operation = *fixture.model.operation
		operation.ctx = nil
		operation.cancel = nil
		if fixture.model.operation.target != nil {
			target := fixture.model.operation.target.clone()
			operation.target = &target
		}
	}
	return us5OpaqueSnapshot{
		selection: fixture.model.browser.selectedID, expanded: expanded,
		treeOffset:   fixture.model.browser.viewport.logicalOffset,
		detailTarget: fixture.model.detailState.targetID, detailOffset: fixture.model.detailState.viewport.logicalOffset,
		focus: fixture.model.focusOwner, formValues: values, formFocus: fixture.model.form.focusedField(),
		formOffset: fixture.model.form.viewport.logicalOffset, formError: fixture.model.form.formError,
		modal: fixture.modal, conflict: fixture.conflict, operation: operation,
		pendingSelection: fixture.model.pendingSelection, security: fixture.security, intent: fixture.intent,
	}
}

func TestUS5ResizeSequencePreservesOpaqueState20Runs(t *testing.T) {
	const runs = 20
	sequence := []int{40, 60, 79, 80, 100, 160, 80, 79, 40}
	fixture := newUS5OpaqueFixture(t)
	want := fixture.snapshot()

	for run := 0; run < runs; run++ {
		for _, height := range []int{12, 24} {
			restored := make(map[int][]us5RenderedViewportSnapshot)
			for _, width := range sequence {
				updated, command := fixture.model.Update(tea.WindowSizeMsg{Width: width, Height: height})
				if updated != fixture.model || command != nil {
					t.Fatalf("run %d resize %dx%d replaced model or returned command", run+1, width, height)
				}
				if got := fixture.snapshot(); !reflect.DeepEqual(got, want) {
					t.Fatalf("run %d resize %dx%d changed opaque state\n got: %#v\nwant: %#v", run+1, width, height, got, want)
				}
				geometry := assertUS5RenderedViewportGeometry(t, fixture.model)
				if previous, ok := restored[width]; ok && !reflect.DeepEqual(geometry, previous) {
					t.Fatalf("run %d resize %dx%d did not restore geometry\n got: %#v\nwant: %#v", run+1, width, height, geometry, previous)
				}
				restored[width] = geometry
				assertLayoutInvariants(t, calculateLayout(width, height, fixture.model.focusedLayoutRegion()))
			}
		}
	}
}

func TestUS5WideShortTransitionsPreserveOpaqueState(t *testing.T) {
	fixture := newUS5OpaqueFixture(t)
	want := fixture.snapshot()
	for run := 0; run < 20; run++ {
		for _, size := range []us5Size{
			{name: "80x12", width: 80, height: 12},
			{name: "80x24", width: 80, height: 24},
			{name: "100x20", width: 100, height: 20},
			{name: "100x30", width: 100, height: 30},
		} {
			updateModel(fixture.model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
			layout := calculateLayout(size.width, size.height, fixture.model.focusedLayoutRegion())
			if layout.mode != layoutWide || layout.reduced != (size.height < completeLayoutHeight) {
				t.Fatalf("run %d %s mode/reduced = %v/%t", run+1, size.name, layout.mode, layout.reduced)
			}
			if got := fixture.snapshot(); !reflect.DeepEqual(got, want) {
				t.Fatalf("run %d %s changed opaque state", run+1, size.name)
			}
		}
	}
}

func assertUS5RenderedViewportGeometry(t testing.TB, model *Model) []us5RenderedViewportSnapshot {
	t.Helper()
	layout := calculateLayout(model.width, model.height, model.focusedLayoutRegion())
	if layout.mode == layoutUndersized {
		if view := model.View().Content; strings.Contains(view, "█") || strings.Contains(view, "↑ more") || strings.Contains(view, "↓ more") {
			t.Fatalf("undersized view retained overflow indicators:\n%s", view)
		}
		return nil
	}

	projections := []struct {
		name       string
		projection viewportProjection
	}{
		{name: "Tree", projection: model.browser.projectTree(model.styles, layout.tree.contentWidth(), layout.tree.contentHeight())},
	}
	if model.screen == screenConnectionForm && model.form != nil {
		fixed := len(model.formConflictLines(layout.details.contentWidth()))
		projections = append(projections, struct {
			name       string
			projection viewportProjection
		}{name: "form", projection: model.form.projectAt(model.styles, layout.details.contentWidth(), max(1, layout.details.contentHeight()-fixed))})
	} else {
		projections = append(projections, struct {
			name       string
			projection viewportProjection
		}{name: "Details", projection: model.detailState.project(layout.details.contentHeight(), layout.details.contentWidth())})
	}
	if model.modal.isOpen() {
		projections = append(projections, struct {
			name       string
			projection viewportProjection
		}{name: "modal", projection: projectOpenModalForTest(model)})
	}

	result := make([]us5RenderedViewportSnapshot, 0, len(projections))
	visibleBar := false
	modalBar := false
	for _, item := range projections {
		projection := item.projection
		want := sc007ExactGeometry(projection.contentLength, projection.availableRows, projection.renderOffset)
		if !reflect.DeepEqual(projection.scrollbar, want) {
			t.Fatalf("%s resize geometry = %#v, want %#v from content=%d rows=%d offset=%d", item.name, projection.scrollbar, want, projection.contentLength, projection.availableRows, projection.renderOffset)
		}
		if projection.activeLine != noActiveLine && (projection.activeLine < projection.renderOffset || projection.activeLine >= projection.renderOffset+projection.visibleRows) {
			t.Fatalf("%s active line %d is outside rendered [%d,%d)", item.name, projection.activeLine, projection.renderOffset, projection.renderOffset+projection.visibleRows)
		}
		visibleBar = visibleBar || projection.scrollbar.visible
		if item.name == "modal" {
			modalBar = projection.scrollbar.visible
		}
		result = append(result, us5RenderedViewportSnapshot{
			name: item.name, render: projection.renderOffset, visible: projection.visibleRows,
			content: projection.contentLength, available: projection.availableRows,
			active: projection.activeLine, bar: projection.scrollbar,
		})
	}
	view := model.View().Content
	if model.modal.isOpen() {
		visibleBar = modalBar
	}
	if visibleBar && !strings.Contains(view, "█") {
		t.Fatalf("derived resize scrollbar is absent from rendered frame:\n%s", view)
	}
	if strings.Contains(view, "↑ more") || strings.Contains(view, "↓ more") {
		t.Fatalf("resize rendered legacy overflow marker:\n%s", view)
	}
	return result
}
