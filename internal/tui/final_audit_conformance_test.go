package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestSC009OperationErrorRetryFolderDeleteScopeAndSSHStart20Runs(t *testing.T) {
	for _, retryFailure := range []bool{false, true} {
		outcome := "success"
		if retryFailure {
			outcome = "failure"
		}
		t.Run("folder_delete_scope/"+outcome, func(t *testing.T) {
			for run := range sc009Runs {
				root := testFolder("root", "", "/", 1)
				folder := testFolder("captured", root.ID, "/captured", 3)
				calls := 0
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				sc009Snapshot(model, root, []app.Folder{folder}, nil, folder.ID)
				model.folders = FolderFuncs{DeleteScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.FolderDeleteScope, error) {
					calls++
					if selector.ID != folder.ID {
						t.Fatalf("run %d: folder retry retargeted to %q", run, selector.ID)
					}
					if calls == 1 || retryFailure {
						return app.FolderDeleteScope{}, errors.New("controlled folder scope failure")
					}
					return app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: folder.Revision, Folders: 1}, nil
				}}

				sc009RunCommand(t, model, sc009Update(t, model, keyPress("d")))
				assertAuditRetryPanel(t, model, operationRetryFolderDeleteScope, folder.ID, 1, calls, run)
				selectSCNode(model, root.ID)
				sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
				if retryFailure {
					assertAuditRetryPanel(t, model, operationRetryFolderDeleteScope, folder.ID, 2, calls, run)
				} else if calls != 2 || model.modal.kind != modalKindDeleteFolder || model.modal.target == nil || model.modal.target.id != folder.ID {
					t.Fatalf("run %d: folder retry success calls=%d modal=%#v", run, calls, model.modal)
				}
			}
		})

		t.Run("ssh_start/"+outcome, func(t *testing.T) {
			for run := range sc009Runs {
				captured := testConnection("captured", syntheticRootID, "/captured", 3)
				other := testConnection("other", syntheticRootID, "/other", 8)
				calls := 0
				model := loadedModel(t, []app.Connection{captured, other}, true)
				model.connect = ConnectFunc(func(_ context.Context, request app.ConnectRequest) (app.ConnectResult, error) {
					calls++
					if request.Connection.ID != captured.ID || request.Expected == nil || *request.Expected != captured.Revision {
						t.Fatalf("run %d: SSH retry request = %#v", run, request)
					}
					if calls == 1 || retryFailure {
						return app.ConnectResult{Attempt: auditAttempt(captured)}, errors.New("controlled SSH start failure")
					}
					return app.ConnectResult{Connection: captured, Attempt: auditAttempt(captured), Session: app.SSHSessionResult{State: app.SessionSucceeded, StartedAt: time.Unix(1, 0)}}, nil
				})
				selectSCNode(model, captured.ID)
				sc009Update(t, model, keyPress("c"))
				if command := sc009Update(t, model, keyPress("y")); command == nil {
					t.Fatalf("run %d: SSH start returned no command", run)
				}
				auditCompleteSSHService(t, model, captured)
				assertAuditRetryPanel(t, model, operationRetrySSHStart, captured.ID, 1, calls, run)
				selectSCNode(model, other.ID)
				retry := sc009Update(t, model, keyPress("r"))
				if retry == nil {
					t.Fatalf("run %d: SSH Retry returned no command", run)
				}
				if retryFailure {
					auditCompleteSSHService(t, model, captured)
					assertAuditRetryPanel(t, model, operationRetrySSHStart, captured.ID, 2, calls, run)
				} else {
					auditCompleteSSHService(t, model, captured)
					if calls != 2 || model.operation != nil || model.sessionResult.Connection.ID != captured.ID {
						t.Fatalf("run %d: SSH retry success calls=%d operation=%#v result=%#v", run, calls, model.operation, model.sessionResult)
					}
				}
			}
		})
	}
}

