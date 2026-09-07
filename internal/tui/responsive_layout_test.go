package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

type us5Size struct {
	name          string
	width, height int
}

var us5ContractSizes = []us5Size{
	{name: "40x12", width: 40, height: 12},
	{name: "40x24", width: 40, height: 24},
	{name: "60x12", width: 60, height: 12},
	{name: "60x24", width: 60, height: 24},
	{name: "79x12", width: 79, height: 12},
	{name: "79x24", width: 79, height: 24},
	{name: "80x12", width: 80, height: 12},
	{name: "80x24", width: 80, height: 24},
	{name: "100x12", width: 100, height: 12},
	{name: "100x24", width: 100, height: 24},
	{name: "160x12", width: 160, height: 12},
	{name: "160x24", width: 160, height: 24},
}

func TestUS5ExactResponsiveGeometryMatrix(t *testing.T) {
	tests := []struct {
		us5Size
		mode                   layoutMode
		reduced                bool
		tree, details, actions layoutRect
	}{
		{us5ContractSizes[0], layoutStacked, true, layoutRect{0, 0, 40, 4}, layoutRect{0, 4, 40, 3}, layoutRect{0, 7, 40, 5}},
		{us5ContractSizes[1], layoutStacked, true, layoutRect{0, 0, 40, 10}, layoutRect{0, 10, 40, 9}, layoutRect{0, 19, 40, 5}},
		{us5ContractSizes[2], layoutStacked, true, layoutRect{0, 0, 60, 4}, layoutRect{0, 4, 60, 3}, layoutRect{0, 7, 60, 5}},
		{us5ContractSizes[3], layoutStacked, true, layoutRect{0, 0, 60, 10}, layoutRect{0, 10, 60, 9}, layoutRect{0, 19, 60, 5}},
		{us5ContractSizes[4], layoutStacked, true, layoutRect{0, 0, 79, 4}, layoutRect{0, 4, 79, 3}, layoutRect{0, 7, 79, 5}},
		{us5ContractSizes[5], layoutStacked, true, layoutRect{0, 0, 79, 10}, layoutRect{0, 10, 79, 9}, layoutRect{0, 19, 79, 5}},
		{us5ContractSizes[6], layoutWide, true, layoutRect{0, 0, 31, 7}, layoutRect{32, 0, 48, 7}, layoutRect{0, 7, 80, 5}},
		{us5ContractSizes[7], layoutWide, false, layoutRect{0, 0, 31, 19}, layoutRect{32, 0, 48, 19}, layoutRect{0, 19, 80, 5}},
		{us5ContractSizes[8], layoutWide, true, layoutRect{0, 0, 39, 7}, layoutRect{40, 0, 60, 7}, layoutRect{0, 7, 100, 5}},
		{us5ContractSizes[9], layoutWide, false, layoutRect{0, 0, 39, 19}, layoutRect{40, 0, 60, 19}, layoutRect{0, 19, 100, 5}},
		{us5ContractSizes[10], layoutWide, true, layoutRect{0, 0, 63, 7}, layoutRect{64, 0, 96, 7}, layoutRect{0, 7, 160, 5}},
		{us5ContractSizes[11], layoutWide, false, layoutRect{0, 0, 63, 19}, layoutRect{64, 0, 96, 19}, layoutRect{0, 19, 160, 5}},
		{us5Size{name: "39x12", width: 39, height: 12}, layoutUndersized, false, layoutRect{}, layoutRect{}, layoutRect{}},
		{us5Size{name: "39x24", width: 39, height: 24}, layoutUndersized, false, layoutRect{}, layoutRect{}, layoutRect{}},
		{us5Size{name: "40x11", width: 40, height: 11}, layoutUndersized, false, layoutRect{}, layoutRect{}, layoutRect{}},
		{us5Size{name: "80x11", width: 80, height: 11}, layoutUndersized, false, layoutRect{}, layoutRect{}, layoutRect{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLayout(tt.width, tt.height, regionTree)
			if got.mode != tt.mode || got.reduced != tt.reduced {
				t.Fatalf("mode/reduced = %v/%t, want %v/%t", got.mode, got.reduced, tt.mode, tt.reduced)
			}
			if got.tree != tt.tree || got.details != tt.details || got.actions != tt.actions {
				t.Fatalf("rectangles = Tree %#v Details %#v Actions %#v, want %#v %#v %#v", got.tree, got.details, got.actions, tt.tree, tt.details, tt.actions)
			}
			assertLayoutInvariants(t, got)

			if got.mode == layoutWide {
				baseWidth := tt.width - wideGutterWidth
				if got.tree.width != baseWidth*40/100 || got.details.width != baseWidth-got.tree.width {
					t.Fatalf("wide split = %d/%d, want floor(%d*.40) and remainder", got.tree.width, got.details.width, baseWidth)
				}
				if got.details.x-got.tree.right() != 1 {
					t.Fatalf("wide gutter = %d, want exactly one cell", got.details.x-got.tree.right())
				}
			}
			if got.mode != layoutUndersized {
				if got.actions.height != 5 || got.actions.y != tt.height-5 {
					t.Fatalf("Actions geometry = %#v, want final five outer rows", got.actions)
				}
				for name, rect := range map[string]layoutRect{"Tree": got.tree, "Details": got.details, "Actions": got.actions} {
					if rect.contentWidth() != max(0, rect.width-4) || rect.contentHeight() != max(0, rect.height-2) {
						t.Fatalf("%s content geometry does not reserve border and one-cell horizontal padding: %#v", name, rect)
					}
				}
			}
		})
	}
}

func TestUS5StackedOddRowFollowsPreservedApplicationFocus(t *testing.T) {
	tree := calculateLayout(79, 12, regionTree)
	details := calculateLayout(79, 12, regionDetails)
	if tree.tree.height != 4 || tree.details.height != 3 {
		t.Fatalf("Tree focus split = %d/%d, want 4/3", tree.tree.height, tree.details.height)
	}
	if details.tree.height != 3 || details.details.height != 4 {
		t.Fatalf("Details/form focus split = %d/%d, want 3/4", details.tree.height, details.details.height)
	}
}

func TestUS5Reduced40x12KeepsIdentityAndSafetyBeforeOverflow(t *testing.T) {
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	view := model.View().Content
	for _, required := range []string{"[*] Tree", "[ ] Details", "[ ] Actions", "> [/] /"} {
		if !strings.Contains(view, required) {
			t.Fatalf("40x12 reduced frame omitted priority content %q:\n%s", required, view)
		}
	}
	for _, control := range []string{"r Reload", "q Quit", "? Help"} {
		if !strings.Contains(view, control) {
			t.Fatalf("40x12 Actions omitted safety control %q:\n%s", control, view)
		}
	}
	if !strings.Contains(view, actionsOverflowMarker) {
		t.Fatalf("40x12 reduced frame omitted overflow marker:\n%s", view)
	}
	lines := strings.Split(view, "\n")
	if len(lines) != 12 {
		t.Fatalf("40x12 frame has %d rows, want exactly 12", len(lines))
	}
	for _, line := range lines {
		if width := ansi.StringWidth(line); width > 40 {
			t.Fatalf("40x12 frame line width = %d: %q", width, line)
		}
	}
}
