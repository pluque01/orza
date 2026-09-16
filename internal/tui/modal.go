package tui

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

type confirmationChoice int

const (
	confirmationCancel confirmationChoice = iota
)

type deleteConfirmation struct {
	scope    app.ConnectionDeleteScope
	endpoint string
	choice   confirmationChoice
}

type folderDeleteConfirmation struct {
	scope  app.FolderDeleteScope
	choice confirmationChoice
}

type deleteConnectionPayload struct{ confirmation *deleteConfirmation }
type deleteFolderPayload struct{ confirmation *folderDeleteConfirmation }
type connectConfirmationPayload struct{ confirmation *connectConfirmation }
type helpPayload struct{ lines []string }
type operationErrorPayload struct{ modal *errorModal }
type sshFailurePayload struct {
	modal        *errorModal
	confirmation *connectConfirmation
}

const modalControlsPrefix = "\x00controls\x00"

func (payload deleteConnectionPayload) validModalPayload() bool {
	return payload.confirmation != nil && payload.confirmation.scope.ID != ""
}

func (payload deleteFolderPayload) validModalPayload() bool {
	return payload.confirmation != nil && payload.confirmation.scope.ID != ""
}

func (payload connectConfirmationPayload) validModalPayload() bool {
	return payload.confirmation != nil && payload.confirmation.connection.ID != ""
}

func (payload helpPayload) validModalPayload() bool { return len(payload.lines) != 0 }
func (payload operationErrorPayload) validModalPayload() bool {
	return payload.modal != nil && payload.modal.failure == nil
}
func (payload sshFailurePayload) validModalPayload() bool {
	return payload.modal != nil && payload.modal.failure != nil
}

func (payload unsavedChangesPayload) validModalPayload() bool { return payload.valid() }

type connectConfirmation struct {
	connection app.Connection
	previous   *app.SSHAttemptTarget
}

func newConnectConfirmation(connection app.Connection) *connectConfirmation {
	return &connectConfirmation{connection: connection}
}

func newRetryConnectConfirmation(previous app.SSHAttemptTarget, connection app.Connection) *connectConfirmation {
	copy := previous
	return &connectConfirmation{connection: connection, previous: &copy}
}

func (m *connectConfirmation) confirmed(msg tea.KeyPressMsg) bool { return msg.String() == "y" }

func newFolderDeleteConfirmation(scope app.FolderDeleteScope) *folderDeleteConfirmation {
	return &folderDeleteConfirmation{scope: scope, choice: confirmationCancel}
}

func (m *folderDeleteConfirmation) confirmed(msg tea.KeyPressMsg) bool { return msg.String() == "y" }

