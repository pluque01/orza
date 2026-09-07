package tui

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestViewportOverflowUsesScrollbarWithoutContentMarkers(t *testing.T) {
	lines := []string{"zero", "one", "two", "three", "four"}
	tests := []struct {
		name   string
		lines  []string
		offset int
		rows   int
		want   []string
		prev   bool
		next   bool
	}{
		{name: "next", lines: lines, offset: 0, rows: 3, want: []string{"zero", "one", "two"}, next: true},
		{name: "previous", lines: lines, offset: 3, rows: 3, want: []string{"two", "three", "four"}, prev: true},
		{name: "both", lines: lines, offset: 1, rows: 3, want: []string{"one", "two", "three"}, prev: true, next: true},
		{name: "neither", lines: lines[:3], offset: 0, rows: 3, want: []string{"zero", "one", "two"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection := newViewportState(test.offset).project(test.lines, test.rows, 20, noActiveLine)
			if !slices.Equal(projection.lines, test.want) {
				t.Fatalf("lines = %#v, want %#v", projection.lines, test.want)
			}
			if projection.hasPrevious != test.prev || projection.hasNext != test.next {
				t.Fatalf("overflow = (%v, %v), want (%v, %v)", projection.hasPrevious, projection.hasNext, test.prev, test.next)
			}
			if strings.Contains(strings.Join(projection.lines, "\n"), viewportPreviousLabel) || strings.Contains(strings.Join(projection.lines, "\n"), viewportNextLabel) {
				t.Fatalf("projection rendered a directional marker: %#v", projection.lines)
			}
			if (test.prev || test.next) != projection.scrollbar.visible {
				t.Fatalf("scrollbar visible = %v, want %v: %#v", projection.scrollbar.visible, test.prev || test.next, projection.scrollbar)
			}
		})
	}
}

func TestViewportScrollbarEligibilityPreservesOneContentCell(t *testing.T) {
	content := []string{"a", "b"}
	for _, test := range []struct {
		width       int
		wantLines   int
		wantVisible bool
	}{
		{width: 0, wantLines: 0, wantVisible: false},
		{width: 1, wantLines: 1, wantVisible: true},
		{width: 2, wantLines: 1, wantVisible: true},
	} {
		projection := newViewportState(0).project(content, 1, test.width, noActiveLine)
		if len(projection.lines) != test.wantLines || projection.scrollbar.visible != test.wantVisible {
			t.Fatalf("width %d projection lines=%#v scrollbar=%#v, want %d lines visible=%t", test.width, projection.lines, projection.scrollbar, test.wantLines, test.wantVisible)
		}
	}
}

func TestViewportRowBudgetsKeepPriorityLineVisible(t *testing.T) {
	lines := []string{"target", "context", "active error", "recovery", "actions"}
	tests := []struct {
		name   string
		rows   int
		active int
		want   []string
	}{
		{name: "one row error", rows: 1, active: 2, want: []string{"active error"}},
		{name: "two rows error", rows: 2, active: 2, want: []string{"context", "active error"}},
		{name: "two rows recovery", rows: 2, active: 3, want: []string{"active error", "recovery"}},
		{name: "many rows active", rows: 4, active: 3, want: []string{"target", "context", "active error", "recovery"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection := newViewportState(0).project(lines, test.rows, 40, test.active)
			if !slices.Equal(projection.lines, test.want) {
				t.Fatalf("lines = %#v, want %#v", projection.lines, test.want)
			}
			if !slices.Contains(projection.lines, lines[test.active]) {
				t.Fatalf("priority line %q is not visible in %#v", lines[test.active], projection.lines)
			}
			if len(projection.lines) > test.rows {
				t.Fatalf("used %d rows with budget %d", len(projection.lines), test.rows)
			}
		})
	}
}

func TestViewportResizeClampsAndRestoresLogicalOffset(t *testing.T) {
	lines := []string{"0", "1", "2", "3", "4", "5", "6", "7"}
	state := newViewportState(2)

	small := state.project(lines, 3, 10, 4)
	large := state.project(lines, 6, 10, 4)
	if small.logicalOffset != 2 || small.renderOffset != 2 {
		t.Fatalf("small offsets = logical %d, render %d; want 2, 2", small.logicalOffset, small.renderOffset)
	}
	if large.logicalOffset != 2 || large.renderOffset != 2 {
		t.Fatalf("large offsets = logical %d, render %d; want restored 2, 2", large.logicalOffset, large.renderOffset)
	}

	bottom := newViewportState(6)
	full := bottom.project(lines, len(lines), 10, 7)
	restored := bottom.project(lines, 3, 10, 7)
	if full.renderOffset != 0 || restored.renderOffset != 5 || bottom.logicalOffset != 6 {
		t.Fatalf("resize clamp/restore = full %d, restored %d, state %d; want 0, 5, 6", full.renderOffset, restored.renderOffset, bottom.logicalOffset)
	}
}

