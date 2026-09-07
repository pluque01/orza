package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestBrowserOrderingIndentationMarkersAndInitialRoot(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	snapshot := newCatalogSnapshot(root, 9)
	folderUpper := testFolder("folder-b", root.ID, "/Beta", 1)
	folderLowerID2 := testFolder("folder-2", root.ID, "/alpha", 1)
	folderLowerID1 := testFolder("folder-1", root.ID, "/alpha", 1)
	connectionB := testConnection("connection-b", root.ID, "/zeta", 1)
	connectionA := testConnection("connection-a", root.ID, "/Alpha", 1)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{
		Folders:     []app.Folder{folderLowerID2, folderUpper, folderLowerID1},
		Connections: []app.Connection{connectionB, connectionA},
	}) {
		t.Fatal("fixture snapshot rejected")
	}
	child := testFolder("child", folderLowerID1.ID, "/alpha/child", 1)
	if !snapshot.addChildren(folderLowerID1.ID, app.ListChildrenResult{Folders: []app.Folder{child}}) {
		t.Fatal("nested fixture rejected")
	}

	var browser browserModel
	browser.setSnapshot(snapshot, "")
	if browser.selectedID != root.ID || len(browser.rows) != 6 {
		t.Fatalf("initial selection/rows = %q/%d, want root/6", browser.selectedID, len(browser.rows))
	}
	want := []app.NodeID{root.ID, folderUpper.ID, folderLowerID1.ID, folderLowerID2.ID, connectionA.ID, connectionB.ID}
	for index, id := range want {
		if browser.rows[index].id != id {
			t.Fatalf("row %d = %q, want %q", index, browser.rows[index].id, id)
		}
	}

	browser.selectedID = folderLowerID1.ID
	browser.toggle()
	view := browser.renderTree(newStyles(true), 80, len(browser.rows))
	for _, line := range []string{">   [-] alpha", "      [+] child", "  [/] /", "    [ssh] Alpha"} {
		if !strings.Contains(view, line) {
			t.Fatalf("tree view omitted fixed marker/indentation %q:\n%s", line, view)
		}
	}
}

func TestBrowserEmptyRootCollapseFallbackPruningAndNonWrapping(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	empty := newCatalogSnapshot(root, 1)
	var browser browserModel
	browser.setSnapshot(empty, "")
	if len(browser.rows) != 1 || browser.selectedID != root.ID {
		t.Fatalf("empty tree = %#v selected %q", browser.rows, browser.selectedID)
	}
	browser.move(-1)
	browser.move(1)
	if browser.selectedID != root.ID {
		t.Fatal("single-row navigation wrapped")
	}

	parent := testFolder("parent", root.ID, "/parent", 1)
	child := testConnection("child", parent.ID, "/parent/child", 1)
	nested := newCatalogSnapshot(root, 2)
	_ = nested.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{parent}})
	_ = nested.addChildren(parent.ID, app.ListChildrenResult{Connections: []app.Connection{child}})
	browser.setSnapshot(nested, "")
	browser.selectedID = parent.ID
	browser.toggle()
	browser.right()
	if browser.selectedID != child.ID {
		t.Fatalf("right selected %q, want child", browser.selectedID)
	}
	browser.left()
	if browser.selectedID != parent.ID {
		t.Fatalf("left selected %q, want parent", browser.selectedID)
	}
	browser.right()
	browser.collapse(parent.ID)
	if browser.selectedID != parent.ID || browser.visible(child.ID) {
		t.Fatalf("collapse did not move selection/hide child: selected %q", browser.selectedID)
	}

	browser.expanded[parent.ID] = struct{}{}
	browser.setSnapshot(empty, "")
	if _, stale := browser.expanded[parent.ID]; stale || browser.selectedID != root.ID {
		t.Fatalf("reload retained stale expansion/selection: %#v %q", browser.expanded, browser.selectedID)
	}
}

