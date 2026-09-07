package tui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

const resizeConformanceRuns = 20

type resizeConformanceCase struct {
	name  string
	setup func(*testing.T) *Model
}

type resizeTextFieldSnapshot struct {
	value              string
	cursor, anchor     int
	selectionStart     int
	selectionEnd       int
	selecting, focused bool
	err                string
}

type resizeConnectionFormSnapshot struct {
	fields      [fieldCount]resizeTextFieldSnapshot
	focus       connectionField
	errors      [fieldCount]string
	formError   string
	baseline    [fieldCount]string
	remember    bool
	original    *app.Connection
	destination *app.Folder
	viewport    viewportState
}

type resizeFolderFormSnapshot struct {
	input       resizeTextFieldSnapshot
	parent      app.ItemSelector
	original    *app.Folder
	baseline    string
	err         string
	destination *app.Folder
}

type resizeConflictSnapshot struct {
	present       bool
	kind          conflictType
	target        capturedTarget
	owner         conflictOwner
	detailVisible bool
	blocked       bool
}

type resizeRetrySnapshot struct {
	present    bool
	kind       operationRetryKind
	reloadKind asyncOperationKind
	owner      operationOwnerSurface
	target     *capturedTarget
	connection app.Connection
	folder     app.Folder
}

type resizeErrorModalSnapshot struct {
	present       bool
	operation     string
	target        string
	message       string
	kind          app.ErrorKind
	attempt       app.SSHAttemptTarget
	failure       *app.SSHFailurePresentation
	detailVisible bool
	recovery      recoveryState
	retry         resizeRetrySnapshot
}

type resizeMovePickerSnapshot struct {
	present  bool
	source   app.Node
	targets  []moveTarget
	selected int
}

type resizeConnectConfirmationSnapshot struct {
	present    bool
	connection app.Connection
	previous   *app.SSHAttemptTarget
}

type resizeModalPayloadSnapshot struct {
	kind             modalKind
	folder           resizeFolderFormSnapshot
	move             resizeMovePickerSnapshot
	deleteConnection *deleteConfirmation
	deleteFolder     *folderDeleteConfirmation
	connect          resizeConnectConfirmationSnapshot
	unsaved          unsavedChangesPayload
	helpLines        []string
	operationError   resizeErrorModalSnapshot
	sshFailure       resizeErrorModalSnapshot
	sshConfirmation  resizeConnectConfirmationSnapshot
}

type resizeModalSnapshot struct {
	kind             modalKind
	openedFrom       focusOwner
	target           *capturedTarget
	payload          resizeModalPayloadSnapshot
	viewport         viewportState
	recoverableError string
	conflict         resizeConflictSnapshot
	helpVisible      bool
	helpOpenedFrom   any
}

type resizeConnectionEditSnapshot struct {
	present       bool
	selectedID    app.NodeID
	expanded      []app.NodeID
	viewport      viewportState
	focus         focusOwner
	target        capturedTarget
	conflict      resizeConflictSnapshot
	recovery      resizeErrorModalSnapshot
	quitAfterSave bool
}

type resizeOperationSnapshot struct {
	present         bool
	id              uint64
	kind            asyncOperationKind
	phase           asyncOperationPhase
	target          *capturedTarget
	owner           operationOwnerSurface
	commitResult    any
	stopReason      operationStopReason
	quitIntent      bool
	pendingConflict resizeConflictSnapshot
	retry           resizeRetrySnapshot
}

type resizeConformanceSnapshot struct {
	selection          app.NodeID
	ownedSelection     app.NodeID
	expanded           []app.NodeID
	treeOffset         int
	detailTarget       app.NodeID
	detailOffset       int
	focus              focusOwner
	screen             screen
	form               *resizeConnectionFormSnapshot
	connectionEdit     resizeConnectionEditSnapshot
	modal              resizeModalSnapshot
	operationID        uint64
	operation          resizeOperationSnapshot
	operationContext   context.Context
	operationCancelSet bool
	status             string
	pendingSelection   app.NodeID
}

type resizeSecuritySnapshot struct {
	present        bool
	kind           securityInputKind
	preservedFocus focusOwner
	viewport       viewportState
	trust          *trustPrompt
	secret         *secretPrompt
	suspended      bool
}

