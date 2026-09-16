package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestModalCapturedTargetsWrapCompletelyWithoutDisplacingControls(t *testing.T) {
	long := "/" + strings.Repeat("captured-segment/", 20) + "final"
	tests := []struct {
		name  string
		state modalState
	}{
		{name: "connect", state: modalState{kind: modalKindConnectConfirmation, payload: connectConfirmationPayload{confirmation: newConnectConfirmation(testConnection("connection", syntheticRootID, long, 7))}}},
		{name: "delete connection", state: modalState{kind: modalKindDeleteConnection, payload: deleteConnectionPayload{confirmation: newDeleteConfirmation(deleteScopeForModalLayout(long))}}},
		{name: "delete folder", state: modalState{kind: modalKindDeleteFolder, payload: deleteFolderPayload{confirmation: newFolderDeleteConfirmation(folderDeleteScopeForModalLayout(long))}}},
		{name: "unsaved", state: modalState{kind: modalKindUnsavedChanges, payload: unsavedChangesPayload{intent: unsavedIntentQuit, target: long}}},
		{name: "operation error", state: modalState{kind: modalKindOperationError, payload: operationErrorPayload{modal: newErrorModal("reload catalog", long, errModalClosed)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines, active := modalContent(test.state, newStyles(true), 20, nil)
			bodyEnd := modalPriorityStart(lines)
			if bodyEnd < 0 {
				t.Fatalf("modal has no fixed controls: %#v", lines)
			}
			joined := strings.Join(lines[:bodyEnd], "")
			if !strings.Contains(strings.ReplaceAll(joined, " ", ""), long) || strings.Contains(joined, safeTextEllipsis) {
				t.Fatalf("captured target was not wrapped completely: %q", joined)
			}
			projection := projectModalViewport(test.state, lines, 8, 20, active)
			projected := strings.Join(projection.lines, "\n")
			if !strings.Contains(projected, "Cancel") && !strings.Contains(projected, "Back") {
				t.Fatalf("fixed cancel/back control was displaced: %#v", projection.lines)
			}
		})
	}
}

func TestModalControlInventoryAndPriorityRemainFixed(t *testing.T) {
	want := map[modalKind][]string{
		modalKindFolderCreate:        {modalControlLine("Ctrl+S/Enter Save  Esc Cancel  F1 Help")},
		modalKindFolderEdit:          {modalControlLine("Ctrl+S/Enter Save  Esc Cancel  F1 Help")},
		modalKindMovePicker:          {modalControlLine("Enter Move  Esc Cancel  ? Help")},
		modalKindDeleteConnection:    {modalControlLine("y Confirm"), modalControlLine("Enter/Esc Cancel"), modalControlLine("? Help")},
		modalKindDeleteFolder:        {modalControlLine("y Confirm"), modalControlLine("Enter/Esc Cancel"), modalControlLine("? Help")},
		modalKindConnectConfirmation: {modalControlLine("y Confirm"), modalControlLine("Enter/Esc Cancel"), modalControlLine("? Help")},
		modalKindUnsavedChanges:      {modalControlLine("s Save"), modalControlLine("d Discard"), modalControlLine("Esc Cancel")},
		modalKindHelp:                {modalControlLine("?/Esc Close")},
		modalKindOperationError:      {modalControlLine("r Reload"), modalControlLine("b/Esc Back"), modalControlLine("q Quit"), modalControlLine("? Help")},
		modalKindSSHFailure:          {modalControlLine("d Detail"), modalControlLine("r Retry"), modalControlLine("e Edit"), modalControlLine("b/Esc Back"), modalControlLine("q Quit"), modalControlLine("? Help")},
	}
	for _, test := range feature008ModalCases() {
		t.Run(string(test.kind), func(t *testing.T) {
			state := modalState{kind: test.kind, payload: test.payload}
			lines, _ := modalContent(state, newStyles(true), 80, nil)
			start := modalPriorityStart(lines)
			if start < 0 || !reflect.DeepEqual(lines[start:], want[test.kind]) {
				t.Fatalf("controls for %s = %#v, want %#v", test.kind, lines[max(0, start):], want[test.kind])
			}
		})
	}
}

func deleteScopeForModalLayout(path string) app.ConnectionDeleteScope {
	return app.ConnectionDeleteScope{ID: "connection", Path: path, Host: "prod.test", Revision: 7}
}

func folderDeleteScopeForModalLayout(path string) app.FolderDeleteScope {
	return app.FolderDeleteScope{ID: "folder", Path: path, Revision: 7}
}

func TestUS4CenteredOverlayBoundsBackgroundUnicodeAndReduced(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {60, 12}, {40, 12}} {
		layout := calculateLayout(size[0], size[1], regionTree)
		rect := layout.modalOverlay()
		assertRectBounded(t, rect, size[0], size[1])
		state, err := (modalState{}).open(newModalRegistry(), modalOpenRequest{kind: modalKindHelp, openedFrom: focusOwnerTree, payload: helpPayload{lines: []string{"Unicode 界🙂", strings.Repeat("long ", 100)}}})
		if err != nil {
			t.Fatal(err)
		}
		view := renderModalOverlay(strings.Repeat("background\n", size[1]-1)+"background", state, layout, newStyles(true), nil)
		if !strings.Contains(view, "background") || !strings.Contains(view, "Unicode") {
			t.Fatalf("%dx%d overlay lost background or payload:\n%s", size[0], size[1], view)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("line width %d > %d: %q", ansi.StringWidth(line), size[0], line)
			}
		}
	}
	if rect := calculateLayout(39, 11, regionTree).modalOverlay(); rect != (layoutRect{}) {
		t.Fatalf("undersized overlay = %#v, want suspended", rect)
	}
}

