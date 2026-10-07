package tui

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

type tunnelPollMsg struct{}
type tunnelQuitReadyMsg struct{}
type tunnelStartedMsg struct {
	token    uint64
	snapshot app.TunnelSnapshot
	err      error
}
type tunnelActionMsg struct{ err error }
type tunnelRetryResolvedMsg struct {
	token      uint64
	snapshot   app.TunnelSnapshot
	connection app.Connection
	err        error
}
type tunnelStartup struct {
	runtime  *sessionRuntime
	snapshot app.TunnelSnapshot
	cancel   context.CancelFunc
	stopping bool
}

type tunnelModalPayload struct {
	action     string
	snapshot   app.TunnelSnapshot
	live       []app.TunnelSnapshot
	connection app.Connection
	config     app.TunnelConfig
	quit       bool
	previous   *modalState
}

func (p tunnelModalPayload) validModalPayload() bool { return p.action != "" }

func tunnelFields(s app.TunnelSnapshot) []displayField {
	listener := "Requested listener"
	fields := []displayField{{label: "Tunnel", value: fmt.Sprintf("%d / %s", s.ID, s.State)}, {label: "Target", value: s.Connection.Path}, {label: "SSH endpoint", value: net.JoinHostPort(s.Connection.Host, strconv.Itoa(int(s.Connection.Port)))}, {label: "Mode", value: string(s.Config.Mode)}, {label: listener, value: s.Config.Listen.String()}}
	if s.Config.Mode != app.TunnelDynamic {
		fields = append(fields, displayField{label: "Destination", value: s.Config.Destination.String()})
	}
	if s.Scope != "" {
		fields = append(fields, displayField{label: "Scope", value: s.Scope})
	}
	if s.Config.Mode == app.TunnelRemote {
		fields = append(fields, displayField{label: "Warning", value: "Actual remote listening scope is unverified and depends on server configuration, even for loopback. Server release after transport loss is not verified."})
	}
	if s.Config.Mode == app.TunnelDynamic {
		fields = append(fields, displayField{label: "Proxy", value: "SOCKS5 proxy; clients are not authenticated."})
	}
	if s.Warning != "" {
		fields = append(fields, displayField{label: "Warning", value: s.Warning})
	}
	if s.Diagnostic != "" {
		fields = append(fields, displayField{label: "Diagnostic", value: s.Diagnostic})
	}
	return fields
}

func (p tunnelModalPayload) lines(style styles, width int) []string {
	var lines []string
	switch p.action {
	case "discard":
		return []string{"Discard forwarding draft?", modalControlLine("d Discard"), modalControlLine("Esc/Enter Cancel")}
	case "quit":
		lines = append(lines, "Quit and close all live tunnels?")
		for _, s := range p.live {
			lines = append(lines, renderWrappedModalFields(style, width, tunnelFields(s))...)
		}
	case "start":
		s := app.TunnelSnapshot{Connection: app.SSHAttemptTarget{ID: p.connection.ID, Revision: p.connection.Revision, Path: p.connection.Path, Host: p.connection.Host, Port: p.connection.Port}, Config: p.config, State: app.TunnelStarting}
		lines = append(lines, "Start forwarding?", "Cancel is the default.")
		lines = append(lines, renderWrappedModalFields(style, width, tunnelFields(s))...)
	case "stop":
		lines = append(lines, "Stop this tunnel?")
		lines = append(lines, renderWrappedModalFields(style, width, tunnelFields(p.snapshot))...)
	case "inspect":
		lines = renderWrappedModalFields(style, width, tunnelFields(p.snapshot))
		if p.snapshot.State == app.TunnelStarting || p.snapshot.State == app.TunnelActive {
			lines = append(lines, modalControlLine("s Stop"))
		}
		if !p.snapshot.Live() {
			lines = append(lines, modalControlLine("r Retry"), modalControlLine("d Dismiss"))
		}
		return append(lines, modalControlLine("Esc Back"))
	}
	return append(lines, modalControlLine("y Confirm"), modalControlLine("Esc/Enter Cancel"))
}

func (m *Model) pollTunnels() tea.Cmd {
	if m.tunnels == nil {
		return nil
	}
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tunnelPollMsg{} })
}

