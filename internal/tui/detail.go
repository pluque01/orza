package tui

import (
	"fmt"
	"sort"

	"github.com/pluque01/orza/internal/app"
)

const detailEmptyConnections = "No direct connections."

type detailKind string

const (
	detailKindRoot       detailKind = "root"
	detailKindFolder     detailKind = "folder"
	detailKindConnection detailKind = "connection"
)

type detailField struct {
	label string
	value string
}

type detailConnection struct {
	id       app.NodeID
	name     string
	endpoint string
}

type detailState struct {
	targetID          app.NodeID
	kind              detailKind
	heading           string
	path              string
	fields            []detailField
	directConnections []detailConnection
	viewport          viewportState
}

func newDetailState(snapshot catalogSnapshot, targetID app.NodeID) (detailState, bool) {
	return detailState{}.withTarget(snapshot, targetID)
}

// withTarget rebuilds public detail content from the snapshot. Scrolling is
// retained across a refresh of the same stable target and reset on selection.
func (state detailState) withTarget(snapshot catalogSnapshot, targetID app.NodeID) (detailState, bool) {
	node, exists := snapshot.nodes[targetID]
	if !exists {
		return detailState{}, false
	}

	viewport := state.viewport
	if state.targetID != targetID {
		viewport = newViewportState(0)
	}
	next := detailState{
		targetID: targetID,
		path:     detailSafeValue(node.node.Path),
		viewport: viewport,
	}

	switch {
	case node.root:
		next.kind = detailKindRoot
		next.heading = "Root"
	case node.folder != nil:
		next.kind = detailKindFolder
		next.heading = "Folder"
	case node.connection != nil:
		next.kind = detailKindConnection
		next.heading = "Connection"
	default:
		return detailState{}, false
	}

	next.fields = []detailField{
		{label: "Kind", value: next.heading},
		{label: "Name", value: detailSafeValue(node.node.Name)},
		{label: "Path", value: next.path},
	}
	if node.connection != nil {
		connection := node.connection
		user := connection.Username
		if user == "" {
			user = "(default)"
		}
		next.fields = append(next.fields,
			detailField{label: "Endpoint", value: detailSafeValue(fmt.Sprintf("%s:%d", connection.Host, connection.Port))},
			detailField{label: "User", value: detailSafeValue(user)},
			detailField{label: "Method", value: detailSafeValue(string(connection.AuthMethod))},
		)
		if connection.IdentityFile != "" {
			next.fields = append(next.fields, detailField{label: "Identity", value: detailSafeValue(connection.IdentityFile)})
		}
		return next, true
	}

	type sortableConnection struct {
		id         app.NodeID
		name       string
		connection *app.Connection
	}
	children := snapshot.children[targetID]
	direct := make([]sortableConnection, 0, len(children))
	for _, childID := range children {
		child, childExists := snapshot.nodes[childID]
		if !childExists || child.connection == nil {
			continue
		}
		direct = append(direct, sortableConnection{id: childID, name: child.node.Name, connection: child.connection})
	}
	sort.Slice(direct, func(i, j int) bool {
		if direct[i].name != direct[j].name {
			return direct[i].name < direct[j].name
		}
		return direct[i].id < direct[j].id
	})
	next.directConnections = make([]detailConnection, len(direct))
	for index, child := range direct {
		next.directConnections[index] = detailConnection{
			id:       child.id,
			name:     detailSafeValue(child.name),
			endpoint: detailSafeValue(fmt.Sprintf("%s:%d", child.connection.Host, child.connection.Port)),
		}
	}
	next.fields = append(next.fields, detailField{label: "Direct connections", value: fmt.Sprintf("%d", len(next.directConnections))})
	return next, true
}

func (state detailState) withOffset(offset int) detailState {
	state.viewport = state.viewport.withOffset(offset)
	return state
}

func (state detailState) content(width int) []string {
	lines := make([]string, 0, len(state.fields)+max(1, len(state.directConnections)))
	for _, field := range state.fields {
		lines = append(lines, safeText(field.label+": "+field.value, width))
	}
	if state.kind == detailKindConnection {
		return lines
	}
	if len(state.directConnections) == 0 {
		return append(lines, safeText(detailEmptyConnections, width))
	}
	for _, connection := range state.directConnections {
		lines = append(lines, safeText(connection.name+": "+connection.endpoint, width))
	}
	return lines
}

func (state detailState) project(rows, width int) viewportProjection {
	return state.viewport.project(state.content(width), rows, width, noActiveLine)
}

func detailSafeValue(value string) string {
	return safeText(value, int(^uint(0)>>1))
}
