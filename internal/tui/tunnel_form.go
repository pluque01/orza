package tui

import (
	"net"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const tunnelFormTitle = "Port forwarding"

var tunnelModes = [...]app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic}
var tunnelModeLabels = [...]string{"Local", "Remote", "Dynamic"}
var tunnelInputLabels = [...]string{"Listen address", "Listen port", "Destination host", "Destination port"}

type tunnelForm struct {
	connection app.Connection
	retryID    uint64
	mode       app.TunnelMode
	inputs     [4]textField
	focus      int
	ack        bool
	initial    string
	error      string
	openedFrom focusOwner
	viewport   viewportState
}

func newTunnelForm(connection app.Connection, config app.TunnelConfig, retry uint64) *tunnelForm {
	f := &tunnelForm{connection: connection, retryID: retry, mode: config.Mode, viewport: newViewportState(0)}
	if f.mode == "" {
		f.mode = app.TunnelLocal
	}
	for i := range f.inputs {
		f.inputs[i] = newTextField()
	}
	if config.Listen.Host == "" {
		config.Listen.Host = "127.0.0.1"
	}
	f.inputs[0].SetValue(config.Listen.Host)
	if config.Listen.Port != 0 {
		f.inputs[1].SetValue(strconv.Itoa(int(config.Listen.Port)))
	}
	f.inputs[2].SetValue(config.Destination.Host)
	if config.Destination.Port != 0 {
		f.inputs[3].SetValue(strconv.Itoa(int(config.Destination.Port)))
	}
	f.initial = f.signature()
	return f
}

func (f *tunnelForm) signature() string {
	parts := []string{string(f.mode), strconv.FormatBool(f.ack)}
	for i := range f.inputs {
		parts = append(parts, f.inputs[i].Value())
	}
	return strings.Join(parts, "\x00")
}

func (f *tunnelForm) dirty() bool { return f.signature() != f.initial }
func (f *tunnelForm) exposureField() int {
	if f.mode == app.TunnelDynamic {
		return 3
	}
	return 5
}

func (f *tunnelForm) startField() int { return f.exposureField() + 1 }

func (f *tunnelForm) setFocus(field int) tea.Cmd {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	f.focus = field
	if field > 0 && field < f.exposureField() {
		return f.inputs[field-1].Focus()
	}
	return nil
}

func (f *tunnelForm) modeView() string {
	parts := make([]string, len(tunnelModes))
	for i, mode := range tunnelModes {
		parts[i] = tunnelModeLabels[i]
		if mode == f.mode {
			parts[i] = "[" + parts[i] + "]"
		}
	}
	return strings.Join(parts, " ")
}

func (f *tunnelForm) update(msg tea.Msg) tea.Cmd {
	before := f.signature()
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "tab", "shift+tab", "f2":
			delta := 1
			if key.String() != "tab" {
				delta = f.startField()
			}
			return f.setFocus((f.focus + delta) % (f.startField() + 1))
		case "left", "right":
			if f.focus == 0 {
				for i, mode := range tunnelModes {
					if mode == f.mode {
						delta := 1
						if key.String() == "left" {
							delta = 2
						}
						f.mode = tunnelModes[(i+delta)%len(tunnelModes)]
						break
					}
				}
				f.ack = false
				f.error = ""
				return nil
			}
		case "space", "enter":
			if f.focus == f.exposureField() {
				f.ack = !f.ack
				f.error = ""
				return nil
			}
		}
	}
	var cmd tea.Cmd
	if f.focus > 0 && f.focus < f.exposureField() {
		cmd = f.inputs[f.focus-1].Update(msg)
	}
	if before != f.signature() && (f.focus == 1 || f.focus == 2) {
		f.ack = false
	}
	if before != f.signature() {
		f.error = ""
	}
	return cmd
}

func (f *tunnelForm) config() (app.TunnelConfig, bool) {
	c := app.TunnelConfig{Mode: f.mode, Listen: app.TunnelEndpoint{Host: f.inputs[0].Value()}, ExposureAcknowledged: f.ack}
	f.error = ""
	parsePort := func(field int, label string) uint16 {
		value := f.inputs[field].Value()
		port, err := strconv.ParseUint(value, 10, 16)
		if err != nil || port == 0 || strings.Trim(value, "0123456789") != "" {
			f.error = label + ": enter a port from 1 to 65535"
			f.setFocus(field + 1)
			return 0
		}
		return uint16(port)
	}
	c.Listen.Port = parsePort(1, "Listen port")
	if f.error != "" {
		return c, false
	}
	if f.mode != app.TunnelDynamic {
		c.Destination.Host = f.inputs[2].Value()
		c.Destination.Port = parsePort(3, "Destination port")
		if f.error != "" {
			return c, false
		}
	}
	normalized, err := app.ValidateTunnelConfig(c)
	if err != nil {
		address := c.Listen.Host
		if address == "localhost" {
			address = "127.0.0.1"
		}
		ip := net.ParseIP(address)
		switch {
		case ip == nil:
			f.error = "Listen address: enter an IP literal or localhost"
			f.setFocus(1)
		case !ip.IsLoopback() && !f.ack:
			f.error = "Acknowledge exposure separately before Start"
			f.setFocus(f.exposureField())
		default:
			f.error = "Destination host: enter a valid hostname or IP literal"
			f.setFocus(3)
		}
		return c, false
	}
	return normalized, true
}

