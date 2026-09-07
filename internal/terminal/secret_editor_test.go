package terminal

import (
	"strings"
	"testing"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestSecretEditorManualPasteEquivalence(t *testing.T) {
	payloads := []struct {
		name  string
		value string
	}{
		{name: "plain", value: "plain"},
		{name: "address", value: "user@example.com"},
		{name: "port", value: "22"},
		{name: "path", value: "/tmp/id_ed25519"},
		{name: "accent", value: "café"},
		{name: "combining", value: "e\u0301"},
		{name: "cjk", value: "用户"},
		{name: "cyrillic", value: "пароль"},
		{name: "wide-emoji", value: "界🙂"},
		{name: "zwj", value: "👩‍💻"},
		{name: "long", value: strings.Repeat("a", 1001)},
	}
	positions := []struct {
		name  string
		setup func(*secretEditor)
	}{
		{name: "start", setup: func(e *secretEditor) { e.insert("tail", false); e.moveTo(0, false) }},
		{name: "middle", setup: func(e *secretEditor) { e.insert("book", false); e.moveTo(2, false) }},
		{name: "selection", setup: func(e *secretEditor) {
			e.insert("replace", false)
			e.moveTo(2, false)
			e.moveRight(true)
			e.moveRight(true)
		}},
	}

	for _, payload := range payloads {
		for _, position := range positions {
			t.Run(payload.name+"/"+position.name, func(t *testing.T) {
				manual := new(secretEditor)
				pasted := new(secretEditor)
				position.setup(manual)
				position.setup(pasted)
				manual.handle(uv.KeyPressEvent(uv.Key{Text: payload.value, Code: uv.KeyExtended}))
				pasted.handle(uv.PasteEvent{Content: payload.value})
				if string(manual.buffer) != string(pasted.buffer) || manual.cursor != pasted.cursor || manual.selecting != pasted.selecting {
					t.Fatal("manual and paste editor states differ")
				}
			})
		}
	}
}

func TestSecretEditorPasteNormalizationAndAtomicRejection(t *testing.T) {
	editor := new(secretEditor)
	editor.paste("a\r\nb\u0085c\u2028d\u2029e")
	if string(editor.buffer) != "abcde" {
		t.Fatal("line separators were not removed")
	}
	editor.moveTo(1, false)
	editor.moveRight(true)
	beforeValue := string(editor.buffer)
	beforeCursor, beforeAnchor, beforeSelecting := editor.cursor, editor.anchor, editor.selecting
	for _, payload := range []string{"x\ty", "x\x00y", "x\x1by", "x\x03y"} {
		editor.paste(payload)
		if string(editor.buffer) != beforeValue || editor.cursor != beforeCursor || editor.anchor != beforeAnchor || editor.selecting != beforeSelecting {
			t.Fatal("rejected control changed editor state")
		}
		if editor.status != secretControlError || strings.Contains(editor.status, payload) {
			t.Fatal("control rejection status is not generic")
		}
	}
}

func TestSecretEditorByteLimitIsAtomic(t *testing.T) {
	editor := new(secretEditor)
	editor.paste(strings.Repeat("a", maxSecretBytes))
	if len(editor.bytes()) != maxSecretBytes {
		t.Fatal("4096-byte secret was not accepted")
	}
	editor.moveTo(10, false)
	editor.moveRight(true)
	beforeValue := string(editor.buffer)
	beforeCursor, beforeAnchor := editor.cursor, editor.anchor
	editor.paste("é")
	if string(editor.buffer) != beforeValue || editor.cursor != beforeCursor || editor.anchor != beforeAnchor || !editor.selecting {
		t.Fatal("over-limit paste was not rejected atomically")
	}
	if editor.status != secretLimitError {
		t.Fatal("over-limit paste did not set a safe status")
	}

	editor.wipe()
	editor.paste(strings.Repeat("é", maxSecretBytes/2))
	if len(editor.bytes()) != maxSecretBytes || utf8.RuneCountInString(string(editor.bytes())) != maxSecretBytes/2 {
		t.Fatal("multibyte value at byte limit was not accepted")
	}
}

func TestSecretEditorSelectionMasksAndEditing(t *testing.T) {
	editor := new(secretEditor)
	editor.insert("a界🙂z", false)
	editor.moveTo(1, false)
	editor.moveRight(true)
	editor.moveRight(true)
	masked, cursor := editor.render()
	if masked != "*[**]*" || cursor != 5 || strings.Contains(masked, "界") {
		t.Fatalf("masked selection rendering = %q at %d", masked, cursor)
	}
	editor.paste("X")
	if string(editor.buffer) != "aXz" || editor.cursor != 2 || editor.selecting {
		t.Fatal("paste did not replace Unicode selection")
	}
	editor.key(uv.Key{Code: uv.KeyBackspace})
	editor.key(uv.Key{Code: uv.KeyDelete})
	if string(editor.buffer) != "a" {
		t.Fatal("backspace/delete did not edit at rune boundaries")
	}
}

func TestSecretEditorSubmitCancelEmptyPasteAndWipe(t *testing.T) {
	editor := new(secretEditor)
	editor.insert("owned-secret", false)
	backing := editor.buffer
	editor.paste("")
	if editor.handle(uv.KeyPressEvent(uv.Key{Code: uv.KeyEnter})) != secretSubmit {
		t.Fatal("Enter did not submit")
	}
	if editor.handle(uv.KeyPressEvent(uv.Key{Code: uv.KeyEscape})) != secretCancel {
		t.Fatal("Escape did not cancel")
	}
	if editor.handle(uv.KeyPressEvent(uv.Key{Code: 'c', Mod: uv.ModCtrl})) != secretQuit {
		t.Fatal("Ctrl+C did not request quit")
	}
	if editor.handle(uv.KeyPressEvent(uv.Key{Code: uv.KeyF1})) != secretContinue || editor.status == "" {
		t.Fatal("F1 did not expose local secret-input Help")
	}
	editor.wipe()
	for _, r := range backing {
		if r != 0 {
			t.Fatal("owned editor buffer was not wiped")
		}
	}
	if editor.buffer != nil || editor.cursor != 0 || editor.selecting || editor.status != "" {
		t.Fatal("editor retained state after wipe")
	}
}
