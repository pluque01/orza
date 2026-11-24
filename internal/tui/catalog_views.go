package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

type catalogView int

const (
	catalogTree catalogView = iota
	catalogFavourites
	catalogRecent
	catalogViewCount
	recentLimit = 10
)

var catalogViewNames = [catalogViewCount]string{"Tree", "Favourites", "Recent"}

const catalogRefreshFailed = "Catalog refresh failed. Press r to retry."

type catalogPosition struct {
	selectedID app.NodeID
	viewport   viewportState
}

type catalogViewRefreshedMsg struct {
	refreshID   uint64
	operationID uint64
	snapshot    *catalogSnapshot
	list        app.ListConnectionsResult
	err         error
}

func (m *Model) refreshCatalogView() tea.Cmd {
	m.catalogViewRefresh++
	if m.browser.view == catalogTree || m.folders == nil && m.connections == nil {
		return nil
	}
	refreshID, operationID := m.catalogViewRefresh, m.operationID
	return func() tea.Msg {
		snapshot, list, err := m.readCatalog(m.ctx)
		return catalogViewRefreshedMsg{refreshID: refreshID, operationID: operationID, snapshot: snapshot, list: list, err: err}
	}
}

func (m *Model) handleCatalogViewRefresh(msg catalogViewRefreshedMsg) {
	if msg.refreshID != m.catalogViewRefresh || msg.operationID != m.operationID || m.screen != screenBrowser || m.modal.isOpen() {
		return
	}
	if msg.err != nil {
		m.status = catalogRefreshFailed
		return
	}
	if msg.snapshot != nil {
		m.browser.setSnapshot(*msg.snapshot, "")
	} else {
		m.browser.setConnectionsPending(msg.list, "")
	}
	m.ownedSelectionID = m.browser.selectedID
	m.syncDetail()
	if m.status == catalogRefreshFailed {
		m.status = "READY"
	}
}

func (b *browserModel) switchView(delta catalogView) {
	b.positions[b.view] = catalogPosition{b.selectedID, b.viewport}
	b.view = (b.view + delta + catalogViewCount) % catalogViewCount
	position := b.positions[b.view]
	b.selectedID, b.viewport = position.selectedID, position.viewport
	b.rebuildRows()
}

func (b *browserModel) rebuildList() {
	index := b.selectedIndex()
	b.rows = b.rows[:0]
	for id, node := range b.snapshot.nodes {
		connection := node.connection
		if connection == nil {
			continue
		}
		if b.view == catalogFavourites && connection.Favourite || b.view == catalogRecent && !connection.LastOpenedAt.IsZero() {
			b.rows = append(b.rows, treeRow{id: id})
		}
	}
	slices.SortFunc(b.rows, func(a, c treeRow) int {
		left, right := b.snapshot.nodes[a.id].connection, b.snapshot.nodes[c.id].connection
		if b.view == catalogRecent {
			if order := right.LastOpenedAt.Compare(left.LastOpenedAt); order != 0 {
				return order
			}
		}
		return strings.Compare(left.Path, right.Path)
	})

	if b.view == catalogRecent && len(b.rows) > recentLimit {
		b.rows = b.rows[:recentLimit]
	}
	if len(b.rows) == 0 {
		b.selectedID = ""
	} else if b.selectedID == "" || !b.visible(b.selectedID) {
		b.selectedID = b.rows[min(index, len(b.rows)-1)].id
	}
}

func (b *browserModel) projectList(style styles, width, rows int) viewportProjection {
	if len(b.rows) == 0 {
		message := "No favourites yet."
		if b.view == catalogRecent {
			message = "No recent sessions yet."
		}
		return b.viewport.project([]string{message}, rows, width, 0)
	}
	content := make([]string, len(b.rows))
	for i, row := range b.rows {
		path := safeText(b.snapshot.nodes[row.id].node.Path, width-2)
		if row.id == b.selectedID {
			content[i] = style.selected.Render("> " + path)
		} else {
			content[i] = "  " + path
		}
	}
	return b.viewport.project(content, rows, width, b.selectedIndex())
}

func (s styles) catalogTitle(view catalogView, focused bool, width int) string {
	focus := s.inactiveTitle.Render("[ ]")
	if focused {
		focus = s.activeTitle.Render("[*]")
	}
	names := catalogViewNames
	activeName := s.activeTitle.Render("[" + names[view] + "]")
	names[view] = activeName
	for _, separator := range []string{" - ", "-"} {
		title := focus + " " + strings.Join(names[:], separator)
		if ansi.StringWidth(title) <= width {
			return title
		}
	}
	return focus + " " + activeName
}
