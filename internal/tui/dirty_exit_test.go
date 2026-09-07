package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestDirtyCancelIntentSaveDiscardCancelTwentyRuns(t *testing.T) {
	for run := range 20 {
		connection := testConnection("connection", syntheticRootID, "/connection", 1)
		model := loadedModel(t, []app.Connection{connection}, true)
		updateModel(model, keyPress("l"))
		updateModel(model, keyPress("e"))
		model.form.inputs[fieldHost].SetValue("pending.test")
		form := model.form

		updateModel(model, keyPress("esc"))
		payload, ok := model.modal.payload.(unsavedChangesPayload)
		if !ok || payload.intent != unsavedIntentCancel || model.focusOwner != focusOwnerModal {
			t.Fatalf("run %d: cancel prompt = %#v", run, model.modal)
		}
		updateModel(model, keyPress("esc"))
		if model.form != form || model.form.inputs[fieldHost].Value() != "pending.test" || model.focusOwner != focusOwnerConnectionForm {
			t.Fatalf("run %d: Cancel did not restore the form", run)
		}

		updateModel(model, keyPress("esc"))
		_, command := model.Update(keyPress("d"))
		if command != nil || model.form != nil || model.screen != screenBrowser || model.browser.selectedID != connection.ID {
			t.Fatalf("run %d: Discard did not restore captured tree context", run)
		}
	}
}

func TestDirtyQuitChoiceMatrixTwentyRuns(t *testing.T) {
	for run := range 20 {
		t.Run("cancel", func(t *testing.T) {
			model, _ := dirtyEditModel(t, nil)
			form := model.form
			updateModel(model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
			payload, ok := model.modal.payload.(unsavedChangesPayload)
			if !ok || payload.intent != unsavedIntentQuit {
				t.Fatalf("run %d: quit prompt = %#v", run, model.modal)
			}
			updateModel(model, keyPress("esc"))
			if model.form != form || model.modal.isOpen() {
				t.Fatalf("run %d: Cancel did not return to dirty form", run)
			}
		})

		t.Run("discard", func(t *testing.T) {
			model, _ := dirtyEditModel(t, nil)
			updateModel(model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
			_, command := model.Update(keyPress("d"))
			if command == nil || model.form != nil || model.operation != nil {
				t.Fatalf("run %d: Discard did not quit immediately", run)
			}
		})

		t.Run("save", func(t *testing.T) {
			var persisted app.Connection
			model, connection := dirtyEditModel(t, nil)
			model.connections = ConnectionFuncs{
				UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
					persisted = connectionWithUpdate(connection, request)
					return app.ConnectionResult{Connection: persisted, CatalogRevision: 8}, nil
				},
				ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
					return app.ListConnectionsResult{Connections: []app.Connection{persisted}, CatalogRevision: 8}, nil
				},
			}
			updateModel(model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
			_, save := model.Update(keyPress("s"))
			if save == nil || model.operation == nil || !model.connectionEdit.quitAfterSave {
				t.Fatalf("run %d: Save did not own deferred quit", run)
			}
			saveID := model.operation.id
			updateModel(model, operationResultMsg{id: saveID + 1, kind: operationUpdate, connection: app.ConnectionResult{Connection: connection}})
			if model.operation == nil || model.form == nil {
				t.Fatalf("run %d: stale save result changed state", run)
			}
			_, reconcile := model.Update(save())
			if reconcile == nil || model.operation == nil || model.operation.kind != asyncOperationReload || model.form != nil {
				t.Fatalf("run %d: commit did not enter reconciliation", run)
			}
			_, quit := model.Update(reconcile())
			if quit == nil || model.operation != nil || model.browser.selectedID != persisted.ID {
				t.Fatalf("run %d: quit preceded reconciliation/cleanup", run)
			}
		})
	}
}

func TestDirtyQuitSaveFailureMatrixTwentyRuns(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "persistence", err: errors.New("private persistence failure")},
		{name: "conflict", err: app.ErrConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for run := range 20 {
				model, connection := dirtyEditModel(t, func(app.UpdateConnectionRequest) (app.ConnectionResult, error) {
					return app.ConnectionResult{}, test.err
				})
				form := model.form
				focus := form.focusedField()
				updateModel(model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
				_, save := model.Update(keyPress("s"))
				if save == nil {
					t.Fatalf("run %d: Save did not start", run)
				}
				_, quit := model.Update(save())
				if quit != nil || model.form != form || form.focusedField() != focus || model.operation != nil || model.modal.kind != modalKindUnsavedChanges || model.focusOwner != focusOwnerModal {
					t.Fatalf("run %d: failed Save changed owner or exited", run)
				}
				payload, ok := model.modal.payload.(unsavedChangesPayload)
				if !ok || payload.intent != unsavedIntentQuit || payload.target != connection.Path {
					t.Fatalf("run %d: failed Save changed unsaved intent/target: %#v", run, model.modal)
				}
				if test.err == app.ErrConflict {
					if model.connectionEdit.conflict == nil || model.modal.conflict == nil || !strings.Contains(model.View().Content, "Warning:") {
						t.Fatalf("run %d: conflict recovery is not visible", run)
					}
				} else if form.formError == "" || model.modal.recoverableError == "" || !strings.Contains(model.View().Content, "Save failed safely") {
					t.Fatalf("run %d: persistence error is not inline", run)
				}
			}
		})
	}
}

func TestDirtyQuitValidationFailureTwentyRuns(t *testing.T) {
	for run := range 20 {
		model := loadedModel(t, nil, true)
		updateModel(model, keyPress("n"))
		model.form.inputs[fieldName].SetValue("bad/name")
		form := model.form
		updateModel(model, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
		_, command := model.Update(keyPress("s"))
		if command != nil || model.form != form || model.modal.kind != modalKindUnsavedChanges || model.operation != nil || form.errors[fieldName] == "" || model.modal.recoverableError == "" {
			t.Fatalf("run %d: validation failure did not remain in the unsaved panel", run)
		}
	}
}

func dirtyEditModel(t *testing.T, update func(app.UpdateConnectionRequest) (app.ConnectionResult, error)) (*Model, app.Connection) {
	t.Helper()
	connection := testConnection("connection", syntheticRootID, "/connection", 1)
	service := ConnectionFuncs{
		ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 7}, nil
		},
	}
	if update != nil {
		service.UpdateFunc = func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
			return update(request)
		}
	}
	model := New(Config{Connections: service, Width: 80, Height: 24, NoColor: true})
	updateModel(model, model.Init()())
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("e"))
	model.form.inputs[fieldHost].SetValue("pending.test")
	return model, connection
}

func connectionWithUpdate(connection app.Connection, request app.UpdateConnectionRequest) app.Connection {
	if request.Host != nil {
		connection.Host = *request.Host
	}
	connection.Revision++
	return connection
}
