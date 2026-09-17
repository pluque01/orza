package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
)

type folderForm struct {
	input       textField
	parent      app.ItemSelector
	original    *app.Folder
	baseline    string
	err         string
	destination *app.Folder
}

type folderCreatePayload struct{ form *folderForm }
type folderEditPayload struct{ form *folderForm }

func (payload folderCreatePayload) validModalPayload() bool {
	return payload.form != nil && payload.form.original == nil && payload.form.destination != nil
}

func (payload folderEditPayload) validModalPayload() bool {
	return payload.form != nil && payload.form.original != nil
}

func (f *folderForm) setDestination(folder app.Folder) {
	copy := folder
	f.destination = &copy
	if folder.ID == syntheticRootID {
		f.parent = app.ItemSelector{Path: folder.Path}
	} else {
		f.parent = app.ItemSelector{ID: folder.ID}
	}
}

func newFolderForm(folder *app.Folder, parent app.ItemSelector) *folderForm {
	input := newTextField()
	input.Focus()
	form := &folderForm{input: input, parent: parent}
	if folder != nil {
		copy := *folder
		form.original = &copy
		form.input.SetValue(folder.Name)
	}
	form.baseline = form.input.Value()
	return form
}

func (f *folderForm) update(msg tea.Msg) tea.Cmd {
	command := f.input.Update(msg)
	f.err = ""
	return command
}

func (f *folderForm) validate() bool {
	if _, err := domain.NewName(f.input.Value()); err != nil {
		f.err = "enter a non-empty name without / or control characters"
		return false
	}
	f.err = ""
	return true
}

func (f *folderForm) dirty() bool { return f.input.Value() != f.baseline }

func (f *folderForm) createRequest() (app.CreateFolderRequest, bool) {
	if f.original != nil || !f.validate() {
		return app.CreateFolderRequest{}, false
	}
	request := app.CreateFolderRequest{Parent: f.parent, Name: f.input.Value()}
	if f.destination != nil {
		expected := f.destination.Revision
		request.ExpectedParent = &expected
		request.ExpectedParentPath = f.destination.Path
	}
	return request, true
}

func (f *folderForm) renameRequest() (app.RenameFolderRequest, bool) {
	if f.original == nil || !f.validate() || !f.dirty() {
		if f.original != nil && !f.dirty() {
			f.err = "change the name before saving"
		}
		return app.RenameFolderRequest{}, false
	}
	expected := f.original.Revision
	return app.RenameFolderRequest{Folder: app.ItemSelector{ID: f.original.ID}, Name: f.input.Value(), Expected: &expected}, true
}

func (f *folderForm) modalLines(width int, semanticStyles ...styles) ([]string, int) {
	style := newStyles(true)
	if len(semanticStyles) != 0 {
		style = semanticStyles[0]
	}
	location := f.parent.Path
	if f.destination != nil {
		location = f.destination.Path
	}
	if f.original != nil {
		location = f.original.Path
	}
	fields := []displayField{{label: "Target", value: safeText(location, int(^uint(0)>>1))}}
	if f.original != nil {
		fields = append(fields, displayField{label: "ID/revision", value: safeText(fmt.Sprintf("%s/%d", f.original.ID, f.original.Revision), int(^uint(0)>>1))})
	} else if f.destination != nil {
		fields = append(fields, displayField{label: "Destination ID", value: safeText(string(f.destination.ID), int(^uint(0)>>1))})
	}
	fields = append(fields, displayField{label: "Name", value: safeText(f.input.View(), int(^uint(0)>>1))})
	lines := newStructuredFieldGroup(fields, 2, width, true).render(style)
	active := len(lines) - 1
	lines[active] = "> " + strings.TrimPrefix(lines[active], "  ")
	message := f.err
	if message == "" {
		message = f.input.Error()
	}
	if message != "" {
		lines = append(lines, style.failureMessage(safeText(message, max(0, width-7))))
	}
	lines = append(lines, "Ctrl+S/Enter Save  Esc Cancel  F1 Help")
	return lines, active
}
