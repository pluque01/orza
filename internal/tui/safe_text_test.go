package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func TestSafeTextPrintableUnicode(t *testing.T) {
	tests := []struct {
		name   string
		source string
		width  int
	}{
		{name: "ASCII", source: "alpha-42", width: 8},
		{name: "combining", source: "Cafe\u0301", width: 4},
		{name: "CJK", source: "東京", width: 4},
		{name: "emoji", source: "Hi 👋", width: 5},
		{name: "emoji ZWJ", source: "👩\u200d💻", width: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := safeText(test.source, test.width); got != test.source {
				t.Fatalf("safeText(%q, %d) = %q, want unchanged", test.source, test.width, got)
			}
		})
	}
}

func TestSafeTextMakesTerminalControlsVisibleAndInert(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "CR and LF", source: "first\rsecond\nthird", want: `first\rsecond\nthird`},
		{name: "ANSI CSI", source: "\x1b[31mred\x1b[0m", want: `\x1B[31mred\x1B[0m`},
		{name: "C0", source: "a\x00\tb\x7f", want: `a\x00\tb\x7F`},
		{name: "C1", source: "a\u0085\u009bb", want: `a\x85\x9Bb`},
		{name: "bidi marks", source: "a\u061c\u200e\u200fb", want: `a\u061C\u200E\u200Fb`},
		{name: "bidi embedding and isolates", source: "a\u202a\u202e\u2066\u2069b", want: `a\u202A\u202E\u2066\u2069b`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := safeText(test.source, 80)
			if got != test.want {
				t.Fatalf("safeText(%q) = %q, want %q", test.source, got, test.want)
			}
			assertTerminalSafe(t, got, 80)
		})
	}
}

func TestSafeTextEscapesEveryInvalidUTF8Byte(t *testing.T) {
	source := string([]byte{'a', 0xff, 0xc3, '(', 0x80, 'z'})
	got := safeText(source, 80)
	if want := `a\xFF\xC3(\x80z`; got != want {
		t.Fatalf("safeText invalid UTF-8 = %q, want %q", got, want)
	}
	assertTerminalSafe(t, got, 80)
}

func TestSafeTextTruncatesByDisplayWidthWithEllipsis(t *testing.T) {
	tests := []struct {
		name   string
		source string
		width  int
		want   string
	}{
		{name: "zero width", source: "abc", width: 0, want: ""},
		{name: "negative width", source: "abc", width: -1, want: ""},
		{name: "one cell", source: "abc", width: 1, want: "…"},
		{name: "ASCII", source: "abcdef", width: 4, want: "abc…"},
		{name: "combining", source: "e\u0301clair", width: 3, want: "e\u0301c…"},
		{name: "CJK", source: "東京駅", width: 5, want: "東京…"},
		{name: "emoji ZWJ", source: "A👩\u200d💻B", width: 3, want: "A…"},
		{name: "escaped control", source: "A\nB", width: 3, want: "A\\…"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := safeText(test.source, test.width)
			if got != test.want {
				t.Fatalf("safeText(%q, %d) = %q, want %q", test.source, test.width, got, test.want)
			}
			assertTerminalSafe(t, got, test.width)
			if test.width > 0 && ansi.StringWidth(safeText(test.source, 80)) > test.width && !strings.HasSuffix(got, safeTextEllipsis) {
				t.Fatalf("truncated projection lacks ellipsis: %q", got)
			}
		})
	}
}

func TestSafeTextOversizedSourceIsBoundedAndUnchanged(t *testing.T) {
	source := strings.Repeat("界e\u0301👩\u200d💻\x1b\n", 10_000)
	original := source
	got := safeText(source, 32)

	if source != original {
		t.Fatal("safeText changed its source string")
	}
	if !strings.HasSuffix(got, safeTextEllipsis) {
		t.Fatalf("oversized projection lacks ellipsis: %q", got)
	}
	assertTerminalSafe(t, got, 32)
}

func assertTerminalSafe(t *testing.T, got string, width int) {
	t.Helper()
	if !utf8.ValidString(got) {
		t.Fatalf("projection is not valid UTF-8: %q", got)
	}
	if strings.ContainsAny(got, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f"+
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
		t.Fatalf("projection contains a C0 control: %q", got)
	}
	for _, r := range got {
		if r >= '\u0080' && r <= '\u009f' || isBidiFormattingControl(r) {
			t.Fatalf("projection contains unsafe rune %U: %q", r, got)
		}
	}
	if displayWidth := ansi.StringWidth(got); displayWidth > max(width, 0) {
		t.Fatalf("projection width = %d, want <= %d: %q", displayWidth, max(width, 0), got)
	}
}
