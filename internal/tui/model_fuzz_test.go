package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const modelFuzzActionLimit = 128

type modelFuzzHarness struct {
	t             *testing.T
	model         *Model
	connection    app.Connection
	commands      []tea.Cmd
	completions   []operationResultMsg
	lastCompleted *operationResultMsg
	step          int
}

func FuzzUS5ModelStateMessages(f *testing.F) {
	// Each owner has a seed that races a real generated result with stale,
	// matching, and duplicate deliveries before moving to the next owner.
	f.Add([]byte{2, 4, 8, 9, 10}, uint16(80), uint8(24))
	f.Add([]byte{3, 5, 8, 9, 10, 11, 7}, uint16(100), uint8(24))
	f.Add([]byte{6, 8, 9, 10, 0, 1, 11}, uint16(39), uint8(11))
	f.Add([]byte{2, 1, 1, 4, 7, 10, 3, 5, 7, 10}, uint16(160), uint8(40))

	f.Fuzz(func(t *testing.T, actions []byte, rawWidth uint16, rawHeight uint8) {
		if len(actions) > modelFuzzActionLimit {
			t.Skip()
		}
		harness := newModelFuzzHarness(t, int(rawWidth%201), int(rawHeight%41))
		for step, action := range actions {
			harness.step = step
			harness.apply(action, int(rawWidth%201), int(rawHeight%41))
			harness.assertState("action")
		}
	})
}

func newModelFuzzHarness(t *testing.T, width, height int) *modelFuzzHarness {
	t.Helper()
	connection := testConnection("fuzz-target", syntheticRootID, "/fuzz-target", 4)
	values := []app.Connection{connection, testConnection("fuzz-other", syntheticRootID, "/fuzz-other", 2)}
	service := ConnectionFuncs{
		ListFunc: func(ctx context.Context, _ app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			if err := ctx.Err(); err != nil {
				return app.ListConnectionsResult{}, err
			}
			return app.ListConnectionsResult{Connections: values, CatalogRevision: 7}, nil
		},
		UpdateFunc: func(ctx context.Context, _ app.UpdateConnectionRequest) (app.ConnectionResult, error) {
			if err := ctx.Err(); err != nil {
				return app.ConnectionResult{}, err
			}
			return app.ConnectionResult{}, app.ErrConflict
		},
		DeleteFunc: func(ctx context.Context, _ app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
			if err := ctx.Err(); err != nil {
				return app.DeleteConnectionResult{}, err
			}
			return app.DeleteConnectionResult{}, app.ErrConflict
		},
	}
	model := New(Config{Connections: service, Width: max(1, width), Height: max(1, height), NoColor: true})
	harness := &modelFuzzHarness{t: t, model: model, connection: connection, step: -1}
	harness.enqueue(model.Init())
	harness.runCommand(false)
	harness.assertState("initial load")
	return harness
}

func (h *modelFuzzHarness) apply(action byte, width, height int) {
	selector := int(action) / 12
	switch action % 12 {
	case 0:
		size := us5ContractSizes[int(action)%len(us5ContractSizes)]
		if action&0x80 != 0 {
			size.width, size.height = width, height
		}
		h.update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
	case 1:
		keys := []tea.KeyPressMsg{
			keyPress("tab"), keyPress("shift+tab"), keyPress("j"), keyPress("k"),
			keyPress("g"), keyPress("G"), keyPress("?"), keyPress("esc"),
			keyPress("b"), keyPress("r"),
		}
		h.update(keys[selector%len(keys)])
	case 2:
		h.openDeleteModal()
	case 3:
		h.openForm(action)
	case 4:
		if h.model.operation == nil && h.model.modal.kind == modalKindDeleteConnection && h.model.modal.conflict == nil {
			h.update(keyPress("y"))
		}
	case 5:
		if h.model.operation == nil && h.model.form != nil && h.model.connectionEdit != nil && h.model.connectionEdit.conflict == nil && !h.model.modal.isOpen() {
			h.update(keyPress("ctrl+s"))
		}
	case 6:
		if h.model.operation == nil {
			h.closeSurfaces()
			h.selectTarget()
			h.update(keyPress("r"))
		}
	case 7:
		h.runCommand(false)
	case 8:
		h.runCommand(true)
	case 9:
		h.deliverMatchingCompletion()
	case 10:
		h.deliverDuplicateCompletion()
	case 11:
		h.mutateCurrentOwner(selector)
	}
}

func (h *modelFuzzHarness) openDeleteModal() {
	if h.model.operation != nil {
		return
	}
	h.closeSurfaces()
	h.selectTarget()
	target := h.model.captureConnectionTarget(h.connection)
	scope := app.ConnectionDeleteScope{
		ID: h.connection.ID, Path: h.connection.Path, Host: h.connection.Host, Revision: h.connection.Revision,
	}
	h.model.openGenericModal(modalKindDeleteConnection, &target, deleteConnectionPayload{confirmation: newDeleteConfirmation(scope)})
}

