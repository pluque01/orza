package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
)

type connectionField int

const (
	fieldName connectionField = iota
	fieldFolder
	fieldHost
	fieldPort
	fieldUsername
	fieldAuth
	fieldIdentity
	fieldRemember
	fieldSave
	fieldCount
)

var formLabels = [...]string{
	"Name", "Folder", "Host", "Port", "User", "Method", "Identity file", "Remember password", "Save",
}

var authenticationMethods = [...]app.AuthMethod{
	app.AuthMethodAgent, app.AuthMethodKey, app.AuthMethodPassword,
}

var authenticationMethodLabels = [...]string{"Agent", "Key", "Password"}

type connectionForm struct {
	inputs       [fieldCount]textField
	focus        connectionField
	errors       map[connectionField]string
	formError    string
	original     *app.Connection
	baseline     [fieldCount]string
	authMethod   app.AuthMethod
	baselineAuth app.AuthMethod
	remember     bool
	destination  *app.Folder
	viewport     viewportState
	width        int
	height       int
}

func newConnectionForm(connection *app.Connection) *connectionForm {
	form := &connectionForm{
		errors:   make(map[connectionField]string),
		viewport: newViewportState(0),
		width:    76,
		height:   21,
	}
	for field := fieldName; field <= fieldIdentity; field++ {
		if field == fieldAuth {
			continue
		}
		form.inputs[field] = newTextField()
	}
	form.inputs[fieldFolder].SetValue("/")
	form.inputs[fieldPort].SetValue(strconv.Itoa(int(domain.DefaultSSHPort)))
	form.authMethod = app.AuthMethodAgent
	if connection != nil {
		copy := *connection
		form.original = &copy
		form.inputs[fieldName].SetValue(connection.Name)
		folder := "/"
		if split := strings.LastIndex(connection.Path, "/"); split > 0 {
			folder = connection.Path[:split]
		}
		form.inputs[fieldFolder].SetValue(folder)
		form.inputs[fieldHost].SetValue(connection.Host)
		form.inputs[fieldPort].SetValue(strconv.Itoa(int(connection.Port)))
		form.inputs[fieldUsername].SetValue(connection.Username)
		form.authMethod = connection.AuthMethod
		form.inputs[fieldIdentity].SetValue(connection.IdentityFile)
	}
	for field := fieldName; field <= fieldIdentity; field++ {
		if field == fieldAuth {
			continue
		}
		form.baseline[field] = form.inputs[field].Value()
	}
	form.baselineAuth = form.authMethod
	form.setDimensions(form.width, form.height)
	form.setFocus(fieldName)
	return form
}

func (f *connectionForm) focusedField() connectionField { return f.focus }

func (f *connectionForm) selectedAuthMethod() app.AuthMethod { return f.authMethod }

func (f *connectionForm) setAuthMethod(method app.AuthMethod) {
	for _, candidate := range authenticationMethods {
		if method == candidate {
			f.authMethod = method
			f.syncDependencies()
			return
		}
	}
}

