package tui

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const (
	sc005ContentWidth  = 24
	sc005ContentHeight = 8
)

type sc005FieldCase struct {
	name  string
	label string
	value string
}

type sc005SurfaceCase struct {
	name           string
	expectEllipsis bool
	render         func(sc005FieldCase, string, int, int) sc005SurfaceResult
}

type sc005SurfaceResult struct {
	logical          []string
	visible          []string
	panel            string
	completeIdentity []string
}

func TestSC005LongUnicodeControlFieldMatrix(t *testing.T) {
	fields := sc005FieldCases()
	surfaces := sc005SurfaceCases()
	if coverage := len(fields) * len(surfaces); coverage < 100 {
		t.Fatalf("SC-005 field coverage = %d, want at least 100", coverage)
	} else {
		t.Logf("SC-005 field coverage: %d cases (%d fields x %d surfaces)", coverage, len(fields), len(surfaces))
	}

	for fieldIndex, field := range fields {
		for _, surface := range surfaces {
			t.Run(fmt.Sprintf("%02d_%s/%s", fieldIndex, field.name, surface.name), func(t *testing.T) {
				canary := fmt.Sprintf("SC005-SECRET-CANARY-%02d-%s", fieldIndex, surface.name)
				result := surface.render(field, canary, sc005ContentWidth, sc005ContentHeight)
				assertSC005SurfaceResult(t, surface, field, canary, sc005ContentWidth, sc005ContentHeight, result)
			})
		}
	}
}

func sc005FieldCases() []sc005FieldCase {
	return []sc005FieldCase{
		{name: "long_ascii", label: strings.Repeat("destination-label-", 7), value: "/" + strings.Repeat("production-segment/", 8) + "tail"},
		{name: "wide_cjk", label: strings.Repeat("接続先界", 12), value: strings.Repeat("東京駅/界面/", 14) + "終端"},
		{name: "emoji_zwj", label: strings.Repeat("operator-👩\u200d💻-", 8), value: strings.Repeat("host-👨\u200d👩\u200d👧\u200d👦-", 12) + "tail"},
		{name: "combining", label: strings.Repeat("e\u0301tiquette-", 12), value: strings.Repeat("re\u0301sume\u0301/cafe\u0301/", 14) + "fin"},
		{name: "bidi_controls", label: strings.Repeat("label\u061c\u202e\u2066", 12), value: strings.Repeat("path\u202e/host\u2069/", 14) + "tail"},
		{name: "ansi_sequences", label: strings.Repeat("label\x1b[31mred\x1b[0m", 8), value: strings.Repeat("segment\x1b[2J\x1b[H/", 14) + "tail"},
		{name: "line_controls", label: strings.Repeat("label\r\n", 14), value: strings.Repeat("row\ncolumn\rvalue/", 12) + "tail"},
		{name: "c0_controls", label: strings.Repeat("label\x00\x07\x08\x0b", 12), value: strings.Repeat("part\x01\x02\x03\x1f/", 14) + "tail"},
		{name: "c1_controls", label: strings.Repeat("label\u0085\u009b", 12), value: strings.Repeat("part\u0080\u008d\u009f/", 14) + "tail"},
		{name: "invalid_utf8", label: strings.Repeat(string([]byte{'l', 'a', 'b', 0xff, 0xc3, '('}), 12), value: strings.Repeat(string([]byte{'v', 'a', 'l', 0xfe, 0xc0, 0xaf, '/'}), 14) + "tail"},
		{name: "tabs_and_delete", label: strings.Repeat("long\tlabel\x7f", 12), value: strings.Repeat("tab\tpath\x7fsegment/", 12) + "tail"},
		{name: "zero_width_mixed", label: strings.Repeat("label\u200b\u200c\u200d", 12), value: strings.Repeat("界e\u0301🙂\u200b/segment/", 14) + "tail"},
	}
}

func sc005SurfaceCases() []sc005SurfaceCase {
	return []sc005SurfaceCase{
		{name: "details", expectEllipsis: true, render: sc005RenderDetails},
		{name: "connection_form", expectEllipsis: true, render: sc005RenderConnectionForm},
		{name: "folder_form", expectEllipsis: true, render: sc005RenderFolderForm},
		{name: "connect_confirmation", render: sc005RenderConnectConfirmation},
		{name: "delete_confirmation", render: sc005RenderDeleteConfirmation},
		{name: "help", expectEllipsis: true, render: sc005RenderHelp},
		{name: "operation_error", render: sc005RenderOperationError},
		{name: "trust", expectEllipsis: true, render: sc005RenderTrust},
		{name: "secret", expectEllipsis: true, render: sc005RenderSecret},
	}
}