func (h *modelFuzzHarness) openForm(action byte) {
	if h.model.operation != nil {
		return
	}
	h.closeSurfaces()
	h.selectTarget()
	h.model.openConnectionForm(newConnectionForm(&h.connection), h.model.captureConnectionTarget(h.connection))
	h.model.form.inputs[fieldHost].SetValue(fmt.Sprintf("fuzz-%02x.test", action))
	h.model.form.setFocus(fieldHost)
}

func (h *modelFuzzHarness) closeSurfaces() {
	if h.model.modal.helpVisible {
		h.model.toggleModalHelp()
	}
	if h.model.modal.isOpen() {
		h.model.closeGenericModal()
	}
	if h.model.form != nil {
		h.model.closeConnectionForm(false)
	}
}

func (h *modelFuzzHarness) selectTarget() {
	h.model.browser.selectedID = h.connection.ID
	h.model.browser.expandAncestors(h.connection.ID)
	h.model.browser.rebuildRows()
	h.model.ownedSelectionID = h.connection.ID
	h.model.syncDetail()
}

func (h *modelFuzzHarness) mutateCurrentOwner(selector int) {
	if h.model.operation != nil {
		h.update(keyPress("esc"))
		return
	}
	key := []string{"esc", "r", "b"}[selector%3]
	switch {
	case h.model.modal.conflict != nil:
		h.update(keyPress(key))
	case h.model.connectionEdit != nil && h.model.connectionEdit.conflict != nil:
		h.update(keyPress(key))
	case h.model.modal.isOpen():
		h.update(keyPress("?"))
	default:
		h.update(keyPress("tab"))
	}
}

func (h *modelFuzzHarness) update(msg tea.Msg) {
	updated, command := h.model.Update(msg)
	if updated != h.model {
		h.t.Fatalf("step %d: Update replaced the root model", h.step)
	}
	h.enqueue(command)
	h.assertState(fmt.Sprintf("message %T", msg))
}

func (h *modelFuzzHarness) enqueue(command tea.Cmd) {
	if command != nil {
		h.commands = append(h.commands, command)
	}
}

func (h *modelFuzzHarness) runCommand(stale bool) {
	if len(h.commands) == 0 {
		return
	}
	command := h.commands[0]
	h.commands = h.commands[1:]
	h.consumeCommandMessage(command(), stale)
}

func (h *modelFuzzHarness) consumeCommandMessage(msg tea.Msg, stale bool) {
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, command := range msg {
			if command != nil {
				h.consumeCommandMessage(command(), stale)
				stale = false
			}
		}
	case operationResultMsg:
		original := msg
		if stale {
			msg.id = staleOperationID(msg.id)
			h.update(msg)
			h.completions = append(h.completions, original)
			return
		}
		h.update(msg)
		h.lastCompleted = &original
	default:
		h.update(msg)
	}
}

func staleOperationID(id uint64) uint64 {
	if id > 1 {
		return id - 1
	}
	return id + 1
}

func (h *modelFuzzHarness) deliverMatchingCompletion() {
	if len(h.completions) == 0 {
		return
	}
	message := h.completions[0]
	h.completions = h.completions[1:]
	h.update(message)
	h.lastCompleted = &message
}

func (h *modelFuzzHarness) deliverDuplicateCompletion() {
	if h.lastCompleted != nil {
		h.update(*h.lastCompleted)
	}
}

