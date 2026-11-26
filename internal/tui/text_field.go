package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const maxTextFieldRunes = 4096
const noTextCursor = -1

const (
	textFieldControlError = "Paste rejected: unsupported control character"
	textFieldLimitError   = "Input rejected: maximum length exceeded"
)

// textField adds atomic terminal paste and selection to Bubbles' single-line
// editor while retaining its ordinary cursor movement and viewport behavior.
type textField struct {
	input     textinput.Model
	anchor    int
	selecting bool
	err       string
	leftCell  int
}

func newTextField() textField {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = maxTextFieldRunes
	input.SetWidth(48)
	input.SetStyles(textinput.Styles{})
	input.SetVirtualCursor(false)
	input.KeyMap.Paste.SetEnabled(false)
	return textField{input: input}
}

func (f *textField) Value() string { return f.input.Value() }

func (f *textField) SetValue(value string) {
	f.input.SetValue(value)
	f.clearSelection()
	f.err = ""
	f.leftCell = 0
}

func (f *textField) Position() int { return f.input.Position() }

func (f *textField) SetCursor(position int) {
	f.input.SetCursor(position)
	f.clearSelection()
}

func (f *textField) CursorStart() { f.SetCursor(0) }

func (f *textField) CursorEnd() { f.SetCursor(utf8.RuneCountInString(f.Value())) }

func (f *textField) Focus() tea.Cmd { return f.input.Focus() }

func (f *textField) Blur() {
	f.input.Blur()
	f.clearSelection()
}

func (f *textField) Focused() bool { return f.input.Focused() }

func (f *textField) SetWidth(width int) {
	width = max(0, width)
	position := f.Position()
	f.input.SetWidth(width)
	// Recalculate Bubbles' horizontal offset without changing this wrapper's
	// value, cursor, or selection state.
	f.input.SetCursor(position)
}

func (f *textField) Width() int { return f.input.Width() }

func (f *textField) Error() string { return f.err }

func (f *textField) HasSelection() bool {
	start, end := f.selection()
	return start != end
}

func (f *textField) Update(msg tea.Msg) tea.Cmd {
	if !f.Focused() {
		return nil
	}

	switch msg := msg.(type) {
	case tea.PasteMsg:
		f.paste(msg.Content)
		return nil
	case tea.PasteStartMsg, tea.PasteEndMsg:
		return nil
	case tea.KeyPressMsg:
		if f.handleSelectionKey(msg) {
			return nil
		}
		if msg.Text != "" {
			f.insert(msg.Text, false)
			return nil
		}
	}

	updated, command := f.input.Update(msg)
	f.input = updated
	return command
}

func (f *textField) handleSelectionKey(msg tea.KeyPressMsg) bool {
	shift := msg.Key().Mod.Contains(tea.ModShift)
	switch msg.Key().Code {
	case tea.KeyLeft, tea.KeyKpLeft:
		f.moveLeft(shift)
		return true
	case tea.KeyRight, tea.KeyKpRight:
		f.moveRight(shift)
		return true
	case tea.KeyHome, tea.KeyKpHome:
		f.moveTo(0, shift)
		return true
	case tea.KeyEnd, tea.KeyKpEnd:
		f.moveTo(utf8.RuneCountInString(f.Value()), shift)
		return true
	}

	if f.HasSelection() {
		switch {
		case key.Matches(msg, f.input.KeyMap.DeleteCharacterBackward), key.Matches(msg, f.input.KeyMap.DeleteCharacterForward):
			start, end := f.selection()
			f.replace(start, end, nil)
			return true
		}
	}
	return false
}

func (f *textField) paste(value string) {
	if !utf8.ValidString(value) {
		f.err = textFieldControlError
		return
	}

	var normalized strings.Builder
	for _, r := range value {
		switch r {
		case '\r', '\n', '\u0085', '\u2028', '\u2029':
			continue
		}
		if unicode.IsControl(r) {
			f.err = textFieldControlError
			return
		}
		normalized.WriteRune(r)
	}
	if normalized.Len() == 0 {
		return
	}
	f.insert(normalized.String(), true)
}

func (f *textField) insert(value string, paste bool) {
	if !utf8.ValidString(value) {
		if paste {
			f.err = textFieldControlError
		}
		return
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			if paste {
				f.err = textFieldControlError
			}
			return
		}
	}

	start, end := f.selection()
	current := []rune(f.Value())
	inserted := []rune(value)
	if len(current)-(end-start)+len(inserted) > maxTextFieldRunes {
		f.err = textFieldLimitError
		return
	}
	f.replace(start, end, inserted)
}

func (f *textField) replace(start, end int, inserted []rune) {
	current := []rune(f.Value())
	candidate := make([]rune, 0, len(current)-(end-start)+len(inserted))
	candidate = append(candidate, current[:start]...)
	candidate = append(candidate, inserted...)
	candidate = append(candidate, current[end:]...)
	f.input.SetValue(string(candidate))
	f.input.SetCursor(start + len(inserted))
	f.clearSelection()
	f.err = ""
}

func (f *textField) moveLeft(shift bool) {
	if !shift && f.HasSelection() {
		start, _ := f.selection()
		f.input.SetCursor(start)
		f.clearSelection()
		return
	}
	f.beginSelection(shift)
	if f.Position() > 0 {
		f.input.SetCursor(f.Position() - 1)
	}
	f.finishSelection(shift)
}

