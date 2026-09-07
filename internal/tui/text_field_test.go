package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTextFieldSelectionEditingAndNonColorMarker(t *testing.T) {
	field := newTextField()
	field.Focus()
	field.SetValue("a界🙂z")
	field.SetCursor(1)
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	if start, end := field.selection(); start != 1 || end != 3 {
		t.Fatalf("selection = %d:%d, want 1:3", start, end)
	}
	if view := field.View(); !strings.Contains(view, "a[界🙂]z") {
		t.Fatalf("selection lacks a non-color marker: %q", view)
	}

	field.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyExtended, Text: "X"}))
	if field.Value() != "aXz" || field.Position() != 2 || field.HasSelection() {
		t.Fatalf("typed replacement = %q at %d, selected=%v", field.Value(), field.Position(), field.HasSelection())
	}

	field.SetValue("abcde")
	field.SetCursor(2)
	field.Update(modifiedKey(tea.KeyEnd, tea.ModShift))
	field.Update(modifiedKey(tea.KeyLeft, 0))
	if field.Position() != 2 || field.HasSelection() {
		t.Fatal("unshifted Left did not collapse the selection to its start")
	}
	field.Update(modifiedKey(tea.KeyHome, tea.ModShift))
	field.Update(modifiedKey(tea.KeyRight, 0))
	if field.Position() != 2 || field.HasSelection() {
		t.Fatal("unshifted Right did not collapse the selection to its end")
	}

	field.Update(modifiedKey(tea.KeyHome, tea.ModShift))
	field.Update(modifiedKey(tea.KeyBackspace, 0))
	if field.Value() != "cde" || field.Position() != 0 || field.HasSelection() {
		t.Fatalf("selection backspace = %q at %d", field.Value(), field.Position())
	}

	field.SetCursor(0)
	field.Update(modifiedKey(tea.KeyEnd, tea.ModShift))
	field.Blur()
	if field.HasSelection() || strings.Contains(field.View(), "[") {
		t.Fatal("blur retained the selection or its marker")
	}
}

func TestTextFieldPasteNormalizationAndAtomicControlRejection(t *testing.T) {
	field := newTextField()
	field.Focus()
	field.Update(tea.PasteMsg{Content: "a\r\nb\u0085c\u2028d\u2029e"})
	if field.Value() != "abcde" {
		t.Fatalf("normalized paste = %q, want abcde", field.Value())
	}

	field.SetCursor(1)
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	beforeValue, beforeCursor := field.Value(), field.Position()
	beforeAnchor, beforeSelecting := field.anchor, field.selecting
	for _, payload := range []string{"x\ty", "x\x00y", "x\x1by", "x\x03y", string([]byte{0xff})} {
		field.Update(tea.PasteMsg{Content: payload})
		if field.Value() != beforeValue || field.Position() != beforeCursor || field.anchor != beforeAnchor || field.selecting != beforeSelecting {
			t.Fatal("rejected control changed value, cursor, or selection")
		}
		if field.Error() != textFieldControlError || strings.Contains(field.Error(), payload) {
			t.Fatalf("unsafe control rejection error %q", field.Error())
		}
	}

	field.Update(tea.PasteMsg{})
	field.Update(tea.PasteMsg{Content: "\r\n\u0085\u2028\u2029"})
	field.Update(tea.PasteStartMsg{})
	field.Update(tea.PasteEndMsg{})
	if field.Value() != beforeValue || field.Position() != beforeCursor || field.anchor != beforeAnchor || field.selecting != beforeSelecting {
		t.Fatal("empty paste or boundary message changed field state")
	}
}

func TestTextFieldRuneLimitIsAtomic(t *testing.T) {
	field := newTextField()
	field.Focus()
	field.Update(tea.PasteMsg{Content: strings.Repeat("界", maxTextFieldRunes)})
	if got := len([]rune(field.Value())); got != maxTextFieldRunes {
		t.Fatalf("accepted rune count = %d, want %d", got, maxTextFieldRunes)
	}

	field.SetCursor(10)
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	beforeValue, beforeCursor := field.Value(), field.Position()
	beforeAnchor := field.anchor
	field.Update(tea.PasteMsg{Content: "ab"})
	if field.Value() != beforeValue || field.Position() != beforeCursor || field.anchor != beforeAnchor || !field.selecting {
		t.Fatal("4097-rune candidate was not rejected atomically")
	}
	if field.Error() != textFieldLimitError {
		t.Fatalf("limit error = %q", field.Error())
	}

	field.SetValue("")
	field.Update(tea.PasteMsg{Content: strings.Repeat("a", maxTextFieldRunes+1)})
	if field.Value() != "" || field.Position() != 0 || field.Error() != textFieldLimitError {
		t.Fatal("oversized paste was truncated or partially committed")
	}
}

func TestTextFieldUnicodeViewportAndDisabledOSClipboard(t *testing.T) {
	field := newTextField()
	field.SetWidth(8)
	field.Focus()
	field.SetValue("prefix界🙂suffix")
	field.CursorEnd()
	if view := field.View(); ansi.StringWidth(view) > field.Width() || !strings.Contains(view, "suffix") {
		t.Fatalf("Unicode viewport = %q, width %d", view, ansi.StringWidth(view))
	}

	field.Update(modifiedKey(tea.KeyLeft, tea.ModShift))
	if view := field.View(); ansi.StringWidth(view) > field.Width() || !strings.Contains(view, "[") || !strings.Contains(view, "]") {
		t.Fatalf("selected Unicode viewport = %q, width %d", view, ansi.StringWidth(view))
	}

	if field.input.KeyMap.Paste.Enabled() {
		t.Fatal("Bubbles OS clipboard binding is enabled")
	}
	before := field.Value()
	if command := field.Update(tea.KeyPressMsg(tea.Key{Code: 'v', Mod: tea.ModCtrl})); command != nil {
		t.Fatal("Ctrl+V returned an OS clipboard command")
	}
	if field.Value() != before {
		t.Fatal("Ctrl+V changed the field")
	}
}

func modifiedKey(code rune, modifier tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: modifier})
}
