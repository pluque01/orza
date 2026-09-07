package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

const safeTextEllipsis = "…"

// safeText returns a single-line, terminal-safe projection of source bounded
// to width display cells. It never changes source; callers must keep secrets
// and raw error causes out of the string passed here.
func safeText(source string, width int) string {
	if width <= 0 || source == "" {
		return ""
	}

	var projected strings.Builder
	projected.Grow(len(source))
	for offset := 0; offset < len(source); {
		r, size := utf8.DecodeRuneInString(source[offset:])
		if r == utf8.RuneError && size == 1 {
			writeByteEscape(&projected, source[offset])
			offset++
			continue
		}
		offset += size

		switch r {
		case '\r':
			projected.WriteString(`\r`)
		case '\n':
			projected.WriteString(`\n`)
		case '\t':
			projected.WriteString(`\t`)
		default:
			switch {
			case r <= '\x1f' || r >= '\x7f' && r <= '\x9f':
				writeRuneEscape(&projected, r)
			case isBidiFormattingControl(r):
				writeRuneEscape(&projected, r)
			default:
				projected.WriteRune(r)
			}
		}
	}

	return ansi.Truncate(projected.String(), width, safeTextEllipsis)
}

func writeByteEscape(out *strings.Builder, b byte) {
	const hex = "0123456789ABCDEF"
	out.WriteString(`\x`)
	out.WriteByte(hex[b>>4])
	out.WriteByte(hex[b&0x0f])
}

func writeRuneEscape(out *strings.Builder, r rune) {
	if r <= 0xff {
		writeByteEscape(out, byte(r))
		return
	}

	const hex = "0123456789ABCDEF"
	out.WriteString(`\u`)
	for shift := 12; shift >= 0; shift -= 4 {
		out.WriteByte(hex[(r>>shift)&0x0f])
	}
}

func isBidiFormattingControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' ||
		r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069'
}