func TestSC005AllStatesAllContractSizes20Runs(t *testing.T) {
	for _, test := range checkedResizeConformanceCases(t) {
		t.Run(test.name, func(t *testing.T) {
			for run := range resizeConformanceRuns {
				model := test.setup(t)
				want := snapshotResizeConformanceModel(model)
				for _, size := range us5ContractSizes {
					updated, command := model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
					if updated != model || command != nil {
						t.Fatalf("run %d %s: resize replaced model or returned command", run+1, size.name)
					}
					assertResizeConformanceSnapshot(t, model, want, run, size.name)
					assertUS5FrameBounded(t, model.View().Content, size.width, size.height)
					assertResizeConformanceSnapshot(t, model, want, run, size.name+" render")
				}
				cancelResizeConformanceOperation(model)
			}
		})
	}
}

func TestSC005AllStatesNormativeResizeSequence20Runs(t *testing.T) {
	sequence := []int{40, 60, 79, 80, 100, 160, 80, 79, 40}
	for _, test := range checkedResizeConformanceCases(t) {
		t.Run(test.name, func(t *testing.T) {
			for run := range resizeConformanceRuns {
				model := test.setup(t)
				want := snapshotResizeConformanceModel(model)
				for _, height := range []int{12, 24} {
					restored := make(map[int][]us5RenderedViewportSnapshot)
					for step, width := range sequence {
						updated, command := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
						if updated != model || command != nil {
							t.Fatalf("run %d step %d %dx%d: resize replaced model or returned command", run+1, step+1, width, height)
						}
						cell := us5Size{name: resizeCellName(width, height), width: width, height: height}
						assertResizeConformanceSnapshot(t, model, want, run, cell.name)
						geometry := assertUS5RenderedViewportGeometry(t, model)
						if previous, ok := restored[width]; ok && !reflect.DeepEqual(geometry, previous) {
							t.Fatalf("run %d step %d %s: repeated size did not restore viewport geometry", run+1, step+1, cell.name)
						}
						restored[width] = geometry
						assertUS5FrameBounded(t, model.View().Content, cell.width, cell.height)
						assertResizeConformanceSnapshot(t, model, want, run, cell.name+" render")
					}
				}
				cancelResizeConformanceOperation(model)
			}
		})
	}
}

func checkedResizeConformanceCases(t *testing.T) []resizeConformanceCase {
	t.Helper()
	modalCases := resizeModalConformanceCases()
	cases := resizeConformanceCases()
	if len(us5ContractSizes) != 12 || len(modalCases) != 10 || len(newModalRegistry().payloadTypes) != len(modalCases) || len(cases) != 17 {
		t.Fatalf("resize matrix inventory = %d sizes, %d modal cases/%d registered kinds, %d total states; want 12, 10/10, 17", len(us5ContractSizes), len(modalCases), len(newModalRegistry().payloadTypes), len(cases))
	}
	return cases
}

func resizeCellName(width, height int) string {
	return fmt.Sprintf("%dx%d", width, height)
}

func resizeConformanceCases() []resizeConformanceCase {
	cases := []resizeConformanceCase{
		{name: "tree_navigation", setup: func(t *testing.T) *Model {
			model, _, _, _ := newResizeConformanceBase(t)
			model.focusOwner = focusOwnerTree
			return model
		}},
		{name: "detail_navigation", setup: func(t *testing.T) *Model {
			model, _, _, _ := newResizeConformanceBase(t)
			model.focusOwner = focusOwnerDetail
			return model
		}},
		{name: "dirty_connection_form", setup: newResizeConformanceDirtyForm},
	}
	cases = append(cases, resizeModalConformanceCases()...)
	for _, operation := range []struct {
		name string
		kind asyncOperationKind
	}{
		{name: "initial_load", kind: asyncOperationInitialLoad},
		{name: "reload", kind: asyncOperationReload},
		{name: "save", kind: asyncOperationSave},
		{name: "ssh_start", kind: asyncOperationSSHStart},
	} {
		operation := operation
		cases = append(cases, resizeConformanceCase{name: "operation_" + operation.name, setup: func(t *testing.T) *Model {
			fixture := newOperationConformanceFixture(t, operation.kind)
			model := fixture.model
			model.browser.viewport = newViewportState(5)
			model.detailState = model.detailState.withOffset(4)
			model.pendingSelection = fixture.connection.ID
			if model.form != nil {
				model.form.viewport = newViewportState(7)
				model.form.inputs[fieldHost].SetCursor(2)
				model.form.inputs[fieldHost].Update(modifiedKey(tea.KeyRight, tea.ModShift))
				model.form.inputs[fieldHost].Update(modifiedKey(tea.KeyRight, tea.ModShift))
			}
			return model
		}})
	}
	return cases
}