func (f *connectionForm) authMethodView() string {
	parts := make([]string, 0, len(authenticationMethods))
	for index, method := range authenticationMethods {
		label := authenticationMethodLabels[index]
		if method == f.authMethod {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " ")
}

func (f *connectionForm) setDestination(folder app.Folder) {
	copy := folder
	f.destination = &copy
	f.inputs[fieldFolder].SetValue(folder.Path)
	f.baseline[fieldFolder] = folder.Path
}

func (f *connectionForm) visibleFields() []connectionField {
	fields := []connectionField{fieldName, fieldFolder, fieldHost, fieldPort, fieldUsername, fieldAuth}
	if f.identityVisible() {
		fields = append(fields, fieldIdentity)
	}
	if f.passwordVisible() {
		fields = append(fields, fieldRemember)
	}
	return append(fields, fieldSave)
}

func (f *connectionForm) focusableFields() []connectionField {
	visible := f.visibleFields()
	fields := make([]connectionField, 0, len(visible)-1)
	for _, field := range visible {
		if field != fieldFolder {
			fields = append(fields, field)
		}
	}
	return fields
}

func (f *connectionForm) setDimensions(width, height int) {
	f.width = max(0, width)
	f.height = max(0, height)
	labelWidth := formLabelWidth(f.width)
	inputWidth := max(0, f.width-4-labelWidth-1)
	for field := fieldName; field <= fieldIdentity; field++ {
		if field == fieldAuth {
			continue
		}
		f.inputs[field].SetWidth(inputWidth)
	}
}

func formLabelWidth(width int) int {
	return min(16, max(0, (width-5)/3))
}

func (f *connectionForm) setFormError(message string) { f.formError = message }

func (f *connectionForm) setFocus(field connectionField) {
	for current := fieldName; current <= fieldIdentity; current++ {
		if current == fieldAuth {
			continue
		}
		f.inputs[current].Blur()
	}
	if field == fieldFolder {
		field = fieldHost
	}
	f.focus = field
	if field <= fieldIdentity && field != fieldFolder && field != fieldAuth {
		f.inputs[field].Focus()
	}
}

func (f *connectionForm) moveFocus(delta int) {
	fields := f.focusableFields()
	index := 0
	for current := range fields {
		if fields[current] == f.focus {
			index = current
			break
		}
	}
	index = (index + delta + len(fields)) % len(fields)
	f.setFocus(fields[index])
}

func (f *connectionForm) update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		keys := newKeyMap()
		switch {
		case key.Matches(keyMsg, keys.Next):
			f.moveFocus(1)
			return nil
		case key.Matches(keyMsg, keys.Previous), keyMsg.Key().Code == tea.KeyF2 && keyMsg.Key().Mod == 0:
			f.moveFocus(-1)
			return nil
		case f.focus == fieldRemember && keyMsg.String() == "space":
			f.remember = !f.remember
			return nil
		case f.focus == fieldAuth && keyMsg.Key().Mod == 0 && (keyMsg.String() == "left" || keyMsg.String() == "right"):
			index := 0
			for current := range authenticationMethods {
				if authenticationMethods[current] == f.authMethod {
					index = current
				}
			}
			if keyMsg.String() == "left" {
				index = (index + len(authenticationMethods) - 1) % len(authenticationMethods)
			} else {
				index = (index + 1) % len(authenticationMethods)
			}
			f.setAuthMethod(authenticationMethods[index])
			delete(f.errors, fieldAuth)
			f.formError = ""
			return nil
		case f.focus == fieldAuth:
			return nil
		}
	}
	if f.focus <= fieldIdentity && f.focus != fieldFolder && f.focus != fieldAuth {
		cmd := f.inputs[f.focus].Update(msg)
		if f.focus == fieldAuth {
			f.syncDependencies()
		}
		delete(f.errors, f.focus)
		f.formError = ""
		return cmd
	}
	return nil
}

func (f *connectionForm) syncDependencies() {
	if f.focus == fieldIdentity && !f.identityVisible() || f.focus == fieldRemember && !f.passwordVisible() {
		f.setFocus(fieldAuth)
	}
}

func (f *connectionForm) identityVisible() bool {
	return f.authMethod == app.AuthMethodKey
}

func (f *connectionForm) passwordVisible() bool {
	return f.authMethod == app.AuthMethodPassword
}

func (f *connectionForm) validate() bool {
	clear(f.errors)
	f.formError = ""
	if _, err := domain.NewName(f.inputs[fieldName].Value()); err != nil {
		f.errors[fieldName] = "enter a non-empty name without / or control characters"
	}
	if _, err := domain.ParseLogicalPath(f.inputs[fieldFolder].Value()); err != nil {
		f.errors[fieldFolder] = "enter an absolute existing folder path"
	}
	port, err := strconv.ParseUint(f.inputs[fieldPort].Value(), 10, 16)
	if err != nil || port == 0 {
		f.errors[fieldPort] = "enter a port from 1 to 65535"
	}
	method, methodErr := domain.ParseAuthMethod(string(f.authMethod))
	if strings.TrimSpace(f.inputs[fieldHost].Value()) == "" {
		f.errors[fieldHost] = "enter a host"
	}
	if methodErr == nil && err == nil && port != 0 {
		identity := ""
		if method == domain.AuthMethodKey {
			identity = f.inputs[fieldIdentity].Value()
		}
		if _, detailsErr := domain.NewConnectionDetails(f.inputs[fieldHost].Value(), port, f.inputs[fieldUsername].Value(), method, identity, ""); detailsErr != nil {
			if strings.TrimSpace(f.inputs[fieldHost].Value()) == "" {
				f.errors[fieldHost] = "enter a host"
			} else if method == domain.AuthMethodKey && strings.TrimSpace(identity) == "" {
				f.errors[fieldIdentity] = "enter the private key path"
			} else {
				f.errors[fieldHost] = "check host, user, and authentication fields"
			}
		}
	}
	return len(f.errors) == 0
}

