package tui

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const sc007ClosedTraversalRuns = 20

type sc007ClosedSurface struct {
	name    string
	project func(int) viewportProjection
}

func TestSC007ClosedSurfaceForwardReverseTraversalTwentyRuns(t *testing.T) {
	surfaces := sc007ClosedSurfaces(t)
	if len(surfaces) != 7 {
		t.Fatalf("closed scrollbar inventory = %d, want 7", len(surfaces))
	}
	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			initial := surface.project(0)
			maximum := viewportMaximumOffset(initial.contentLength, initial.availableRows)
			if maximum < 2 {
				t.Fatalf("surface has only %d effective offsets; start/middle/end are not distinct: %#v", maximum+1, initial)
			}
			middle := (maximum + 1) / 2
			for run := range sc007ClosedTraversalRuns {
				previousTop := -1
				for offset := 0; offset <= maximum; offset++ {
					projection := surface.project(offset)
					assertSC007TraversalProjection(t, surface.name, run, "forward", offset, maximum, middle, projection)
					if projection.scrollbar.thumbTop < previousTop {
						t.Fatalf("run %d forward offset %d thumb moved backward: %d < %d", run+1, offset, projection.scrollbar.thumbTop, previousTop)
					}
					previousTop = projection.scrollbar.thumbTop
				}

				previousTop = initial.availableRows
				for offset := maximum; offset >= 0; offset-- {
					projection := surface.project(offset)
					assertSC007TraversalProjection(t, surface.name, run, "reverse", offset, maximum, middle, projection)
					if projection.scrollbar.thumbTop > previousTop {
						t.Fatalf("run %d reverse offset %d thumb moved forward: %d > %d", run+1, offset, projection.scrollbar.thumbTop, previousTop)
					}
					previousTop = projection.scrollbar.thumbTop
				}
			}
		})
	}
}

func sc007ClosedSurfaces(t testing.TB) []sc007ClosedSurface {
	t.Helper()
	const rows, width = 4, 24
	styles := newStyles(true)

	root := testFolder("closed-root", "", "/", 1)
	connections := make([]app.Connection, 10)
	for index := range connections {
		connections[index] = testConnection(fmt.Sprintf("closed-connection-%02d", index), root.ID, fmt.Sprintf("/closed-%02d", index), 1)
	}
	snapshot := newCatalogSnapshot(root, 1)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: connections}) {
		t.Fatal("closed Tree fixture setup failed")
	}
	var browser browserModel
	browser.setSnapshot(snapshot, "")

	detail := detailState{kind: detailKindConnection}
	for index := range 10 {
		detail.fields = append(detail.fields, detailField{label: fmt.Sprintf("Field %02d", index), value: fmt.Sprintf("value-%02d", index)})
	}

	form := validConnectionForm()
	form.setAuthMethod(app.AuthMethodKey)
	form.syncDependencies()
	formLines, _ := form.content(styles)

	modalSurface := func(state modalState, selectOffset func(int)) func(int) viewportProjection {
		return func(offset int) viewportProjection {
			if selectOffset != nil {
				selectOffset(offset)
			}
			state.viewport = newViewportState(offset)
			lines, active := modalContent(state, styles, width, nil)
			priority := modalPriorityStart(lines)
			controlRows := 0
			if priority >= 0 {
				controlRows = len(packModalControls(lines[priority:], width, 100))
			}
			return projectModalViewport(state, lines, rows+controlRows, width, active)
		}
	}

	helpLines := make([]string, 10)
	folders := make([]app.Folder, 10)
	for index := range 10 {
		helpLines[index] = fmt.Sprintf("Help command %02d", index)
		folders[index] = testFolder(fmt.Sprintf("closed-folder-%02d", index), root.ID, fmt.Sprintf("/destination-%02d", index), 1)
	}
	help := modalState{kind: modalKindHelp, payload: helpPayload{lines: helpLines}}
	picker := newMovePicker(connections[0].Node, folders)
	pickerState := modalState{kind: modalKindMovePicker, payload: movePickerPayload{picker: picker}}
	previous := app.SSHAttemptTarget{ID: connections[0].ID, Revision: 1, Path: "/previous/target", Host: "previous.test", Port: 22}
	confirmation := modalState{kind: modalKindConnectConfirmation, payload: connectConfirmationPayload{confirmation: newRetryConnectConfirmation(previous, connections[0])}}
	failure := newErrorModal("reload catalog after a controlled recoverable failure", "/"+strings.Repeat("long-target-segment/", 8), app.ErrConflict)
	errorState := modalState{kind: modalKindOperationError, payload: operationErrorPayload{modal: failure}}

	return []sc007ClosedSurface{
		{name: "Tree", project: func(offset int) viewportProjection {
			index := min(offset, len(browser.rows)-1)
			browser.selectedID = browser.rows[index].id
			browser.viewport = newViewportState(offset)
			return browser.projectTree(styles, width, rows)
		}},
		{name: "Details", project: func(offset int) viewportProjection {
			return detail.withOffset(offset).project(rows, width)
		}},
		{name: "form", project: func(offset int) viewportProjection {
			return projectActiveBlock(formLines, offset, offset, rows, width)
		}},
		{name: "Help", project: modalSurface(help, nil)},
		{name: "picker", project: modalSurface(pickerState, func(offset int) { picker.selected = min(offset, len(picker.targets)-1) })},
		{name: "confirmation", project: modalSurface(confirmation, nil)},
		{name: "recoverable error", project: modalSurface(errorState, nil)},
	}
}