func resizeModalConformanceCases() []resizeConformanceCase {
	return []resizeConformanceCase{
		{name: "modal_folder_create", setup: func(t *testing.T) *Model {
			model, _, folder, _ := newResizeConformanceBase(t)
			form := newFolderForm(nil, app.ItemSelector{ID: folder.ID})
			form.setDestination(folder)
			prepareResizeFolderField(&form.input, "created-folder")
			openResizeModal(t, model, modalKindFolderCreate, capturedTargetFromFolder(folder), folderCreatePayload{form: form})
			return model
		}},
		{name: "modal_folder_edit", setup: func(t *testing.T) *Model {
			model, _, folder, _ := newResizeConformanceBase(t)
			form := newFolderForm(&folder, app.ItemSelector{})
			prepareResizeFolderField(&form.input, "renamed-folder")
			openResizeModal(t, model, modalKindFolderEdit, capturedTargetFromFolder(folder), folderEditPayload{form: form})
			return model
		}},
		{name: "modal_move_picker", setup: func(t *testing.T) *Model {
			model, root, folder, connection := newResizeConformanceBase(t)
			destination := testFolder("destination-2", root.ID, "/destination-2", 8)
			picker := newMovePicker(connection.Node, []app.Folder{root, folder, destination})
			picker.selected = 2
			openResizeModal(t, model, modalKindMovePicker, capturedTargetFromConnection(connection), movePickerPayload{picker: picker})
			conflict, ok := newConflictState(conflictTypeRevisionChanged, capturedTargetFromConnection(connection), conflictOwnerModal)
			if !ok {
				t.Fatal("move conflict setup failed")
			}
			model.modal.conflict = &conflict
			return model
		}},
		{name: "modal_delete_connection", setup: func(t *testing.T) *Model {
			model, _, _, connection := newResizeConformanceBase(t)
			scope := app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Name: connection.Name, Host: connection.Host, Username: connection.Username, Revision: connection.Revision, HasRememberedPassword: true}
			openResizeModal(t, model, modalKindDeleteConnection, capturedTargetFromConnection(connection), deleteConnectionPayload{confirmation: newDeleteConfirmation(scope, "captured.test:2222")})
			return model
		}},
		{name: "modal_delete_folder", setup: func(t *testing.T) *Model {
			model, _, folder, _ := newResizeConformanceBase(t)
			scope := app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Name: folder.Name, Revision: folder.Revision, Folders: 2, Connections: 3, RememberedCredentials: 1, Snapshot: []app.NodeRevision{{ID: folder.ID, Revision: folder.Revision}}}
			openResizeModal(t, model, modalKindDeleteFolder, capturedTargetFromFolder(folder), deleteFolderPayload{confirmation: newFolderDeleteConfirmation(scope)})
			return model
		}},
		{name: "modal_connect_confirmation", setup: func(t *testing.T) *Model {
			model, _, _, connection := newResizeConformanceBase(t)
			previous := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision - 1, Path: "/team/old", Host: "old.test", Port: 2200}
			confirmation := newRetryConnectConfirmation(previous, connection)
			openResizeModal(t, model, modalKindConnectConfirmation, capturedTargetFromConnection(connection), connectConfirmationPayload{confirmation: confirmation})
			return model
		}},
		{name: "modal_unsaved_changes", setup: func(t *testing.T) *Model {
			model := newResizeConformanceDirtyForm(t)
			model.openUnsavedChanges(unsavedIntentQuit)
			if model.modal.kind != modalKindUnsavedChanges {
				t.Fatal("unsaved changes modal setup failed")
			}
			model.modal.viewport = newViewportState(7)
			return model
		}},
		{name: "modal_help", setup: func(t *testing.T) *Model {
			model, _, _, connection := newResizeConformanceBase(t)
			lines := []string{"Up Move up", "Down Move down", "Home First item", "End Last item", "Esc Close", "Captured help payload"}
			openResizeModal(t, model, modalKindHelp, capturedTargetFromConnection(connection), helpPayload{lines: lines})
			return model
		}},
		{name: "modal_operation_error", setup: func(t *testing.T) *Model {
			model, _, _, connection := newResizeConformanceBase(t)
			target := capturedTargetFromConnection(connection)
			intent := catalogRetryIntent(asyncOperationReload, &target, operationOwnerRoot)
			failure := newErrorModal("move catalog item", connection.Path, errors.New("controlled failure"))
			failure.retry = &intent
			openResizeModal(t, model, modalKindOperationError, target, operationErrorPayload{modal: failure})
			model.modal.recoverableError = "retained modal-level error"
			return model
		}},
		{name: "modal_ssh_failure", setup: func(t *testing.T) *Model {
			model, _, _, connection := newResizeConformanceBase(t)
			attempt := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
			failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()
			diagnostic := newSSHFailureModal(attempt, failure)
			diagnostic.detailVisible = true
			diagnostic.recovery = recoveryConflict
			openResizeModal(t, model, modalKindSSHFailure, capturedTargetFromConnection(connection), sshFailurePayload{modal: diagnostic})
			return model
		}},
	}
}