func TestBrowserNearestParentFallbackAndPendingSelection(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	parent := testFolder("parent", root.ID, "/parent", 1)
	child := testConnection("child", parent.ID, "/parent/child", 1)
	before := newCatalogSnapshot(root, 1)
	_ = before.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{parent}})
	_ = before.addChildren(parent.ID, app.ListChildrenResult{Connections: []app.Connection{child}})
	var browser browserModel
	browser.setSnapshot(before, "")
	browser.selectedID = child.ID
	browser.expandAncestors(child.ID)
	browser.rebuildRows()

	afterDelete := newCatalogSnapshot(root, 2)
	_ = afterDelete.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{parent}})
	browser.setSnapshot(afterDelete, "")
	if browser.selectedID != parent.ID {
		t.Fatalf("deleted selection fallback = %q, want parent", browser.selectedID)
	}

	recreated := testConnection("new", parent.ID, "/parent/new", 1)
	afterCreate := newCatalogSnapshot(root, 3)
	_ = afterCreate.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{parent}})
	_ = afterCreate.addChildren(parent.ID, app.ListChildrenResult{Connections: []app.Connection{recreated}})
	browser.setSnapshot(afterCreate, recreated.ID)
	if browser.selectedID != recreated.ID || !browser.visible(recreated.ID) {
		t.Fatalf("pending selection = %q visible %v", browser.selectedID, browser.visible(recreated.ID))
	}
}

func TestBrowserTreeViewportKeepsSelectionVisibleWithScrollbarMetadata(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	snapshot := newCatalogSnapshot(root, 1)
	connections := make([]app.Connection, 5)
	for index, name := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
		connections[index] = testConnection(name, root.ID, "/"+name, 1)
	}
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: connections}) {
		t.Fatal("fixture snapshot rejected")
	}

	var browser browserModel
	browser.setSnapshot(snapshot, "")
	browser.selectedID = connections[2].ID
	selectedID := browser.selectedID
	projection := browser.projectTree(newStyles(true), 30, 3)
	lines := projection.lines
	if len(lines) != 3 || !projection.hasPrevious || !projection.hasNext || !projection.scrollbar.visible {
		t.Fatalf("bounded Tree projection = %#v, overflow=(%v,%v), scrollbar=%#v", lines, projection.hasPrevious, projection.hasNext, projection.scrollbar)
	}
	if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, ">   [ssh] charlie") }) {
		t.Fatalf("selected row is not visible: %#v", lines)
	}
	if strings.Contains(strings.Join(lines, "\n"), viewportPreviousLabel) || strings.Contains(strings.Join(lines, "\n"), viewportNextLabel) {
		t.Fatalf("Tree projection rendered legacy markers: %#v", lines)
	}
	if browser.selectedID != selectedID {
		t.Fatalf("render changed stable selection from %q to %q", selectedID, browser.selectedID)
	}
}

func TestBrowserTreeViewportSafelyEllipsizesLongDeepUnicodeRow(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	snapshot := newCatalogSnapshot(root, 1)
	parent := root.ID
	folders := make([]app.Folder, 4)
	path := ""
	for index := range folders {
		path += "/level"
		folders[index] = testFolder(string(rune('a'+index)), parent, path, 1)
		if !snapshot.addChildren(parent, app.ListChildrenResult{Folders: []app.Folder{folders[index]}}) {
			t.Fatal("nested fixture rejected")
		}
		parent = folders[index].ID
	}
	leaf := testConnection("leaf", parent, path+"/leaf", 1)
	leaf.Name = "東京\n駅界界界界界界"
	if !snapshot.addChildren(parent, app.ListChildrenResult{Connections: []app.Connection{leaf}}) {
		t.Fatal("leaf fixture rejected")
	}

	var browser browserModel
	browser.setSnapshot(snapshot, "")
	for _, folder := range folders {
		browser.selectedID = folder.ID
		browser.toggle()
	}
	browser.selectedID = leaf.ID
	projection := browser.projectTree(newStyles(true), 24, 2)
	lines := projection.lines
	if len(lines) != 2 || !projection.hasPrevious || !projection.scrollbar.visible {
		t.Fatalf("deep Tree projection = %#v, overflow=(%v,%v), scrollbar=%#v", lines, projection.hasPrevious, projection.hasNext, projection.scrollbar)
	}
	selected := lines[len(lines)-1]
	if !strings.Contains(selected, "東京") || strings.Contains(selected, "\n") || !strings.HasSuffix(selected, safeTextEllipsis) {
		t.Fatalf("deep Unicode row was not safely ellipsized: %q", selected)
	}
	if width := ansi.StringWidth(selected); width > 24 {
		t.Fatalf("deep Unicode row width = %d, want <= 24: %q", width, selected)
	}
	assertTerminalSafe(t, selected, 24)
	if got := browser.snapshot.nodes[leaf.ID].node.Name; got != leaf.Name {
		t.Fatalf("render changed source name from %q to %q", leaf.Name, got)
	}
}

