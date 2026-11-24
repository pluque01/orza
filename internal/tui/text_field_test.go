package tui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestTextFieldShowsCursorAtInsertionPoint(t *testing.T) {
	field := newTextField()
	field.Focus()
	if got := field.View(); got != (textFieldView{text: " ", cursor: 0}) {
		t.Fatalf("empty focused field = %+v, want cursor on a blank cell", got)
	}
	const value = "a界🙂e\u0301z"
	field.SetValue(value)
	field.CursorEnd()
	for _, step := range []struct {
		key    rune
		text   string
		column int
	}{
		{tea.KeyHome, value, 0},
		{tea.KeyRight, value, 1},
		{tea.KeyRight, value, 3},
		{tea.KeyRight, value, 5},
		{tea.KeyRight, value, 5},
		{tea.KeyRight, value, 6},
		{tea.KeyEnd, value + " ", 7},
	} {
		field.Update(modifiedKey(step.key, 0))
		if got := field.View(); got != (textFieldView{text: step.text, cursor: step.column}) {
			t.Fatalf("field after key %U = %+v, want %q and cursor at column %d", step.key, got, step.text, step.column)
		}
	}
	field.Blur()
	if got := field.View(); got != (textFieldView{text: value, cursor: noTextCursor}) || field.Value() != value {
		t.Fatalf("blurred field = %+v, value = %q", got, field.Value())
	}
}

func TestTextFieldKeepsCursorInNarrowViewport(t *testing.T) {
	field := newTextField()
	field.Focus()
	value := "prefix界🙂e\u0301suffix"
	field.SetValue(value)
	for _, width := range []int{1, 2, 3, 8} {
		field.SetWidth(width)
		for position := 0; position <= utf8.RuneCountInString(value); position++ {
			field.SetCursor(position)
			view := field.View()
			if cells := ansi.StringWidth(view.text); cells > width || view.cursor < 0 || view.cursor >= cells {
				t.Fatalf("viewport at width %d and position %d = %+v", width, position, view)
			}
		}
	}
	field.SetWidth(8)
	field.SetCursor(0)
	before := field.View()
	field.Update(modifiedKey(tea.KeyRight, 0))
	if after := field.View(); after.text != before.text || after.cursor != before.cursor+1 {
		t.Fatalf("moving within the visible part of a clipped field changed its text: %+v to %+v", before, after)
	}
}

func TestFolderNameCursorStaysVisibleInModal(t *testing.T) {
	m := New(Config{Width: 40, Height: 12, NoColor: true})
	m.Update(keyPress("f"))
	m.Update(tea.PasteMsg{Content: strings.Repeat("a", 40)})
	view := m.View()
	if view.Cursor == nil {
		t.Fatalf("folder Name cursor is missing:\n%s", view.Content)
	}
	line := strings.Split(view.Content, "\n")[view.Cursor.Y]
	before := ansi.Cut(line, 0, view.Cursor.X)
	if !strings.Contains(before, "Name") || !strings.HasSuffix(before, "a") || ansi.Cut(line, view.Cursor.X, view.Cursor.X+1) != " " {
		t.Fatalf("folder Name cursor at %+v is away from the end of its value: %q", view.Cursor, line)
	}
	m.Update(keyPress("esc"))
	if closed := m.View(); closed.Cursor != nil {
		t.Fatalf("closing the folder form retained its cursor: %+v", closed.Cursor)
	}
}

func TestTextCursorInFormsAndTreeFilter(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 12}} {
		for _, noColor := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d/noColor=%t", size[0], size[1], noColor), func(t *testing.T) {
				config := Config{Width: size[0], Height: size[1], NoColor: noColor}
				connection := testConnection("host", syntheticRootID, "/host", 1)
				connection.Host = "host.invalid"
				form := newConnectionForm(&connection)
				form.setFocus(fieldHost)
				browser := New(config)
				browser.openConnectionForm(form, browser.captureConnectionTarget(connection))
				browser.Update(modifiedKey(tea.KeyHome, 0))
				browser.Update(modifiedKey(tea.KeyRight, 0))
				assertTextCursor(t, browser.View(), "host.invalid", 1)

				browser = New(config)
				browser.openTunnelForm(connection, app.TunnelConfig{
					Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Port: 8080},
				}, 0)
				browser.Update(modifiedKey(tea.KeyTab, 0))
				browser.Update(modifiedKey(tea.KeyTab, 0))
				browser.Update(modifiedKey(tea.KeyLeft, 0))
				assertTextCursor(t, browser.View(), "8080", 3)

				browser = New(config)
				browser.Update(keyPress("/"))
				browser.Update(tea.PasteMsg{Content: "abc"})
				browser.Update(modifiedKey(tea.KeyLeft, 0))
				assertTextCursor(t, browser.View(), "Filter: abc", len("Filter: ab"))
				browser.Update(keyPress("esc"))
				if view := browser.View(); view.Cursor != nil {
					t.Fatalf("closing the filter retained its cursor: %+v", view.Cursor)
				}
			})
		}
	}
}

func assertTextCursor(t *testing.T, view tea.View, text string, offset int) {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(view.Content), "\n") {
		before, _, found := strings.Cut(line, text)
		if found && view.Cursor != nil && view.Cursor.Shape == tea.CursorBlock &&
			view.Cursor.X == ansi.StringWidth(before)+offset && view.Cursor.Y == row {
			return
		}
	}
	t.Fatalf("block cursor at %+v, want %q + %d:\n%s", view.Cursor, text, offset, view.Content)
}

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
	if view := field.View(); view != (textFieldView{text: "a[界🙂]z", cursor: 7}) {
		t.Fatalf("selection moving right = %+v, want cursor after selection", view)
	}
	field.SetCursor(3)
	field.Update(modifiedKey(tea.KeyLeft, tea.ModShift))
	field.Update(modifiedKey(tea.KeyLeft, tea.ModShift))
	if view := field.View(); view != (textFieldView{text: "a[界🙂]z", cursor: 2}) {
		t.Fatalf("selection moving left = %+v, want cursor on the first selected character", view)
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
	if field.HasSelection() || strings.Contains(field.View().text, "[") {
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
	if view := field.View().text; ansi.StringWidth(view) > field.Width() || !strings.Contains(view, "suffix") {
		t.Fatalf("Unicode viewport = %q, width %d", view, ansi.StringWidth(view))
	}

	field.Update(modifiedKey(tea.KeyLeft, tea.ModShift))
	if view := field.View().text; ansi.StringWidth(view) > field.Width() || !strings.Contains(view, "[") || !strings.Contains(view, "]") {
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