func (f *tunnelForm) project(style styles, width, height int) viewportProjection {
	count := 4
	if f.mode == app.TunnelDynamic {
		count = 2
	}
	labelWidth := ansi.StringWidth("Path")
	for _, label := range tunnelInputLabels[:count] {
		labelWidth = max(labelWidth, ansi.StringWidth(label))
	}
	labelWidth = min(labelWidth, max(0, width-connectionFormMarkerWidth-1))
	valueWidth := max(0, width-connectionFormMarkerWidth-labelWidth-1)
	lines := []string{
		style.contentBadge(tunnelFormTitle),
		style.item(compactConnectionFormRow(style, "Path", safeText(f.connection.Path, valueWidth), labelWidth), itemSemantics{}),
	}
	selector := f.modeView()
	modeLabelWidth := min(labelWidth, max(0, width-connectionFormMarkerWidth-ansi.StringWidth(selector)-1))
	lines = append(lines, style.item(compactConnectionFormRow(style, "Mode", selector, modeLabelWidth), itemSemantics{focused: f.focus == 0}))
	active, activeStart := 2, -1
	errorShown := false
	for i := 0; i < count; i++ {
		f.inputs[i].SetWidth(valueWidth)
		message := f.inputs[i].Error()
		if f.error != "" && strings.HasPrefix(f.error, tunnelInputLabels[i]+":") {
			message = f.error
			errorShown = true
		}
		focused := f.focus == i+1
		if focused {
			active = len(lines)
		}
		line := compactConnectionFormRow(style, tunnelInputLabels[i], f.inputs[i].View(), labelWidth)
		lines = append(lines, style.item(line, itemSemantics{focused: focused, invalid: message != ""}))
		if message != "" {
			lines = append(lines, style.failureMessage(safeText(message, max(0, width-7))))
			if focused {
				activeStart, active = active, len(lines)-1
			}
		}
	}
	ack := " "
	if f.ack {
		ack = "x"
	}
	if f.focus == f.exposureField() {
		active = len(lines)
	}
	exposureError := f.error == "Acknowledge exposure separately before Start"
	lines = append(lines, style.item("["+ack+"] Acknowledge external access", itemSemantics{focused: f.focus == f.exposureField(), invalid: exposureError}))
	if f.error != "" && !errorShown {
		lines = append(lines, style.failureMessage(safeText(f.error, max(0, width-7))))
		if exposureError && f.focus == f.exposureField() {
			activeStart, active = active, len(lines)-1
		} else if f.focus == f.startField() {
			activeStart = len(lines) - 1
		}
	}
	if f.focus == f.startField() {
		active = len(lines)
	}
	lines = append(lines, style.item("[ Start forwarding ]", itemSemantics{focused: f.focus == f.startField(), primary: true}))
	for _, line := range tunnelDirection(f.mode) {
		lines = append(lines, style.descriptiveLabel(line))
	}
	if activeStart >= 0 {
		if height == 1 {
			// The legend carries the error when only its focused control fits.
			control := activeStart
			if f.focus == f.startField() {
				control = active
			}
			return f.viewport.project(lines, height, width, control)
		}
		return projectActiveBlock(lines, activeStart, active, height, width)
	}
	return f.viewport.project(lines, height, width, active)
}

func tunnelDirection(mode app.TunnelMode) []string {
	switch mode {
	case app.TunnelRemote:
		return []string{"Saved host listens; your computer reaches", "the destination and resolves its name.", "Remote scope is unverified and depends on", "server configuration, even for loopback.", "Server release after transport loss is not verified."}
	case app.TunnelDynamic:
		return []string{"SOCKS5 proxy: your computer listens.", "Configure clients to use this endpoint.", "Saved host reaches and resolves destinations.", "Proxy clients are not authenticated."}
	default:
		return []string{"Your computer listens; the saved host", "reaches the destination and resolves its name.", "Example: listen 15432; db.internal:5432"}
	}
}

func (m *Model) openTunnelForm(connection app.Connection, config app.TunnelConfig, retry uint64) {
	f := newTunnelForm(connection, config, retry)
	f.openedFrom = m.focusOwner
	m.tunnelForm = f
	m.focusOwner = focusOwnerConnectionForm
}

func (m *Model) closeTunnelForm() {
	if m.tunnelForm != nil {
		m.focusOwner = m.tunnelForm.openedFrom
	}
	m.tunnelForm = nil
}

func (m *Model) handleTunnelFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "f1":
		lines := append(tunnelDirection(m.tunnelForm.mode), "Tab Next; Shift+Tab/F2 Previous; Left/Right Mode", "Space Acknowledge exposure; Ctrl+S Start; Esc Cancel")
		if m.tunnelForm.error != "" {
			lines = append([]string{m.tunnelForm.error}, lines...)
		}
		m.openGenericModal(modalKindHelp, nil, helpPayload{lines: lines})
	case "esc", "ctrl+c":
		if m.tunnelForm.dirty() {
			m.openGenericModal(modalKindTunnel, nil, tunnelModalPayload{action: "discard", quit: msg.String() == "ctrl+c"})
		} else {
			m.closeTunnelForm()
			if msg.String() == "ctrl+c" {
				return m, m.requestTunnelQuit()
			}
		}
	case "ctrl+s", "enter":
		if msg.String() == "enter" && m.tunnelForm.focus != m.tunnelForm.startField() {
			return m, m.tunnelForm.update(msg)
		}
		if config, valid := m.tunnelForm.config(); valid {
			target := m.captureConnectionTarget(m.tunnelForm.connection)
			m.openGenericModal(modalKindTunnel, &target, tunnelModalPayload{action: "start", config: config, connection: m.tunnelForm.connection})
		}
	default:
		return m, m.tunnelForm.update(msg)
	}
	return m, nil
}