func TestSC012ConnectConfirmationConflictRecovery20Runs(t *testing.T) {
	conflicts := []struct {
		name string
		err  error
	}{
		{name: "missing", err: app.ErrNotFound},
		{name: "revision_changed", err: app.ErrConflict},
	}
	for _, conflict := range conflicts {
		for _, action := range []string{"reload", "back", "esc"} {
			t.Run(conflict.name+"/"+action, func(t *testing.T) {
				for run := range sc012ConflictRuns {
					captured := testConnection("captured", syntheticRootID, "/captured", 3)
					other := testConnection("other", syntheticRootID, "/other", 8)
					current := captured
					current.Revision++
					connectCalls, reloadCalls := 0, 0
					model := loadedModel(t, []app.Connection{captured, other}, true)
					model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
						connectCalls++
						return app.ConnectResult{Attempt: auditAttempt(captured)}, conflict.err
					})
					model.connections = ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
						reloadCalls++
						values := []app.Connection{other}
						if conflict.err == app.ErrConflict {
							values = append(values, current)
						}
						return app.ListConnectionsResult{Connections: values, CatalogRevision: 9}, nil
					}}
					selectSCNode(model, captured.ID)
					sc009Update(t, model, keyPress("c"))
					payload := model.modal.payload
					if command := sc009Update(t, model, keyPress("y")); command == nil {
						t.Fatalf("run %d: connect confirmation returned no command", run)
					}
					auditCompleteSSHService(t, model, captured)
					if connectCalls != 1 || model.modal.kind != modalKindConnectConfirmation || model.modal.conflict == nil || model.modal.target == nil || model.modal.target.id != captured.ID || !reflect.DeepEqual(model.modal.payload, payload) {
						t.Fatalf("run %d: connect conflict lost panel/target/payload: calls=%d modal=%#v", run, connectCalls, model.modal)
					}
					switch action {
					case "reload":
						sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
						if reloadCalls != 1 || model.modal.kind != modalKindConnectConfirmation || model.modal.conflict == nil || !reflect.DeepEqual(model.modal.payload, payload) || model.focusOwner != focusOwnerModal {
							t.Fatalf("run %d: connect Reload changed owner: reloads=%d modal=%#v", run, reloadCalls, model.modal)
						}
					case "back":
						sc009Update(t, model, keyPress("b"))
						if model.modal.isOpen() || model.browser.selectedID != syntheticRootID || model.focusOwner != focusOwnerTree || reloadCalls != 0 {
							t.Fatalf("run %d: connect Back result modal=%v selection=%q focus=%v reloads=%d", run, model.modal.isOpen(), model.browser.selectedID, model.focusOwner, reloadCalls)
						}
					case "esc":
						sc009Update(t, model, keyPress("esc"))
						if model.modal.conflict == nil || model.modal.conflict.detailVisible || !model.modal.conflict.blocked || !reflect.DeepEqual(model.modal.payload, payload) || reloadCalls != 0 {
							t.Fatalf("run %d: connect Esc lost compact conflict/payload", run)
						}
					}
					if connectCalls != 1 {
						t.Fatalf("run %d: recovery started SSH %d times", run, connectCalls)
					}
				}
			})
		}
	}
}

