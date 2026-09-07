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

func (f *textField) View() string {
	start, end := f.selection()
	if start == end || !f.Focused() {
		return f.viewport(f.Value(), f.Position())
	}

	value := []rune(f.Value())
	marked := make([]rune, 0, len(value)+2)
	marked = append(marked, value[:start]...)
	marked = append(marked, '[')
	marked = append(marked, value[start:end]...)
	marked = append(marked, ']')
	marked = append(marked, value[end:]...)

	return f.viewport(string(marked), end+2)
}

func (f *textField) viewport(value string, cursor int) string {
	if f.Width() <= 0 {
		return ""
	}
	width := f.Width()
	runes := []rune(value)
	cursor = min(max(0, cursor), len(runes))
	projected := safeText(value, max(1, len(value)*6+1))
	if ansi.StringWidth(projected) <= width {
		return projected
	}

	prefix := safeText(string(runes[:cursor]), max(1, len(value)*6+1))
	cursorWidth := ansi.StringWidth(prefix)
	totalWidth := ansi.StringWidth(projected)
	if width == 1 {
		return "…"
	}
	if cursorWidth < width-1 {
		return ansi.Cut(projected, 0, width-1) + "…"
	}
	if totalWidth-cursorWidth < width-1 {
		available := width - 1
		projectedRunes := []rune(projected)
		start := len(projectedRunes)
		used := 0
		for start > 0 {
			runeWidth := ansi.StringWidth(string(projectedRunes[start-1]))
			if used+runeWidth > available {
				break
			}
			start--
			used += runeWidth
		}
		return "…" + string(projectedRunes[start:])
	}
	available := max(0, width-2)
	start := min(max(1, cursorWidth-available/2), max(1, totalWidth-available-1))
	return "…" + ansi.Cut(projected, start, start+available) + "…"
}
