package tui

import (
	"sort"
	"strings"

	"github.com/pluque01/orza/internal/app"
)

const syntheticRootID app.NodeID = "__tui_root__"

type treeNode struct {
	node       app.Node
	folder     *app.Folder
	connection *app.Connection
	root       bool
}

type catalogSnapshot struct {
	revision app.CatalogRevision
	rootID   app.NodeID
	nodes    map[app.NodeID]treeNode
	children map[app.NodeID][]app.NodeID
	parents  map[app.NodeID]app.NodeID
}

func newCatalogSnapshot(root app.Folder, revision app.CatalogRevision) catalogSnapshot {
	if root.ID == "" {
		root.ID = syntheticRootID
	}
	if root.Path == "" {
		root.Path = "/"
	}
	if root.Name == "" || root.Name == "." {
		root.Name = "/"
	}
	root.Kind = app.NodeKindFolder
	copy := root
	return catalogSnapshot{
		revision: revision,
		rootID:   root.ID,
		nodes:    map[app.NodeID]treeNode{root.ID: {node: root.Node, folder: &copy, root: true}},
		children: make(map[app.NodeID][]app.NodeID),
		parents:  make(map[app.NodeID]app.NodeID),
	}
}

func (s *catalogSnapshot) addChildren(parent app.NodeID, result app.ListChildrenResult) bool {
	if _, exists := s.nodes[parent]; !exists {
		return false
	}
	ids := make([]app.NodeID, 0, len(result.Folders)+len(result.Connections))
	for _, folder := range result.Folders {
		if folder.ID == "" {
			return false
		}
		if _, duplicate := s.nodes[folder.ID]; duplicate {
			return false
		}
		folder.ParentID = parent
		folder.Kind = app.NodeKindFolder
		copy := folder
		s.nodes[folder.ID] = treeNode{node: folder.Node, folder: &copy}
		s.parents[folder.ID] = parent
		ids = append(ids, folder.ID)
	}
	for _, connection := range result.Connections {
		if connection.ID == "" {
			return false
		}
		if _, duplicate := s.nodes[connection.ID]; duplicate {
			return false
		}
		connection.ParentID = parent
		connection.Kind = app.NodeKindConnection
		copy := connection
		s.nodes[connection.ID] = treeNode{node: connection.Node, connection: &copy}
		s.parents[connection.ID] = parent
		ids = append(ids, connection.ID)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := s.nodes[ids[i]], s.nodes[ids[j]]
		if left.node.Kind != right.node.Kind {
			return left.node.Kind == app.NodeKindFolder
		}
		if left.node.Name != right.node.Name {
			return left.node.Name < right.node.Name
		}
		return left.node.ID < right.node.ID
	})
	s.children[parent] = ids
	return true
}

type treeRow struct {
	id          app.NodeID
	depth       int
	expanded    bool
	hasChildren bool
}

type browserModel struct {
	snapshot    catalogSnapshot
	expanded    map[app.NodeID]struct{}
	selectedID  app.NodeID
	rows        []treeRow
	viewport    viewportState
	revision    app.CatalogRevision
	changed     bool
	initialized bool
}

func (b *browserModel) setConnectionsPending(result app.ListConnectionsResult, pending app.NodeID) {
	root := rootFolder()
	root.ID = syntheticRootID
	snapshot := newCatalogSnapshot(root, result.CatalogRevision)
	_ = snapshot.addChildren(snapshot.rootID, app.ListChildrenResult{Connections: result.Connections})
	b.setSnapshot(snapshot, pending)
}

