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

func TestOperationKindsOwnExactStatusAndSuppressMutations(t *testing.T) {
	target := capturedTarget{id: "server", revision: 4, kind: app.NodeKindConnection, path: "/prod/server", endpointOrScope: "server.test:22"}
	tests := []struct {
		name   string
		kind   asyncOperationKind
		target *capturedTarget
		status string
	}{
		{name: "initial load", kind: asyncOperationInitialLoad, status: "Loading: Initial load — catalog"},
		{name: "reload", kind: asyncOperationReload, status: "Loading: Reload — catalog"},
		{name: "save", kind: asyncOperationSave, target: &target, status: "Loading: Save — /prod/server"},
		{name: "ssh", kind: asyncOperationSSHStart, target: &target, status: "Loading: SSH start — server.test:22"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadedModel(t, []app.Connection{testConnection("server", syntheticRootID, "/prod/server", 4)}, true)
			id, _, ok := model.beginOperationWith(test.kind, test.target, operationOwnerRoot)
			if !ok || id == 0 {
				t.Fatal("operation did not start")
			}
			view := model.View().Content
			if !strings.Contains(view, test.status) || !strings.Contains(view, "Esc Cancel") || !strings.Contains(view, "? Help") || !strings.Contains(view, "q Quit") {
				t.Fatalf("operation view omitted exact inventory:\n%s", view)
			}
			for _, value := range []string{"n", "f", "e", "m", "d", "r", "c", "j", "tab"} {
				before := model.browser.selectedID
				_, command := model.Update(keyPress(value))
				if command != nil || model.browser.selectedID != before || model.form != nil || model.modal.isOpen() {
					t.Fatalf("key %q escaped operation filtering", value)
				}
			}
		})
	}
}

func TestOperationHelpScrollIsLocalAndCancelQuitWaitForResult(t *testing.T) {
	model := loadedModel(t, nil, true)
	id, _, ok := model.beginOperationWith(asyncOperationReload, nil, operationOwnerRoot)
	if !ok {
		t.Fatal("operation did not start")
	}
	updateModel(model, keyPress("?"))
	updateModel(model, tea.WindowSizeMsg{Width: 40, Height: 12})
	updateModel(model, keyPress("G"))
	if model.modal.kind != modalKindHelp || model.modal.viewport.logicalOffset == 0 || model.focusOwner != focusOwnerModal || model.operation == nil || model.operation.phase != asyncPhaseRunning {
		t.Fatalf("Help-modal navigation changed owner: modal=%#v focus=%v operation=%+v", model.modal, model.focusOwner, model.operation)
	}
	updateModel(model, keyPress("esc"))
	if model.modal.isOpen() || model.focusOwner != focusOwnerTree || model.operation == nil || model.operation.phase != asyncPhaseRunning {
		t.Fatal("Esc from Help canceled the operation instead of closing Help")
	}
	updateModel(model, keyPress("esc"))
	if model.operation == nil || model.operation.phase != asyncPhaseCancelRequested {
		t.Fatal("Cancel released operation ownership before cleanup result")
	}
	updateModel(model, operationResultMsg{id: id, kind: operationReload, err: context.Canceled})
	if model.operation != nil {
		t.Fatal("matching cancellation result did not release operation")
	}

	id, _, _ = model.beginOperationWith(asyncOperationReload, nil, operationOwnerRoot)
	_, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if command != nil || model.operation == nil || !model.operation.quitIntent {
		t.Fatal("Quit did not wait for operation cleanup")
	}
	_, command = model.Update(operationResultMsg{id: id, kind: operationReload, err: context.Canceled})
	if command == nil {
		t.Fatal("matching cleanup result did not complete deferred Quit")
	}
}