func assertSC007TraversalProjection(t testing.TB, surface string, run int, direction string, offset, maximum, middle int, projection viewportProjection) {
	t.Helper()
	want := sc007ExactGeometry(projection.contentLength, projection.availableRows, offset)
	if projection.renderOffset != offset || !reflect.DeepEqual(projection.scrollbar, want) {
		position := "offset"
		switch offset {
		case 0:
			position = "start"
		case middle:
			position = "middle"
		case maximum:
			position = "end"
		}
		t.Fatalf("%s run %d %s %s geometry at offset %d = render %d, %#v; want render %d, %#v", surface, run+1, direction, position, offset, projection.renderOffset, projection.scrollbar, offset, want)
	}
	assertSC007RenderedScrollbar(t, surface, projection)
	if offset == 0 && projection.hasPrevious || offset > 0 && !projection.hasPrevious || offset == maximum && projection.hasNext || offset < maximum && !projection.hasNext {
		t.Fatalf("%s run %d %s offset %d overflow flags = previous %t next %t", surface, run+1, direction, offset, projection.hasPrevious, projection.hasNext)
	}
}

func assertSC007RenderedScrollbar(t testing.TB, surface string, projection viewportProjection) {
	t.Helper()
	rect := layoutRect{width: 28, height: len(projection.lines) + 2}
	panel := renderRegionPanelWithScrollbar(surface, projection.lines, rect, newStyles(true), projection.scrollbar, projection.scrollbarStart)
	rows := strings.Split(panel, "\n")
	for row := range projection.lines {
		cells := []rune(rows[row+1])
		got := cells[len(cells)-2]
		trackRow := row - projection.scrollbarStart
		if trackRow < 0 || trackRow >= projection.scrollbar.trackHeight {
			if got != ' ' {
				t.Fatalf("%s fixed row %d has scrollbar cell %q: %q", surface, row, got, rows[row+1])
			}
			continue
		}
		want := '│'
		if projection.scrollbar.thumbAt(trackRow) {
			want = '█'
		}
		if got != want {
			t.Fatalf("%s body row %d scrollbar cell = %q, want %q: %q", surface, row, got, want, rows[row+1])
		}
	}
}

func sc007ExactGeometry(contentLength, visibleRows, offset int) scrollbarGeometry {
	if contentLength <= visibleRows || visibleRows <= 0 {
		return scrollbarGeometry{}
	}
	maximum := contentLength - visibleRows
	thumbLength := min(visibleRows, max(1, (visibleRows*visibleRows+contentLength/2)/contentLength))
	travel := visibleRows - thumbLength
	thumbTop := (travel*offset + maximum/2) / maximum
	return scrollbarGeometry{visible: true, trackHeight: visibleRows, thumbTop: thumbTop, thumbLength: thumbLength}
}