func newResizeConformanceBase(t *testing.T) (*Model, app.Folder, app.Folder, app.Connection) {
	t.Helper()
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/team", 3)
	destination := testFolder("destination", root.ID, "/destination", 5)
	connection := testConnection("connection", folder.ID, "/team/production", 7)
	connection.Name = "Production"
	connection.Host = "production.test"
	connection.Username = "deploy"
	snapshot := newCatalogSnapshot(root, 13)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder, destination}}) ||
		!snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
		t.Fatal("resize fixture snapshot setup failed")
	}
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(snapshot, connection.ID)
	model.browser.expanded[folder.ID] = struct{}{}
	model.browser.expanded[destination.ID] = struct{}{}
	model.browser.rebuildRows()
	model.browser.viewport = newViewportState(5)
	model.ownedSelectionID = connection.ID
	model.syncDetail()
	model.detailState = model.detailState.withOffset(4)
	model.pendingSelection = destination.ID
	model.status = "RESIZE SNAPSHOT"
	return model, root, folder, connection
}

func newResizeConformanceDirtyForm(t *testing.T) *Model {
	t.Helper()
	model, _, _, connection := newResizeConformanceBase(t)
	form := newConnectionForm(&connection)
	form.inputs[fieldName].SetValue("Production dirty")
	form.inputs[fieldFolder].SetValue("/team")
	form.inputs[fieldHost].SetValue("dirty.production.test")
	form.inputs[fieldPort].SetValue("2202")
	form.inputs[fieldUsername].SetValue("dirty-deploy")
	form.inputs[fieldAuth].SetValue(string(app.AuthMethodKey))
	form.inputs[fieldIdentity].SetValue("/keys/dirty_ed25519")
	form.remember = true
	form.setFocus(fieldHost)
	form.inputs[fieldHost].SetCursor(3)
	form.inputs[fieldHost].Update(modifiedKey(tea.KeyRight, tea.ModShift))
	form.inputs[fieldHost].Update(modifiedKey(tea.KeyRight, tea.ModShift))
	form.errors[fieldPort] = "retained field error"
	form.formError = "retained save error"
	form.viewport = newViewportState(8)
	model.openConnectionForm(form, model.captureConnectionTarget(connection))
	conflict, ok := newConflictState(conflictTypeRevisionChanged, model.connectionEdit.target, conflictOwnerForm)
	if !ok {
		t.Fatal("dirty form conflict setup failed")
	}
	model.connectionEdit.conflict = &conflict
	model.connectionEdit.quitAfterSave = true
	return model
}

func prepareResizeFolderField(field *textField, value string) {
	field.SetValue(value)
	field.SetCursor(2)
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
}

func openResizeModal(t *testing.T, model *Model, kind modalKind, target capturedTarget, payload any) {
	t.Helper()
	model.focusOwner = focusOwnerDetail
	if !model.openGenericModal(kind, &target, payload) {
		t.Fatalf("open %s modal failed", kind)
	}
	model.modal.viewport = newViewportState(7)
}

func snapshotResizeConformanceModel(model *Model) resizeConformanceSnapshot {
	snapshot := resizeConformanceSnapshot{
		selection: model.browser.selectedID, ownedSelection: model.ownedSelectionID,
		expanded: resizeExpandedIDs(model.browser.expanded), treeOffset: model.browser.viewport.logicalOffset,
		detailTarget: model.detailState.targetID, detailOffset: model.detailState.viewport.logicalOffset,
		focus: model.focusOwner, screen: model.screen,
		connectionEdit: snapshotResizeConnectionEdit(model.connectionEdit), modal: snapshotResizeModal(model.modal),
		operationID: model.operationID, operation: snapshotResizeOperation(model.operation),
		status: model.status, pendingSelection: model.pendingSelection,
	}
	if model.operation != nil {
		snapshot.operationContext = model.operation.ctx
		snapshot.operationCancelSet = model.operation.cancel != nil
	}
	if model.form != nil {
		form := snapshotResizeConnectionForm(model.form)
		snapshot.form = &form
	}
	return snapshot
}

