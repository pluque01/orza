package tui

import "testing"

func TestLayoutMatrix(t *testing.T) {
	tests := []struct {
		name                   string
		width, height          int
		mode                   layoutMode
		reduced                bool
		tree, details, actions layoutRect
	}{
		{"40x12", 40, 12, layoutStacked, true, layoutRect{0, 0, 40, 4}, layoutRect{0, 4, 40, 3}, layoutRect{0, 7, 40, 5}},
		{"40x24", 40, 24, layoutStacked, true, layoutRect{0, 0, 40, 10}, layoutRect{0, 10, 40, 9}, layoutRect{0, 19, 40, 5}},
		{"60x12", 60, 12, layoutStacked, true, layoutRect{0, 0, 60, 4}, layoutRect{0, 4, 60, 3}, layoutRect{0, 7, 60, 5}},
		{"60x24", 60, 24, layoutStacked, true, layoutRect{0, 0, 60, 10}, layoutRect{0, 10, 60, 9}, layoutRect{0, 19, 60, 5}},
		{"79x12", 79, 12, layoutStacked, true, layoutRect{0, 0, 79, 4}, layoutRect{0, 4, 79, 3}, layoutRect{0, 7, 79, 5}},
		{"79x24", 79, 24, layoutStacked, true, layoutRect{0, 0, 79, 10}, layoutRect{0, 10, 79, 9}, layoutRect{0, 19, 79, 5}},
		{"80x12", 80, 12, layoutWide, true, layoutRect{0, 0, 31, 7}, layoutRect{32, 0, 48, 7}, layoutRect{0, 7, 80, 5}},
		{"80x24", 80, 24, layoutWide, false, layoutRect{0, 0, 31, 19}, layoutRect{32, 0, 48, 19}, layoutRect{0, 19, 80, 5}},
		{"100x12", 100, 12, layoutWide, true, layoutRect{0, 0, 39, 7}, layoutRect{40, 0, 60, 7}, layoutRect{0, 7, 100, 5}},
		{"100x24", 100, 24, layoutWide, false, layoutRect{0, 0, 39, 19}, layoutRect{40, 0, 60, 19}, layoutRect{0, 19, 100, 5}},
		{"160x12", 160, 12, layoutWide, true, layoutRect{0, 0, 63, 7}, layoutRect{64, 0, 96, 7}, layoutRect{0, 7, 160, 5}},
		{"160x24", 160, 24, layoutWide, false, layoutRect{0, 0, 63, 19}, layoutRect{64, 0, 96, 19}, layoutRect{0, 19, 160, 5}},
		{"39x12", 39, 12, layoutUndersized, false, layoutRect{}, layoutRect{}, layoutRect{}},
		{"40x11", 40, 11, layoutUndersized, false, layoutRect{}, layoutRect{}, layoutRect{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLayout(tt.width, tt.height, regionTree)
			if got.width != tt.width || got.height != tt.height {
				t.Fatalf("terminal size = %dx%d, want %dx%d", got.width, got.height, tt.width, tt.height)
			}
			if got.mode != tt.mode || got.reduced != tt.reduced {
				t.Fatalf("mode/reduced = %v/%t, want %v/%t", got.mode, got.reduced, tt.mode, tt.reduced)
			}
			if got.tree != tt.tree || got.details != tt.details || got.actions != tt.actions {
				t.Fatalf("rectangles = tree %#v details %#v actions %#v, want %#v %#v %#v", got.tree, got.details, got.actions, tt.tree, tt.details, tt.actions)
			}

			assertLayoutInvariants(t, got)
			if got.mode == layoutWide && got.details.x-got.tree.right() != wideGutterWidth {
				t.Fatalf("wide gutter = %d, want %d", got.details.x-got.tree.right(), wideGutterWidth)
			}
			if got.mode != layoutUndersized && got.actions.height != actionsOuterHeight {
				t.Fatalf("Actions height = %d, want %d", got.actions.height, actionsOuterHeight)
			}
		})
	}
}

func TestStackedOddRowGoesToFocusedBaseRegion(t *testing.T) {
	tests := []struct {
		name                     string
		focus                    layoutRegion
		treeHeight, detailHeight int
	}{
		{"tree", regionTree, 4, 3},
		{"details", regionDetails, 3, 4},
		{"invalid defaults to tree", layoutRegion(99), 4, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLayout(40, 12, tt.focus)
			if got.tree.height != tt.treeHeight || got.details.height != tt.detailHeight {
				t.Fatalf("heights = Tree %d Details %d, want %d/%d", got.tree.height, got.details.height, tt.treeHeight, tt.detailHeight)
			}
			assertLayoutInvariants(t, got)
		})
	}
}