func sc005RenderDetails(field sc005FieldCase, canary string, width, height int) sc005SurfaceResult {
	root := testFolder("root", "", "/", 1)
	connection := testConnection("sc005-details", root.ID, "/sc005-details", 7)
	connection.Name = field.value
	connection.Path = field.value
	connection.Host = field.value
	connection.Username = field.value
	connection.IdentityFile = field.value
	connection.CredentialRef = canary
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	state, _ := newDetailState(snapshot, connection.ID)
	state.fields = append([]detailField{{label: field.label, value: field.value}}, state.fields...)
	for index := 0; index < height; index++ {
		state.fields = append(state.fields, detailField{label: fmt.Sprintf("Extra %d", index), value: field.value})
	}
	logical := state.content(width, newStyles(true))
	projection := state.project(height, width, newStyles(true))
	return sc005ProjectedResult(logical, projection, width, height)
}

func sc005RenderConnectionForm(field sc005FieldCase, canary string, width, height int) sc005SurfaceResult {
	connection := testConnection("sc005-form", syntheticRootID, "/sc005-form", 7)
	connection.CredentialRef = canary
	form := newConnectionForm(&connection)
	form.setAuthMethod(app.AuthMethodKey)
	form.inputs[fieldName].SetValue(field.value)
	form.inputs[fieldHost].SetValue(field.value)
	form.inputs[fieldIdentity].SetValue(field.value)
	form.setFocus(fieldName)
	form.setDimensions(width, height)
	logical, _ := form.content(newStyles(true))
	return sc005ProjectedResult(logical, form.project(newStyles(true)), width, height)
}

func sc005RenderFolderForm(field sc005FieldCase, _ string, width, height int) sc005SurfaceResult {
	destination := testFolder("destination", syntheticRootID, field.value, 7)
	form := newFolderForm(nil, app.ItemSelector{Path: field.value})
	form.setDestination(destination)
	form.input.SetValue(field.value)
	logical, active := form.modalLines(width, newStyles(true))
	state := modalState{kind: modalKindFolderCreate, payload: folderCreatePayload{form: form}}
	projection := projectModalViewport(state, sc005ModalControls(logical), height, width, active)
	return sc005ProjectedResult(logical, projection, width, height)
}

func sc005RenderConnectConfirmation(field sc005FieldCase, canary string, width, height int) sc005SurfaceResult {
	connection := testConnection("sc005-connect", syntheticRootID, "/sc005-connect", 7)
	connection.Path = field.value
	connection.Host = field.value
	connection.CredentialRef = canary
	state := modalState{kind: modalKindConnectConfirmation, payload: connectConfirmationPayload{confirmation: newConnectConfirmation(connection)}}
	result := sc005RenderModalState(state, width, height)
	result.completeIdentity = []string{field.value, net.JoinHostPort(field.value, "22")}
	return result
}

func sc005RenderDeleteConfirmation(field sc005FieldCase, _ string, width, height int) sc005SurfaceResult {
	scope := app.ConnectionDeleteScope{ID: "sc005-delete", Path: field.value, Host: field.value, Username: "operator", Revision: 7, HasRememberedPassword: true}
	state := modalState{kind: modalKindDeleteConnection, payload: deleteConnectionPayload{confirmation: newDeleteConfirmation(scope)}}
	result := sc005RenderModalState(state, width, height)
	result.completeIdentity = []string{field.value, "operator@" + field.value}
	return result
}

func sc005RenderHelp(field sc005FieldCase, _ string, width, height int) sc005SurfaceResult {
	lines := make([]string, height*2)
	for index := range lines {
		lines[index] = fmt.Sprintf("%s %02d %s", field.label, index, field.value)
	}
	state := modalState{kind: modalKindHelp, payload: helpPayload{lines: lines}}
	return sc005RenderModalState(state, width, height)
}

func sc005RenderOperationError(field sc005FieldCase, canary string, width, height int) sc005SurfaceResult {
	err := fmt.Errorf("%s: %w", canary, app.ErrInvalidRequest)
	modal := newErrorModal(field.label, field.value, err)
	state := modalState{kind: modalKindOperationError, payload: operationErrorPayload{modal: modal}}
	result := sc005RenderModalState(state, width, height)
	result.completeIdentity = []string{field.label, field.value}
	return result
}

