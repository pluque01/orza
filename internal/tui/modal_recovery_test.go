package tui

import "testing"

func TestModalHelpShortcutDoesNotChangeEmbeddedRecovery(t *testing.T) {
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	payload := unsavedChangesPayload{intent: unsavedIntentQuit, target: "/captured"}
	if !model.openGenericModal(modalKindUnsavedChanges, nil, payload) {
		t.Fatal("open failed")
	}
	model.modal.recoverableError = "retry safely"
	updateModel(model, keyPress("?"))
	if model.modal.helpVisible || model.modal.kind != modalKindUnsavedChanges || model.modal.recoverableError != "retry safely" {
		t.Fatal("Help changed the modal")
	}
}