func (f *connectionForm) dirty() bool {
	if f.remember {
		return true
	}
	if f.authMethod != f.baselineAuth {
		return true
	}
	for field := fieldName; field <= fieldIdentity; field++ {
		if field == fieldFolder || field == fieldAuth {
			continue
		}
		if f.inputs[field].Value() != f.baseline[field] {
			return true
		}
	}
	return false
}

func (f *connectionForm) cancelNeedsConfirmation() bool { return f.dirty() }

func (f *connectionForm) createRequest() (app.CreateConnectionRequest, bool) {
	if !f.validate() {
		return app.CreateConnectionRequest{}, false
	}
	port, _ := strconv.ParseUint(f.inputs[fieldPort].Value(), 10, 16)
	method := f.authMethod
	identity := ""
	if method == app.AuthMethodKey {
		identity = f.inputs[fieldIdentity].Value()
	}
	parent := app.ItemSelector{Path: f.inputs[fieldFolder].Value()}
	if f.destination != nil {
		if f.destination.ID == syntheticRootID {
			parent = app.ItemSelector{Path: f.destination.Path}
		} else {
			parent = app.ItemSelector{ID: f.destination.ID}
		}
	}
	request := app.CreateConnectionRequest{
		Parent: parent, Name: f.inputs[fieldName].Value(),
		Host: f.inputs[fieldHost].Value(), Port: uint16(port), Username: f.inputs[fieldUsername].Value(),
		AuthMethod: method, IdentityFile: identity,
	}
	if f.destination != nil {
		expected := f.destination.Revision
		request.ExpectedParent = &expected
		request.ExpectedParentPath = f.destination.Path
	}
	return request, true
}

func (f *connectionForm) updateRequest() (app.UpdateConnectionRequest, bool) {
	if f.original == nil || !f.validate() {
		return app.UpdateConnectionRequest{}, false
	}
	original := f.original
	expected := original.Revision
	request := app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: original.ID}, Expected: &expected}
	setStringChange := func(field connectionField, old string, destination **string) {
		value := f.inputs[field].Value()
		if value != old {
			copy := value
			*destination = &copy
		}
	}
	setStringChange(fieldName, original.Name, &request.Name)
	setStringChange(fieldHost, original.Host, &request.Host)
	setStringChange(fieldUsername, original.Username, &request.Username)
	port, _ := strconv.ParseUint(f.inputs[fieldPort].Value(), 10, 16)
	if uint16(port) != original.Port {
		value := uint16(port)
		request.Port = &value
	}
	method := f.authMethod
	if method != original.AuthMethod {
		request.AuthMethod = &method
	}
	identity := ""
	if method == app.AuthMethodKey {
		identity = f.inputs[fieldIdentity].Value()
	}
	if identity != original.IdentityFile {
		request.IdentityFile = &identity
	}
	changed := request.Name != nil || request.Host != nil || request.Port != nil || request.Username != nil || request.AuthMethod != nil || request.IdentityFile != nil || f.passwordVisible() && f.remember
	return request, changed
}

func (f *connectionForm) view(style styles, dimensions ...int) string {
	if len(dimensions) >= 2 {
		f.setDimensions(dimensions[0], dimensions[1])
	}
	return strings.Join(f.project(style).lines, "\n")
}