func (m *Model) reconcileTunnels() {
	if m.tunnels == nil {
		return
	}
	previous := 0
	for i, s := range m.tunnelSnapshots {
		if s.ID == m.selectedTunnel {
			previous = i
		}
	}
	m.tunnelSnapshots = m.tunnels.Snapshots()
	if p, ok := m.modal.payload.(tunnelModalPayload); ok && p.action == "inspect" {
		for _, s := range m.tunnelSnapshots {
			if s.ID == p.snapshot.ID {
				p.snapshot = s
				m.modal.payload = p
				break
			}
		}
	}
	for _, s := range m.tunnelSnapshots {
		if s.ID == m.selectedTunnel {
			return
		}
	}
	m.selectedTunnel = 0
	if len(m.tunnelSnapshots) > 0 {
		m.selectedTunnel = m.tunnelSnapshots[min(previous, len(m.tunnelSnapshots)-1)].ID
	}
}

func (m *Model) selectedTunnelSnapshot() (app.TunnelSnapshot, bool) {
	for _, s := range m.tunnelSnapshots {
		if s.ID == m.selectedTunnel {
			return s, true
		}
	}
	return app.TunnelSnapshot{}, false
}

func (m *Model) renderTunnels(rect layoutRect) string {
	active, selected := 0, noActiveLine
	lines := make([]string, 0, len(m.tunnelSnapshots))
	for i, s := range m.tunnelSnapshots {
		if s.State == app.TunnelActive {
			active++
		}
		prefix := "  "
		if s.ID == m.selectedTunnel {
			selected = i
			prefix = "> "
		}
		line := prefix + string(s.State) + " " + string(s.Config.Mode) + " " + s.Connection.Path + " " + s.Config.Listen.String()
		if s.Config.Mode != app.TunnelDynamic {
			line += " -> " + s.Config.Destination.String()
		}
		if s.Warning != "" {
			line += " Warning"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = []string{"No tunnels; select host, p Forward"}
	}
	projection := m.tunnelViewport.project(lines, rect.contentHeight(), rect.contentWidth(), selected)
	return renderRegionPanelWithScrollbar(m.styles.regionTitle(fmt.Sprintf("Tunnels (%d active)", active), m.focusOwner == focusOwnerTunnels), projection.lines, rect, m.styles, projection.scrollbar, 0)
}

func (m *Model) tunnelLegend(width int) []string {
	if m.search != nil {
		return renderActionLegend(m.styles, treeSearchActionDescriptors, width)
	}
	if m.operation != nil {
		return packActions(m.operation.loadingStatus(), m.visibleActionDescriptors(m.currentActionDescriptors()), width, 3)
	}
	descriptors := m.currentActionDescriptors()
	if m.tunnelForm != nil {
		descriptors = []actionDescriptor{{key: "Ctrl+S", label: "Start"}, {key: "Esc", label: "Cancel"}, {key: "F1", label: "Help"}, {key: "Tab", label: "Next"}, {key: "Shift+Tab/F2", label: "Previous"}}
		if m.tunnelForm.error == "" && m.tunnelForm.focus == 0 {
			descriptors = append([]actionDescriptor{{key: "Left/Right", label: "Change mode"}}, descriptors...)
		} else if m.tunnelForm.error == "" && m.tunnelForm.focus == m.tunnelForm.startField() {
			descriptors = append([]actionDescriptor{{key: "Enter", label: "Start"}}, descriptors...)
		}
	}
	lines := renderBrowserLegend(m.styles, descriptors, width)
	if m.tunnelForm != nil {
		lines = renderActionLegend(m.styles, descriptors, width)
	}
	if len(lines) > 3 {
		lines = lines[:3]
	}
	notice := ""
	if strings.HasPrefix(m.status, "Shell") || strings.HasPrefix(m.status, "Tunnel") {
		notice = m.status
	}
	if m.tunnelForm != nil && m.tunnelForm.error != "" {
		notice = m.tunnelForm.error
	}
	if notice != "" {
		lines = append([]string{safeText(notice, width)}, lines...)
		if len(lines) > 3 {
			lines = lines[:3]
		}
	}
	return lines
}

func tunnelActionDescriptors(state app.TunnelState) []actionDescriptor {
	actions := []actionDescriptor{{id: actionQuit, key: "q", label: "Quit", priority: actionPriorityRecovery}, {id: actionHelp, key: "?", label: "Help", priority: actionPriorityRecovery}, {id: actionShowTree, key: "Tab/Shift+Tab/F2", label: "Focus", priority: actionPriorityNavigation}, {id: actionUp, key: "Up/k", label: "Move up", priority: actionPriorityNavigation}, {id: actionDown, key: "Down/j", label: "Move down", priority: actionPriorityNavigation}, {id: actionBack, key: "Esc", label: "Tree", priority: actionPriorityNavigation}}
	if state != "" {
		actions = append(actions, actionDescriptor{id: actionInspectTunnel, key: "Enter", label: "Inspect", priority: actionPriorityPrimary})
	}
	if state == app.TunnelStarting || state == app.TunnelActive {
		actions = append(actions, actionDescriptor{id: actionStopTunnel, key: "s", label: "Stop", priority: actionPriorityPrimary})
	}
	if state == app.TunnelStopped || state == app.TunnelFailed {
		actions = append(actions, actionDescriptor{id: actionRetry, key: "r", label: "Retry", priority: actionPriorityRecovery}, actionDescriptor{id: actionDismissTunnel, key: "d", label: "Dismiss", priority: actionPriorityDomain})
	}
	return actions
}

func (m *Model) handleTunnelsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s, ok := m.selectedTunnelSnapshot()
	switch msg.String() {
	case "esc":
		m.focusOwner = focusOwnerTree
	case "q", "ctrl+c":
		return m, m.requestTunnelQuit()
	case "?", "f1":
		m.openGenericModal(modalKindHelp, nil, helpPayload{lines: []string{"Up/Down or j/k Select tunnel", "Enter Inspect complete endpoints and warnings", "s Confirm stop; r Review current host and retry", "d Dismiss Stopped/Failed; Esc Tree", "Tab/Shift+Tab/F2 Cycle panels; q Safe quit"}})
	case "up", "k", "down", "j", "home", "g", "end", "G":
		index := 0
		for i, entry := range m.tunnelSnapshots {
			if entry.ID == m.selectedTunnel {
				index = i
			}
		}
		switch msg.String() {
		case "up", "k":
			index--
		case "down", "j":
			index++
		case "home", "g":
			index = 0
		case "end", "G":
			index = len(m.tunnelSnapshots) - 1
		}
		if len(m.tunnelSnapshots) > 0 {
			m.selectedTunnel = m.tunnelSnapshots[min(max(0, index), len(m.tunnelSnapshots)-1)].ID
		}
	case "enter":
		if ok {
			m.openGenericModal(modalKindTunnel, nil, tunnelModalPayload{action: "inspect", snapshot: s})
		}
	case "s":
		if ok && (s.State == app.TunnelStarting || s.State == app.TunnelActive) {
			m.openGenericModal(modalKindTunnel, nil, tunnelModalPayload{action: "stop", snapshot: s})
		}
	case "r":
		if ok && !s.Live() {
			return m, m.retryTunnelCommand(s)
		}
	case "d":
		if ok && !s.Live() {
			return m, m.dismissTunnelCommand(s.ID)
		}
	}
	return m, nil
}