func TestSC012UnsavedConflictRecovery20Runs(t *testing.T) {
	for _, conflict := range []struct {
		name string
		err  error
	}{
		{name: "missing", err: app.ErrNotFound},
		{name: "revision_changed", err: app.ErrConflict},
	} {
		for _, action := range []string{"reload", "back", "esc"} {
			t.Run(conflict.name+"/"+action, func(t *testing.T) {
				for run := range sc012ConflictRuns {
					captured := testConnection("captured", syntheticRootID, "/captured", 3)
					other := testConnection("other", syntheticRootID, "/other", 8)
					current := captured
					current.Revision++
					updates, reloads := 0, 0
					model := loadedModel(t, []app.Connection{captured, other}, true)
					model.connections = ConnectionFuncs{
						UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
							updates++
							if request.Connection.ID != captured.ID {
								t.Fatalf("run %d: unsaved Save retargeted to %q", run, request.Connection.ID)
							}
							return app.ConnectionResult{}, conflict.err
						},
						ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
							reloads++
							values := []app.Connection{other}
							if conflict.err == app.ErrConflict {
								values = append(values, current)
							}
							return app.ListConnectionsResult{Connections: values, CatalogRevision: 9}, nil
						},
					}
					selectSCNode(model, captured.ID)
					sc009Update(t, model, keyPress("e"))
					form := model.form
					form.inputs[fieldHost].SetValue("pending.test")
					form.setFocus(fieldHost)
					sc009Update(t, model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
					payload := model.modal.payload
					sc009RunCommand(t, model, sc009Update(t, model, keyPress("s")))
					if updates != 1 || model.modal.kind != modalKindUnsavedChanges || model.modal.conflict == nil || model.form != form || form.inputs[fieldHost].Value() != "pending.test" || form.focusedField() != fieldHost || !reflect.DeepEqual(model.modal.payload, payload) {
						t.Fatalf("run %d: unsaved conflict lost panel/form/value/focus: updates=%d modal=%#v", run, updates, model.modal)
					}
					switch action {
					case "reload":
						sc009RunCommand(t, model, sc009Update(t, model, keyPress("r")))
						if reloads != 1 || model.modal.kind != modalKindUnsavedChanges || model.modal.conflict == nil || model.form != form || form.inputs[fieldHost].Value() != "pending.test" || form.focusedField() != fieldHost {
							t.Fatalf("run %d: unsaved Reload changed retained state", run)
						}
					case "back":
						sc009Update(t, model, keyPress("b"))
						if model.modal.isOpen() || model.form != nil || model.browser.selectedID != syntheticRootID || model.focusOwner != focusOwnerTree || reloads != 0 {
							t.Fatalf("run %d: unsaved Back did not abandon interaction", run)
						}
					case "esc":
						sc009Update(t, model, keyPress("esc"))
						if model.modal.conflict == nil || model.modal.conflict.detailVisible || !model.modal.conflict.blocked || model.form != form || reloads != 0 {
							t.Fatalf("run %d: unsaved Esc lost compact conflict/form", run)
						}
					}
					if updates != 1 {
						t.Fatalf("run %d: recovery persisted %d times", run, updates)
					}
				}
			})
		}
	}
}

func TestSC013MatchingOwnerResultAfterEsc20Runs(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		outcome := "success"
		if conflict {
			outcome = "conflict"
		}
		for _, kind := range []asyncOperationKind{asyncOperationInitialLoad, asyncOperationReload, asyncOperationSave, asyncOperationSSHStart} {
			name := map[asyncOperationKind]string{asyncOperationInitialLoad: "initial_load", asyncOperationReload: "reload", asyncOperationSave: "save", asyncOperationSSHStart: "ssh_start"}[kind]
			t.Run(name+"/"+outcome, func(t *testing.T) {
				for run := range operationConformanceRuns {
					auditMatchingOwnerAfterEsc(t, kind, conflict, run)
				}
			})
		}
	}
}

func TestSC015RemoteStreamAbsentFromTUIModelModalAndDiagnostics(t *testing.T) {
	const remoteStreamCanary = "SC015-REMOTE-STREAM-BYTE-CANARY-7f3a"
	connection := testConnection("stream-target", syntheticRootID, "/stream-target", 3)
	model := loadedModel(t, []app.Connection{connection}, true)
	var remoteStdout, remoteStderr bytes.Buffer
	model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
		_, _ = remoteStdout.WriteString(remoteStreamCanary)
		_, _ = remoteStderr.WriteString(remoteStreamCanary)
		return app.ConnectResult{
			Connection: connection,
			Attempt:    auditAttempt(connection),
			Session: app.SSHSessionResult{
				State: app.SessionFailed, Outcome: app.SessionOutcomeTransportFailure, StartedAt: time.Unix(1, 0),
			},
		}, errors.New("network interruption")
	})
	selectSCNode(model, connection.ID)
	sc009Update(t, model, keyPress("c"))
	sc009RunExec(t, model, sc009Update(t, model, keyPress("y")))

	if remoteStdout.String() != remoteStreamCanary || remoteStderr.String() != remoteStreamCanary {
		t.Fatal("controlled remote stream did not reach its external writers")
	}
	if model.operation != nil || model.modal.isOpen() {
		t.Fatalf("completed active stream retained operation/modal state: operation=%#v modal=%#v", model.operation, model.modal)
	}
	if sc015ValueContains(reflect.ValueOf(model), remoteStreamCanary, make(map[sc015Visit]struct{})) {
		t.Fatal("TUI model retained remote stream content")
	}
	inspected := []string{
		model.View().Content,
		model.status,
		strings.Join(model.detailState.content(512), "\n"),
		fmt.Sprintf("%+v", model.sessionResult.Connection),
		fmt.Sprintf("%+v", model.sessionResult.Attempt),
		fmt.Sprintf("%+v", model.sessionResult.Session),
	}
	if model.sessionErr != nil {
		inspected = append(inspected, model.sessionErr.Error())
	}
	for index, value := range inspected {
		if strings.Contains(value, remoteStreamCanary) {
			t.Fatalf("TUI inspection surface %d retained remote stream content", index)
		}
	}

	failure := app.NewSSHStartError(
		app.SSHFailureUnexpected,
		app.SSHFailureStageUnknown,
		remoteStreamCanary,
		errors.New("raw cause "+remoteStreamCanary),
	).Presentation()
	modal := newSSHFailureModal(auditAttempt(connection), failure)
	modal.detailVisible = true
	diagnostic := strings.Join(sshFailureLines(modal, 512), "\n")
	if sc015ValueContains(reflect.ValueOf(modal), remoteStreamCanary, make(map[sc015Visit]struct{})) || strings.Contains(diagnostic, remoteStreamCanary) || failure.TechnicalDetail != "" {
		t.Fatalf("modal/diagnostic retained uncontrolled remote content: %q / %#v", diagnostic, failure)
	}
}

