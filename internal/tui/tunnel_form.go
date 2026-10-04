package tui

import (
	"net"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

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
}

func newTunnelForm(connection app.Connection, config app.TunnelConfig, retry uint64) *tunnelForm {
	f := &tunnelForm{connection: connection, retryID: retry, mode: config.Mode}
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
func (f *tunnelForm) lastField() int {
	if f.mode == app.TunnelDynamic {
		return 3
	}
	return 5
}

func (f *tunnelForm) update(msg tea.Msg) tea.Cmd {
	before := f.signature()
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "tab", "shift+tab", "f2":
			for i := range f.inputs {
				f.inputs[i].Blur()
			}
			delta := 1
			if key.String() != "tab" {
				delta = f.lastField()
			}
			f.focus = (f.focus + delta) % (f.lastField() + 1)
			if f.focus > 0 && f.focus < f.lastField() {
				return f.inputs[f.focus-1].Focus()
			}
			return nil
		case "left", "right":
			if f.focus == 0 {
				modes := []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic}
				for i, mode := range modes {
					if mode == f.mode {
						delta := 1
						if key.String() == "left" {
							delta = 2
						}
						f.mode = modes[(i+delta)%3]
						break
					}
				}
				f.ack = false
				return nil
			}
		case "space", "enter":
			if f.focus == f.lastField() {
				f.ack = !f.ack
				return nil
			}
		}
	}
	var cmd tea.Cmd
	if f.focus > 0 && f.focus < f.lastField() {
		cmd = f.inputs[f.focus-1].Update(msg)
	}
	if before != f.signature() && (f.focus == 1 || f.focus == 2) {
		f.ack = false
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
			f.focus = field + 1
			f.inputs[field].Focus()
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
		case !ip.IsLoopback() && !f.ack:
			f.error = "Acknowledge exposure separately before Start"
		default:
			f.error = "Destination host: enter a valid hostname or IP literal"
		}
		return c, false
	}
	return normalized, true
}

func (f *tunnelForm) project(style styles, width, height int) viewportProjection {
	fields := []string{"Listen address", "Listen port", "Destination host", "Destination port"}
	prefix := "  "
	if f.focus == 0 {
		prefix = "> "
	}
	lines := []string{"Forwarding: " + safeText(f.connection.Path, width), prefix + "Mode: " + string(f.mode) + " (Left/Right)"}
	active := 1
	count := 4
	if f.mode == app.TunnelDynamic {
		count = 2
	}
	for i := 0; i < count; i++ {
		f.inputs[i].SetWidth(max(1, width-len(fields[i])-4))
		prefix := "  "
		if f.focus == i+1 {
			prefix = "> "
			active = len(lines)
		}
		lines = append(lines, prefix+fields[i]+": "+f.inputs[i].View())
	}
	ack := "[ ]"
	if f.ack {
		ack = "[x]"
	}
	if f.focus == f.lastField() {
		active = len(lines)
	}
	lines = append(lines, ack+" Acknowledge external access (Space)")
	if f.error != "" {
		lines = append(lines, style.failureMessage(f.error))
	}
	lines = append(lines, tunnelDirection(f.mode)...)
	lines = append(lines, "Ctrl+S Start  Esc Cancel  F1 Help")
	return newViewportState(0).project(lines, height, width, active)
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
	case "ctrl+s":
		if config, valid := m.tunnelForm.config(); valid {
			target := m.captureConnectionTarget(m.tunnelForm.connection)
			m.openGenericModal(modalKindTunnel, &target, tunnelModalPayload{action: "start", config: config, connection: m.tunnelForm.connection})
		}
	default:
		return m, m.tunnelForm.update(msg)
	}
	return m, nil
}
