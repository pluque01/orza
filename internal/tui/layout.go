package tui

const (
	minimumLayoutWidth   = 40
	minimumLayoutHeight  = 12
	wideLayoutWidth      = 80
	completeLayoutHeight = 24
	actionsOuterHeight   = 5
	wideGutterWidth      = 1
)

type layoutMode uint8

const (
	layoutWide layoutMode = iota
	layoutStacked
	layoutUndersized
)

type layoutRegion uint8

const (
	regionTree layoutRegion = iota
	regionDetails
)

type layoutRect struct {
	x      int
	y      int
	width  int
	height int
}

func (r layoutRect) right() int  { return r.x + r.width }
func (r layoutRect) bottom() int { return r.y + r.height }

func (r layoutRect) contentWidth() int {
	return nonNegative(r.width - 4)
}

func (r layoutRect) contentHeight() int {
	return nonNegative(r.height - 2)
}

type layoutState struct {
	width   int
	height  int
	mode    layoutMode
	reduced bool
	tree    layoutRect
	details layoutRect
	actions layoutRect
}

func (s layoutState) modalOverlay() layoutRect {
	if s.mode == layoutUndersized {
		return layoutRect{}
	}
	width := min(72, max(0, s.width-4))
	height := min(18, max(0, s.height-2))
	if s.reduced {
		width = max(1, s.width-2)
		height = max(1, s.height-2)
	}
	return s.centeredOverlay(width, height)
}

func calculateLayout(width, height int, focusedBase layoutRegion) layoutState {
	width = nonNegative(width)
	height = nonNegative(height)
	state := layoutState{width: width, height: height}

	if width < minimumLayoutWidth || height < minimumLayoutHeight {
		state.mode = layoutUndersized
		return state
	}

	state.reduced = width < wideLayoutWidth || height < completeLayoutHeight
	baseHeight := height - actionsOuterHeight
	state.actions = layoutRect{x: 0, y: baseHeight, width: width, height: actionsOuterHeight}

	if width >= wideLayoutWidth {
		state.mode = layoutWide
		baseWidth := width - wideGutterWidth
		treeWidth := baseWidth/5*2 + baseWidth%5*2/5
		state.tree = layoutRect{x: 0, y: 0, width: treeWidth, height: baseHeight}
		state.details = layoutRect{
			x:      treeWidth + wideGutterWidth,
			y:      0,
			width:  width - treeWidth - wideGutterWidth,
			height: baseHeight,
		}
		return state
	}

	state.mode = layoutStacked
	treeHeight := baseHeight / 2
	if baseHeight%2 != 0 && focusedBase != regionDetails {
		treeHeight++
	}
	detailsHeight := baseHeight - treeHeight
	state.tree = layoutRect{x: 0, y: 0, width: width, height: treeHeight}
	state.details = layoutRect{x: 0, y: treeHeight, width: width, height: detailsHeight}
	return state
}

func (s layoutState) centeredOverlay(width, height int) layoutRect {
	width = min(nonNegative(width), s.width)
	height = min(nonNegative(height), s.height)
	return layoutRect{
		x:      (s.width - width) / 2,
		y:      (s.height - height) / 2,
		width:  width,
		height: height,
	}
}

func nonNegative(value int) int {
	return max(0, value)
}