func TestSC013HelpPreservesRootFormAndModalOperationOwners20Runs(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) *Model
		modal bool
	}{
		{name: "root", setup: func(t *testing.T) *Model {
			return newOperationConformanceFixture(t, asyncOperationReload).model
		}},
		{name: "form", setup: func(t *testing.T) *Model {
			return newOperationConformanceFixture(t, asyncOperationSave).model
		}},
		{name: "modal", modal: true, setup: func(t *testing.T) *Model {
			model, _, _, connection := newResizeConformanceBase(t)
			attempt := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
			failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", context.DeadlineExceeded).Presentation()
			diagnostic := newSSHFailureModal(attempt, failure)
			diagnostic.detailVisible = true
			diagnostic.recovery = recoveryResolving
			openResizeModal(t, model, modalKindSSHFailure, capturedTargetFromConnection(connection), sshFailurePayload{modal: diagnostic})
			model.modal.viewport = newViewportState(0)
			target := capturedTargetFromConnection(connection)
			if _, _, ok := model.beginOperationWith(asyncOperationReload, &target, operationOwnerModal, catalogRetryIntent(asyncOperationReload, &target, operationOwnerModal)); !ok {
				t.Fatal("modal-owned operation setup failed")
			}
			return model
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for run := range resizeConformanceRuns {
				model := test.setup(t)
				before := snapshotResizeConformanceModel(model)
				beforeOperation := snapshotResizeOperation(model.operation)

				updateModel(model, keyPress("?"))
				if test.modal {
					if !model.modal.helpVisible || model.modal.kind != before.modal.kind || model.focusOwner != focusOwnerModal {
						t.Fatalf("run %d: modal Help did not remain inline with its owner", run+1)
					}
				} else if model.modal.kind != modalKindHelp || model.modal.helpVisible || model.focusOwner != focusOwnerModal || model.modal.openedFrom != before.focus {
					t.Fatalf("run %d: surface Help did not open a standalone modal from %s", run+1, test.name)
				}
				if got := snapshotResizeOperation(model.operation); !reflect.DeepEqual(got, beforeOperation) {
					t.Fatalf("run %d: opening Help changed %s operation owner\n got: %#v\nwant: %#v", run+1, test.name, got, beforeOperation)
				}

				updateModel(model, keyPress("esc"))
				assertResizeConformanceSnapshot(t, model, before, run, test.name+" Help close")
				if model.operation == nil || model.operation.phase != asyncPhaseRunning {
					t.Fatalf("run %d: first Esc canceled %s operation instead of closing Help", run+1, test.name)
				}

				updateModel(model, keyPress("esc"))
				if model.operation == nil || model.operation.phase != asyncPhaseCancelRequested || model.operation.id != beforeOperation.id {
					t.Fatalf("run %d: second Esc did not request %s operation cancellation: %#v", run+1, test.name, model.operation)
				}
				cancelResizeConformanceOperation(model)
			}
		})
	}
}

func TestOperationMatchingCommitConflictAndStaleMatrix(t *testing.T) {
	connection := testConnection("server", syntheticRootID, "/server", 1)
	model := loadedModel(t, []app.Connection{connection}, true)
	before := model.browser.revision
	id, _, _ := model.beginOperationWith(asyncOperationReload, nil, operationOwnerRoot)
	updateModel(model, operationResultMsg{id: id + 1, kind: operationReload, list: app.ListConnectionsResult{CatalogRevision: 99}})
	if model.browser.revision != before || model.operation == nil {
		t.Fatal("stale result mutated or released owner")
	}
	updateModel(model, operationResultMsg{id: id, kind: operationReload, list: app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: before + 1}})
	if model.browser.revision != before+1 || model.operation != nil {
		t.Fatal("matching committed snapshot was not reconciled")
	}

	model.screen = screenConnectionForm
	model.form = newConnectionForm(&connection)
	model.connectionEdit = &connectionEditState{target: model.captureConnectionTarget(connection)}
	id, _, _ = model.beginOperationWith(asyncOperationSave, model.capturedSelectedTarget(), operationOwnerForm)
	updateModel(model, operationResultMsg{id: id, kind: operationUpdate, err: app.ErrConflict})
	if model.operation != nil || model.connectionEdit == nil || model.connectionEdit.conflict == nil || model.form == nil {
		t.Fatal("pre-commit conflict did not preserve form and release owner")
	}

	started := app.ConnectResult{Connection: connection, Session: app.SSHSessionResult{State: app.SessionFailed, Outcome: app.SessionOutcomeTransportFailure, StartedAt: time.Unix(1, 0)}}
	model.connectionEdit.conflict = nil
	id, _, _ = model.beginOperationWith(asyncOperationSSHStart, model.capturedSelectedTarget(), operationOwnerModal)
	_, command := model.Update(sessionFinishedMsg{id: id, attempt: app.SSHAttemptTarget{ID: connection.ID}, result: started, err: errors.New("network interruption")})
	if command == nil || model.sessionResult.Session.StartedAt.IsZero() || model.operation != nil {
		t.Fatal("post-commit SSH result was suppressed or not cleaned up")
	}
}
