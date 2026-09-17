package tui

import (
	"slices"
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
		{name: "content badge", got: s.contentBadge("Connection"), want: "Connection"},
		{name: "descriptive label", got: s.descriptiveLabel("Endpoint"), want: "Endpoint"},
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
		{name: "content badge", color: color.contentBadge("Connection"), plain: plain.contentBadge("Connection")},
		{name: "descriptive label", color: color.descriptiveLabel("Endpoint"), plain: plain.descriptiveLabel("Endpoint")},
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

func TestStylesTypeBadgeConstraintsAndPlacementRendering(t *testing.T) {
	plain := newStyles(true)
	color := newStyles(false)

	plainBadge, ok := plain.typeBadge("Connection", badgePlacementContent, true)
	if !ok {
		t.Fatal("controlled content badge was rejected")
	}
	if plainBadge.label != "Connection" || plainBadge.placement != badgePlacementContent || plainBadge.active || plainBadge.accented {
		t.Fatalf("plain content badge = %#v", plainBadge)
	}
	colorBadge, ok := color.typeBadge("Connection", badgePlacementContent, false)
	if !ok || !colorBadge.accented {
		t.Fatalf("color content badge = %#v, valid %t", colorBadge, ok)
	}

	plainText := plain.renderTypeBadge(plainBadge)
	coloredText := color.renderTypeBadge(colorBadge)
	if plainText != "Connection" || ansi.Strip(coloredText) != plainText {
		t.Fatalf("badge color/plain = %q/%q", coloredText, plainText)
	}
	if want := "\x1b[1mConnection\x1b[m"; coloredText != want {
		t.Fatalf("bold-only context badge = %q, want exact %q", coloredText, want)
	}

	plainTitle, ok := plain.typeBadge("Details", badgePlacementTitle, true)
	if !ok || plain.renderTypeBadge(plainTitle) != "Details" {
		t.Fatalf("plain structural title = %#v / %q", plainTitle, plain.renderTypeBadge(plainTitle))
	}
	coloredTitle, ok := color.typeBadge("Details", badgePlacementTitle, true)
	if !ok || ansi.Strip(color.renderTypeBadge(coloredTitle)) != "Details" {
		t.Fatalf("colored structural title = %#v / %q", coloredTitle, color.renderTypeBadge(coloredTitle))
	}
	if strings.Contains(ansi.Strip(color.renderTypeBadge(coloredTitle)), "[") {
		t.Fatalf("structural title retained brackets: %q", color.renderTypeBadge(coloredTitle))
	}

	for _, test := range []struct {
		name      string
		label     string
		placement badgePlacement
	}{
		{name: "empty", placement: badgePlacementContent},
		{name: "invalid placement", label: "Connection"},
		{name: "control bearing", label: "Con\nnection", placement: badgePlacementContent},
		{name: "ANSI bearing", label: "\x1b[31mConnection", placement: badgePlacementContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, valid := color.typeBadge(test.label, test.placement, false); valid {
				t.Fatalf("accepted uncontrolled badge label %q at placement %d", test.label, test.placement)
			}
		})
	}
}

func TestStylesStructuredFieldGroupUsesDisplayWidthAndLocalThreshold(t *testing.T) {
	fields := []displayField{
		{label: "Name", value: "production"},
		{label: "界面", value: "catalog"},
		{label: "Endpoint", value: "host:2222"},
	}

	aligned := newStructuredFieldGroup(fields, 0, 17, false)
	if aligned.labelWidth != 8 || aligned.availableValueWidth() != 8 || aligned.minimumValueWidth != 8 || aligned.mode != fieldModeAligned {
		t.Fatalf("aligned group = %#v, available value width %d", aligned, aligned.availableValueWidth())
	}
	if got, want := aligned.render(newStyles(true)), []string{"Name     product…", "界面     catalog", "Endpoint host:22…"}; !slices.Equal(got, want) {
		t.Fatalf("aligned fields = %#v, want %#v", got, want)
	}

	stacked := newStructuredFieldGroup(fields, 0, 16, false)
	if stacked.availableValueWidth() != 7 || stacked.mode != fieldModeStacked {
		t.Fatalf("stacked group = %#v, available value width %d", stacked, stacked.availableValueWidth())
	}
	if got, want := stacked.render(newStyles(true)), []string{"Name", "  production", "界面", "  catalog", "Endpoint", "  host:2222"}; !slices.Equal(got, want) {
		t.Fatalf("stacked fields = %#v, want %#v", got, want)
	}

	compact := newStructuredFieldGroup(fields, 4, 9, true)
	if compact.prefixWidth != 4 || compact.contentWidth != 9 || compact.mode != fieldModeCompactInteractive {
		t.Fatalf("compact interactive group = %#v", compact)
	}
	fields[0].label = "mutated"
	if compact.fields[0].label != "Name" {
		t.Fatalf("group did not retain ordered field snapshot: %#v", compact.fields)
	}
}

func TestStylesDescriptiveLabelsAreMutedColonlessAndStatusesUnchanged(t *testing.T) {
	plain := newStyles(true)
	color := newStyles(false)
	if got := plain.descriptiveLabel("Endpoint:"); got != "Endpoint" {
		t.Fatalf("colonless label = %q", got)
	}
	colored := color.descriptiveLabel("Endpoint:")
	if ansi.Strip(colored) != plain.descriptiveLabel("Endpoint:") || !strings.Contains(colored, "\x1b[90m") {
		t.Fatalf("muted color/plain label = %q/%q", colored, plain.descriptiveLabel("Endpoint:"))
	}
	if got := plain.warningMessage("target changed"); got != "Warning: target changed" {
		t.Fatalf("warning prefix = %q", got)
	}
	if got := plain.failureMessage("save failed"); got != "Error: save failed" {
		t.Fatalf("error prefix = %q", got)
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
