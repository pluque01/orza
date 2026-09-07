package tui

import (
	"errors"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

type modalStateTestPayload struct {
	selected string
}

type modalStateOtherPayload struct{}

func TestModalStateTypedPayloadRegistration(t *testing.T) {
	request := modalOpenRequest{
		kind:       modalKindMovePicker,
		openedFrom: focusOwnerTree,
		payload:    modalStateTestPayload{selected: "destination"},
	}

	var state modalState
	if _, err := state.open(modalRegistry{}, request); !errors.Is(err, errModalKindUnregistered) {
		t.Fatalf("unregistered open error = %v, want %v", err, errModalKindUnregistered)
	}
	unknown := request
	unknown.kind = modalKind("plugin_modal")
	if _, err := state.open(modalRegistry{}, unknown); !errors.Is(err, errModalUnknownKind) {
		t.Fatalf("unknown open error = %v, want %v", err, errModalUnknownKind)
	}

	registry, err := registerModalPayload[modalStateTestPayload](modalRegistry{}, modalKindMovePicker)
	if err != nil {
		t.Fatalf("register payload: %v", err)
	}
	opened, err := state.open(registry, request)
	if err != nil {
		t.Fatalf("open registered payload: %v", err)
	}
	if got, ok := opened.payload.(modalStateTestPayload); !ok || got.selected != "destination" {
		t.Fatalf("payload = %#v, want typed payload", opened.payload)
	}

	request.payload = modalStateOtherPayload{}
	if _, err := state.open(registry, request); !errors.Is(err, errModalPayloadType) {
		t.Fatalf("wrong payload error = %v, want %v", err, errModalPayloadType)
	}
	if _, err := registerModalPayload[modalStateTestPayload](registry, modalKind("plugin_modal")); !errors.Is(err, errModalUnknownKind) {
		t.Fatalf("unknown registration error = %v, want %v", err, errModalUnknownKind)
	}
}

func TestModalStateSingleSlotIsolatesInputAndRestoresOpener(t *testing.T) {
	registry, err := registerModalPayload[modalStateTestPayload](modalRegistry{}, modalKindDeleteConnection)
	if err != nil {
		t.Fatal(err)
	}
	if (modalState{}).blocksBackgroundInput() {
		t.Fatal("closed modal blocked background input")
	}

	target := capturedTarget{
		id:          "connection-id",
		revision:    7,
		kind:        app.NodeKindConnection,
		path:        "/work/prod",
		ancestorIDs: []app.NodeID{"folder-id"},
	}
	opened, err := (modalState{}).open(registry, modalOpenRequest{
		kind:       modalKindDeleteConnection,
		openedFrom: focusOwnerDetail,
		target:     &target,
		payload:    modalStateTestPayload{selected: "cancel"},
		viewport:   newViewportState(3),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !opened.blocksBackgroundInput() {
		t.Fatal("open modal did not isolate background input")
	}

	closed, restored, err := opened.close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if restored != focusOwnerDetail {
		t.Fatalf("restored focus = %v, want detail", restored)
	}
	if closed.isOpen() || closed.blocksBackgroundInput() || closed.payload != nil || closed.target != nil {
		t.Fatalf("closed slot retained modal state: %#v", closed)
	}
}

func TestModalStateRejectsNestedAndReplacementOpen(t *testing.T) {
	registry, err := registerModalPayload[modalStateTestPayload](modalRegistry{}, modalKindFolderCreate)
	if err != nil {
		t.Fatal(err)
	}
	registry, err = registerModalPayload[modalStateOtherPayload](registry, modalKindHelp)
	if err != nil {
		t.Fatal(err)
	}
	original, err := (modalState{}).open(registry, modalOpenRequest{
		kind:       modalKindFolderCreate,
		openedFrom: focusOwnerTree,
		payload:    modalStateTestPayload{selected: "name"},
	})
	if err != nil {
		t.Fatal(err)
	}

	nested, err := original.open(registry, modalOpenRequest{
		kind:       modalKindFolderCreate,
		openedFrom: focusOwnerModal,
		payload:    modalStateTestPayload{},
	})
	if !errors.Is(err, errModalNestedOpen) || nested.payload != original.payload {
		t.Fatalf("nested open = (%#v, %v), want unchanged state and nested error", nested, err)
	}
	replaced, err := original.open(registry, modalOpenRequest{
		kind:       modalKindHelp,
		openedFrom: focusOwnerModal,
		payload:    modalStateOtherPayload{},
	})
	if !errors.Is(err, errModalReplacement) || replaced.kind != original.kind || replaced.payload != original.payload {
		t.Fatalf("replacement open = (%#v, %v), want unchanged state and replacement error", replaced, err)
	}
}

func TestModalStateInlineHelpEscRestoresPayloadControl(t *testing.T) {
	registry, err := registerModalPayload[modalStateTestPayload](modalRegistry{}, modalKindUnsavedChanges)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := (modalState{}).open(registry, modalOpenRequest{
		kind:       modalKindUnsavedChanges,
		openedFrom: focusOwnerConnectionForm,
		payload:    modalStateTestPayload{selected: "save"},
	})
	if err != nil {
		t.Fatal(err)
	}

	help, restored, err := opened.toggleHelp("discard")
	if err != nil || restored != nil || !help.helpVisible || help.helpOpenedFrom != "discard" {
		t.Fatalf("open inline Help = (%#v, %#v, %v)", help, restored, err)
	}
	if _, _, err := help.close(); !errors.Is(err, errModalInlineHelpActive) {
		t.Fatalf("close while inline Help visible error = %v, want %v", err, errModalInlineHelpActive)
	}

	// Esc while inline Help is visible uses the same pure toggle as '?'.
	restoredModal, restored, err := help.toggleHelp(nil)
	if err != nil {
		t.Fatalf("Esc from inline Help: %v", err)
	}
	if restored != "discard" || restoredModal.helpVisible || restoredModal.helpOpenedFrom != nil {
		t.Fatalf("Esc restoration = (%#v, %#v), want discard control and hidden Help", restoredModal, restored)
	}
	if restoredModal.kind != opened.kind || restoredModal.payload != opened.payload {
		t.Fatal("inline Help replaced or mutated its owning modal")
	}
	if _, owner, err := restoredModal.close(); err != nil || owner != focusOwnerConnectionForm {
		t.Fatalf("close after Help = (owner %v, error %v)", owner, err)
	}
}
