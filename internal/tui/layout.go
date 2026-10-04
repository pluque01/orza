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
	regionTunnels
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
	tunnels layoutRect
	legend  layoutRect
	actions layoutRect
}

func calculateTunnelLayout(width, height int, focus layoutRegion) layoutState {
	s := calculateLayout(width, height, focus)
	if s.mode == layoutUndersized {
		return s
	}
	base := height - 3
	s.legend = layoutRect{0, base, width, 3}
	if s.mode == layoutWide {
		s.tree.height = base
		if focus == regionTree && width > wideLayoutWidth {
			extra := (width - wideLayoutWidth) / 10
			s.tree.width += extra
			s.details.x += extra
			s.details.width -= extra
		}
		detailHeight := (base + 1) / 2
		if focus == regionTunnels && base > 21 {
			detailHeight = max(3, detailHeight-1)
		} else if focus == regionDetails && base > 21 {
			detailHeight = min(base-3, detailHeight+1)
		}
		s.details.height = detailHeight
		s.tunnels = layoutRect{s.details.x, detailHeight, s.details.width, base - detailHeight}
	} else {
		heights := [3]int{3, 3, 3}
		focused := 0
		if focus == regionDetails {
			focused = 1
		} else if focus == regionTunnels {
			focused = 2
		}
		heights[focused] += base - 9
		s.tree = layoutRect{0, 0, width, heights[0]}
		s.details = layoutRect{0, heights[0], width, heights[1]}
		s.tunnels = layoutRect{0, heights[0] + heights[1], width, heights[2]}
	}
	return s
}

func (s layoutState) modalOverlay() layoutRect {
	if s.mode == layoutUndersized {
		return layoutRect{}
	}
	width := min(72, max(0, s.width-4))
	if s.width <= minimumLayoutWidth+2 {
		// Preserve room for the confirmation controls and their shortest critical values.
		width = s.width - 2
	}
	height := min(18, max(0, s.height-2))
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
	legendHeight := browserLegendRows(width)
	baseHeight := height - legendHeight
	state.legend = layoutRect{x: 0, y: baseHeight, width: width, height: legendHeight}

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

// calculateLayoutWithActions retains the dedicated control area for surfaces
// that accept input there, such as forms and operation/conflict states.
func calculateLayoutWithActions(width, height int, focusedBase layoutRegion) layoutState {
	state := calculateLayout(width, height, focusedBase)
	if state.mode == layoutUndersized {
		return state
	}
	baseHeight := state.height - actionsOuterHeight
	state.actions = layoutRect{x: 0, y: baseHeight, width: state.width, height: actionsOuterHeight}
	state.legend = layoutRect{}
	if state.mode == layoutWide {
		state.tree.height = baseHeight
		state.details.height = baseHeight
		return state
	}
	treeHeight := baseHeight / 2
	if baseHeight%2 != 0 && focusedBase != regionDetails {
		treeHeight++
	}
	state.tree.height = treeHeight
	state.details.y = treeHeight
	state.details.height = baseHeight - treeHeight
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