func TestSC007ScaleDetailsFormActionsAndHelpOverflowAcrossTwelveSizes(t *testing.T) {
	model, snapshot := newSC007ScaleModel(t)
	detailIndex := sc007DeepFolderWithConnections(model.browser.rows, snapshot)
	updateModel(model, keyPress("g"))
	for range detailIndex {
		updateModel(model, keyPress("j"))
	}
	updateModel(model, keyPress("tab"))
	for _, size := range us5ContractSizes {
		updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
		layout := calculateLayout(size.width, size.height, regionDetails)
		projection := model.detailState.project(layout.details.contentHeight(), layout.details.contentWidth())
		view := model.View().Content
		assertUS5FrameBounded(t, view, size.width, size.height)
		assertSC007Projection(t, size.name+" Details", projection, model.detailState.content(layout.details.contentWidth()), layout.details.contentWidth())
		assertSC007ViewOverflow(t, size.name+" Details", view, projection)
	}

	for _, method := range []app.AuthMethod{app.AuthMethodAgent, app.AuthMethodKey, app.AuthMethodPassword} {
		t.Run("form/"+string(method), func(t *testing.T) {
			for run := range scConformanceRuns {
				formModel, _ := newSC007ScaleModel(t)
				selectSCNode(formModel, snapshot.rootID)
				updateModel(formModel, keyPress("n"))
				formModel.form.setAuthMethod(method)
				formModel.form.baselineAuth = method
				formModel.form.inputs[fieldName].SetValue(strings.Repeat("focused-field-value-", 12))
				formModel.form.syncDependencies()

				for fieldIndex, field := range formModel.form.focusableFields() {
					if fieldIndex != 0 {
						updateModel(formModel, keyPress("tab"))
					}
					if formModel.form.focusedField() != field {
						t.Fatalf("run %d field traversal reached %v, want %v", run+1, formModel.form.focusedField(), field)
					}
					for _, size := range us5ContractSizes {
						updateModel(formModel, tea.WindowSizeMsg{Width: size.width, Height: size.height})
						layout := calculateLayout(size.width, size.height, regionDetails)
						projection := formModel.form.projectAt(formModel.styles, layout.details.contentWidth(), layout.details.contentHeight())
						view := formModel.View().Content
						assertUS5FrameBounded(t, view, size.width, size.height)
						if !slices.ContainsFunc(projection.lines, func(line string) bool { return strings.HasPrefix(line, ">") }) {
							t.Fatalf("run %d %s field %v: focused field is not in projected form: %#v", run+1, size.name, field, projection.lines)
						}
						content, _ := formModel.form.content(formModel.styles)
						assertSC007Projection(t, size.name+" form", projection, content, layout.details.contentWidth())
						assertSC007ViewOverflow(t, size.name+" form", view, projection)
					}
				}

				updateModel(formModel, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
				for _, size := range us5ContractSizes {
					updateModel(formModel, tea.WindowSizeMsg{Width: size.width, Height: size.height})
					layout := calculateLayout(size.width, size.height, regionDetails)
					rect := layout.modalOverlay()
					lines, active := modalContent(formModel.modal, formModel.styles, rect.contentWidth(), nil)
					top := projectModalViewport(formModel.modal, lines, rect.contentHeight(), rect.contentWidth(), active)
					view := formModel.View().Content
					assertSC007Projection(t, size.name+" form Help top", top, lines, rect.contentWidth())
					assertSC007ViewOverflow(t, size.name+" form Help top", view, top)
					updateModel(formModel, keyPress("G"))
					bottom := projectModalViewport(formModel.modal, lines, rect.contentHeight(), rect.contentWidth(), active)
					view = formModel.View().Content
					assertSC007Projection(t, size.name+" form Help bottom", bottom, lines, rect.contentWidth())
					assertSC007ViewOverflow(t, size.name+" form Help bottom", view, bottom)
					updateModel(formModel, keyPress("g"))
				}
			}
		})
	}

	for run := range scConformanceRuns {
		actionModel, _ := newSC007ScaleModel(t)
		connectionIndex := slices.IndexFunc(actionModel.browser.rows, func(row treeRow) bool {
			return snapshot.nodes[row.id].connection != nil
		})
		actionModel.browser.selectedID = actionModel.browser.rows[connectionIndex].id
		actionModel.ownedSelectionID = actionModel.browser.selectedID
		actionModel.syncDetail()
		for _, size := range us5ContractSizes {
			updateModel(actionModel, tea.WindowSizeMsg{Width: size.width, Height: size.height})
			layout := calculateLayout(size.width, size.height, regionTree)
			lines := packActions("", actionsFor(actionModel.actionContext()), layout.actions.contentWidth(), layout.actions.contentHeight())
			view := actionModel.View().Content
			assertUS5FrameBounded(t, view, size.width, size.height)
			if slices.Contains(lines, actionsOverflowMarker) && !strings.Contains(view, actionsOverflowMarker) {
				t.Fatalf("run %d %s Actions projection overflow marker is absent from Model.View", run+1, size.name)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > layout.actions.contentWidth() {
					t.Fatalf("run %d %s Actions line exceeds region: %q", run+1, size.name, line)
				}
			}
		}
	}
}

func TestSC007PickerAndConfirmationOverflowAcrossTwelveSizesTwentyRuns(t *testing.T) {
	long := "/" + strings.Repeat("overflow-界-segment/", 30) + "target"
	for _, test := range []struct {
		name     string
		setup    func(*Model)
		control  string
		ellipsis bool
	}{
		{name: "move_picker", control: "Esc Cancel", ellipsis: true, setup: func(model *Model) {
			folders := make([]app.Folder, 40)
			for index := range folders {
				folders[index] = testFolder("overflow-folder-"+strconv.Itoa(index), syntheticRootID, long+"/destination-"+strconv.Itoa(index), 1)
			}
			picker := newMovePicker(app.Node{ID: "overflow-source", Kind: app.NodeKindConnection, Path: long, Revision: 1}, folders)
			target := capturedTarget{id: "overflow-source", revision: 1, kind: app.NodeKindConnection, path: long}
			model.openGenericModal(modalKindMovePicker, &target, movePickerPayload{picker: picker})
		}},
		{name: "connect_confirmation", control: "Enter/Esc Cancel", setup: func(model *Model) {
			connection := testConnection("overflow-connection", syntheticRootID, long, 3)
			connection.Host = strings.Repeat("host.", 40) + "example"
			target := capturedTargetFromConnection(connection)
			model.openGenericModal(modalKindConnectConfirmation, &target, connectConfirmationPayload{confirmation: newConnectConfirmation(connection)})
		}},
		{name: "delete_connection", control: "Enter/Esc Cancel", setup: func(model *Model) {
			scope := app.ConnectionDeleteScope{ID: "overflow-delete-connection", Path: long, Host: strings.Repeat("host.", 40) + "example", Revision: 3, HasRememberedPassword: true}
			target := capturedTarget{id: scope.ID, revision: scope.Revision, kind: app.NodeKindConnection, path: scope.Path, endpointOrScope: scope.Host}
			model.openGenericModal(modalKindDeleteConnection, &target, deleteConnectionPayload{confirmation: newDeleteConfirmation(scope)})
		}},
		{name: "delete_folder", control: "Enter/Esc Cancel", setup: func(model *Model) {
			scope := app.FolderDeleteScope{ID: "overflow-delete-folder", Path: long, Revision: 4, Folders: 100, Connections: 1000, RememberedCredentials: 99}
			target := capturedTarget{id: scope.ID, revision: scope.Revision, kind: app.NodeKindFolder, path: scope.Path}
			model.openGenericModal(modalKindDeleteFolder, &target, deleteFolderPayload{confirmation: newFolderDeleteConfirmation(scope)})
		}},
		{name: "help", control: "?/Esc Close", setup: func(model *Model) {
			lines := make([]string, 40)
			for index := range lines {
				lines[index] = "Help " + strconv.Itoa(index) + " " + strings.Repeat("long-help-界 ", 20)
			}
			model.openGenericModal(modalKindHelp, nil, helpPayload{lines: lines})
		}},
		{name: "operation_error", control: "b/Esc Back", setup: func(model *Model) {
			failure := newErrorModal("reload catalog", long, app.ErrInvalidRequest)
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: failure})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for run := range scConformanceRuns {
				for _, size := range us5ContractSizes {
					for _, offset := range []int{0, 3, 1000} {
						model := New(Config{Width: size.width, Height: size.height, NoColor: true})
						test.setup(model)
						model.modal.viewport = newViewportState(offset)
						layout := calculateLayout(size.width, size.height, regionTree)
						rect := layout.modalOverlay()
						content, active := modalContent(model.modal, model.styles, rect.contentWidth(), actionHelpLines(model.currentActionDescriptors()))
						projection := projectModalViewport(model.modal, content, rect.contentHeight(), rect.contentWidth(), active)
						view := model.View().Content
						assertUS5FrameBounded(t, view, size.width, size.height)
						assertSC007ViewOverflow(t, size.name+" "+test.name, view, projection)
						for _, line := range projection.lines {
							if ansi.StringWidth(line) > rect.contentWidth() {
								t.Fatalf("run %d %s offset %d line exceeds modal width: %q", run+1, size.name, offset, line)
							}
						}
						if !strings.Contains(view, test.control) {
							t.Fatalf("run %d %s offset %d displaced %q", run+1, size.name, offset, test.control)
						}
						if test.ellipsis && ansi.StringWidth(long) > rect.contentWidth() && !strings.Contains(view, safeTextEllipsis) {
							t.Fatalf("run %d %s offset %d clipped long content without ellipsis", run+1, size.name, offset)
						}
					}
				}
			}
		})
	}
}

