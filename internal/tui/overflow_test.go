package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestUS5SharedViewportUniversalOverflowMatrix(t *testing.T) {
	longLines := make([]string, 9)
	for index := range longLines {
		longLines[index] = fmt.Sprintf("line-%02d-%s", index, strings.Repeat("界", 20))
	}

	form := validConnectionForm()
	form.inputs[fieldName].SetValue(strings.Repeat("form-value-", 10))
	form.setFocus(fieldHost)
	form.setDimensions(18, 3)
	form.viewport = newViewportState(2)

	detail := detailState{kind: detailKindConnection, viewport: newViewportState(2)}
	for index, line := range longLines {
		detail.fields = append(detail.fields, detailField{label: fmt.Sprintf("Field %d", index), value: line})
	}

	registry, err := registerModalPayload[us5ModalPayload](modalRegistry{}, modalKindHelp)
	if err != nil {
		t.Fatal(err)
	}
	modal, err := (modalState{}).open(registry, modalOpenRequest{
		kind: modalKindHelp, openedFrom: focusOwnerTree,
		payload: us5ModalPayload{value: strings.Join(longLines, "\n")}, viewport: newViewportState(2),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		project func() viewportProjection
	}{
		{name: "Tree", project: func() viewportProjection { return newViewportState(2).project(longLines, 3, 18, 4) }},
		{name: "Details", project: func() viewportProjection { return detail.project(3, 18) }},
		{name: "connection form", project: func() viewportProjection { return form.project(newStyles(true)) }},
		{name: "Help", project: func() viewportProjection {
			return newViewportState(2).project(actionHelpLines(connectionActionDescriptors), 3, 18, noActiveLine)
		}},
		{name: "move picker", project: func() viewportProjection { return newViewportState(2).project(longLines, 3, 18, 4) }},
		{name: "confirmation", project: func() viewportProjection { return newViewportState(2).project(longLines, 3, 18, noActiveLine) }},
		{name: "error", project: func() viewportProjection { return newViewportState(2).project(longLines, 3, 18, 4) }},
		{name: "generic modal payload", project: func() viewportProjection {
			return modal.viewport.project(strings.Split(modal.payload.(us5ModalPayload).value, "\n"), 3, 18, noActiveLine)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projection := tt.project()
			if !projection.hasPrevious || !projection.hasNext {
				t.Fatalf("overflow flags = previous %t next %t, want both", projection.hasPrevious, projection.hasNext)
			}
			if !projection.scrollbar.visible || projection.scrollbar.trackHeight != 3 {
				t.Fatalf("overflow scrollbar = %#v, want visible three-row track", projection.scrollbar)
			}
			joined := strings.Join(projection.lines, "\n")
			if strings.Contains(joined, viewportPreviousLabel) || strings.Contains(joined, viewportNextLabel) {
				t.Fatalf("legacy directional marker rendered in %#v", projection.lines)
			}
			for _, line := range projection.lines {
				if width := ansi.StringWidth(line); width > 18 {
					t.Fatalf("line width = %d, want <=18: %q", width, line)
				}
			}
		})
	}
}

func TestUS5ClosedSurfacesRenderedScrollbarCellsAndFitPadding(t *testing.T) {
	const contentWidth = 24
	for _, surface := range sc007ClosedSurfaces(t) {
		t.Run(surface.name, func(t *testing.T) {
			initial := surface.project(0)
			maximum := viewportMaximumOffset(initial.contentLength, initial.availableRows)
			projection := surface.project((maximum + 1) / 2)
			if !projection.scrollbar.visible || projection.scrollbar.trackHeight != projection.availableRows {
				t.Fatalf("overflow projection has incomplete body track: %#v", projection)
			}

			rect := layoutRect{width: contentWidth + 4, height: len(projection.lines) + 2}
			rendered := renderRegionPanelWithScrollbar(surface.name, projection.lines, rect, newStyles(true), projection.scrollbar, 0)
			rows := strings.Split(rendered, "\n")
			for bodyRow := 0; bodyRow < len(projection.lines); bodyRow++ {
				cells := []rune(rows[bodyRow+1])
				if len(cells) != rect.width {
					t.Fatalf("rendered row width = %d cells, want %d: %q", len(cells), rect.width, rows[bodyRow+1])
				}
				penultimate := cells[len(cells)-2]
				if bodyRow < projection.scrollbar.trackHeight {
					want := '│'
					if projection.scrollbar.thumbAt(bodyRow) {
						want = '█'
					}
					if penultimate != want {
						t.Fatalf("body row %d penultimate cell = %q, want track/thumb %q: %q", bodyRow, penultimate, want, rows[bodyRow+1])
					}
				} else if penultimate != ' ' {
					t.Fatalf("fixed-control row %d has scrollbar cell %q instead of padding: %q", bodyRow, penultimate, rows[bodyRow+1])
				}
			}

			fit := newViewportState(0).project(projection.lines, len(projection.lines), contentWidth, noActiveLine)
			if fit.scrollbar.visible {
				t.Fatalf("fitting projection retained scrollbar: %#v", fit.scrollbar)
			}
			fitRect := layoutRect{width: contentWidth + 4, height: len(fit.lines) + 2}
			fitRows := strings.Split(renderRegionPanelWithScrollbar(surface.name, fit.lines, fitRect, newStyles(true), fit.scrollbar, 0), "\n")
			for row := 1; row < len(fitRows)-1; row++ {
				cells := []rune(fitRows[row])
				if cells[len(cells)-2] != ' ' {
					t.Fatalf("fitting row %d did not restore right padding: %q", row-1, fitRows[row])
				}
			}
		})
	}
}

func TestRenderedScrollbarEligibilityAtNarrowContentWidths(t *testing.T) {
	for _, width := range []int{0, 1, 2} {
		projection := newViewportState(0).project([]string{"a", "b"}, 1, width, noActiveLine)
		rect := layoutRect{width: width + 4, height: 3}
		rows := strings.Split(renderRegionPanelWithScrollbar("Narrow", projection.lines, rect, newStyles(true), projection.scrollbar, 0), "\n")
		cells := []rune(rows[1])
		want := ' '
		if width >= 1 {
			want = '█'
		}
		if got := cells[len(cells)-2]; got != want {
			t.Fatalf("content width %d right-padding/bar cell = %q, want %q: %q", width, got, want, rows[1])
		}
	}
}

func TestUS5CurrentTreeDetailsFormActionsAndHelpUseOverflowContract(t *testing.T) {
	t.Run("Tree", func(t *testing.T) {
		root := testFolder("root", "", "/", 1)
		connections := make([]app.Connection, 10)
		for index := range connections {
			connections[index] = testConnection(fmt.Sprintf("connection-%02d", index), root.ID, "/"+strings.Repeat("very-long-name-", 4)+fmt.Sprint(index), 1)
		}
		snapshot := newCatalogSnapshot(root, 1)
		_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: connections})
		var browser browserModel
		browser.setSnapshot(snapshot, "")
		browser.selectedID = connections[5].ID
		browser.rebuildRows()
		browser.viewport = newViewportState(3)
		projection := browser.projectTree(newStyles(true), 24, 3)
		view := strings.Join(projection.lines, "\n")
		for _, want := range []string{"…", "> "} {
			if !strings.Contains(view, want) {
				t.Fatalf("Tree overflow omitted %q:\n%s", want, view)
			}
		}
		if !projection.hasPrevious || !projection.hasNext || !projection.scrollbar.visible {
			t.Fatalf("Tree overflow metadata = (%v,%v), scrollbar %#v", projection.hasPrevious, projection.hasNext, projection.scrollbar)
		}
		if strings.Contains(view, viewportPreviousLabel) || strings.Contains(view, viewportNextLabel) {
			t.Fatalf("Tree rendered legacy markers:\n%s", view)
		}
	})

	t.Run("Details", func(t *testing.T) {
		state := detailState{kind: detailKindConnection, viewport: newViewportState(2)}
		for index := 0; index < 9; index++ {
			state.fields = append(state.fields, detailField{label: fmt.Sprintf("Field%d", index), value: strings.Repeat("value", 20)})
		}
		projection := state.project(3, 20)
		if !projection.hasPrevious || !projection.hasNext || !projection.scrollbar.visible {
			t.Fatalf("Details overflow metadata = (%v,%v), scrollbar %#v", projection.hasPrevious, projection.hasNext, projection.scrollbar)
		}
		if strings.Contains(strings.Join(projection.lines, "\n"), viewportPreviousLabel) || strings.Contains(strings.Join(projection.lines, "\n"), viewportNextLabel) {
			t.Fatalf("Details rendered legacy markers: %#v", projection.lines)
		}
	})

	t.Run("connection form", func(t *testing.T) {
		form := validConnectionForm()
		form.setFocus(fieldSave)
		form.setDimensions(24, 3)
		projection := form.project(newStyles(true))
		if !projection.hasPrevious || !projection.hasNext || !projection.scrollbar.visible || !strings.Contains(strings.Join(projection.lines, "\n"), "Save") {
			t.Fatalf("form did not retain active Save with scrollbar: %#v, geometry %#v", projection.lines, projection.scrollbar)
		}
		if strings.Contains(strings.Join(projection.lines, "\n"), viewportPreviousLabel) || strings.Contains(strings.Join(projection.lines, "\n"), viewportNextLabel) {
			t.Fatalf("form rendered legacy markers: %#v", projection.lines)
		}
	})

	t.Run("Actions", func(t *testing.T) {
		descriptors := actionsFor(actionContext{state: actionStateNormal, selection: actionSelectionConnection, focus: actionFocusTree, canToggle: true})
		lines := packActions("", descriptors, 24, 3)
		if !slices.Contains(lines, actionsOverflowMarker) {
			t.Fatalf("Actions omitted exact overflow marker: %#v", lines)
		}
		for _, safety := range []string{"r Reload", "q Quit", "? Help"} {
			if !strings.Contains(strings.Join(lines, "\n"), safety) {
				t.Fatalf("Actions marker displaced safety control %q: %#v", safety, lines)
			}
		}
	})

	t.Run("Help", func(t *testing.T) {
		model := New(Config{Width: 60, Height: 24, NoColor: true})
		lines := make([]string, 30)
		for index := range lines {
			lines[index] = fmt.Sprintf("Help line %02d", index)
		}
		model.openGenericModal(modalKindHelp, nil, helpPayload{lines: lines})
		model.modal.viewport = newViewportState(3)
		view := model.View().Content
		projection := projectOpenModalForTest(model)
		if !projection.hasPrevious || !projection.hasNext || !projection.scrollbar.visible || !strings.Contains(view, "█") {
			t.Fatalf("Help overflow omitted scrollbar: projection %#v, geometry %#v\n%s", projection.lines, projection.scrollbar, view)
		}
		if strings.Contains(view, viewportPreviousLabel) || strings.Contains(view, viewportNextLabel) {
			t.Fatalf("Help rendered legacy markers:\n%s", view)
		}
	})
}