func (m *Model) handleTunnelModalKey(msg tea.KeyPressMsg, p tunnelModalPayload) (tea.Model, tea.Cmd) {
	if p.action == "inspect" {
		if fresh, ok := m.tunnels.Get(p.snapshot.ID); ok {
			p.snapshot = fresh
			m.modal.payload = p
		}
		if msg.String() == "s" && (p.snapshot.State == app.TunnelStarting || p.snapshot.State == app.TunnelActive) {
			p.action = "stop"
			m.modal.payload = p
			m.modal.viewport = newViewportState(0)
			return m, nil
		}
		if !p.snapshot.Live() && (msg.String() == "r" || msg.String() == "d") {
			m.closeGenericModal()
			if msg.String() == "r" {
				return m, m.retryTunnelCommand(p.snapshot)
			}
			return m, m.dismissTunnelCommand(p.snapshot.ID)
		}
		if msg.String() == "esc" {
			m.closeGenericModal()
		} else {
			m.scrollModal(msg)
		}
		return m, nil
	}
	if msg.String() == "esc" || msg.String() == "enter" {
		m.closeGenericModal()
		if p.action == "quit" && p.previous != nil {
			m.modal = *p.previous
			m.focusOwner = focusOwnerModal
		}
		return m, nil
	}
	if p.action == "discard" && msg.String() == "d" {
		m.closeGenericModal()
		m.closeTunnelForm()
		if p.quit {
			return m, m.requestTunnelQuit()
		}
		return m, nil
	}
	if msg.String() != "y" {
		m.scrollModal(msg)
		return m, nil
	}
	m.closeGenericModal()
	switch p.action {
	case "start":
		return m, m.startTunnelCommand(p.connection, p.config)
	case "stop":
		service, id := m.tunnels, p.snapshot.ID
		return m, func() tea.Msg { return tunnelActionMsg{err: service.Stop(context.WithoutCancel(m.ctx), id)} }
	case "quit":
		return m, m.closeTunnelsAndQuit()
	}
	return m, nil
}