func TestLayoutInteriorDimensionsAccountForBorderAndPadding(t *testing.T) {
	tests := []struct {
		outerWidth, outerHeight     int
		contentWidth, contentHeight int
	}{
		{0, 0, 0, 0},
		{3, 1, 0, 0},
		{4, 2, 0, 0},
		{5, 3, 1, 1},
		{40, 5, 36, 3},
	}

	for _, tt := range tests {
		r := layoutRect{width: tt.outerWidth, height: tt.outerHeight}
		if got := r.contentWidth(); got != tt.contentWidth {
			t.Errorf("outer width %d: content width = %d, want %d", tt.outerWidth, got, tt.contentWidth)
		}
		if got := r.contentHeight(); got != tt.contentHeight {
			t.Errorf("outer height %d: content height = %d, want %d", tt.outerHeight, got, tt.contentHeight)
		}
	}
}

func TestCenteredOverlayIsClampedBoundedAndCentered(t *testing.T) {
	tests := []struct {
		name                            string
		terminalWidth, terminalHeight   int
		requestedWidth, requestedHeight int
		want                            layoutRect
	}{
		{"centered", 80, 24, 50, 12, layoutRect{15, 6, 50, 12}},
		{"odd remainder uses top left floor", 40, 12, 17, 5, layoutRect{11, 3, 17, 5}},
		{"oversized fills terminal", 79, 23, 100, 30, layoutRect{0, 0, 79, 23}},
		{"zero request remains centered", 40, 12, 0, 0, layoutRect{20, 6, 0, 0}},
		{"negative values are normalized", -1, -2, -3, -4, layoutRect{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := calculateLayout(tt.terminalWidth, tt.terminalHeight, regionTree)
			got := state.centeredOverlay(tt.requestedWidth, tt.requestedHeight)
			if got != tt.want {
				t.Fatalf("overlay = %#v, want %#v", got, tt.want)
			}
			assertRectBounded(t, got, state.width, state.height)
			if got.x != (state.width-got.width)/2 || got.y != (state.height-got.height)/2 {
				t.Fatalf("overlay %#v is not centered in %dx%d", got, state.width, state.height)
			}
		})
	}
}

func TestLayoutHandlesMaximumReportedWidth(t *testing.T) {
	width := int(^uint(0) >> 1)
	got := calculateLayout(width, minimumLayoutHeight, regionTree)
	assertLayoutInvariants(t, got)
	if got.tree.width != (width-wideGutterWidth)/5*2+(width-wideGutterWidth)%5*2/5 {
		t.Fatalf("Tree width = %d, does not satisfy floor((width-1)*.40)", got.tree.width)
	}
}

func BenchmarkLayout(b *testing.B) {
	sizes := [...]struct{ width, height int }{
		{40, 12}, {60, 16}, {79, 23}, {80, 12}, {80, 24}, {100, 20}, {100, 30}, {160, 40},
	}
	b.ResetTimer()
	for index := range b.N {
		size := sizes[index%len(sizes)]
		_ = calculateLayout(size.width, size.height, layoutRegion(index%2))
	}
}

func assertLayoutInvariants(t *testing.T, state layoutState) {
	t.Helper()
	for name, rect := range map[string]layoutRect{"Tree": state.tree, "Details": state.details, "Actions": state.actions} {
		assertRectBounded(t, rect, state.width, state.height)
		if state.mode == layoutUndersized && rect != (layoutRect{}) {
			t.Errorf("undersized %s rectangle = %#v, want absent", name, rect)
		}
	}
	if state.mode == layoutUndersized {
		return
	}
	if overlaps(state.tree, state.details) || overlaps(state.tree, state.actions) || overlaps(state.details, state.actions) {
		t.Fatalf("base rectangles overlap: Tree %#v Details %#v Actions %#v", state.tree, state.details, state.actions)
	}
}

func assertRectBounded(t *testing.T, rect layoutRect, width, height int) {
	t.Helper()
	if rect.x < 0 || rect.y < 0 || rect.width < 0 || rect.height < 0 {
		t.Fatalf("rectangle has negative geometry: %#v", rect)
	}
	if rect.right() > width || rect.bottom() > height {
		t.Fatalf("rectangle %#v exceeds %dx%d", rect, width, height)
	}
}

func overlaps(a, b layoutRect) bool {
	if a.width == 0 || a.height == 0 || b.width == 0 || b.height == 0 {
		return false
	}
	return a.x < b.right() && b.x < a.right() && a.y < b.bottom() && b.y < a.bottom()
}