func TestUS5ModalScrollbarTrackStopsBeforeFixedControls(t *testing.T) {
	for _, surface := range sc007ClosedSurfaces(t) {
		if surface.name != "Help" && surface.name != "picker" && surface.name != "confirmation" && surface.name != "recoverable error" {
			continue
		}
		t.Run(surface.name, func(t *testing.T) {
			projection := surface.project(1)
			if len(projection.lines) <= projection.scrollbar.trackHeight {
				t.Fatalf("fixture has no fixed modal controls after %d body rows: %#v", projection.scrollbar.trackHeight, projection.lines)
			}
			rect := layoutRect{width: 28, height: len(projection.lines) + 2}
			rows := strings.Split(renderRegionPanelWithScrollbar(surface.name, projection.lines, rect, newStyles(true), projection.scrollbar, 0), "\n")
			for row := projection.scrollbar.trackHeight; row < len(projection.lines); row++ {
				cells := []rune(rows[row+1])
				if cells[len(cells)-2] != ' ' {
					t.Fatalf("fixed control row %d contains track/thumb in penultimate cell: %q", row, rows[row+1])
				}
			}
		})
	}
}

func TestModalScrollbarTrackExcludesLeadingFixedNotices(t *testing.T) {
	helpLines := make([]string, 14)
	for index := range helpLines {
		helpLines[index] = fmt.Sprintf("Help row %02d", index)
	}
	target := capturedTarget{id: "target", revision: 7, path: "/target"}
	tests := []struct {
		name  string
		state modalState
		start int
	}{
		{
			name:  "operation status",
			state: modalState{kind: modalKindHelp, payload: helpPayload{lines: helpLines}, operationStatus: "Loading safely"},
			start: 1,
		},
		{
			name:  "conflict",
			state: modalState{kind: modalKindHelp, payload: helpPayload{lines: helpLines}, conflict: &conflictState{target: target, detailVisible: true}},
			start: 3,
		},
		{
			name:  "recoverable error",
			state: modalState{kind: modalKindHelp, payload: helpPayload{lines: helpLines}, recoverableError: "retry safely"},
			start: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const rows, width = 12, 28
			lines, active := modalContent(test.state, newStyles(true), width-4, nil)
			projection := projectModalViewport(test.state, lines, rows, width-4, active)
			if !projection.scrollbar.visible || projection.scrollbarStart != test.start {
				t.Fatalf("scrollbar = %#v start=%d, want visible start=%d", projection.scrollbar, projection.scrollbarStart, test.start)
			}
			panel := renderRegionPanelWithScrollbar(test.name, projection.lines, layoutRect{width: width, height: rows + 2}, newStyles(true), projection.scrollbar, projection.scrollbarStart)
			panelRows := strings.Split(panel, "\n")
			for row := 0; row < test.start; row++ {
				cells := []rune(panelRows[row+1])
				if cells[len(cells)-2] != ' ' {
					t.Fatalf("leading fixed row %d contains track/thumb: %q", row, panelRows[row+1])
				}
			}
			trackRow := []rune(panelRows[test.start+1])
			if trackRow[len(trackRow)-2] == ' ' {
				t.Fatalf("first body row has no track/thumb: %q", panelRows[test.start+1])
			}
		})
	}
}