func sc005RenderTrust(field sc005FieldCase, canary string, width, height int) sc005SurfaceResult {
	prompt := app.TrustDecisionPrompt{
		Status: app.HostTrustChanged,
		Host: app.PresentedHost{
			Endpoint: app.HostEndpoint{CanonicalHost: field.value, Port: 22}, RemoteAddress: field.value,
			KeyAlgorithm: field.value, FingerprintSHA256: field.value, PublicKey: []byte(canary),
		},
		Known: &app.TrustedHost{FingerprintSHA256: field.value, PublicKey: []byte(canary)},
	}
	logical := strings.Split(newTrustPrompt(prompt).view(newStyles(true)), "\n")
	return sc005RawPanelResult(logical, width, height)
}

func sc005RenderSecret(field sc005FieldCase, canary string, width, height int) sc005SurfaceResult {
	prompt := newSecretPrompt(app.SecretPassword, field.value)
	secret := []byte(canary)
	prompt.setForTest(secret)
	logical := strings.Split(prompt.view(newStyles(true)), "\n")
	return sc005RawPanelResult(logical, width, height)
}

func sc005RenderModalState(state modalState, width, height int) sc005SurfaceResult {
	logical, active := modalContent(state, newStyles(true), width, nil)
	projection := projectModalViewport(state, logical, height, width, active)
	return sc005ProjectedResult(logical, projection, width, height)
}

func sc005ModalControls(lines []string) []string {
	projected := append([]string(nil), lines...)
	projected[len(projected)-1] = modalControlLine(projected[len(projected)-1])
	return projected
}

func sc005ProjectedResult(logical []string, projection viewportProjection, width, height int) sc005SurfaceResult {
	rect := layoutRect{width: width + 4, height: height + 2}
	panel := renderRegionPanelWithScrollbar("SC-005", projection.lines, rect, newStyles(true), projection.scrollbar, projection.scrollbarStart)
	return sc005SurfaceResult{logical: logical, visible: projection.lines, panel: panel}
}

func sc005RawPanelResult(logical []string, width, height int) sc005SurfaceResult {
	visible := viewportTruncateLines(logical[:min(len(logical), height)], width)
	panel := renderRegionPanel("SC-005", logical, layoutRect{width: width + 4, height: height + 2})
	return sc005SurfaceResult{logical: logical, visible: visible, panel: panel}
}

func assertSC005SurfaceResult(t *testing.T, surface sc005SurfaceCase, field sc005FieldCase, canary string, width, height int, result sc005SurfaceResult) {
	t.Helper()
	if len(result.visible) > height {
		t.Fatalf("local content height = %d, want <= %d", len(result.visible), height)
	}
	for _, line := range result.visible {
		assertTerminalSafe(t, line, width)
	}

	panelLines := strings.Split(result.panel, "\n")
	if len(panelLines) != height+2 {
		t.Fatalf("panel height = %d, want %d: %q", len(panelLines), height+2, result.panel)
	}
	for row, line := range panelLines {
		assertTerminalSafe(t, line, width+4)
		if got := ansi.StringWidth(line); got != width+4 {
			t.Fatalf("panel row %d width = %d, want %d: %q", row, got, width+4, line)
		}
		if row > 0 && row < len(panelLines)-1 && (!strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│")) {
			t.Fatalf("panel row %d overlapped its border: %q", row, line)
		}
	}

	allOutput := strings.Join(result.logical, "\n") + "\n" + result.panel
	if strings.Contains(allOutput, canary) {
		t.Fatalf("secret canary rendered: %q", canary)
	}
	for _, identity := range result.completeIdentity {
		projected := safeText(identity, int(^uint(0)>>1))
		if !strings.Contains(sc005CompactIdentity(strings.Join(result.logical, "\n")), sc005CompactIdentity(projected)) {
			t.Fatalf("required identity was not wrapped completely: source %q, logical output %#v", identity, result.logical)
		}
	}
	if surface.expectEllipsis && ansi.StringWidth(safeText(field.value, int(^uint(0)>>1))) > width && !strings.Contains(allOutput, safeTextEllipsis) {
		t.Fatalf("oversized field has neither safe ellipsis nor a full-wrap contract: %q", field.value)
	}
}

func sc005CompactIdentity(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, ansi.Strip(value))
}

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
