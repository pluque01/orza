package terminal

import (
	"strings"
	"unicode"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
)

const maxSecretBytes = 4096

const (
	secretControlError = "Paste rejected: unsupported control character"
	secretLimitError   = "Secret rejected: maximum length exceeded"
)

type secretEditor struct {
	buffer    []rune
	cursor    int
	anchor    int
	selecting bool
	status    string
}

type secretEditorAction uint8

const (
	secretContinue secretEditorAction = iota
	secretSubmit
	secretCancel
	secretQuit
)

func (e *secretEditor) handle(event uv.Event) secretEditorAction {
	switch event := event.(type) {
	case uv.PasteEvent:
		e.paste(event.Content)
	case uv.KeyPressEvent:
		return e.key(event.Key())
	}
	return secretContinue
}

func (e *secretEditor) key(key uv.Key) secretEditorAction {
	if key.Mod.Contains(uv.ModCtrl) && (key.Code == 'c' || key.BaseCode == 'c') {
		return secretQuit
	}

	shift := key.Mod.Contains(uv.ModShift)
	switch key.Code {
	case uv.KeyEnter, uv.KeyKpEnter:
		return secretSubmit
	case uv.KeyEscape:
		return secretCancel
	case uv.KeyF1:
		e.status = "Help: Enter submit, Esc cancel, Ctrl+C quit"
	case uv.KeyLeft:
		e.moveLeft(shift)
	case uv.KeyRight:
		e.moveRight(shift)
	case uv.KeyHome, uv.KeyKpHome:
		e.moveTo(0, shift)
	case uv.KeyEnd, uv.KeyKpEnd:
		e.moveTo(len(e.buffer), shift)
	case uv.KeyBackspace:
		e.backspace()
	case uv.KeyDelete, uv.KeyKpDelete:
		e.delete()
	default:
		if key.Text != "" && key.Mod&(uv.ModCtrl|uv.ModAlt|uv.ModMeta|uv.ModHyper|uv.ModSuper) == 0 {
			e.insert(key.Text, false)
		}
	}
	return secretContinue
}

func (e *secretEditor) paste(value string) {
	var normalized strings.Builder
	for _, r := range value {
		switch r {
		case '\r', '\n', '\u0085', '\u2028', '\u2029':
			continue
		}
		if unicode.IsControl(r) {
			e.status = secretControlError
			return
		}
		normalized.WriteRune(r)
	}
	if normalized.Len() == 0 {
		return
	}
	e.insert(normalized.String(), true)
}

func (e *secretEditor) insert(value string, paste bool) {
	if !utf8.ValidString(value) {
		if paste {
			e.status = secretControlError
		}
		return
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			if paste {
				e.status = secretControlError
			}
			return
		}
	}

	start, end := e.selection()
	inserted := []rune(value)
	candidate := make([]rune, 0, len(e.buffer)-(end-start)+len(inserted))
	candidate = append(candidate, e.buffer[:start]...)
	candidate = append(candidate, inserted...)
	candidate = append(candidate, e.buffer[end:]...)
	if secretByteLen(candidate) > maxSecretBytes {
		wipeRunes(candidate)
		wipeRunes(inserted)
		e.status = secretLimitError
		return
	}

	wipeRunes(e.buffer)
	wipeRunes(inserted)
	e.buffer = candidate
	e.cursor = start + utf8.RuneCountInString(value)
	e.clearSelection()
	e.status = ""
}

func (e *secretEditor) moveLeft(shift bool) {
	if !shift && e.selecting {
		start, _ := e.selection()
		e.cursor = start
		e.clearSelection()
		return
	}
	e.beginSelection(shift)
	if e.cursor > 0 {
		e.cursor--
	}
	e.finishSelection(shift)
}

func (e *secretEditor) moveRight(shift bool) {
	if !shift && e.selecting {
		_, end := e.selection()
		e.cursor = end
		e.clearSelection()
		return
	}
	e.beginSelection(shift)
	if e.cursor < len(e.buffer) {
		e.cursor++
	}
	e.finishSelection(shift)
}

func (e *secretEditor) moveTo(position int, shift bool) {
	e.beginSelection(shift)
	e.cursor = position
	e.finishSelection(shift)
}

func (e *secretEditor) beginSelection(shift bool) {
	if shift && !e.selecting {
		e.anchor = e.cursor
		e.selecting = true
	}
	if !shift {
		e.clearSelection()
	}
}

func (e *secretEditor) finishSelection(shift bool) {
	if shift && e.anchor == e.cursor {
		e.clearSelection()
	}
	if !shift {
		e.clearSelection()
	}
}

func (e *secretEditor) backspace() {
	start, end := e.selection()
	if start == end {
		if start == 0 {
			return
		}
		start--
	}
	e.remove(start, end)
}

func (e *secretEditor) delete() {
	start, end := e.selection()
	if start == end {
		if end == len(e.buffer) {
			return
		}
		end++
	}
	e.remove(start, end)
}

func (e *secretEditor) remove(start, end int) {
	candidate := make([]rune, 0, len(e.buffer)-(end-start))
	candidate = append(candidate, e.buffer[:start]...)
	candidate = append(candidate, e.buffer[end:]...)
	wipeRunes(e.buffer)
	e.buffer = candidate
	e.cursor = start
	e.clearSelection()
	e.status = ""
}

func (e *secretEditor) selection() (int, int) {
	if !e.selecting || e.anchor == e.cursor {
		return e.cursor, e.cursor
	}
	if e.anchor < e.cursor {
		return e.anchor, e.cursor
	}
	return e.cursor, e.anchor
}

func (e *secretEditor) clearSelection() {
	e.anchor = 0
	e.selecting = false
}

func (e *secretEditor) render() (string, int) {
	start, end := e.selection()
	selected := start != end
	var masked strings.Builder
	cursor := 0
	for i := 0; i <= len(e.buffer); i++ {
		if selected && i == start {
			masked.WriteByte('[')
		}
		if selected && i == end {
			masked.WriteByte(']')
		}
		if i == e.cursor {
			cursor = masked.Len()
		}
		if i < len(e.buffer) {
			masked.WriteByte('*')
		}
	}
	return masked.String(), cursor
}

func (e *secretEditor) bytes() []byte {
	result := make([]byte, 0, secretByteLen(e.buffer))
	var encoded [utf8.UTFMax]byte
	for _, r := range e.buffer {
		n := utf8.EncodeRune(encoded[:], r)
		result = append(result, encoded[:n]...)
	}
	return result
}

func (e *secretEditor) wipe() {
	wipeRunes(e.buffer)
	e.buffer = nil
	e.cursor = 0
	e.clearSelection()
	e.status = ""
}

func secretByteLen(value []rune) int {
	length := 0
	for _, r := range value {
		length += utf8.RuneLen(r)
	}
	return length
}

func wipeRunes(value []rune) {
	for i := range value {
		value[i] = 0
	}
}