func (m *Model) dismissTunnelCommand(id uint64) tea.Cmd {
	service := m.tunnels
	return func() tea.Msg { return tunnelActionMsg{err: service.Dismiss(id)} }
}

func (m *Model) retryTunnelCommand(s app.TunnelSnapshot) tea.Cmd {
	id, ctx, ok := m.beginOperationWith(asyncOperationReload, nil, operationOwnerRoot)
	if !ok {
		return nil
	}
	service := m.connections
	return func() tea.Msg {
		msg := tunnelRetryResolvedMsg{token: id, snapshot: s}
		if service == nil {
			msg.err = errUnavailable
			return msg
		}
		result, err := service.Get(ctx, app.ItemSelector{ID: s.Connection.ID})
		msg.connection, msg.err = result.Connection, err
		if err == nil && result.Connection.ID != s.Connection.ID {
			msg.err = app.ErrConflict
		}
		return msg
	}
}

func (m *Model) requestTunnelQuit() tea.Cmd {
	m.reconcileTunnels()
	live := []app.TunnelSnapshot{}
	for _, s := range m.tunnelSnapshots {
		if s.Live() {
			live = append(live, s)
		}
	}
	if len(live) == 0 {
		return m.closeTunnelsAndQuit()
	}
	var previous *modalState
	if m.modal.isOpen() {
		copy := m.modal
		previous = &copy
		m.closeGenericModal()
	}
	m.openGenericModal(modalKindTunnel, nil, tunnelModalPayload{action: "quit", live: live, previous: previous})
	return nil
}

func (m *Model) closeTunnelsAndQuit() tea.Cmd {
	if m.tunnels == nil {
		return tea.Quit
	}
	service := m.tunnels
	m.cancelCurrentOperation(false)
	return func() tea.Msg {
		if err := service.Close(); err != nil {
			return tunnelActionMsg{err: err}
		}
		return tunnelQuitReadyMsg{}
	}
}

func (m *Model) startTunnelCommand(connection app.Connection, config app.TunnelConfig) tea.Cmd {
	target := m.captureConnectionTarget(connection)
	id, ctx, ok := m.beginOperationWith(asyncOperationSSHStart, &target, operationOwnerForm)
	if !ok {
		return nil
	}
	r := &sessionRuntime{id: id, ctx: ctx, requests: make(chan tea.Msg, 1), done: make(chan struct{})}
	m.sessionRuntime = r
	m.tunnelStartup = &tunnelStartup{runtime: r, cancel: m.operation.cancel}
	if !m.modal.isOpen() {
		m.openGenericModal(modalKindTunnel, &target, tunnelModalPayload{action: "start", connection: connection, config: config})
	}
	revision := connection.Revision
	request := app.TunnelRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: &revision, Config: config, DecideTrust: r.decideTrust, ReadSecret: r.readSecret}
	retry := m.tunnelForm.retryID
	service := m.tunnels
	return tea.Batch(r.wait, func() tea.Msg {
		if service == nil {
			return tunnelStartedMsg{token: id, err: errUnavailable}
		}
		var s app.TunnelSnapshot
		var err error
		if retry != 0 {
			s, err = service.Retry(ctx, retry, request)
		} else {
			s, err = service.Start(ctx, request)
		}
		return tunnelStartedMsg{token: id, snapshot: s, err: err}
	})
}

