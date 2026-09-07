package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestConnectionFormConflictMissingAndRevisionChangedTwentyRuns(t *testing.T) {
	for _, kind := range []conflictType{conflictTypeMissing, conflictTypeRevisionChanged} {
		name := "missing"
		if kind == conflictTypeRevisionChanged {
			name = "revision changed"
		}
		t.Run(name, func(t *testing.T) {
			for run := range 20 {
				first := testConnection("first", syntheticRootID, "/first", 4)
				second := testConnection("second", syntheticRootID, "/second", 9)
				updates := 0
				model := loadedModel(t, []app.Connection{first, second}, true)
				model.connections = ConnectionFuncs{UpdateFunc: func(_ context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
					updates++
					if request.Connection.ID != first.ID || request.Expected == nil || *request.Expected != first.Revision {
						t.Fatalf("run %d: save retargeted request: %#v", run, request)
					}
					return app.ConnectionResult{}, app.ErrConflict
				}}
				updateModel(model, keyPress("l"))
				updateModel(model, keyPress("e"))
				form := model.form
				form.inputs[fieldHost].SetValue("pending.test")
				model.browser.selectedID = second.ID
				_, save := model.Update(keyPress("ctrl+s"))
				updateModel(model, save())
				if kind == conflictTypeMissing {
					model.installFormConflict(conflictTypeMissing)
				}
				conflict := model.connectionEdit.conflict
				if updates != 1 || conflict == nil || conflict.kind != kind || conflict.target.id != first.ID || conflict.target.revision != first.Revision || model.form != form {
					t.Fatalf("run %d: captured conflict = %#v", run, conflict)
				}
				_, blocked := model.Update(keyPress("ctrl+s"))
				if blocked != nil || updates != 1 || model.form != form {
					t.Fatalf("run %d: blocked Save performed persistence", run)
				}
			}
		})
	}
}

func TestConnectionFormConflictReloadBackCancelRecoveryTwentyRuns(t *testing.T) {
	for run := range 20 {
		root := testFolder("root", "", "/", 1)
		folder := testFolder("folder", root.ID, "/folder", 1)
		connection := testConnection("connection", folder.ID, "/folder/connection", 4)
		snapshot := newCatalogSnapshot(root, 1)
		_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}})
		_ = snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
		model := New(Config{Width: 80, Height: 24, NoColor: true})
		model.browser.setSnapshot(snapshot, connection.ID)
		model.openConnectionForm(newConnectionForm(&connection), model.captureConnectionTarget(connection))
		model.form.inputs[fieldHost].SetValue("pending.test")
		model.installFormConflict(conflictTypeRevisionChanged)

		updateModel(model, keyPress("esc"))
		if conflict := model.connectionEdit.conflict; conflict == nil || conflict.detailVisible || !conflict.blocked {
			t.Fatalf("run %d: Cancel did not leave compact blocked banner", run)
		}
		view := model.View().Content
		for _, want := range []string{"Save blocked", "r Reload", "b Back", "Esc Cancel"} {
			if !strings.Contains(view, want) {
				t.Fatalf("run %d: compact conflict omitted %q: %q", run, want, view)
			}
		}

		model.installFormConflict(conflictTypeRevisionChanged)
		updateModel(model, keyPress("b"))
		if model.form != nil || model.browser.selectedID != folder.ID || model.focusOwner != focusOwnerTree {
			t.Fatalf("run %d: Back selection = %q", run, model.browser.selectedID)
		}
	}
}

func TestConnectionFormConflictReloadRechecksSameIDWithoutRetargetTwentyRuns(t *testing.T) {
	for run := range 20 {
		captured := testConnection("captured", syntheticRootID, "/captured", 4)
		other := testConnection("other", syntheticRootID, "/other", 1)
		observed := captured
		observed.Revision = 5
		model := loadedModel(t, []app.Connection{captured, other}, true)
		updateModel(model, keyPress("l"))
		updateModel(model, keyPress("e"))
		form := model.form
		form.inputs[fieldHost].SetValue("pending.test")
		form.setFocus(fieldHost)
		model.installFormConflict(conflictTypeRevisionChanged)
		model.browser.selectedID = other.ID
		model.connections = ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{Connections: []app.Connection{observed, other}, CatalogRevision: 8}, nil
		}}

		_, reload := model.Update(keyPress("r"))
		if reload == nil {
			t.Fatalf("run %d: Reload did not start", run)
		}
		updateModel(model, reload())
		if model.form != form || form.inputs[fieldHost].Value() != "pending.test" || form.focusedField() != fieldHost || model.connectionEdit.conflict == nil || model.connectionEdit.conflict.target.id != captured.ID {
			t.Fatalf("run %d: unresolved Reload retargeted or lost form", run)
		}

		model.connections = ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
			return app.ListConnectionsResult{Connections: []app.Connection{captured, other}, CatalogRevision: 9}, nil
		}}
		_, reload = model.Update(keyPress("r"))
		updateModel(model, reload())
		if model.connectionEdit.conflict != nil || model.form != form || model.browser.selectedID == other.ID {
			t.Fatalf("run %d: same-ID/revision Reload did not resolve captured target", run)
		}
	}
}
