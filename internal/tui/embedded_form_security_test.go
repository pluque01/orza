package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestEmbeddedConnectionFormPrintableCommandsRemainLocalText(t *testing.T) {
	for _, payload := range []string{"p", "q", "?"} {
		t.Run(payload, func(t *testing.T) {
			form := validConnectionForm()
			form.setFocus(fieldName)
			before := form.inputs[fieldName].Value()
			form.update(tea.KeyPressMsg(tea.Key{Code: []rune(payload)[0], Text: payload}))
			if got := form.inputs[fieldName].Value(); got != before+payload {
				t.Fatalf("printable command-like input = %q, want %q", got, before+payload)
			}
			if form.focusedField() != fieldName {
				t.Fatal("printable input moved focus")
			}
		})
	}
}

func TestEmbeddedConnectionFormPasteControlAndResizeSafety(t *testing.T) {
	form := validConnectionForm()
	form.setFocus(fieldHost)
	field := &form.inputs[fieldHost]
	field.SetValue("before界after")
	field.SetCursor(6)
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	beforeValue, beforeCursor := field.Value(), field.Position()
	beforeAnchor, beforeSelecting := field.anchor, field.selecting

	for _, payload := range []string{"prefix\x00suffix", "prefix\x1bsuffix", "prefix\x03suffix", string([]byte{0xff})} {
		form.update(tea.PasteStartMsg{})
		form.update(tea.PasteMsg{Content: payload})
		form.update(tea.PasteEndMsg{})
		_ = form.validate()
		form.setDimensions(24, 5)
		form.setDimensions(72, 18)
		if field.Value() != beforeValue || field.Position() != beforeCursor || field.anchor != beforeAnchor || field.selecting != beforeSelecting {
			t.Fatalf("rejected paste/resize changed field state for %q", payload)
		}
		if field.Error() != textFieldControlError {
			t.Fatalf("control paste error = %q", field.Error())
		}
		if form.focusedField() != fieldHost {
			t.Fatal("paste, validation, or resize moved embedded-form focus")
		}
		if view := form.view(newStyles(true)); strings.Contains(view, payload) || strings.ContainsAny(view, "\x00\x1b\x03") {
			t.Fatalf("unsafe paste reached form projection: %q", view)
		}
	}

	form.update(tea.PasteMsg{Content: "-safe-用户"})
	_ = form.validate()
	if got := field.Value(); got != "before-safe-用户after" {
		t.Fatalf("safe bracketed paste replacement = %q", got)
	}
}

func TestEmbeddedConnectionFormResizePreservesUnicodeCursorAndSelection(t *testing.T) {
	form := validConnectionForm()
	form.setFocus(fieldHost)
	field := &form.inputs[fieldHost]
	field.SetValue("prefix界🙂suffix")
	field.SetCursor(6)
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	field.Update(modifiedKey(tea.KeyRight, tea.ModShift))
	beforeValue, beforeCursor := field.Value(), field.Position()
	beforeAnchor, beforeSelecting := field.anchor, field.selecting

	for _, size := range [][2]int{{20, 4}, {80, 24}, {12, 2}, {48, 12}} {
		_ = form.view(newStyles(true), size[0], size[1])
		if field.Value() != beforeValue || field.Position() != beforeCursor || field.anchor != beforeAnchor || field.selecting != beforeSelecting {
			t.Fatalf("resize to %dx%d changed value/cursor/selection", size[0], size[1])
		}
	}
}

func TestEmbeddedConnectionFormSecretCanaryNeverProjectsThroughValidationOrResize(t *testing.T) {
	const secretCanary = "secret-password-canary-never-render"
	connection := app.Connection{
		Node:          app.Node{ID: "connection-id", Name: "node", Path: "/node", Revision: 1},
		Host:          "host.test",
		Port:          22,
		AuthMethod:    app.AuthMethodPassword,
		CredentialRef: secretCanary,
	}
	form := newConnectionForm(&connection)
	form.inputs[fieldHost].SetValue("")
	form.validate()
	form.setFormError("credential save failed")

	for _, size := range [][2]int{{80, 24}, {32, 5}, {12, 2}, {80, 24}} {
		form.setDimensions(size[0], size[1])
		view := form.view(newStyles(true))
		if strings.Contains(view, secretCanary) {
			t.Fatalf("secret canary projected at %dx%d: %q", size[0], size[1], view)
		}
	}
}
