package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestModelOwnsTreeFocusAndSynchronizesDetailInSameUpdate(t *testing.T) {
	first := testConnection("first", syntheticRootID, "/first", 1)
	second := testConnection("second", syntheticRootID, "/second", 1)
	model := loadedModel(t, []app.Connection{first, second}, true)

	if model.focusOwner != focusOwnerTree {
		t.Fatalf("initial focus = %v, want Tree", model.focusOwner)
	}
	if model.detailState.targetID != model.browser.selectedID {
		t.Fatalf("initial detail target = %q, selection %q", model.detailState.targetID, model.browser.selectedID)
	}

	updateModel(model, keyPress("l"))
	if model.browser.selectedID != first.ID || model.detailState.targetID != first.ID {
		t.Fatalf("same-transition IDs = selection %q detail %q, want %q", model.browser.selectedID, model.detailState.targetID, first.ID)
	}
	view := model.View().Content
	if !strings.Contains(view, ">   [ssh] first") || !strings.Contains(view, "Path: /first") || strings.Contains(view, "Path: /second") {
		t.Fatalf("first frame mixed selection detail:\n%s", view)
	}
}

func TestModelTabAndShiftTabToggleTreeAndDetailsWithoutSelectionChange(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/production", 1)
	model := loadedModel(t, []app.Connection{connection}, true)
	selected := model.browser.selectedID

	updateModel(model, keyPress("tab"))
	if model.focusOwner != focusOwnerDetail || model.browser.selectedID != selected {
		t.Fatalf("Tab = focus %v selection %q, want Details and %q", model.focusOwner, model.browser.selectedID, selected)
	}
	if view := model.View().Content; !strings.Contains(view, "[*] Details") || !strings.Contains(view, "[ ] Tree") {
		t.Fatalf("Details focus is not textual in shell:\n%s", view)
	}

	updateModel(model, keyPress("shift+tab"))
	if model.focusOwner != focusOwnerTree || model.browser.selectedID != selected {
		t.Fatalf("Shift+Tab = focus %v selection %q, want Tree and %q", model.focusOwner, model.browser.selectedID, selected)
	}
}

func TestDetailsNavigationOnlyScrollsAndTargetChangeResetsOffset(t *testing.T) {
	connections := make([]app.Connection, 8)
	for index := range connections {
		connections[index] = testConnection(string(rune('a'+index)), syntheticRootID, "/"+string(rune('a'+index)), 1)
	}
	model := loadedModel(t, connections, true)
	updateModel(model, tea.WindowSizeMsg{Width: 80, Height: 12})
	selected := model.browser.selectedID
	updateModel(model, keyPress("tab"))

	updateModel(model, keyPress("j"))
	if model.browser.selectedID != selected || model.detailState.viewport.logicalOffset != 1 {
		t.Fatalf("Details Down = selection %q offset %d, want unchanged/%d", model.browser.selectedID, model.detailState.viewport.logicalOffset, 1)
	}
	updateModel(model, keyPress("G"))
	if model.browser.selectedID != selected || model.detailState.viewport.logicalOffset == 0 {
		t.Fatalf("Details End = selection %q offset %d", model.browser.selectedID, model.detailState.viewport.logicalOffset)
	}
	updateModel(model, keyPress("g"))
	if model.detailState.viewport.logicalOffset != 0 {
		t.Fatalf("Details Home offset = %d, want 0", model.detailState.viewport.logicalOffset)
	}

	model.detailState = model.detailState.withOffset(3)
	updateModel(model, keyPress("tab"))
	updateModel(model, keyPress("l"))
	if model.browser.selectedID != connections[0].ID || model.detailState.targetID != connections[0].ID {
		t.Fatalf("target transition = selection %q detail %q", model.browser.selectedID, model.detailState.targetID)
	}
	if model.detailState.viewport.logicalOffset != 0 {
		t.Fatalf("new target retained detail offset %d", model.detailState.viewport.logicalOffset)
	}
}

func TestUndersizedShellPreservesNavigationStateAndBlocksNavigation(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/production", 1)
	model := loadedModel(t, []app.Connection{connection}, true)
	updateModel(model, keyPress("tab"))
	updateModel(model, keyPress("j"))
	updateModel(model, keyPress("tab"))
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("tab"))
	updateModel(model, keyPress("j"))
	selection := model.browser.selectedID
	detail := model.detailState
	focus := model.focusOwner
	offset := model.detailState.viewport.logicalOffset

	updateModel(model, tea.WindowSizeMsg{Width: 39, Height: 11})
	updateModel(model, keyPress("tab"))
	updateModel(model, keyPress("g"))
	view := model.View().Content
	if !strings.Contains(view, "40x12") || !strings.Contains(view, "? Help") || !strings.Contains(view, "q Quit") {
		t.Fatalf("undersized shell omitted required controls:\n%s", view)
	}
	if strings.Contains(view, "production") || strings.Contains(view, "Details") || strings.Contains(view, "Actions") {
		t.Fatalf("undersized shell exposed base content:\n%s", view)
	}
	if model.browser.selectedID != selection || model.detailState.targetID != detail.targetID || model.focusOwner != focus || model.detailState.viewport.logicalOffset != offset {
		t.Fatalf("undersized input changed opaque state: selection %q detail %q focus %v offset %d", model.browser.selectedID, model.detailState.targetID, model.focusOwner, model.detailState.viewport.logicalOffset)
	}
}

func TestUnavailableRootObjectShortcutsAreInert(t *testing.T) {
	model := loadedModel(t, nil, true)
	for _, shortcut := range []string{"c", "e", "m", "d"} {
		selection := model.browser.selectedID
		updateModel(model, keyPress(shortcut))
		if model.browser.selectedID != selection || model.form != nil || model.modal.isOpen() || model.operation != nil {
			t.Fatalf("unavailable root shortcut %q changed model state", shortcut)
		}
	}
}

func TestBrowserShellIsTitledAndBoundedInWideAndStackedLayouts(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/production", 1)
	model := loadedModel(t, []app.Connection{connection}, true)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 40, Height: 12}} {
		updateModel(model, size)
		view := model.View().Content
		for _, title := range []string{"[*] Tree", "[ ] Details", "[ ] Actions"} {
			if !strings.Contains(view, title) {
				t.Fatalf("%dx%d shell omitted %q:\n%s", size.Width, size.Height, title, view)
			}
		}
		lines := strings.Split(view, "\n")
		if len(lines) > size.Height {
			t.Fatalf("%dx%d frame has %d lines", size.Width, size.Height, len(lines))
		}
		for _, line := range lines {
			if width := ansi.StringWidth(line); width > size.Width {
				t.Fatalf("%dx%d frame line width = %d: %q", size.Width, size.Height, width, line)
			}
		}
		if size.Width < wideLayoutWidth && !(strings.Index(view, "Tree") < strings.Index(view, "Details") && strings.Index(view, "Details") < strings.Index(view, "Actions")) {
			t.Fatalf("stacked panel order is wrong:\n%s", view)
		}
	}
}

func TestConnectionShellRendersEndpointWithoutDuplicateHostLine(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/production", 1)
	model := loadedModel(t, []app.Connection{connection}, true)
	updateModel(model, keyPress("l"))
	view := model.View().Content
	if !strings.Contains(view, "Endpoint: host.test:22") || strings.Contains(view, "Host: host.test:22") {
		t.Fatalf("connection detail did not retain only the canonical endpoint:\n%s", view)
	}
}
