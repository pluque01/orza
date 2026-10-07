package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestTunnelFormUsesConnectionFormStyles(t *testing.T) {
	for _, mode := range []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic} {
		for _, noColor := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/noColor=%t", mode, noColor), func(t *testing.T) {
				style := newStyles(noColor)
				f := newTunnelForm(testConnection("host", syntheticRootID, "/work/host", 1), app.TunnelConfig{
					Mode: mode, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080},
					Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432},
				}, 0)
				lines := f.project(style, 72, 40).lines
				if lines[0] != style.contentBadge("Port forwarding") {
					t.Fatalf("form title does not use the content badge: %q", lines[0])
				}
				labels := []string{"Listen address", "Listen port", "Destination host", "Destination port"}
				if mode == app.TunnelDynamic {
					labels = labels[:2]
				}
				labelWidth := 0
				for _, label := range labels {
					labelWidth = max(labelWidth, ansi.StringWidth(label))
				}
				wantPath := style.item(compactConnectionFormRow(style, "Path", "/work/host", labelWidth), itemSemantics{})
				if lines[1] != wantPath {
					t.Fatalf("path differs from connection form styling:\ngot  %q\nwant %q", lines[1], wantPath)
				}
				choices := []string{"Local", "Remote", "Dynamic"}
				selected := map[app.TunnelMode]int{app.TunnelLocal: 0, app.TunnelRemote: 1, app.TunnelDynamic: 2}[mode]
				choices[selected] = "[" + choices[selected] + "]"
				wantMode := style.item(compactConnectionFormRow(style, "Mode", strings.Join(choices, " "), labelWidth), itemSemantics{focused: true})
				if lines[2] != wantMode {
					t.Fatalf("mode does not use the connection selector style:\ngot  %q\nwant %q", lines[2], wantMode)
				}
				for i, label := range labels {
					want := style.item(compactConnectionFormRow(style, label, f.inputs[i].Value(), labelWidth), itemSemantics{})
					if lines[i+3] != want {
						t.Fatalf("%s differs from connection form styling:\ngot  %q\nwant %q", label, lines[i+3], want)
					}
					if f.inputs[i].Width() != 72-connectionFormMarkerWidth-labelWidth-1 {
						t.Fatalf("%s has inconsistent control width %d", label, f.inputs[i].Width())
					}
				}
				if !strings.Contains(strings.Join(lines, "\n"), style.item("[ Start forwarding ]", itemSemantics{primary: true})) {
					t.Fatal("missing styled primary Start control")
				}
			})
		}
	}
}

func TestTunnelFormValidationUsesConnectionErrorStyles(t *testing.T) {
	for _, noColor := range []bool{true, false} {
		style := newStyles(noColor)
		f := newTunnelForm(testConnection("host", syntheticRootID, "/host", 1), app.TunnelConfig{}, 0)
		f.inputs[1].SetValue("0")
		if _, valid := f.config(); valid {
			t.Fatal("invalid port accepted")
		}
		lines := f.project(style, 44, 40).lines
		wantField := style.item(compactConnectionFormRow(style, "Listen port", "0", 16), itemSemantics{focused: true, invalid: true})
		wantError := style.failureMessage(safeText(f.error, 44-7))
		found := false
		for i := 0; i+1 < len(lines); i++ {
			if lines[i] == wantField && lines[i+1] == wantError {
				found = true
			}
		}
		if !found {
			t.Fatalf("validation does not pair the styled invalid field with its error:\n%s", strings.Join(lines, "\n"))
		}
	}
}

func TestTunnelFormStartControlOpensConfirmation(t *testing.T) {
	m := New(Config{Width: 80, Height: 24, NoColor: true})
	m.openTunnelForm(testConnection("host", syntheticRootID, "/host", 1), app.TunnelConfig{
		Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080},
		Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432},
	}, 0)
	f := m.tunnelForm
	f.focus = f.exposureField()
	f.update(keyPress("tab"))
	if f.focus != f.startField() {
		t.Fatalf("Tab skipped Start control: focus=%d", f.focus)
	}
	view := strings.Join(f.project(newStyles(true), 44, 20).lines, "\n")
	if !strings.Contains(view, "> * [ Start forwarding ]") {
		t.Fatalf("Start does not show focused/primary markers:\n%s", view)
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	p, ok := m.modal.payload.(tunnelModalPayload)
	if !ok || p.action != "start" || m.operation != nil {
		t.Fatal("Start must open review without starting network activity")
	}
}

func TestTunnelFormStyledProjectionStaysBounded(t *testing.T) {
	for _, width := range []int{8, 36, 44, 72} {
		for _, height := range []int{1, 2, 10} {
			for _, noColor := range []bool{true, false} {
				f := newTunnelForm(testConnection("host", syntheticRootID, "/"+strings.Repeat("long", 30), 1), app.TunnelConfig{}, 0)
				f.inputs[0].SetValue(strings.Repeat("long", 30))
				for focus := 0; focus <= f.startField(); focus++ {
					f.focus = focus
					projection := f.project(newStyles(noColor), width, height)
					if len(projection.lines) > height {
						t.Fatal("form exceeded available height")
					}
					for _, line := range projection.lines {
						if ansi.StringWidth(line) > width {
							t.Fatalf("form exceeded width %d: %q", width, line)
						}
					}
				}
			}
		}
	}
}

