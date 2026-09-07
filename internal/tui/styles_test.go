package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestStylesNoColorTextualSemantics(t *testing.T) {
	s := newStyles(true)
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "active title", got: s.regionTitle("Tree", true), want: "[*] Tree"},
		{name: "inactive title", got: s.regionTitle("Details", false), want: "[ ] Details"},
		{name: "selected", got: s.item("Prod", itemSemantics{selected: true}), want: ">   Prod"},
		{name: "focused", got: s.item("Name", itemSemantics{focused: true}), want: ">   Name"},
		{name: "invalid", got: s.item("Host", itemSemantics{invalid: true}), want: " !  Host"},
		{name: "primary", got: s.item("Save", itemSemantics{primary: true}), want: "  * Save"},
		{name: "combined", got: s.item("Save", itemSemantics{focused: true, invalid: true, primary: true}), want: ">!* Save"},
		{name: "empty slots", got: s.item("Cancel", itemSemantics{}), want: "    Cancel"},
		{name: "warning", got: s.warningMessage("target changed"), want: "Warning: target changed"},
		{name: "failure", got: s.failureMessage("save failed"), want: "Error: save failed"},
		{name: "scrollbar track", got: s.scrollbarCell(false), want: "│"},
		{name: "scrollbar thumb", got: s.scrollbarCell(true), want: "█"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("rendered text = %q, want %q", tt.got, tt.want)
			}
			if strings.Contains(tt.got, "\x1b[") {
				t.Fatalf("no-color text contains ANSI: %q", tt.got)
			}
		})
	}
}

func TestStylesColorRetainsTextualSemantics(t *testing.T) {
	color := newStyles(false)
	plain := newStyles(true)
	tests := []struct {
		name  string
		color string
		plain string
	}{
		{name: "active title", color: color.regionTitle("Tree", true), plain: plain.regionTitle("Tree", true)},
		{name: "inactive title", color: color.regionTitle("Details", false), plain: plain.regionTitle("Details", false)},
		{name: "selected", color: color.item("Prod", itemSemantics{selected: true}), plain: plain.item("Prod", itemSemantics{selected: true})},
		{name: "focused", color: color.item("Name", itemSemantics{focused: true}), plain: plain.item("Name", itemSemantics{focused: true})},
		{name: "invalid", color: color.item("Host", itemSemantics{invalid: true}), plain: plain.item("Host", itemSemantics{invalid: true})},
		{name: "primary", color: color.item("Save", itemSemantics{primary: true}), plain: plain.item("Save", itemSemantics{primary: true})},
		{name: "combined", color: color.item("Save", itemSemantics{focused: true, invalid: true, primary: true}), plain: plain.item("Save", itemSemantics{focused: true, invalid: true, primary: true})},
		{name: "warning", color: color.warningMessage("target changed"), plain: plain.warningMessage("target changed")},
		{name: "failure", color: color.failureMessage("save failed"), plain: plain.failureMessage("save failed")},
		{name: "scrollbar track", color: color.scrollbarCell(false), plain: plain.scrollbarCell(false)},
		{name: "scrollbar thumb", color: color.scrollbarCell(true), plain: plain.scrollbarCell(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ansi.Strip(tt.color); got != tt.plain {
				t.Fatalf("color text semantics = %q, want %q (rendered %q)", got, tt.plain, tt.color)
			}
			if !strings.Contains(tt.color, "\x1b[") {
				t.Fatalf("color text has no ANSI styling: %q", tt.color)
			}
		})
	}
}

func TestScrollbarGlyphFallsBackToOneCellASCII(t *testing.T) {
	tests := []struct {
		name                string
		preferred, fallback string
		want                string
	}{
		{name: "preferred track", preferred: "│", fallback: "|", want: "│"},
		{name: "preferred thumb", preferred: "█", fallback: "#", want: "█"},
		{name: "wide preferred", preferred: "界", fallback: "|", want: "|"},
		{name: "empty preferred", preferred: "", fallback: "#", want: "#"},
		{name: "invalid pair", preferred: "界", fallback: "界", want: "|"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oneCellGlyph(tt.preferred, tt.fallback)
			if got != tt.want || ansi.StringWidth(got) != 1 {
				t.Fatalf("oneCellGlyph(%q, %q) = %q width %d, want %q width 1", tt.preferred, tt.fallback, got, ansi.StringWidth(got), tt.want)
			}
		})
	}
}