func TestUS5PickerConfirmationAndErrorOverflowAreBoundedAndDiscoverable(t *testing.T) {
	long := "/" + strings.Repeat("shared-界-segment/", 30) + "distinguishing-suffix"
	tests := []struct {
		name            string
		setup           func(*Model) bool
		wantEllipsis    bool
		wantOverflow    bool
		priorityControl string
	}{
		{name: "move picker", setup: func(model *Model) bool {
			folders := make([]app.Folder, 20)
			for index := range folders {
				folders[index] = testFolder(fmt.Sprintf("folder-%02d", index), "root", fmt.Sprintf("%s/%02d", long, index), 1)
			}
			picker := newMovePicker(app.Node{ID: "source", Kind: app.NodeKindConnection, Path: long}, folders)
			target := capturedTarget{id: "source", revision: 1, kind: app.NodeKindConnection, path: long}
			return model.openGenericModal(modalKindMovePicker, &target, movePickerPayload{picker: picker})
		}, wantEllipsis: true, wantOverflow: true, priorityControl: "Esc Cancel"},
		{name: "confirmation", setup: func(model *Model) bool {
			connection := testConnection("captured", "root", long, 3)
			connection.Host = strings.Repeat("host.", 40) + "example"
			confirmation := newConnectConfirmation(connection)
			target := capturedTargetFromConnection(connection)
			return model.openGenericModal(modalKindConnectConfirmation, &target, connectConfirmationPayload{confirmation: confirmation})
		}, wantOverflow: true, priorityControl: "Enter/Esc Cancel"},
		{name: "error", setup: func(model *Model) bool {
			modal := newErrorModal("save connection", long, app.ErrConflict)
			return model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: modal})
		}, wantEllipsis: false, priorityControl: "b/Esc Back"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(Config{Width: 40, Height: 12, NoColor: true})
			if !tt.setup(model) {
				t.Fatalf("failed to open concrete %s payload", tt.name)
			}
			view := model.View().Content
			if tt.wantEllipsis && !strings.Contains(view, "…") {
				t.Fatalf("%s clipped long content without in-width ellipsis:\n%s", tt.name, view)
			}
			projection := projectOpenModalForTest(model)
			if tt.wantOverflow && (!projection.hasNext || !projection.scrollbar.visible || !strings.Contains(view, "█")) {
				t.Fatalf("%s clipped vertical content without scrollbar: projection %#v geometry %#v\n%s", tt.name, projection.lines, projection.scrollbar, view)
			}
			if strings.Contains(view, viewportPreviousLabel) || strings.Contains(view, viewportNextLabel) {
				t.Fatalf("%s rendered legacy markers:\n%s", tt.name, view)
			}
			if !strings.Contains(view, tt.priorityControl) {
				t.Fatalf("%s overflow displaced priority control %q:\n%s", tt.name, tt.priorityControl, view)
			}
			assertUS5FrameBounded(t, view, 40, 12)
		})
	}
}

func projectOpenModalForTest(model *Model) viewportProjection {
	rect := calculateLayout(model.width, model.height, model.focusedLayoutRegion()).modalOverlay()
	lines, active := modalContent(model.modal, model.styles, rect.contentWidth(), wrapHelpLines(actionHelpLines(model.currentActionDescriptors()), rect.contentWidth()))
	return projectModalViewport(model.modal, lines, rect.contentHeight(), rect.contentWidth(), active)
}
