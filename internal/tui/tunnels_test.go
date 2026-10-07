package tui

import (
	"github.com/charmbracelet/x/ansi"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestTunnelBrowserGoldens(t *testing.T) {
	for _, test := range []struct {
		width, height int
		file          string
	}{{40, 12, "testdata/narrow.golden"}, {80, 24, "testdata/empty.golden"}} {
		want, err := os.ReadFile(test.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, noColor := range []bool{true, false} {
			m := New(Config{Width: test.width, Height: test.height, NoColor: noColor})
			got := ansi.Strip(m.View().Content)
			assertUS5FrameBounded(t, got, test.width, test.height)
			if got != strings.TrimSuffix(string(want), "\n") {
				t.Fatalf("%s browser golden differs:\n%s", test.file, got)
			}
		}
	}
}

func TestPermanentTunnelsPanelMinimumGeometry(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 12}} {
		m := New(Config{Width: size[0], Height: size[1], NoColor: true})
		frame := m.View().Content
		for _, text := range []string{"Tree", "Details", "Tunnels", "No tunnels"} {
			if !strings.Contains(frame, text) {
				t.Errorf("%dx%d missing %q", size[0], size[1], text)
			}
		}
		if strings.Contains(frame, "Actions") {
			t.Fatal("Actions panel returned")
		}
	}
}

func TestShellReturnDoesNotQuitOrRetainRemoteStatus(t *testing.T) {
	m := New(Config{NoColor: true})
	id, _, _ := m.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerModal)
	status := 23
	cmd := m.handleSession(sessionFinishedMsg{id: id, result: app.ConnectResult{Session: app.SSHSessionResult{StartedAt: time.Now(), RemoteExitStatus: &status}}})
	if cmd != nil {
		t.Fatal("shell completion must return to browser, not quit")
	}
	if m.sessionResult.Session.RemoteExitStatus != nil || m.sessionErr != nil {
		t.Fatal("old shell outcome leaked into TUI result")
	}
	if !strings.Contains(m.status, "Shell") {
		t.Fatal("missing safe shell outcome notice")
	}
}