func TestTunnelFormMinimumErrorKeepsFocusedControl(t *testing.T) {
	for _, exposure := range []bool{false, true} {
		m := New(Config{Width: 40, Height: 12, NoColor: true})
		config := app.TunnelConfig{Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080}, Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432}}
		if exposure {
			config.Listen.Host = "0.0.0.0"
		}
		m.openTunnelForm(testConnection("host", syntheticRootID, "/host", 1), config, 0)
		if !exposure {
			m.tunnelForm.inputs[1].SetValue("0")
		}
		m.Update(keyPress("ctrl+s"))
		projection := m.tunnelForm.project(newStyles(true), 36, 1)
		want := ">!  Listen port      0"
		if exposure {
			want = ">!  [ ] Acknowledge external access"
		}
		if len(projection.lines) != 1 || !strings.Contains(projection.lines[0], want) {
			t.Fatalf("minimum-size validation hid the focused control: %q", projection.lines)
		}
		if !strings.Contains(m.View().Content, safeText(m.tunnelForm.error, 40)) {
			t.Fatal("validation error is not visible in the legend")
		}
	}
}

func TestTunnelFormFrameKeepsConnectionFormPresentation(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 12}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			var plain string
			for _, noColor := range []bool{true, false} {
				m := New(Config{Width: size[0], Height: size[1], NoColor: noColor})
				m.openTunnelForm(testConnection("host", syntheticRootID, "/work/host", 1), app.TunnelConfig{
					Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080},
					Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432},
				}, 0)
				frame := ansi.Strip(m.View().Content)
				for _, want := range []string{"Port forwarding", "[Local] Remote Dynamic", "Left/Right Change mode", "Tunnels"} {
					if !strings.Contains(frame, want) {
						t.Fatalf("form frame omitted %q:\n%s", want, frame)
					}
				}
				assertUS5FrameBounded(t, frame, size[0], size[1])
				if noColor {
					plain = frame
					t.Logf("\n%s", frame)
				} else if frame != plain {
					t.Fatal("form differs between color and no-color after stripping ANSI")
				}
			}
		})
	}
}

func TestTunnelFormMinimumFailureKeepsRecoveryKeys(t *testing.T) {
	m := New(Config{Width: 40, Height: 12, NoColor: true})
	m.openTunnelForm(testConnection("host", syntheticRootID, "/host", 1), app.TunnelConfig{}, 0)
	m.tunnelForm.error = "Tunnel could not start; check settings and current host, then retry."
	for _, focus := range []int{0, m.tunnelForm.startField()} {
		m.tunnelForm.focus = focus
		view := m.View().Content
		for _, want := range []string{"Ctrl+S Start", "Esc Cancel", "F1 Help"} {
			if !strings.Contains(view, want) {
				t.Fatalf("minimum failure hid %q:\n%s", want, view)
			}
		}
	}
}

func TestTunnelFormHelpContainsOnlyKeyboardActions(t *testing.T) {
	m := New(Config{Width: 80, Height: 24, NoColor: true})
	m.openTunnelForm(testConnection("host", syntheticRootID, "/host", 1), app.TunnelConfig{Mode: app.TunnelLocal}, 0)
	m.Update(keyPress("f1"))
	lines, _ := modalContent(m.modal, m.styles, 74, nil)
	got := strings.Join(lines, "\n")
	want := strings.Join(append(browserHelpLines(m.styles, tunnelFormActionDescriptors, 74), modalControlLine("?/Esc Close")), "\n")
	if got != want {
		t.Fatalf("Help does not use the standard action layout:\ngot:\n%s\nwant:\n%s", got, want)
	}
	for _, unwanted := range []string{"127.0.0.1", "Destination host", "SOCKS5", "PostgreSQL"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("Help includes forwarding documentation %q:\n%s", unwanted, got)
		}
	}
}

func TestTunnelFormHelpUsesActionDescriptors(t *testing.T) {
	m := New(Config{Width: 80, Height: 24, NoColor: true})
	m.openTunnelForm(testConnection("host", syntheticRootID, "/host", 1), app.TunnelConfig{}, 0)
	m.Update(keyPress("f1"))
	payload, ok := m.modal.payload.(helpPayload)
	if !ok || len(payload.lines) != 0 {
		t.Fatalf("tunnel Help does not use a descriptor payload: %#v", m.modal.payload)
	}
	if strings.Join(actionHelpLines(payload.descriptors), "\n") != strings.Join(actionHelpLines(tunnelFormActionDescriptors), "\n") {
		t.Fatalf("tunnel Help descriptors differ from its form actions: %#v", payload.descriptors)
	}
}

func TestTunnelStartConfirmationContainsNoDirectionDescription(t *testing.T) {
	p := tunnelModalPayload{action: "start", connection: testConnection("host", syntheticRootID, "/host", 1), config: app.TunnelConfig{Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080}, Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432}}}
	got := strings.Join(p.lines(newStyles(true), 74), "\n")
	for _, unwanted := range []string{"SSH host is the selected", "Local port ->", "Destination 127.0.0.1", "PostgreSQL example"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("confirmation includes forwarding documentation %q:\n%s", unwanted, got)
		}
	}
}