func (f *connectionForm) project(style styles) viewportProjection {
	lines, activeLine := f.content(style)
	if f.errors[f.focus] != "" || f.focus <= fieldIdentity && f.focus != fieldAuth && f.inputs[f.focus].Error() != "" {
		return projectActiveBlock(lines, activeLine-1, activeLine, f.height, f.width)
	}
	if f.focus == fieldSave && f.formError != "" {
		return projectActiveBlock(lines, activeLine-1, activeLine, f.height, f.width)
	}
	return f.viewport.project(lines, f.height, f.width, activeLine)
}

func projectActiveBlock(lines []string, activeStart, activeEnd, rows, width int) viewportProjection {
	projection := viewportProjection{
		contentLength: len(lines), availableRows: max(0, rows), activeLine: activeEnd,
	}
	if rows <= 0 || width <= 0 || activeStart < 0 || activeEnd < activeStart || activeEnd >= len(lines) {
		return projection
	}
	block := lines[activeStart : activeEnd+1]
	if len(block) >= rows {
		projection.renderOffset = activeEnd - rows + 1
		projection.visibleRows = rows
		projection.hasPrevious = projection.renderOffset > 0
		projection.hasNext = activeEnd+1 < len(lines)
		projection.lines = viewportTruncateLines(block[len(block)-rows:], width)
		if width >= 1 {
			projection.scrollbar = newScrollbarGeometry(len(lines), rows, projection.renderOffset)
		}
		return projection
	}
	start := min(max(0, activeStart), viewportMaximumOffset(len(lines), rows))
	if activeEnd >= start+rows {
		start = activeEnd - rows + 1
	}
	end := min(len(lines), start+rows)
	projection.renderOffset = start
	projection.visibleRows = end - start
	projection.hasPrevious = start > 0
	projection.hasNext = end < len(lines)
	projection.lines = viewportTruncateLines(lines[start:end], width)
	if width >= 1 {
		projection.scrollbar = newScrollbarGeometry(len(lines), rows, start)
	}
	return projection
}

func (f *connectionForm) content(style styles) ([]string, int) {
	title := "New connection"
	if f.original != nil {
		title = "Edit connection  " + f.original.Path
	} else if f.destination != nil {
		title += "  " + f.destination.Path
	}
	lines := []string{style.title.Render(safeText(title, f.width))}
	activeLine := 0
	labelWidth := formLabelWidth(f.width)
	for _, field := range f.visibleFields() {
		invalid := f.errors[field] != "" || field != fieldAuth && f.inputs[field].Error() != ""
		semantics := itemSemantics{focused: field == f.focus, invalid: invalid, primary: field == fieldSave}
		line := ""
		switch field {
		case fieldAuth:
			label := safeText(formLabels[field]+":", labelWidth)
			line = style.item(fmt.Sprintf("%-*s %s", labelWidth, label, f.authMethodView()), semantics)
		case fieldRemember:
			checked := " "
			if f.remember {
				checked = "x"
			}
			line = style.item(fmt.Sprintf("[%s] Remember password in the operating system credential store", checked), semantics)
		case fieldSave:
			if f.formError != "" {
				lines = append(lines, style.failureMessage(safeText(f.formError, max(0, f.width-7))))
			}
			line = style.item("[ Save connection ]", semantics)
		default:
			label := safeText(formLabels[field]+":", labelWidth)
			line = style.item(fmt.Sprintf("%-*s %s", labelWidth, label, f.inputs[field].View()), semantics)
		}
		if field == f.focus {
			activeLine = len(lines)
		}
		lines = append(lines, line)
		message := f.errors[field]
		if message == "" && field != fieldAuth {
			message = f.inputs[field].Error()
		}
		if message != "" {
			lines = append(lines, style.failureMessage(safeText(message, max(0, f.width-7))))
			if field == f.focus {
				activeLine = len(lines) - 1
			}
		}
	}
	lines = append(lines, "Left/Right Change method  Tab Next  Shift+Tab/F2 Previous  Ctrl+S Save  Esc Cancel  F1 Help")
	return lines, activeLine
}