func newSC007ScaleModel(t testing.TB) (*Model, catalogSnapshot) {
	t.Helper()
	service, _, _ := scaleTreeService(100, 1000)
	snapshot, err := loadCatalogSnapshot(context.Background(), service)
	if err != nil {
		t.Fatal(err)
	}
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(*snapshot, "")
	for id, node := range snapshot.nodes {
		if node.folder != nil {
			model.browser.expanded[id] = struct{}{}
		}
	}
	model.browser.rebuildRows()
	model.ownedSelectionID = model.browser.selectedID
	model.syncDetail()
	return model, *snapshot
}

func sc007DeepFolderWithConnections(rows []treeRow, snapshot catalogSnapshot) int {
	bestIndex, bestDepth := -1, -1
	for index, row := range rows {
		node := snapshot.nodes[row.id]
		if node.folder == nil {
			continue
		}
		hasConnection := slices.ContainsFunc(snapshot.children[row.id], func(id app.NodeID) bool {
			return snapshot.nodes[id].connection != nil
		})
		if hasConnection && row.depth > bestDepth {
			bestIndex, bestDepth = index, row.depth
		}
	}
	return bestIndex
}

func assertSC007Projection(t testing.TB, name string, projection viewportProjection, content []string, width int) {
	t.Helper()
	for _, line := range projection.lines {
		if ansi.StringWidth(line) > width {
			t.Fatalf("%s line width %d exceeds %d: %q", name, ansi.StringWidth(line), width, line)
		}
	}
	joined := strings.Join(projection.lines, "\n")
	if strings.Contains(joined, viewportPreviousLabel) || strings.Contains(joined, viewportNextLabel) {
		t.Fatalf("%s rendered legacy directional marker: %#v", name, projection.lines)
	}
	if projection.scrollbar.visible != (projection.hasPrevious || projection.hasNext) {
		t.Fatalf("%s scrollbar visibility %v disagrees with overflow (%v,%v): %#v", name, projection.scrollbar.visible, projection.hasPrevious, projection.hasNext, projection.scrollbar)
	}
	if projection.scrollbar.visible {
		geometry := projection.scrollbar
		if geometry.trackHeight != projection.availableRows || geometry.thumbLength < 1 || geometry.thumbTop < 0 || geometry.thumbTop+geometry.thumbLength > geometry.trackHeight {
			t.Fatalf("%s invalid scrollbar geometry %#v for %d rows", name, geometry, projection.availableRows)
		}
	}

	start := projection.renderOffset
	if start < 0 || start > len(content) {
		t.Fatalf("%s invalid render offset %d for %d lines", name, start, len(content))
	}
	for index, line := range projection.lines[:projection.visibleRows] {
		if start+index >= len(content) {
			break
		}
		if ansi.StringWidth(content[start+index]) > width && !strings.HasSuffix(line, safeTextEllipsis) {
			t.Fatalf("%s truncated line lacks ellipsis: source %q projected %q", name, content[start+index], line)
		}
	}
}

func assertSC007ViewOverflow(t testing.TB, name, view string, projection viewportProjection) {
	t.Helper()
	if strings.Contains(view, viewportPreviousLabel) || strings.Contains(view, viewportNextLabel) {
		t.Fatalf("%s rendered legacy directional marker", name)
	}
	if projection.scrollbar.visible && !strings.Contains(view, "█") {
		t.Fatalf("%s projection scrollbar is absent from Model.View", name)
	}
}
