package tui

import (
	"math/bits"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const noActiveLine = -1

type viewportState struct {
	logicalOffset int
}

type scrollbarGeometry struct {
	visible     bool
	trackHeight int
	thumbTop    int
	thumbLength int
}

func (geometry scrollbarGeometry) thumbAt(row int) bool {
	return geometry.visible && row >= geometry.thumbTop && row < geometry.thumbTop+geometry.thumbLength
}

type viewportProjection struct {
	lines          []string
	logicalOffset  int
	renderOffset   int
	contentLength  int
	availableRows  int
	visibleRows    int
	hasPrevious    bool
	hasNext        bool
	activeLine     int
	scrollbar      scrollbarGeometry
	scrollbarStart int
}

func newViewportState(offset int) viewportState {
	return viewportState{logicalOffset: max(0, offset)}
}

func (v viewportState) withOffset(offset int) viewportState {
	v.logicalOffset = max(0, offset)
	return v
}

func (v viewportState) project(content []string, rows, width, activeLine int) viewportProjection {
	rows = max(0, rows)
	width = max(0, width)
	projection := viewportProjection{
		logicalOffset: v.logicalOffset,
		contentLength: len(content),
		availableRows: rows,
		activeLine:    activeLine,
	}
	if len(content) == 0 || rows == 0 || width == 0 {
		return projection
	}
	if activeLine < 0 || activeLine >= len(content) {
		activeLine = noActiveLine
		projection.activeLine = noActiveLine
	}

	maxOffset := viewportMaximumOffset(len(content), rows)
	start := min(max(0, v.logicalOffset), maxOffset)
	if activeLine != noActiveLine {
		switch {
		case activeLine < start:
			start = activeLine
		case activeLine >= start+rows:
			start = activeLine - rows + 1
		}
		start = min(max(0, start), maxOffset)
	}
	end := min(len(content), start+rows)

	projection.renderOffset = start
	projection.visibleRows = end - start
	projection.hasPrevious = start > 0
	projection.hasNext = end < len(content)
	projection.lines = viewportTruncateLines(content[start:end], width)
	if width >= 1 {
		projection.scrollbar = newScrollbarGeometry(len(content), rows, start)
	}
	return projection
}

func newScrollbarGeometry(contentLength, visibleRows, offset int) scrollbarGeometry {
	if contentLength <= visibleRows || visibleRows <= 0 {
		return scrollbarGeometry{}
	}
	trackHeight := visibleRows
	maximum := contentLength - visibleRows
	offset = min(max(0, offset), maximum)
	thumbLength := min(trackHeight, max(1, roundMulDiv(trackHeight, visibleRows, contentLength)))
	thumbTop := roundMulDiv(trackHeight-thumbLength, offset, maximum)
	return scrollbarGeometry{
		visible:     true,
		trackHeight: trackHeight,
		thumbTop:    min(max(0, thumbTop), trackHeight-thumbLength),
		thumbLength: thumbLength,
	}
}

// roundMulDiv returns round-half-up(a*b/divisor) for non-negative inputs
// without overflowing the intermediate product.
func roundMulDiv(a, b, divisor int) int {
	if a <= 0 || b <= 0 || divisor <= 0 {
		return 0
	}
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	quotient, remainder := bits.Div64(hi, lo, uint64(divisor))
	threshold := uint64(divisor)/2 + uint64(divisor)%2
	if remainder >= threshold {
		quotient++
	}
	return int(quotient)
}

func viewportTruncateLines(lines []string, width int) []string {
	truncated := make([]string, len(lines))
	for index, line := range lines {
		for _, label := range []string{"Target: ", "ID: "} {
			if strings.HasPrefix(line, label) {
				line = label + safeText(strings.TrimPrefix(line, label), max(0, width-len(label)))
				break
			}
		}
		truncated[index] = viewportEllipsis(line, width)
	}
	return truncated
}

func viewportMaximumOffset(contentLength, rows int) int {
	if contentLength <= rows || rows <= 0 {
		return 0
	}
	return contentLength - rows
}

func viewportEllipsis(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "…")
}
