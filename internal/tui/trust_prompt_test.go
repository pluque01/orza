package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestUS4TrustUnknownChangedExplicitDecisionMatrixTwentyRuns(t *testing.T) {
	for run := range 20 {
		for _, status := range []app.HostTrustStatus{app.HostTrustUnknown, app.HostTrustChanged} {
			prompt := newTrustPrompt(app.TrustDecisionPrompt{Status: status})
			if _, decided := prompt.update(keyPress("enter")); decided {
				t.Fatalf("run %d %s: Enter accepted trust", run, status)
			}
			if decision, decided := prompt.update(keyPress("y")); !decided || decision != app.TrustOnce {
				t.Fatalf("run %d %s: explicit accept failed", run, status)
			}
			prompt = newTrustPrompt(app.TrustDecisionPrompt{Status: status})
			if decision, decided := prompt.update(keyPress("esc")); !decided || decision != app.TrustReject {
				t.Fatalf("run %d %s: cancel failed", run, status)
			}
		}
	}
}

func TestTrustPromptProjectsEveryDynamicValueBeforeRendering(t *testing.T) {
	unsafeValues := []string{
		"ansi\x1b[31mred\x1b[0m",
		"carriage\rreturn\nlinefeed",
		"c0\x00\x07\x7f",
		"c1\u0085\u009b",
		"bidi\u061c\u202e\u2066",
		string([]byte{'i', 'n', 'v', 'a', 'l', 'i', 'd', 0xff, 0xc3, '('}),
	}

	for _, unsafeValue := range unsafeValues {
		prompt := app.TrustDecisionPrompt{
			Status: app.HostTrustChanged,
			Host: app.PresentedHost{
				Endpoint:          app.HostEndpoint{CanonicalHost: unsafeValue, Port: 22},
				RemoteAddress:     unsafeValue,
				KeyAlgorithm:      unsafeValue,
				FingerprintSHA256: unsafeValue,
			},
			Known: &app.TrustedHost{FingerprintSHA256: unsafeValue},
		}
		original := prompt
		view := newTrustPrompt(prompt).view(newStyles(true))
		projection := safeText(unsafeValue, 256)

		if count := strings.Count(view, projection); count != 5 {
			t.Fatalf("projection %q appears %d times, want every dynamic trust value:\n%s", projection, count, view)
		}
		for _, line := range strings.Split(view, "\n") {
			assertTerminalSafe(t, line, max(ansi.StringWidth(line), 1))
		}
		if !reflect.DeepEqual(prompt, original) {
			t.Fatalf("rendering changed trust prompt bytes: got %#v, want %#v", prompt, original)
		}
	}
}

func TestTrustPromptOversizedValuesAreBoundedWithoutChangingSource(t *testing.T) {
	oversized := strings.Repeat("host-segment-", 100_000) + "tail"
	prompt := app.TrustDecisionPrompt{
		Status: app.HostTrustChanged,
		Host: app.PresentedHost{
			Endpoint:          app.HostEndpoint{CanonicalHost: oversized, Port: 22},
			RemoteAddress:     oversized,
			KeyAlgorithm:      oversized,
			FingerprintSHA256: oversized,
		},
		Known: &app.TrustedHost{FingerprintSHA256: oversized},
	}
	view := newTrustPrompt(prompt).view(newStyles(true))

	if len(view) > 5*(256+64) || strings.Count(view, safeTextEllipsis) != 5 {
		t.Fatalf("oversized trust projection is not bounded: bytes=%d ellipses=%d", len(view), strings.Count(view, safeTextEllipsis))
	}
	if prompt.Host.Endpoint.CanonicalHost != oversized || prompt.Host.RemoteAddress != oversized || prompt.Known.FingerprintSHA256 != oversized {
		t.Fatal("rendering changed an underlying oversized trust value")
	}
}