func assertResizeConformanceSnapshot(t *testing.T, model *Model, want resizeConformanceSnapshot, run int, cell string) {
	t.Helper()
	got := snapshotResizeConformanceModel(model)
	if got.operationContext != want.operationContext {
		t.Fatalf("run %d %s: operation context identity changed", run+1, cell)
	}
	got.operationContext, want.operationContext = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("run %d %s: logical state changed\n got: %#v\nwant: %#v", run+1, cell, got, want)
	}
}

func snapshotResizeTextField(field *textField) resizeTextFieldSnapshot {
	if field == nil {
		return resizeTextFieldSnapshot{}
	}
	start, end := field.selection()
	return resizeTextFieldSnapshot{
		value: field.Value(), cursor: field.Position(), anchor: field.anchor,
		selectionStart: start, selectionEnd: end, selecting: field.selecting,
		focused: field.Focused(), err: field.Error(),
	}
}

func snapshotResizeConnectionForm(form *connectionForm) resizeConnectionFormSnapshot {
	snapshot := resizeConnectionFormSnapshot{
		focus: form.focus, formError: form.formError, baseline: form.baseline,
		remember: form.remember, viewport: form.viewport,
	}
	for field := fieldName; field < fieldCount; field++ {
		snapshot.fields[field] = snapshotResizeTextField(&form.inputs[field])
		snapshot.errors[field] = form.errors[field]
	}
	if form.original != nil {
		copy := *form.original
		snapshot.original = &copy
	}
	if form.destination != nil {
		copy := *form.destination
		snapshot.destination = &copy
	}
	return snapshot
}

func snapshotResizeFolderForm(form *folderForm) resizeFolderFormSnapshot {
	if form == nil {
		return resizeFolderFormSnapshot{}
	}
	snapshot := resizeFolderFormSnapshot{
		input: snapshotResizeTextField(&form.input), parent: form.parent,
		baseline: form.baseline, err: form.err,
	}
	if form.original != nil {
		copy := *form.original
		snapshot.original = &copy
	}
	if form.destination != nil {
		copy := *form.destination
		snapshot.destination = &copy
	}
	return snapshot
}

func snapshotResizeConflict(conflict *conflictState) resizeConflictSnapshot {
	if conflict == nil {
		return resizeConflictSnapshot{}
	}
	return resizeConflictSnapshot{
		present: true, kind: conflict.kind, target: conflict.target.clone(), owner: conflict.owner,
		detailVisible: conflict.detailVisible, blocked: conflict.blocked,
	}
}

func snapshotResizeRetry(retry *operationRetryIntent) resizeRetrySnapshot {
	if retry == nil {
		return resizeRetrySnapshot{}
	}
	snapshot := resizeRetrySnapshot{
		present: true, kind: retry.kind, reloadKind: retry.reloadKind, owner: retry.owner,
		connection: retry.connection, folder: retry.folder,
	}
	if retry.target != nil {
		copy := retry.target.clone()
		snapshot.target = &copy
	}
	return snapshot
}

func snapshotResizeErrorModal(modal *errorModal) resizeErrorModalSnapshot {
	if modal == nil {
		return resizeErrorModalSnapshot{}
	}
	snapshot := resizeErrorModalSnapshot{
		present: true, operation: modal.operation, target: modal.target, message: modal.message,
		kind: modal.kind, attempt: modal.attempt, detailVisible: modal.detailVisible,
		recovery: modal.recovery, retry: snapshotResizeRetry(modal.retry),
	}
	if modal.failure != nil {
		copy := *modal.failure
		snapshot.failure = &copy
	}
	return snapshot
}

func snapshotResizeConnectConfirmation(confirmation *connectConfirmation) resizeConnectConfirmationSnapshot {
	if confirmation == nil {
		return resizeConnectConfirmationSnapshot{}
	}
	snapshot := resizeConnectConfirmationSnapshot{present: true, connection: confirmation.connection}
	if confirmation.previous != nil {
		copy := *confirmation.previous
		snapshot.previous = &copy
	}
	return snapshot
}