func (f *textField) moveRight(shift bool) {
	if !shift && f.HasSelection() {
		_, end := f.selection()
		f.input.SetCursor(end)
		f.clearSelection()
		return
	}
	f.beginSelection(shift)
	if f.Position() < utf8.RuneCountInString(f.Value()) {
		f.input.SetCursor(f.Position() + 1)
	}
	f.finishSelection(shift)
}

func (f *textField) moveTo(position int, shift bool) {
	f.beginSelection(shift)
	f.input.SetCursor(position)
	f.finishSelection(shift)
}

func (f *textField) beginSelection(shift bool) {
	if shift && !f.selecting {
		f.anchor = f.Position()
		f.selecting = true
	}
	if !shift {
		f.clearSelection()
	}
}

func (f *textField) finishSelection(shift bool) {
	if !shift || f.anchor == f.Position() {
		f.clearSelection()
	}
}

func (f *textField) selection() (int, int) {
	position := f.Position()
	if !f.selecting || f.anchor == position {
		return position, position
	}
	if f.anchor < position {
		return f.anchor, position
	}
	return position, f.anchor
}

func (f *textField) clearSelection() {
	f.anchor = 0
	f.selecting = false
}

type textFieldView struct {
	text   string
	cursor int
}

func (f *textField) View() textFieldView {
	width := f.Width()
	if width <= 0 {
		return textFieldView{cursor: noTextCursor}
	}
	value := f.Value()
	position := f.Position()
	start, end := f.selection()
	if start != end {
		runes := []rune(value)
		value = string(runes[:start]) + "[" + string(runes[start:end]) + "]" + string(runes[end:])
		position++
		if f.Position() == end {
			position++
		}
	}

	limit := len(value)*6 + 1
	projected := safeText(value, limit)
	prefix := safeText(string([]rune(value)[:position]), limit)
	column, cell, cellWidth := textFieldCursorCell(projected, len(prefix))
	focused := f.Focused()
	if focused && cellWidth == 0 {
		// The terminal cursor needs a real cell when the insertion point is
		// at the end of the value, including an empty value.
		projected += " "
		cell, cellWidth = " ", 1
	}
	w := makeTextFieldWindow(projected, width, f.leftCell)
	if column < w.left || column+cellWidth > w.right {
		// Keep the current window while the cursor fits. Scroll only when it
		// leaves the window, putting the new location near the middle.
		var offset int
		switch {
		case column+cellWidth <= width-1:
		case ansi.StringWidth(projected)-column <= width-1:
			offset = ansi.StringWidth(projected) - width + 1
		default:
			offset = max(0, column-max(0, width-2-cellWidth)/2)
		}
		w = makeTextFieldWindow(projected, width, offset)
	}
	f.leftCell = w.left
	if !focused {
		return textFieldView{text: w.text, cursor: noTextCursor}
	}
	if column < w.left || column+cellWidth > w.right {
		// With too little room for both the current grapheme and clipping
		// indicators, give the cursor cell priority.
		if cellWidth > width {
			cell, cellWidth = " ", 1
		}
		if cellWidth < width {
			if column > 0 {
				return textFieldView{text: "…" + cell, cursor: 1}
			}
			return textFieldView{text: cell + "…"}
		}
		return textFieldView{text: cell}
	}
	column -= w.left
	if w.left > 0 {
		column++ // The left clipping indicator occupies one cell.
	}
	return textFieldView{text: w.text, cursor: column}
}

// textFieldCursorCell locates the projected grapheme, including when the rune
// insertion point falls between a base character and its combining marks.
func textFieldCursorCell(value string, byteOffset int) (int, string, int) {
	column := 0
	for value != "" {
		cluster, width := ansi.FirstGraphemeCluster(value, ansi.GraphemeWidth)
		if byteOffset < len(cluster) && width > 0 {
			return column, cluster, width
		}
		value = value[len(cluster):]
		byteOffset -= len(cluster)
		column += width
	}
	return column, "", 0
}

type textFieldWindow struct {
	text        string
	left, right int // Columns in the full value, excluding the clipping indicators.
}

func makeTextFieldWindow(value string, width, offset int) textFieldWindow {
	totalWidth := ansi.StringWidth(value)
	if totalWidth <= width {
		return textFieldWindow{text: value, right: totalWidth}
	}
	offset = min(offset, totalWidth-1)
	left := 0
	for left < offset && value != "" {
		cluster, cells := ansi.FirstGraphemeCluster(value, ansi.GraphemeWidth)
		value = value[len(cluster):]
		left += cells
	}
	prefix := ""
	if left > 0 {
		prefix = "…"
		width--
	}
	if totalWidth-left <= width {
		return textFieldWindow{text: prefix + value, left: left, right: totalWidth}
	}
	if width <= 0 {
		return textFieldWindow{text: prefix, left: left, right: left}
	}
	end, visible := 0, 0
	for end < len(value) {
		cluster, cells := ansi.FirstGraphemeCluster(value[end:], ansi.GraphemeWidth)
		if visible+cells > width-1 {
			break
		}
		end += len(cluster)
		visible += cells
	}
	return textFieldWindow{text: prefix + value[:end] + "…", left: left, right: left + visible}
}
