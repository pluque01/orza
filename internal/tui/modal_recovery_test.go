package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestUS4InlineHelpAndEmbeddedRecoveryPreserveOwner(t *testing.T) {
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	payload := unsavedChangesPayload{intent: unsavedIntentQuit, target: "/captured"}
	if !model.openGenericModal(modalKindUnsavedChanges, nil, payload) {
		t.Fatal("open failed")
	}
	original := model.modal.payload
	model.modal.recoverableError = "retry safely"
	updateModel(model, keyPress("?"))
	if !model.modal.helpVisible || !reflect.DeepEqual(model.modal.payload, original) || model.modal.kind != modalKindUnsavedChanges {
		t.Fatal("Help replaced or changed modal owner")
	}
	for range 20 {
		updateModel(model, keyPress("j"))
	}
	view := model.View().Content
	if strings.Contains(view, viewportPreviousLabel) || strings.Contains(view, viewportNextLabel) || !strings.Contains(view, "█") {
		t.Fatalf("bounded inline Help did not use its scrollbar: %q", view)
	}
	updateModel(model, keyPress("esc"))
	if model.modal.helpVisible || !reflect.DeepEqual(model.modal.payload, original) || model.modal.recoverableError != "retry safely" {
		t.Fatal("Esc did not restore payload/error")
	}
}