func (b *browserModel) setSnapshot(snapshot catalogSnapshot, pending app.NodeID) {
	previousSelection := b.selectedID
	ancestors := b.selectionAncestors()
	previousExpanded := b.expanded

	b.snapshot = snapshot
	b.revision = snapshot.revision
	b.expanded = make(map[app.NodeID]struct{})
	for id := range previousExpanded {
		if node, exists := snapshot.nodes[id]; exists && node.folder != nil {
			b.expanded[id] = struct{}{}
		}
	}
	b.expanded[snapshot.rootID] = struct{}{}

	selected := snapshot.rootID
	if b.initialized {
		switch {
		case pending != "" && b.hasNode(pending):
			selected = pending
		case previousSelection != "" && b.hasNode(previousSelection):
			selected = previousSelection
		default:
			selected = b.nearestExistingAncestor(ancestors)
		}
	}
	b.initialized = true
	b.selectedID = selected
	b.expandAncestors(selected)
	b.rebuildRows()
	b.changed = false
}

func (b *browserModel) hasNode(id app.NodeID) bool {
	_, exists := b.snapshot.nodes[id]
	return exists
}

func (b *browserModel) selectionAncestors() []app.NodeID {
	if b.selectedID == "" || len(b.snapshot.nodes) == 0 {
		return nil
	}
	ancestors := make([]app.NodeID, 0, 8)
	for id := b.snapshot.parents[b.selectedID]; id != ""; id = b.snapshot.parents[id] {
		ancestors = append(ancestors, id)
	}
	return ancestors
}

func (b *browserModel) nearestExistingAncestor(ancestors []app.NodeID) app.NodeID {
	for _, id := range ancestors {
		if b.hasNode(id) {
			return id
		}
	}
	return b.snapshot.rootID
}

func (b *browserModel) expandAncestors(id app.NodeID) {
	for parent := b.snapshot.parents[id]; parent != ""; parent = b.snapshot.parents[parent] {
		b.expanded[parent] = struct{}{}
	}
}

func (b *browserModel) rebuildRows() {
	b.rows = b.rows[:0]
	if b.snapshot.rootID == "" {
		return
	}
	var appendNode func(app.NodeID, int)
	appendNode = func(id app.NodeID, depth int) {
		_, expanded := b.expanded[id]
		children := b.snapshot.children[id]
		b.rows = append(b.rows, treeRow{id: id, depth: depth, expanded: expanded, hasChildren: len(children) != 0})
		node := b.snapshot.nodes[id]
		if node.folder == nil || !expanded {
			return
		}
		for _, child := range children {
			appendNode(child, depth+1)
		}
	}
	appendNode(b.snapshot.rootID, 0)
	if !b.visible(b.selectedID) {
		b.selectedID = b.snapshot.rootID
	}
}

func (b *browserModel) visible(id app.NodeID) bool {
	for _, row := range b.rows {
		if row.id == id {
			return true
		}
	}
	return false
}

func (b *browserModel) selectedIndex() int {
	for index, row := range b.rows {
		if row.id == b.selectedID {
			return index
		}
	}
	return 0
}

func (b *browserModel) move(delta int) {
	if len(b.rows) == 0 {
		return
	}
	index := b.selectedIndex() + delta
	if index < 0 {
		index = 0
	}
	if index >= len(b.rows) {
		index = len(b.rows) - 1
	}
	b.selectedID = b.rows[index].id
}

func (b *browserModel) home() {
	if len(b.rows) != 0 {
		b.selectedID = b.rows[0].id
	}
}

func (b *browserModel) end() {
	if len(b.rows) != 0 {
		b.selectedID = b.rows[len(b.rows)-1].id
	}
}

func (b *browserModel) toggle() {
	node := b.selectedTreeNode()
	if node == nil || node.folder == nil || node.root {
		return
	}
	if _, expanded := b.expanded[node.node.ID]; expanded {
		b.collapse(node.node.ID)
	} else {
		b.expanded[node.node.ID] = struct{}{}
		b.rebuildRows()
	}
}

func (b *browserModel) right() {
	node := b.selectedTreeNode()
	if node == nil || node.folder == nil {
		return
	}
	if _, expanded := b.expanded[node.node.ID]; !expanded {
		b.expanded[node.node.ID] = struct{}{}
		b.rebuildRows()
		return
	}
	if children := b.snapshot.children[node.node.ID]; len(children) != 0 {
		b.selectedID = children[0]
	}
}