type sc015Visit struct {
	kind reflect.Kind
	ptr  uintptr
}

func sc015ValueContains(value reflect.Value, canary string, visited map[sc015Visit]struct{}) bool {
	if !value.IsValid() {
		return false
	}
	switch value.Kind() {
	case reflect.String:
		return strings.Contains(value.String(), canary)
	case reflect.Interface:
		return !value.IsNil() && sc015ValueContains(value.Elem(), canary, visited)
	case reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return false
		}
		visit := sc015Visit{kind: value.Kind(), ptr: value.Pointer()}
		if _, seen := visited[visit]; seen {
			return false
		}
		visited[visit] = struct{}{}
	}
	switch value.Kind() {
	case reflect.Pointer:
		return sc015ValueContains(value.Elem(), canary, visited)
	case reflect.Struct:
		for index := range value.NumField() {
			if sc015ValueContains(value.Field(index), canary, visited) {
				return true
			}
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if sc015ValueContains(iterator.Key(), canary, visited) || sc015ValueContains(iterator.Value(), canary, visited) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for index := range value.Len() {
			if sc015ValueContains(value.Index(index), canary, visited) {
				return true
			}
		}
	}
	return false
}

func auditMatchingOwnerAfterEsc(t *testing.T, kind asyncOperationKind, conflict bool, run int) {
	t.Helper()
	connection := testConnection("captured", syntheticRootID, "/captured", 3)
	saved := connection
	saved.Host = "saved.test"
	saved.Revision++
	calls, reconcileCalls := 0, 0
	resultErr := error(nil)
	if conflict {
		resultErr = app.ErrConflict
	}
	var model *Model
	var command tea.Cmd
	switch kind {
	case asyncOperationInitialLoad:
		model = New(Config{Width: 80, Height: 24, NoColor: true, Connections: ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			calls++
			return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 9}, resultErr
		}}})
		command = model.Init()
	case asyncOperationReload:
		model = loadedModel(t, []app.Connection{connection}, true)
		model.connections = ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			calls++
			return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 9}, resultErr
		}}
		command = sc009Update(t, model, keyPress("r"))
	case asyncOperationSave:
		model = loadedModel(t, []app.Connection{connection}, true)
		model.connections = ConnectionFuncs{
			UpdateFunc: func(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error) {
				calls++
				return app.ConnectionResult{Connection: saved, CatalogRevision: 9}, resultErr
			},
			ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
				reconcileCalls++
				return app.ListConnectionsResult{Connections: []app.Connection{saved}, CatalogRevision: 9}, nil
			},
		}
		selectSCNode(model, connection.ID)
		sc009Update(t, model, keyPress("e"))
		model.form.inputs[fieldHost].SetValue(saved.Host)
		command = sc009Update(t, model, keyPress("ctrl+s"))
	case asyncOperationSSHStart:
		model = loadedModel(t, []app.Connection{connection}, true)
		model.connect = ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
			calls++
			result := app.ConnectResult{Connection: connection, Attempt: auditAttempt(connection)}
			if !conflict {
				result.Session = app.SSHSessionResult{State: app.SessionSucceeded, StartedAt: time.Unix(1, 0)}
			}
			return result, resultErr
		})
		selectSCNode(model, connection.ID)
		sc009Update(t, model, keyPress("c"))
		command = sc009Update(t, model, keyPress("y"))
	}
	if command == nil || model.operation == nil {
		t.Fatalf("run %d: operation %v did not start", run, kind)
	}
	id := model.operation.id
	sc009Update(t, model, keyPress("esc"))
	if model.operation == nil || model.operation.id != id || model.operation.phase != asyncPhaseCancelRequested || model.operation.quitIntent {
		t.Fatalf("run %d: ordinary Esc was not a non-Quit cancellation: %#v", run, model.operation)
	}
	before := captureOperationConformanceState(model)
	var stale tea.Msg = operationResultMsg{id: id + 1, kind: operationReload}
	if kind == asyncOperationSave {
		stale = operationResultMsg{id: id + 1, kind: operationUpdate}
	} else if kind == asyncOperationSSHStart {
		stale = sessionFinishedMsg{id: id + 1, attempt: auditAttempt(connection)}
	}
	sc009Update(t, model, stale)
	if calls != 0 || !reflect.DeepEqual(captureOperationConformanceState(model), before) {
		t.Fatalf("run %d: stale result changed canceled owner or called service", run)
	}

	if kind == asyncOperationSSHStart {
		auditCompleteSSHService(t, model, connection)
	} else {
		next := sc009RunCommand(t, model, command)
		if kind == asyncOperationSave && !conflict {
			sc009RunCommand(t, model, next)
		}
	}
	if calls != 1 || model.operation != nil {
		t.Fatalf("run %d: matching owner result calls=%d operation=%#v", run, calls, model.operation)
	}
	if conflict {
		switch kind {
		case asyncOperationSave:
			if model.connectionEdit == nil || model.connectionEdit.conflict == nil || model.form == nil {
				t.Fatalf("run %d: matching Save conflict was not published", run)
			}
		case asyncOperationSSHStart:
			if model.modal.kind != modalKindConnectConfirmation || model.modal.conflict == nil {
				t.Fatalf("run %d: matching SSH conflict was not published in confirmation", run)
			}
		default:
			if model.modal.kind != modalKindOperationError {
				t.Fatalf("run %d: matching catalog conflict did not publish an error panel", run)
			}
		}
		if reconcileCalls != 0 {
			t.Fatalf("run %d: pre-commit conflict reconciled %d times", run, reconcileCalls)
		}
		return
	}
	if kind == asyncOperationSave && reconcileCalls != 1 {
		t.Fatalf("run %d: matching Save success reconciliation calls=%d", run, reconcileCalls)
	}
	if kind == asyncOperationSSHStart && model.sessionResult.Session.StartedAt.IsZero() {
		t.Fatalf("run %d: matching SSH success was not committed", run)
	}
}

func assertAuditRetryPanel(t *testing.T, model *Model, kind operationRetryKind, target app.NodeID, wantCalls, gotCalls, run int) {
	t.Helper()
	payload, ok := model.modal.payload.(operationErrorPayload)
	if !ok || payload.modal.retry == nil || payload.modal.retry.kind != kind || model.modal.kind != modalKindOperationError || model.modal.target == nil || model.modal.target.id != target || model.operation != nil || gotCalls != wantCalls {
		t.Fatalf("run %d: retry panel calls=%d/%d modal=%#v", run, gotCalls, wantCalls, model.modal)
	}
}

func auditAttempt(connection app.Connection) app.SSHAttemptTarget {
	return app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
}

func auditCompleteSSHService(t *testing.T, model *Model, connection app.Connection) {
	t.Helper()
	if model.operation == nil || model.operation.kind != asyncOperationSSHStart {
		t.Fatal("SSH operation is not active")
	}
	id := model.operation.id
	expected := connection.Revision
	operationCtx := model.operation.ctx
	result, err := model.connect.Connect(operationCtx, app.ConnectRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: &expected})
	sc009Update(t, model, sessionFinishedMsg{id: id, attempt: auditAttempt(connection), result: result, err: err})
}