func snapshotResizeModalPayload(kind modalKind, payload any) resizeModalPayloadSnapshot {
	snapshot := resizeModalPayloadSnapshot{kind: kind}
	switch payload := payload.(type) {
	case folderCreatePayload:
		snapshot.folder = snapshotResizeFolderForm(payload.form)
	case folderEditPayload:
		snapshot.folder = snapshotResizeFolderForm(payload.form)
	case movePickerPayload:
		if payload.picker != nil {
			snapshot.move = resizeMovePickerSnapshot{present: true, source: payload.picker.source, targets: append([]moveTarget(nil), payload.picker.targets...), selected: payload.picker.selected}
		}
	case deleteConnectionPayload:
		if payload.confirmation != nil {
			copy := *payload.confirmation
			snapshot.deleteConnection = &copy
		}
	case deleteFolderPayload:
		if payload.confirmation != nil {
			copy := *payload.confirmation
			copy.scope.Snapshot = append([]app.NodeRevision(nil), payload.confirmation.scope.Snapshot...)
			snapshot.deleteFolder = &copy
		}
	case connectConfirmationPayload:
		snapshot.connect = snapshotResizeConnectConfirmation(payload.confirmation)
	case unsavedChangesPayload:
		snapshot.unsaved = payload
	case helpPayload:
		snapshot.helpLines = append([]string(nil), payload.lines...)
	case operationErrorPayload:
		snapshot.operationError = snapshotResizeErrorModal(payload.modal)
	case sshFailurePayload:
		snapshot.sshFailure = snapshotResizeErrorModal(payload.modal)
		snapshot.sshConfirmation = snapshotResizeConnectConfirmation(payload.confirmation)
	}
	return snapshot
}

func snapshotResizeModal(modal modalState) resizeModalSnapshot {
	snapshot := resizeModalSnapshot{
		kind: modal.kind, openedFrom: modal.openedFrom,
		payload: snapshotResizeModalPayload(modal.kind, modal.payload), viewport: modal.viewport,
		recoverableError: modal.recoverableError, conflict: snapshotResizeConflict(modal.conflict),
		helpVisible: modal.helpVisible, helpOpenedFrom: modal.helpOpenedFrom,
	}
	if modal.target != nil {
		copy := modal.target.clone()
		snapshot.target = &copy
	}
	return snapshot
}

func snapshotResizeConnectionEdit(edit *connectionEditState) resizeConnectionEditSnapshot {
	if edit == nil {
		return resizeConnectionEditSnapshot{}
	}
	return resizeConnectionEditSnapshot{
		present: true, selectedID: edit.treeContext.selectedID,
		expanded: resizeExpandedIDs(edit.treeContext.expanded), viewport: edit.treeContext.viewport,
		focus: edit.treeContext.focus, target: edit.target.clone(), conflict: snapshotResizeConflict(edit.conflict),
		recovery: snapshotResizeErrorModal(edit.recovery), quitAfterSave: edit.quitAfterSave,
	}
}

func snapshotResizeOperation(operation *operationState) resizeOperationSnapshot {
	if operation == nil {
		return resizeOperationSnapshot{}
	}
	snapshot := resizeOperationSnapshot{
		present: true, id: operation.id, kind: operation.kind, phase: operation.phase,
		owner: operation.owner, stopReason: operation.stopReason, quitIntent: operation.quitIntent,
		pendingConflict: snapshotResizeConflict(operation.pendingConflict), retry: snapshotResizeRetry(operation.retry),
	}
	if operation.target != nil {
		copy := operation.target.clone()
		snapshot.target = &copy
	}
	if operation.commitResult != nil {
		snapshot.commitResult = operation.commitResult.value
	}
	return snapshot
}

func snapshotResizeSecurity(state *securityInputState) resizeSecuritySnapshot {
	if state == nil {
		return resizeSecuritySnapshot{}
	}
	snapshot := resizeSecuritySnapshot{
		present: true, kind: state.kind, preservedFocus: state.preservedFocus,
		viewport: state.viewport, suspended: state.suspended,
	}
	if state.trust != nil {
		copy := *state.trust
		snapshot.trust = &copy
	}
	if state.secret != nil {
		copy := *state.secret
		snapshot.secret = &copy
	}
	return snapshot
}

func resizeExpandedIDs(expanded map[app.NodeID]struct{}) []app.NodeID {
	ids := make([]app.NodeID, 0, len(expanded))
	for id := range expanded {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func cancelResizeConformanceOperation(model *Model) {
	if model.operation != nil && model.operation.cancel != nil {
		model.operation.cancel()
	}
}