func (m *Model) handleTunnelStarted(msg tunnelStartedMsg) tea.Cmd {
	if m.tunnelStartup == nil || !m.acceptsOperationResult(msg.token, asyncOperationSSHStart) {
		if m.tunnels != nil && msg.snapshot.ID != 0 {
			service, id, attempt := m.tunnels, msg.snapshot.ID, msg.snapshot.Attempt
			if current, ok := service.Get(id); ok && current.Attempt == attempt && current.Live() {
				// Retry may precede this delayed command; match the attempt atomically.
				return func() tea.Msg { return tunnelActionMsg{err: service.StopAttempt(context.Background(), id, attempt)} }
			}
		}
		return nil
	}
	m.tunnelStartup.snapshot = msg.snapshot
	if msg.err != nil {
		close(m.tunnelStartup.runtime.done)
		m.tunnelStartup = nil
		m.sessionRuntime = nil
		quit := m.completeOperation(msg.token, nil)
		if p, ok := m.modal.payload.(tunnelModalPayload); ok && p.action == "start" {
			m.closeGenericModal()
		}
		m.tunnelForm.error = "Tunnel could not start; check settings and current host, then retry."
		if quit {
			return m.requestTunnelQuit()
		}
		return nil
	}
	m.selectedTunnel = msg.snapshot.ID
	m.reconcileTunnels()
	if m.operation.ctx.Err() != nil {
		return m.stopCanceledTunnelStartup()
	}
	return m.finishTunnelStartup()
}

func (m *Model) stopCanceledTunnelStartup() tea.Cmd {
	if m.tunnelStartup.stopping {
		return nil
	}
	m.tunnelStartup.stopping = true
	service, id := m.tunnels, m.tunnelStartup.snapshot.ID
	return func() tea.Msg { return tunnelActionMsg{err: service.Stop(context.Background(), id)} }
}

func (m *Model) finishTunnelStartup() tea.Cmd {
	startup := m.tunnelStartup
	if startup == nil || startup.snapshot.ID == 0 {
		return nil
	}
	s, ok := m.tunnels.Get(startup.snapshot.ID)
	if m.operation.ctx.Err() != nil && ok && s.Live() {
		return m.stopCanceledTunnelStartup()
	}
	if !ok || s.Attempt != startup.snapshot.Attempt || s.State == app.TunnelStarting || s.State == app.TunnelStopping {
		return nil
	}
	if p, ok := m.modal.payload.(tunnelModalPayload); ok && p.action == "quit" {
		return nil
	}
	quit := m.operation.quitIntent
	// Security callbacks are no longer needed after activation. Complete only
	// the input-owning operation; its cancellation must not stop the manager.
	m.operation.cancel = nil
	startup.cancel()
	m.completeOperation(startup.runtime.id, nil)
	close(startup.runtime.done)
	m.clearSessionSecurityInput()
	m.sessionRuntime = nil
	m.tunnelStartup = nil
	if p, ok := m.modal.payload.(tunnelModalPayload); ok && p.action == "start" {
		m.closeGenericModal()
	}
	if s.State == app.TunnelActive {
		m.closeTunnelForm()
		m.focusOwner = focusOwnerTunnels
		m.status = "Tunnel Active"
	} else {
		m.tunnelForm.error = "Tunnel " + string(s.State) + ": " + s.Diagnostic + "; edit settings or retry."
	}
	if quit {
		return m.requestTunnelQuit()
	}
	return nil
}

func (m *Model) releaseTunnelInput() {
	if m.tunnelStartup == nil {
		return
	}
	m.tunnelStartup.cancel()
	if m.operation != nil {
		m.completeOperation(m.operation.id, nil)
	}
	close(m.tunnelStartup.runtime.done)
	m.tunnelStartup = nil
	m.sessionRuntime = nil
	m.clearSessionSecurityInput()
}