func TestScrollbarGeometryUsesHalfUpRoundingAndClampedEndpoints(t *testing.T) {
	tests := []struct {
		name                       string
		content, rows, offset      int
		track, thumbTop, thumbSize int
	}{
		{name: "half-up thumb length", content: 6, rows: 3, track: 3, thumbSize: 2},
		{name: "half-up thumb position", content: 8, rows: 4, offset: 1, track: 4, thumbTop: 1, thumbSize: 2},
		{name: "one-row track", content: 2, rows: 1, offset: 1, track: 1, thumbSize: 1},
		{name: "top endpoint", content: 10, rows: 4, offset: 0, track: 4, thumbTop: 0, thumbSize: 2},
		{name: "bottom endpoint", content: 10, rows: 4, offset: 6, track: 4, thumbTop: 2, thumbSize: 2},
		{name: "negative stale offset", content: 10, rows: 4, offset: -100, track: 4, thumbTop: 0, thumbSize: 2},
		{name: "large stale offset", content: 10, rows: 4, offset: 100, track: 4, thumbTop: 2, thumbSize: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := newScrollbarGeometry(test.content, test.rows, test.offset)
			if !got.visible || got.trackHeight != test.track || got.thumbTop != test.thumbTop || got.thumbLength != test.thumbSize {
				t.Fatalf("geometry = %#v, want track=%d top=%d length=%d", got, test.track, test.thumbTop, test.thumbSize)
			}
			if !got.thumbAt(got.thumbTop) || got.thumbAt(got.thumbTop-1) || got.thumbAt(got.thumbTop+got.thumbLength) {
				t.Fatalf("thumb bounds are inconsistent: %#v", got)
			}
		})
	}
}

func TestScrollbarGeometryHandlesMaximumIntegers(t *testing.T) {
	geometry := newScrollbarGeometry(math.MaxInt, math.MaxInt-1, math.MaxInt)
	if !geometry.visible || geometry.trackHeight != math.MaxInt-1 || geometry.thumbLength < 1 {
		t.Fatalf("maximum geometry is invalid: %#v", geometry)
	}
	if geometry.thumbTop < 0 || geometry.thumbTop+geometry.thumbLength > geometry.trackHeight {
		t.Fatalf("maximum geometry is out of bounds: %#v", geometry)
	}
}

func TestSC003ExactGeometryMatrix(t *testing.T) {
	for _, contentLength := range []int{5, 10, 101, 1000} {
		for _, visibleRows := range []int{1, 2, 4} {
			maximum := contentLength - visibleRows
			for _, offset := range []int{0, (maximum + 1) / 2, maximum} {
				geometry := newScrollbarGeometry(contentLength, visibleRows, offset)
				wantLength := max(1, int(math.Floor(float64(visibleRows*visibleRows)/float64(contentLength)+0.5)))
				wantTop := int(math.Floor(float64((visibleRows-wantLength)*offset)/float64(maximum) + 0.5))
				if !geometry.visible || geometry.trackHeight != visibleRows || geometry.thumbLength != wantLength || geometry.thumbTop != wantTop {
					t.Fatalf("content=%d visible=%d offset=%d geometry=%#v, want length=%d top=%d", contentLength, visibleRows, offset, geometry, wantLength, wantTop)
				}
			}
		}
	}
}

func TestViewportEffectiveActiveVisibilityDoesNotShiftOffset(t *testing.T) {
	lines := []string{"zero", "one", "two", "three", "four"}
	projection := newViewportState(1).project(lines, 3, 20, 3)
	if projection.renderOffset != 1 || !slices.Equal(projection.lines, []string{"one", "two", "three"}) {
		t.Fatalf("effective active visibility shifted projection: offset=%d lines=%#v", projection.renderOffset, projection.lines)
	}
	if projection.activeLine != 3 || !projection.scrollbar.visible {
		t.Fatalf("active/scrollbar metadata = active %d scrollbar %#v", projection.activeLine, projection.scrollbar)
	}
}

func TestRenderRegionPanelPlacesScrollbarAtRightEdge(t *testing.T) {
	geometry := newScrollbarGeometry(6, 3, 3)
	panel := renderRegionPanelWithScrollbar("Panel", []string{"three", "four", "five"}, layoutRect{width: 12, height: 5}, newStyles(true), geometry, 0)
	lines := strings.Split(panel, "\n")
	wantSuffixes := []string{"", "││", "█│", "█│", ""}
	for row, suffix := range wantSuffixes {
		if suffix != "" && !strings.HasSuffix(lines[row], suffix) {
			t.Fatalf("row %d right edge = %q, want suffix %q\npanel:\n%s", row, lines[row], suffix, panel)
		}
	}
	if strings.Contains(panel, viewportPreviousLabel) || strings.Contains(panel, viewportNextLabel) {
		t.Fatalf("panel rendered legacy viewport marker:\n%s", panel)
	}
}

func TestViewportEllipsisStaysWithinVisibleWidth(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
	}{
		{name: "ASCII", line: "abcdefgh", width: 5},
		{name: "combining", line: "e\u0301clair", width: 4},
		{name: "CJK", line: "界界abc", width: 4},
		{name: "emoji grapheme", line: "A👩\u200d💻BC", width: 4},
		{name: "narrow marker", line: "content", width: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection := newViewportState(0).project([]string{test.line, "next"}, 2, test.width, 0)
			for _, line := range projection.lines {
				if width := ansi.StringWidth(line); width > test.width {
					t.Fatalf("visible width of %q = %d, limit %d", line, width, test.width)
				}
			}
			if got := projection.lines[0]; !strings.Contains(got, "…") {
				t.Fatalf("truncated line %q has no ellipsis", got)
			}
		})
	}
}
