package tui

import (
	"errors"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestUS4ExactModalInventoryAndTypedPayloads(t *testing.T) {
	registry := newModalRegistry()
	if len(registry.payloadTypes) != 10 {
		t.Fatalf("registered modal kinds = %d, want 10", len(registry.payloadTypes))
	}
	connection := testConnection("connection", syntheticRootID, "/connection", 2)
	folder := testFolder("folder", syntheticRootID, "/folder", 3)
	folderForm := newFolderForm(nil, app.ItemSelector{ID: folder.ID})
	folderForm.setDestination(folder)
	editForm := newFolderForm(&folder, app.ItemSelector{})
	failure := newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: 2, Path: connection.Path, Host: connection.Host, Port: connection.Port}, app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", nil).Presentation())
	tests := []struct {
		kind    modalKind
		payload any
	}{
		{modalKindFolderCreate, folderCreatePayload{folderForm}},
		{modalKindFolderEdit, folderEditPayload{editForm}},
		{modalKindMovePicker, movePickerPayload{newMovePicker(connection.Node, []app.Folder{folder})}},
		{modalKindDeleteConnection, deleteConnectionPayload{newDeleteConfirmation(app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Revision: 2})}},
		{modalKindDeleteFolder, deleteFolderPayload{newFolderDeleteConfirmation(app.FolderDeleteScope{ID: folder.ID, Path: folder.Path, Revision: 3})}},
		{modalKindConnectConfirmation, connectConfirmationPayload{newConnectConfirmation(connection)}},
		{modalKindUnsavedChanges, unsavedChangesPayload{intent: unsavedIntentQuit, target: connection.Path}},
		{modalKindHelp, helpPayload{lines: []string{"Esc Close"}}},
		{modalKindOperationError, operationErrorPayload{newErrorModal("reload", "/", app.ErrConflict)}},
		{modalKindSSHFailure, sshFailurePayload{modal: failure}},
	}
	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			opened, err := (modalState{}).open(registry, modalOpenRequest{kind: test.kind, openedFrom: focusOwnerTree, payload: test.payload})
			if err != nil || !opened.isOpen() || !opened.blocksBackgroundInput() {
				t.Fatalf("open = %#v, %v", opened, err)
			}
			if _, err := opened.open(registry, modalOpenRequest{kind: test.kind, openedFrom: focusOwnerModal, payload: test.payload}); !errors.Is(err, errModalNestedOpen) {
				t.Fatalf("nested error = %v", err)
			}
		})
	}
}