func TestBrowserSnapshotPreservesExpansionAndStableSelection(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("folder", root.ID, "/folder", 1)
	child := testConnection("child", folder.ID, "/folder/child", 1)
	makeSnapshot := func(revision app.CatalogRevision) catalogSnapshot {
		snapshot := newCatalogSnapshot(root, revision)
		_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}})
		_ = snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{child}})
		return snapshot
	}

	var browser browserModel
	browser.setSnapshot(makeSnapshot(1), "")
	browser.selectedID = folder.ID
	browser.toggle()
	browser.setSnapshot(makeSnapshot(2), "")
	if browser.selectedID != folder.ID || !browser.visible(child.ID) {
		t.Fatalf("snapshot replacement selected %q, child visible %v", browser.selectedID, browser.visible(child.ID))
	}
	if _, expanded := browser.expanded[folder.ID]; !expanded {
		t.Fatal("snapshot replacement lost folder expansion")
	}
	if view := browser.renderTree(newStyles(true), 40, 4); !strings.Contains(view, "[-] folder") || !strings.Contains(view, "[ssh] child") {
		t.Fatalf("preserved expansion was not rendered:\n%s", view)
	}
}

func TestBrowserSnapshotFallsBackToNearestCapturedExistingAncestor(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	grandparent := testFolder("grandparent", root.ID, "/grandparent", 1)
	parent := testFolder("parent", grandparent.ID, "/grandparent/parent", 1)
	child := testConnection("child", parent.ID, "/grandparent/parent/child", 1)
	before := newCatalogSnapshot(root, 1)
	_ = before.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{grandparent}})
	_ = before.addChildren(grandparent.ID, app.ListChildrenResult{Folders: []app.Folder{parent}})
	_ = before.addChildren(parent.ID, app.ListChildrenResult{Connections: []app.Connection{child}})

	var browser browserModel
	browser.setSnapshot(before, "")
	browser.selectedID = child.ID
	browser.expandAncestors(child.ID)
	browser.rebuildRows()

	after := newCatalogSnapshot(root, 2)
	_ = after.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{grandparent}})
	browser.setSnapshot(after, "")
	if browser.selectedID != grandparent.ID {
		t.Fatalf("fallback selection = %q, want nearest existing ancestor %q", browser.selectedID, grandparent.ID)
	}
	if got := browser.nearestExistingAncestor([]app.NodeID{"missing"}); got != root.ID {
		t.Fatalf("root fallback = %q, want %q", got, root.ID)
	}
}

func testFolder(id string, parent app.NodeID, path string, revision app.Revision) app.Folder {
	name := strings.TrimPrefix(path, "/")
	if path == "/" {
		name = "/"
	} else if split := strings.LastIndex(path, "/"); split >= 0 {
		name = path[split+1:]
	}
	return app.Folder{Node: app.Node{ID: app.NodeID(id), ParentID: parent, Kind: app.NodeKindFolder, Name: name, Path: path, Revision: revision}}
}

func testConnection(id string, parent app.NodeID, path string, revision app.Revision) app.Connection {
	return app.Connection{Node: app.Node{ID: app.NodeID(id), ParentID: parent, Kind: app.NodeKindConnection, Name: strings.TrimPrefix(path[strings.LastIndex(path, "/"):], "/"), Path: path, Revision: revision}, Host: "host.test", Port: 22, AuthMethod: app.AuthMethodAgent}
}