func countLabel(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func newDeleteConfirmation(scope app.ConnectionDeleteScope, endpoint ...string) *deleteConfirmation {
	capturedEndpoint := scope.Host
	if len(endpoint) != 0 && endpoint[0] != "" {
		capturedEndpoint = endpoint[0]
	}
	return &deleteConfirmation{scope: scope, endpoint: capturedEndpoint, choice: confirmationCancel}
}

func (m *deleteConfirmation) confirmed(msg tea.KeyPressMsg) bool {
	return msg.String() == "y"
}

type unsavedChangesIntent uint8

const (
	unsavedIntentCancel unsavedChangesIntent = iota + 1
	unsavedIntentQuit
)

type unsavedChangesPayload struct {
	intent unsavedChangesIntent
	target string
}

func (payload unsavedChangesPayload) valid() bool {
	return payload.intent == unsavedIntentCancel || payload.intent == unsavedIntentQuit
}

func (payload unsavedChangesPayload) lines(width int, modalStyle ...styles) []string {
	action := "Cancel connection editing?"
	if payload.intent == unsavedIntentQuit {
		action = "Quit with unsaved changes?"
	}
	fields := []displayField{{label: "Action", value: action}}
	if payload.target != "" {
		fields = append(fields, displayField{label: "Target", value: payload.target})
	}
	lines := renderWrappedModalFields(modalRenderStyle(modalStyle), width, fields)
	return append(lines, modalControlLine("s Save"), modalControlLine("d Discard"), modalControlLine("Esc Cancel"))
}

func modalRenderStyle(optional []styles) styles {
	if len(optional) != 0 {
		return optional[0]
	}
	return newStyles(true)
}

func renderWrappedModalFields(style styles, width int, fields []displayField) []string {
	if len(fields) == 0 {
		return nil
	}
	projected := make([]displayField, len(fields))
	for index, field := range fields {
		projected[index] = displayField{label: field.label, value: safeText(field.value, max(1, len(field.value)*6+1))}
	}

	group := newStructuredFieldGroup(projected, 0, width, false)
	if group.mode == fieldModeAligned {
		expanded := make([]displayField, 0, len(projected))
		for _, field := range projected {
			wrapped := wrapModalValue(field.value, max(1, group.availableValueWidth()))
			for index, value := range wrapped {
				label := ""
				if index == 0 {
					label = field.label
				}
				expanded = append(expanded, displayField{label: label, value: value})
			}
		}
		return newStructuredFieldGroup(expanded, 0, width, false).render(style)
	}

	continuations := make([][]string, len(projected))
	for index := range projected {
		wrapped := wrapModalValue(projected[index].value, max(1, width-2))
		projected[index].value = wrapped[0]
		continuations[index] = wrapped[1:]
	}
	group = newStructuredFieldGroup(projected, 0, width, false)
	base := group.render(style)
	lines := make([]string, 0, len(base))
	for index := range projected {
		lines = append(lines, base[index*2], base[index*2+1])
		for _, continuation := range continuations[index] {
			lines = append(lines, "  "+continuation)
		}
	}
	return lines
}

func wrapModalValue(value string, width int) []string {
	if value == "" {
		return []string{""}
	}
	return strings.Split(ansi.Hardwrap(value, max(1, width), false), "\n")
}

func appendWrappedModalLine(lines []string, label, value string, width int) []string {
	width = max(1, width)
	projected := safeText(value, max(1, len(value)*6+1))
	wrapped := strings.Split(ansi.Hardwrap(label+projected, width, false), "\n")
	return append(lines, wrapped...)
}

func modalContent(state modalState, style styles, width int, help []string) ([]string, int) {
	if state.helpVisible {
		return append(safeHelpLines(help), modalControlLine("?/Esc Close")), noActiveLine
	}
	if state.operationStatus != "" {
		captured := state
		captured.operationStatus = ""
		body, active := modalContent(captured, style, width, help)
		if controls := modalPriorityStart(body); controls >= 0 {
			body = body[:controls]
			if active >= controls {
				active = noActiveLine
			}
		}
		lines := append([]string{safeText(state.operationStatus, width)}, body...)
		lines = append(lines, modalControlLine("Esc Cancel"), modalControlLine("? Help"), modalControlLine("q Quit"))
		if active != noActiveLine {
			active++
		}
		return lines, active
	}
	if state.conflict != nil {
		conflict := state.conflict
		header := []string{style.warningMessage("captured target is no longer current.")}
		if conflict.detailVisible {
			header = append(header, renderWrappedModalFields(style, width, []displayField{
				{label: "Target", value: conflict.target.path},
				{label: "ID/revision", value: fmt.Sprintf("%s/%d", conflict.target.id, conflict.target.revision)},
			})...)
		}
		body, active := modalPayloadContent(state, style, width)
		if controls := modalPriorityStart(body); controls >= 0 {
			body = body[:controls]
		}
		lines := append(header, body...)
		lines = append(lines, modalControlLine("r Reload"), modalControlLine("b Back"), modalControlLine("Esc Cancel warning"), modalControlLine("? Help"), modalControlLine("q Quit"))
		return lines, max(0, active+len(header))
	}
	if state.recoverableError != "" {
		header := []string{style.failureMessage(state.recoverableError)}
		body, active := modalPayloadContent(state, style, width)
		return append(header, body...), max(0, active+len(header))
	}
	return modalPayloadContent(state, style, width)
}

func modalPayloadContent(state modalState, style styles, width int) ([]string, int) {
	if !validModalPayloadForKind(state.kind, state.payload) {
		return []string{"Error: invalid modal payload", modalControlLine("Esc Close")}, noActiveLine
	}
	switch payload := state.payload.(type) {
	case folderCreatePayload:
		lines, active := payload.form.modalLines(width, style)
		lines[len(lines)-1] = modalControlLine(lines[len(lines)-1])
		return lines, active
	case folderEditPayload:
		lines, active := payload.form.modalLines(width, style)
		lines[len(lines)-1] = modalControlLine(lines[len(lines)-1])
		return lines, active
	case movePickerPayload:
		lines, active := payload.picker.modalLines(width, style)
		lines[len(lines)-1] = modalControlLine(lines[len(lines)-1])
		return lines, active
	case deleteConnectionPayload:
		return deleteConnectionLines(payload.confirmation, width, style), noActiveLine
	case deleteFolderPayload:
		return deleteFolderLines(payload.confirmation.scope, width, style), noActiveLine
	case connectConfirmationPayload:
		return connectConfirmationLines(payload.confirmation, width, style), noActiveLine
	case unsavedChangesPayload:
		return payload.lines(width, style), noActiveLine
	case helpPayload:
		return append(safeHelpLines(payload.lines), modalControlLine("?/Esc Close")), noActiveLine
	case operationErrorPayload:
		return operationErrorLines(payload.modal, width, style), noActiveLine
	case sshFailurePayload:
		if payload.confirmation != nil {
			return connectConfirmationLines(payload.confirmation, width, style), noActiveLine
		}
		return sshFailureLines(payload.modal, width, style), noActiveLine
	default:
		return []string{"Error: invalid modal payload", modalControlLine("Esc Close")}, noActiveLine
	}
}

func safeHelpLines(lines []string) []string {
	projected := make([]string, len(lines))
	for index, line := range lines {
		projected[index] = safeText(line, int(^uint(0)>>1))
	}
	return projected
}

func validModalPayloadForKind(kind modalKind, value any) bool {
	switch kind {
	case modalKindFolderCreate:
		payload, ok := value.(folderCreatePayload)
		return ok && payload.validModalPayload()
	case modalKindFolderEdit:
		payload, ok := value.(folderEditPayload)
		return ok && payload.validModalPayload()
	case modalKindMovePicker:
		payload, ok := value.(movePickerPayload)
		return ok && payload.validModalPayload()
	case modalKindDeleteConnection:
		payload, ok := value.(deleteConnectionPayload)
		return ok && payload.validModalPayload()
	case modalKindDeleteFolder:
		payload, ok := value.(deleteFolderPayload)
		return ok && payload.validModalPayload()
	case modalKindConnectConfirmation:
		payload, ok := value.(connectConfirmationPayload)
		return ok && payload.validModalPayload()
	case modalKindUnsavedChanges:
		payload, ok := value.(unsavedChangesPayload)
		return ok && payload.validModalPayload()
	case modalKindHelp:
		payload, ok := value.(helpPayload)
		return ok && payload.validModalPayload()
	case modalKindOperationError:
		payload, ok := value.(operationErrorPayload)
		return ok && payload.validModalPayload()
	case modalKindSSHFailure:
		payload, ok := value.(sshFailurePayload)
		return ok && payload.validModalPayload()
	default:
		return false
	}
}

func deleteConnectionLines(confirmation *deleteConfirmation, width int, modalStyle ...styles) []string {
	if confirmation == nil {
		return nil
	}
	scope := confirmation.scope
	endpoint := confirmation.endpoint
	username := scope.Username
	if username == "" {
		username = "(default)"
	}
	endpoint = username + "@" + endpoint
	fields := []displayField{
		{label: "Action", value: "permanently delete connection"},
		{label: "Path", value: scope.Path},
		{label: "Target", value: endpoint},
		{label: "ID/revision", value: fmt.Sprintf("%s/%d", scope.ID, scope.Revision)},
	}
	if scope.HasRememberedPassword {
		fields = append(fields, displayField{label: "Effect", value: "also deletes the saved password from the operating system credential store"})
	}
	lines := renderWrappedModalFields(modalRenderStyle(modalStyle), width, fields)
	return append(lines, modalControlLine("y Confirm"), modalControlLine("Enter/Esc Cancel"), modalControlLine("? Help"))
}

func deleteFolderLines(scope app.FolderDeleteScope, width int, modalStyle ...styles) []string {
	lines := renderWrappedModalFields(modalRenderStyle(modalStyle), width, []displayField{
		{label: "Action", value: "permanently delete the folder subtree"},
		{label: "Path", value: scope.Path},
		{label: "ID/revision", value: fmt.Sprintf("%s/%d", scope.ID, scope.Revision)},
		{label: "Scope", value: fmt.Sprintf("%s, %s, %s", countLabel(scope.Folders, "folder"), countLabel(scope.Connections, "connection"), countLabel(scope.RememberedCredentials, "remembered credential"))},
	})
	return append(lines, modalControlLine("y Confirm"), modalControlLine("Enter/Esc Cancel"), modalControlLine("? Help"))
}

func connectConfirmationLines(confirmation *connectConfirmation, width int, modalStyle ...styles) []string {
	if confirmation == nil {
		return nil
	}
	fields := make([]displayField, 0, 7)
	if confirmation.previous != nil {
		fields = append(fields,
			displayField{label: "Previous path", value: confirmation.previous.Path},
			displayField{label: "Previous endpoint", value: net.JoinHostPort(confirmation.previous.Host, strconv.Itoa(int(confirmation.previous.Port)))},
			displayField{label: "Previous ID/revision", value: fmt.Sprintf("%s/%d", confirmation.previous.ID, confirmation.previous.Revision)},
		)
	}
	connection := confirmation.connection
	fields = append(fields,
		displayField{label: "Path", value: connection.Path},
		displayField{label: "Endpoint", value: net.JoinHostPort(connection.Host, strconv.Itoa(int(connection.Port)))},
		displayField{label: "ID", value: string(connection.ID)},
		displayField{label: "Revision", value: fmt.Sprint(connection.Revision)},
	)
	lines := append([]string{"Connect to SSH target?"}, renderWrappedModalFields(modalRenderStyle(modalStyle), width, fields)...)
	return append(lines, modalControlLine("y Confirm"), modalControlLine("Enter/Esc Cancel"), modalControlLine("? Help"))
}

func operationErrorLines(modal *errorModal, width int, modalStyle ...styles) []string {
	if modal == nil {
		return nil
	}
	message := modal.message
	if modal.kind == app.ErrorKindConflict {
		message = "Stale version was never overwritten. The connection changed."
	}
	lines := renderWrappedModalFields(modalRenderStyle(modalStyle), width, []displayField{
		{label: "Operation", value: modal.operation},
		{label: "Target", value: modal.target},
		{label: "Cause", value: message},
	})
	if modal.kind == app.ErrorKindConflict || modal.kind == app.ErrorKindNotFound {
		return append(lines, modalControlLine("r Reload"), modalControlLine("b/Esc Back"), modalControlLine("q Quit"), modalControlLine("? Help"))
	}
	if modal.retry != nil && modal.retry.valid() {
		lines = append(lines, modalControlLine("r Retry"))
	}
	return append(lines, modalControlLine("b/Esc Back"), modalControlLine("q Quit"), modalControlLine("? Help"))
}

func sshFailureLines(modal *errorModal, width int, modalStyle ...styles) []string {
	if modal == nil || modal.failure == nil {
		return nil
	}
	fields := []displayField{
		{label: "Summary", value: modal.failure.Summary},
		{label: "Category", value: string(modal.failure.Category)},
		{label: "Path", value: modal.attempt.Path},
	}
	if modal.attempt.Host != "" && modal.attempt.Port != 0 {
		fields = append(fields, displayField{label: "Endpoint", value: net.JoinHostPort(modal.attempt.Host, strconv.Itoa(int(modal.attempt.Port)))})
	}
	fields = append(fields,
		displayField{label: "ID/revision", value: fmt.Sprintf("%s/%d", modal.attempt.ID, modal.attempt.Revision)},
		displayField{label: "Stage", value: string(modal.failure.Stage)},
		displayField{label: "Recommendation", value: modal.failure.Recommendation},
	)
	if modal.detailVisible && modal.failure.TechnicalDetail != "" {
		fields = append(fields, displayField{label: "Technical detail", value: modal.failure.TechnicalDetail})
	}
	lines := append([]string{"SSH startup failed"}, renderWrappedModalFields(modalRenderStyle(modalStyle), width, fields)...)
	return append(lines, sshFailureModalControlLines(modal)...)
}

func sshFailureModalControlLines(modal *errorModal) []string {
	controls := []string{modalControlLine("d Detail")}
	switch modal.recovery {
	case recoveryMissing, recoveryConflict:
		controls = append(controls, modalControlLine("r Reload"))
	case recoveryResolving:
		controls = []string{modalControlLine("Resolving current target...")}
	default:
		controls = append(controls, modalControlLine("r Retry"), modalControlLine("e Edit"))
	}
	return append(controls, modalControlLine("b/Esc Back"), modalControlLine("q Quit"), modalControlLine("? Help"))
}

func renderModalOverlay(background string, state modalState, layout layoutState, style styles, help []string) string {
	rect := layout.modalOverlay()
	if rect.width <= 0 || rect.height <= 0 {
		return background
	}
	contentWidth, contentHeight := rect.contentWidth(), rect.contentHeight()
	lines, active := modalContent(state, style, contentWidth, help)
	projection := projectModalViewport(state, lines, contentHeight, contentWidth, active)
	panel := renderRegionPanelWithScrollbar(style.regionTitle(modalTitle(state.kind), true), projection.lines, rect, style, projection.scrollbar, projection.scrollbarStart)
	return placeOverlay(background, panel, rect, layout.width, layout.height)
}

func projectModalViewport(state modalState, lines []string, rows, width, active int) viewportProjection {
	priority := modalPriorityStart(lines)
	if priority < 0 || priority >= len(lines) {
		return state.viewport.project(lines, rows, width, active)
	}

	projection := viewportProjection{
		logicalOffset: state.viewport.logicalOffset,
		activeLine:    active,
	}
	if rows <= 0 || width <= 0 {
		return projection
	}
	controlLines := packModalControls(lines[priority:], width, rows)
	if len(controlLines) >= rows {
		projection.lines = controlLines[:rows]
		return projection
	}

	leadingCount := min(modalLeadingFixedCount(state), priority)
	leadingLines := lines[:leadingCount]
	remainingRows := rows - len(controlLines)
	if len(leadingLines) >= remainingRows {
		projection.lines = append(append([]string(nil), leadingLines[:remainingRows]...), controlLines...)
		return projection
	}
	body := lines[leadingCount:priority]
	bodyRows := remainingRows - len(leadingLines)
	projection.contentLength = len(body)
	projection.availableRows = bodyRows
	bodyActive := active
	if active < leadingCount || active >= priority {
		bodyActive = noActiveLine
	} else {
		bodyActive -= leadingCount
	}
	bodyProjection := state.viewport.project(body, bodyRows, width, bodyActive)
	projection.lines = append(append(append([]string(nil), leadingLines...), bodyProjection.lines...), controlLines...)
	projection.renderOffset = bodyProjection.renderOffset
	projection.hasPrevious = bodyProjection.hasPrevious
	projection.hasNext = bodyProjection.hasNext
	projection.visibleRows = bodyProjection.visibleRows
	projection.activeLine = bodyProjection.activeLine
	projection.scrollbar = bodyProjection.scrollbar
	projection.scrollbarStart = len(leadingLines)
	return projection
}

func modalLeadingFixedCount(state modalState) int {
	switch {
	case state.helpVisible:
		return 0
	case state.operationStatus != "":
		return 1
	case state.conflict != nil:
		if state.conflict.detailVisible {
			return 3
		}
		return 1
	case state.recoverableError != "":
		return 1
	default:
		return 0
	}
}

func modalControlLine(control string) string {
	return modalControlsPrefix + control
}

func modalPriorityStart(lines []string) int {
	for index, line := range lines {
		if strings.HasPrefix(line, modalControlsPrefix) {
			return index
		}
	}
	return noActiveLine
}

func packModalControls(lines []string, width, rows int) []string {
	controls := make([]string, 0, len(lines))
	for _, line := range lines {
		controls = append(controls, strings.TrimPrefix(line, modalControlsPrefix))
	}
	packed := make([]string, 0, min(rows, len(controls)))
	for _, control := range controls {
		control = viewportEllipsis(control, width)
		if len(packed) == 0 || ansi.StringWidth(packed[len(packed)-1])+1+ansi.StringWidth(control) > width {
			packed = append(packed, control)
		} else {
			packed[len(packed)-1] += " " + control
		}
	}
	return packed
}

func modalTitle(kind modalKind) string {
	switch kind {
	case modalKindFolderCreate:
		return "Create Folder"
	case modalKindFolderEdit:
		return "Edit Folder"
	case modalKindMovePicker:
		return "Move"
	case modalKindDeleteConnection, modalKindDeleteFolder:
		return "Delete"
	case modalKindConnectConfirmation:
		return "Connect"
	case modalKindUnsavedChanges:
		return "Unsaved Changes"
	case modalKindHelp:
		return "Help"
	case modalKindOperationError:
		return "Operation Error"
	case modalKindSSHFailure:
		return "SSH Failure"
	default:
		return "Panel"
	}
}

func placeOverlay(background, overlay string, rect layoutRect, width, height int) string {
	base := strings.Split(fitContent(background, width, height), "\n")
	for len(base) < height {
		base = append(base, "")
	}
	overlayLines := strings.Split(overlay, "\n")
	for row := 0; row < rect.height && row < len(overlayLines); row++ {
		line := base[rect.y+row]
		lineWidth := ansi.StringWidth(line)
		if lineWidth < width {
			line += strings.Repeat(" ", width-lineWidth)
		}
		left := ansi.Cut(line, 0, rect.x)
		right := ansi.Cut(line, rect.x+rect.width, width)
		base[rect.y+row] = left + overlayLines[row] + right
	}
	return strings.Join(base[:height], "\n")
}