func (b *browserModel) left() {
	node := b.selectedTreeNode()
	if node == nil {
		return
	}
	if node.folder != nil && !node.root {
		if _, expanded := b.expanded[node.node.ID]; expanded {
			b.collapse(node.node.ID)
			return
		}
	}
	if parent := b.snapshot.parents[node.node.ID]; parent != "" {
		b.selectedID = parent
	}
}

func (b *browserModel) collapse(id app.NodeID) {
	node, exists := b.snapshot.nodes[id]
	if !exists || node.folder == nil || node.root {
		return
	}
	if b.descendantOf(b.selectedID, id) {
		b.selectedID = id
	}
	delete(b.expanded, id)
	b.rebuildRows()
}

func (b *browserModel) descendantOf(id, ancestor app.NodeID) bool {
	for parent := b.snapshot.parents[id]; parent != ""; parent = b.snapshot.parents[parent] {
		if parent == ancestor {
			return true
		}
	}
	return false
}

func (b *browserModel) selectedTreeNode() *treeNode {
	node, exists := b.snapshot.nodes[b.selectedID]
	if !exists {
		return nil
	}
	return &node
}

func (b *browserModel) selectionNode() *app.Node {
	selected := b.selectedTreeNode()
	if selected == nil {
		return nil
	}
	node := selected.node
	return &node
}

func (b *browserModel) selectionFolder() *app.Folder {
	selected := b.selectedTreeNode()
	if selected == nil || selected.folder == nil || selected.root {
		return nil
	}
	copy := *selected.folder
	return &copy
}

func (b *browserModel) selection() *app.Connection {
	selected := b.selectedTreeNode()
	if selected == nil || selected.connection == nil {
		return nil
	}
	copy := *selected.connection
	return &copy
}

func (b *browserModel) destinationFolder() *app.Folder {
	selected := b.selectedTreeNode()
	if selected == nil {
		return nil
	}
	if selected.folder != nil {
		copy := *selected.folder
		return &copy
	}
	parent := b.snapshot.nodes[b.snapshot.parents[selected.node.ID]]
	if parent.folder == nil {
		return nil
	}
	copy := *parent.folder
	return &copy
}

func (b *browserModel) folders() []app.Folder {
	folders := make([]app.Folder, 0)
	for _, node := range b.snapshot.nodes {
		if node.folder != nil {
			folders = append(folders, *node.folder)
		}
	}
	sort.Slice(folders, func(i, j int) bool {
		if folders[i].Path != folders[j].Path {
			return folders[i].Path < folders[j].Path
		}
		return folders[i].ID < folders[j].ID
	})
	return folders
}

// renderTree projects only the Tree panel content. Its output is bounded by
// display cells and rows so a later layout compositor can place it safely.
func (b browserModel) renderTree(style styles, width, rows int) string {
	return strings.Join(b.projectTree(style, width, rows).lines, "\n")
}

func (b browserModel) projectTree(style styles, width, rows int) viewportProjection {
	content := make([]string, len(b.rows))
	for index, row := range b.rows {
		node := b.snapshot.nodes[row.id]
		selector := "  "
		if row.id == b.selectedID {
			selector = "> "
		}
		marker := "[ssh]"
		name := node.node.Name
		switch {
		case node.root:
			marker, name = "[/]", "/"
		case node.folder != nil && row.expanded:
			marker = "[-]"
		case node.folder != nil:
			marker = "[+]"
		}
		prefix := selector + strings.Repeat("  ", row.depth) + marker + " "
		line := prefix + safeText(name, width-len(prefix))
		if row.id == b.selectedID {
			line = style.selected.Render(line)
		}
		content[index] = line
	}

	return b.viewport.project(content, rows, width, b.selectedIndex())
}
func rootFolder() app.Folder {
	return app.Folder{Node: app.Node{ID: syntheticRootID, Kind: app.NodeKindFolder, Name: "/", Path: "/", Revision: 1}}
}
