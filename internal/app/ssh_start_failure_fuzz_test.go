package app

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func FuzzSanitizeSSHFailureDetail(f *testing.F) {
	for _, seed := range []string{
		"operation timed out",
		"line one\nline two",
		"\x1b[31mchanged\x1b[0m",
		"safe\u202esecret",
		"password=canary",
		strings.Repeat("界", 300),
		string([]byte{0xff, 0xfe, 'x'}),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		output := sanitizeSSHFailureDetail(input)
		if !utf8.ValidString(output) {
			t.Fatalf("output is not valid UTF-8: %q", output)
		}
		if strings.ContainsAny(output, "\r\n") || strings.Contains(output, "\x1b") || strings.ContainsAny(output, "\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069") {
			t.Fatalf("output contains line, ANSI, or bidi controls: %q", output)
		}
		if width := ansi.StringWidth(output); width > 256 {
			t.Fatalf("output occupies %d visible cells", width)
		}
		lower := strings.ToLower(output)
		for _, sensitive := range []string{"password", "passphrase", "private key", "credential", "secret"} {
			if strings.Contains(strings.ToLower(input), sensitive) && output != "" {
				t.Fatalf("sensitive input was retained: %q", lower)
			}
		}
	})
}

func TestSanitizeSSHFailureDetailVisibleCellBoundary(t *testing.T) {
	exact := strings.Repeat("界", 128)
	if got := sanitizeSSHFailureDetail(exact); got != exact {
		t.Fatalf("exact-boundary detail changed: %q", got)
	}
	over := strings.Repeat("界", 129)
	got := sanitizeSSHFailureDetail(over)
	if ansi.StringWidth(got) != 255 || !strings.HasSuffix(got, "…") {
		t.Fatalf("over-boundary detail width = %d ending %q", ansi.StringWidth(got), got)
	}
}
