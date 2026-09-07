//go:build acceptance

package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSC007EveryScaleNodeAcrossTwelveSizesTwentyRuns(t *testing.T) {
	model, snapshot := newSC007ScaleModel(t)
	if len(snapshot.nodes) != 1101 {
		t.Fatalf("scale catalog nodes = %d, want root + 100 folders + 1000 connections", len(snapshot.nodes))
	}
	maximumDepth := 0
	for _, row := range model.browser.rows {
		maximumDepth = max(maximumDepth, row.depth)
	}
	if maximumDepth != 10 {
		t.Fatalf("scale catalog visible depth = %d, want exactly 10", maximumDepth)
	}

	for run := range scConformanceRuns {
		for index, selected := range model.browser.rows {
			model.browser.selectedID = selected.id
			model.ownedSelectionID = selected.id
			model.syncDetail()
			if model.detailState.targetID != selected.id {
				t.Fatalf("run %d node %d did not synchronize selection/detail", run+1, index)
			}
			for _, size := range us5ContractSizes {
				updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
				view := model.View().Content
				assertUS5FrameBounded(t, view, size.width, size.height)
				layout := calculateLayout(size.width, size.height, regionTree)
				treeWidth, treeRows := layout.tree.contentWidth(), layout.tree.contentHeight()
				treeContent := sc007TreeContent(model.browser, model.styles, treeWidth)
				treeProjection := model.browser.viewport.project(treeContent, treeRows, treeWidth, index)
				tree := model.browser.renderTree(model.styles, treeWidth, treeRows)
				if tree != strings.Join(treeProjection.lines, "\n") {
					t.Fatalf("run %d node %d %s: Tree render differs from shared viewport projection", run+1, index, size.name)
				}
				assertSC007Projection(t, size.name+" Tree", treeProjection, treeContent, treeWidth)
				assertSC007ProjectionOverflow(t, size.name+" Tree", tree, treeProjection)
				assertSC007EveryVisibleTreeTruncation(t, model.browser, treeProjection, treeWidth, run+1, index, size.name)
				selectedLine := sc007SelectedLine(tree)
				if selectedLine == "" {
					t.Fatalf("run %d node %d %s: focused node %q is not visible:\n%s", run+1, index, size.name, selected.id, view)
				}
				rawWidth := 2 + selected.depth*2 + len(sc007NodeMarker(snapshot.nodes[selected.id], selected)) + 1 + ansi.StringWidth(snapshot.nodes[selected.id].node.Name)
				if rawWidth > layout.tree.contentWidth() && !strings.HasSuffix(selectedLine, safeTextEllipsis) {
					t.Fatalf("run %d node %d %s: truncated selected row lacks ellipsis: %q", run+1, index, size.name, selectedLine)
				}

				detailWidth, detailRows := layout.details.contentWidth(), layout.details.contentHeight()
				detailContent := model.detailState.content(detailWidth)
				detailProjection := model.detailState.project(detailRows, detailWidth)
				assertSC007Projection(t, size.name+" Details", detailProjection, detailContent, detailWidth)
				assertSC007ProjectionOverflow(t, size.name+" Details", strings.Join(detailProjection.lines, "\n"), detailProjection)
			}
		}
	}
	t.Logf("SC-007 full matrix: %d nodes x %d exact sizes x %d runs = %d frames", len(model.browser.rows), len(us5ContractSizes), scConformanceRuns, len(model.browser.rows)*len(us5ContractSizes)*scConformanceRuns)
}

func TestSC007AcceptanceIncludesClosedSurfaceTraversalMatrix(t *testing.T) {
	surfaces := sc007ClosedSurfaces(t)
	if len(surfaces) != 7 || sc007ClosedTraversalRuns != 20 {
		t.Fatalf("acceptance closed traversal matrix = %d surfaces x %d runs, want 7 x 20", len(surfaces), sc007ClosedTraversalRuns)
	}
	for _, surface := range surfaces {
		initial := surface.project(0)
		maximum := viewportMaximumOffset(initial.contentLength, initial.availableRows)
		for _, offset := range []int{0, (maximum + 1) / 2, maximum} {
			assertSC007RenderedScrollbar(t, surface.name, surface.project(offset))
		}
	}
}

func assertSC007EveryVisibleTreeTruncation(t *testing.T, browser browserModel, projection viewportProjection, width, run, selectedIndex int, size string) {
	t.Helper()
	contentIndex := projection.renderOffset
	for _, line := range projection.lines {
		if contentIndex >= len(browser.rows) {
			break
		}
		row := browser.rows[contentIndex]
		node := browser.snapshot.nodes[row.id]
		name := node.node.Name
		if node.root {
			name = "/"
		}
		rawWidth := 2 + row.depth*2 + len(sc007NodeMarker(node, row)) + 1 + ansi.StringWidth(name)
		if rawWidth > width && !strings.HasSuffix(line, safeTextEllipsis) {
			t.Fatalf("run %d selected %d visible node %d %s: truncated Tree line lacks ellipsis: %q", run, selectedIndex, contentIndex, size, line)
		}
		contentIndex++
	}
}

func sc007TreeContent(browser browserModel, style styles, width int) []string {
	content := make([]string, len(browser.rows))
	for index, row := range browser.rows {
		node := browser.snapshot.nodes[row.id]
		selector := "  "
		if row.id == browser.selectedID {
			selector = "> "
		}
		marker, name := sc007NodeMarker(node, row), node.node.Name
		if node.root {
			name = "/"
		}
		prefix := selector + strings.Repeat("  ", row.depth) + marker + " "
		line := prefix + safeText(name, width-len(prefix))
		if row.id == browser.selectedID {
			line = style.selected.Render(line)
		}
		content[index] = line
	}
	return content
}

func assertSC007ProjectionOverflow(t *testing.T, name, rendered string, projection viewportProjection) {
	t.Helper()
	if strings.Contains(rendered, viewportPreviousLabel) || strings.Contains(rendered, viewportNextLabel) {
		t.Fatalf("%s rendered legacy directional marker: %q", name, rendered)
	}
	if projection.scrollbar.visible != (projection.hasPrevious || projection.hasNext) {
		t.Fatalf("%s scrollbar visibility %v disagrees with overflow (%v,%v): %#v", name, projection.scrollbar.visible, projection.hasPrevious, projection.hasNext, projection.scrollbar)
	}
}

func sc007SelectedLine(tree string) string {
	for _, line := range strings.Split(tree, "\n") {
		if strings.HasPrefix(line, "> ") {
			return line
		}
	}
	return ""
}

func sc007NodeMarker(node treeNode, row treeRow) string {
	switch {
	case node.root:
		return "[/]"
	case node.folder != nil && row.expanded:
		return "[-]"
	case node.folder != nil:
		return "[+]"
	default:
		return "[ssh]"
	}
}