func (h *modelFuzzHarness) assertState(stage string) {
	t := h.t
	t.Helper()
	m := h.model
	if !m.focusOwner.valid() {
		t.Fatalf("step %d %s: invalid focus owner %v", h.step, stage, m.focusOwner)
	}
	if m.modal.isOpen() != (m.focusOwner == focusOwnerModal) {
		t.Fatalf("step %d %s: modal/focus ownership mismatch: modal=%q focus=%v", h.step, stage, m.modal.kind, m.focusOwner)
	}
	if m.modal.isOpen() {
		if !m.modal.kind.valid() || !m.modal.openedFrom.validModalOpener() || !m.modal.blocksBackgroundInput() {
			t.Fatalf("step %d %s: invalid modal owner: %+v", h.step, stage, m.modal)
		}
	} else if m.modal != (modalState{}) {
		t.Fatalf("step %d %s: closed modal retained owner state: %+v", h.step, stage, m.modal)
	}
	if (m.form == nil) != (m.connectionEdit == nil) || (m.form == nil) != (m.screen != screenConnectionForm) {
		t.Fatalf("step %d %s: form ownership mismatch: form=%v edit=%v screen=%v", h.step, stage, m.form != nil, m.connectionEdit != nil, m.screen)
	}
	if m.form != nil && !m.modal.isOpen() && m.focusOwner != focusOwnerConnectionForm {
		t.Fatalf("step %d %s: unmodalized form does not own focus: %v", h.step, stage, m.focusOwner)
	}
	if m.form == nil && !m.modal.isOpen() && m.focusOwner != focusOwnerTree && m.focusOwner != focusOwnerDetail {
		t.Fatalf("step %d %s: browser has invalid focus owner %v", h.step, stage, m.focusOwner)
	}
	if m.connectionEdit != nil {
		h.assertTarget("form", m.connectionEdit.target)
		if conflict := m.connectionEdit.conflict; conflict != nil {
			if conflict.owner != conflictOwnerForm || !conflict.blocked || !sameCapturedTarget(conflict.target, m.connectionEdit.target) {
				t.Fatalf("step %d %s: form conflict lost target ownership: %+v", h.step, stage, conflict)
			}
		}
	}
	if m.modal.target != nil {
		h.assertTarget("modal", *m.modal.target)
	}
	if conflict := m.modal.conflict; conflict != nil {
		if !m.modal.isOpen() || m.modal.target == nil || conflict.owner != conflictOwnerModal || !conflict.blocked || !sameCapturedTarget(conflict.target, *m.modal.target) {
			t.Fatalf("step %d %s: modal conflict lost target ownership: %+v", h.step, stage, conflict)
		}
	}
	h.assertOperation(stage)

	for name, offset := range map[string]int{
		"tree": m.browser.viewport.logicalOffset, "details": m.detailState.viewport.logicalOffset,
		"modal": m.modal.viewport.logicalOffset,
	} {
		if offset < 0 {
			t.Fatalf("step %d %s: %s viewport offset is negative: %d", h.step, stage, name, offset)
		}
	}
	if m.form != nil && m.form.viewport.logicalOffset < 0 {
		t.Fatalf("step %d %s: form viewport offset is negative: %d", h.step, stage, m.form.viewport.logicalOffset)
	}
	layout := calculateLayout(m.width, m.height, m.focusedLayoutRegion())
	assertLayoutInvariants(t, layout)
	if m.modal.isOpen() {
		assertRectBounded(t, layout.modalOverlay(), max(0, m.width), max(0, m.height))
	}
	assertUS5FrameBounded(t, m.View().Content, m.width, m.height)
	rows := max(1, m.height%32)
	contentLength := rows + 1 + max(0, m.width%64)
	geometry := newScrollbarGeometry(contentLength, rows, m.browser.viewport.logicalOffset)
	if !geometry.visible || geometry.trackHeight != rows || geometry.thumbLength < 1 || geometry.thumbTop < 0 || geometry.thumbTop+geometry.thumbLength > geometry.trackHeight {
		t.Fatalf("step %d %s: invalid derived scrollbar geometry: %#v", h.step, stage, geometry)
	}
}

func (h *modelFuzzHarness) assertOperation(stage string) {
	t, m := h.t, h.model
	if m.operation == nil {
		return
	}
	operation := m.operation
	if !operation.owns(operation.id) || operation.id > m.operationID || !operation.kind.valid() || !operation.owner.valid() || operation.cancel == nil || operation.ctx == nil {
		t.Fatalf("step %d %s: invalid operation ownership: operation=%+v last=%d", h.step, stage, operation, m.operationID)
	}
	if operation.phase != asyncPhaseRunning && operation.phase != asyncPhaseCancelRequested {
		t.Fatalf("step %d %s: active operation has invalid phase %v", h.step, stage, operation.phase)
	}
	switch operation.owner {
	case operationOwnerRoot:
		if operation.target != nil || m.form != nil || m.modal.isOpen() {
			t.Fatalf("step %d %s: root operation captured another surface: %+v", h.step, stage, operation)
		}
	case operationOwnerForm:
		if m.connectionEdit == nil || operation.target == nil || !sameCapturedTarget(*operation.target, m.connectionEdit.target) {
			t.Fatalf("step %d %s: form operation lost captured target: %+v", h.step, stage, operation)
		}
	case operationOwnerModal:
		if !m.modal.isOpen() || m.modal.target == nil || operation.target == nil || !sameCapturedTarget(*operation.target, *m.modal.target) {
			t.Fatalf("step %d %s: modal operation lost captured target: %+v", h.step, stage, operation)
		}
	default:
		t.Fatalf("step %d %s: unexpected operation owner %v", h.step, stage, operation.owner)
	}
}

func (h *modelFuzzHarness) assertTarget(owner string, target capturedTarget) {
	if target.id != h.connection.ID || target.revision != h.connection.Revision || target.kind != app.NodeKindConnection || target.path != h.connection.Path {
		h.t.Fatalf("step %d: %s target was retargeted: %+v", h.step, owner, target)
	}
}

func sameCapturedTarget(left, right capturedTarget) bool {
	return left.id == right.id && left.revision == right.revision && left.kind == right.kind && left.path == right.path
}

func assertUS5FrameBounded(t testing.TB, view string, width, height int) {
	t.Helper()
	if width <= 0 || height <= 0 {
		if view != "" {
			t.Fatalf("non-empty frame for %dx%d viewport: %q", width, height, view)
		}
		return
	}
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("frame has %d rows, viewport height %d", len(lines), height)
	}
	for _, line := range lines {
		if visible := ansi.StringWidth(line); visible > width {
			t.Fatalf("frame line width %d exceeds %d: %q", visible, width, line)
		}
	}
}
