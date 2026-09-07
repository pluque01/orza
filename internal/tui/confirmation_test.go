package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestUS4ConfirmationsRequireExplicitAcceptanceAndRevealFullTargets(t *testing.T) {
	long := "/" + strings.Repeat("segment-界/", 80) + "final"
	connection := testConnection("stable-id", syntheticRootID, long, 9)
	connection.Host = strings.Repeat("host.", 40) + "example"
	connection.Port = 2202
	tests := []struct {
		name    string
		payload any
		kind    modalKind
		accept  func(tea.KeyPressMsg) bool
	}{
		{"connect", connectConfirmationPayload{newConnectConfirmation(connection)}, modalKindConnectConfirmation, newConnectConfirmation(connection).confirmed},
		{"delete connection", deleteConnectionPayload{newDeleteConfirmation(app.ConnectionDeleteScope{ID: connection.ID, Path: long, Host: connection.Host, Revision: 9})}, modalKindDeleteConnection, newDeleteConfirmation(app.ConnectionDeleteScope{}).confirmed},
		{"delete folder", deleteFolderPayload{newFolderDeleteConfirmation(app.FolderDeleteScope{ID: "folder", Path: long, Revision: 4})}, modalKindDeleteFolder, newFolderDeleteConfirmation(app.FolderDeleteScope{}).confirmed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.accept(keyPress("enter")) || test.accept(keyPress("esc")) || !test.accept(keyPress("y")) {
				t.Fatal("acceptance was not explicit Y only")
			}
			state, err := (modalState{}).open(newModalRegistry(), modalOpenRequest{kind: test.kind, openedFrom: focusOwnerTree, payload: test.payload})
			if err != nil {
				t.Fatal(err)
			}
			lines, _ := modalContent(state, newStyles(true), 20, nil)
			joined := strings.Join(lines, "")
			if !strings.Contains(joined, "final") || strings.Contains(joined, "…") {
				t.Fatalf("captured target was truncated: %q", joined)
			}
		})
	}
}

func TestUS5LongConfirmationIdentityIsScrollRevealableWithCancelAlwaysVisible(t *testing.T) {
	long := "/" + strings.Repeat("segment-界/", 80) + "distinguishing-final"
	connection := testConnection("captured-id", syntheticRootID, long, 9)
	connection.Host = strings.Repeat("host.", 40) + "example"
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	target := capturedTargetFromConnection(connection)
	if !model.openGenericModal(modalKindConnectConfirmation, &target, connectConfirmationPayload{newConnectConfirmation(connection)}) {
		t.Fatal("failed to open concrete connection confirmation")
	}

	first := model.View().Content
	for _, want := range []string{"Connect", "Enter/Esc Cancel", "█"} {
		if !strings.Contains(first, want) {
			t.Fatalf("initial confirmation omitted %q:\n%s", want, first)
		}
	}
	firstProjection := projectOpenModalForTest(model)
	if firstProjection.hasPrevious || !firstProjection.hasNext || !firstProjection.scrollbar.visible || firstProjection.scrollbar.thumbTop != 0 {
		t.Fatalf("initial confirmation scrollbar metadata = overflow (%v,%v), geometry %#v", firstProjection.hasPrevious, firstProjection.hasNext, firstProjection.scrollbar)
	}
	if strings.Contains(first, viewportPreviousLabel) || strings.Contains(first, viewportNextLabel) {
		t.Fatalf("initial confirmation rendered legacy markers:\n%s", first)
	}
	if strings.Contains(first, "final") {
		t.Fatal("long identity unexpectedly fit without requiring scroll")
	}

	revealed := ""
	for range 200 {
		updateModel(model, keyPress("j"))
		revealed = model.View().Content
		if strings.Contains(revealed, "final") {
			break
		}
	}
	last := revealed
	for _, want := range []string{"final", "Enter/Esc Cancel", "█"} {
		if !strings.Contains(last, want) {
			t.Fatalf("end-scrolled confirmation omitted %q:\n%s", want, last)
		}
	}
	lastProjection := projectOpenModalForTest(model)
	if !lastProjection.hasPrevious || !lastProjection.scrollbar.visible || lastProjection.scrollbar.thumbTop <= 0 {
		t.Fatalf("revealed confirmation scrollbar metadata = overflow (%v,%v), geometry %#v", lastProjection.hasPrevious, lastProjection.hasNext, lastProjection.scrollbar)
	}
	if strings.Contains(last, viewportPreviousLabel) || strings.Contains(last, viewportNextLabel) {
		t.Fatalf("end confirmation rendered legacy markers:\n%s", last)
	}
	if strings.Contains(last, "…") {
		t.Fatalf("wrapped confirmation identity was truncated instead of revealed: %q", last)
	}
}

func TestUS5DeleteConfirmationPreservesCapturedEndpointThroughScopeLookup(t *testing.T) {
	connection := testConnection("captured-id", syntheticRootID, "/production", 9)
	connection.Host = "prod.example"
	connection.Port = 2202
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	target := capturedTargetFromConnection(connection)
	operation, _, ok := startOperation(4, nil, asyncOperationSave, &target, operationOwnerModal)
	if !ok {
		t.Fatal("operation setup failed")
	}
	model.operation = operation

	updateModel(model, operationResultMsg{
		id: operation.id, kind: operationDeleteScope,
		scope: app.ConnectionDeleteScope{ID: connection.ID, Path: connection.Path, Host: connection.Host, Revision: connection.Revision},
	})
	view := model.View().Content
	lines, _ := modalContent(model.modal, model.styles, calculateLayout(40, 12, regionTree).modalOverlay().contentWidth(), nil)
	compact := strings.Join(lines, "")
	for _, want := range []string{"Path: /production", "Target: (default)@prod.example:2202", "ID/revision: captured-id/9"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("delete confirmation omitted captured identity %q:\n%s", want, view)
		}
	}
}
