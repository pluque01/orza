package tui

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestTrustPromptUsesControlledBadgeColonlessFieldsAndColorParity(t *testing.T) {
	fixtures := []struct {
		status app.HostTrustStatus
		label  string
	}{
		{status: app.HostTrustUnknown, label: "Verify host identity"},
		{status: app.HostTrustChanged, label: "Verify host identity"},
		{status: app.HostTrustRevoked, label: "Verify host identity"},
	}
	for _, fixture := range fixtures {
		t.Run(string(fixture.status), func(t *testing.T) {
			prompt := app.TrustDecisionPrompt{
				Status: fixture.status,
				Host: app.PresentedHost{
					Endpoint:          app.HostEndpoint{CanonicalHost: "host.example", Port: 2222},
					RemoteAddress:     "192.0.2.10:2222",
					KeyAlgorithm:      "ssh-ed25519",
					FingerprintSHA256: "SHA256:presented",
				},
				Known: &app.TrustedHost{FingerprintSHA256: "SHA256:known"},
			}
			plain := newTrustPrompt(prompt).view(newStyles(true))
			colored := newTrustPrompt(prompt).view(newStyles(false))

			if got := ansi.Strip(colored); got != plain {
				t.Fatalf("color text semantics = %q, want %q", got, plain)
			}
			if strings.Count(plain, "["+fixture.label+"]") != 1 {
				t.Fatalf("controlled badge inventory = %q", plain)
			}
			if !strings.Contains(colored, "\x1b[") {
				t.Fatalf("color prompt has no styling: %q", colored)
			}
			for _, label := range []string{"Host", "Remote address", "Algorithm", "SHA-256 fingerprint"} {
				assertColonlessPromptLabel(t, plain, label)
			}
			valueColumn := -1
			for _, value := range []string{"host.example:2222", "192.0.2.10:2222", "ssh-ed25519", "SHA256:presented"} {
				column := promptValueColumn(t, plain, value)
				if valueColumn == -1 {
					valueColumn = column
				} else if column != valueColumn {
					t.Fatalf("value %q starts in column %d, want %d: %q", value, column, valueColumn, plain)
				}
			}
			if fixture.status == app.HostTrustChanged {
				assertColonlessPromptLabel(t, plain, "Known fingerprint")
				if !strings.Contains(plain, "WARNING: changed key; this may indicate a possible attack.") {
					t.Fatalf("changed warning changed: %q", plain)
				}
			}
			if fixture.status == app.HostTrustRevoked {
				if !strings.Contains(plain, "This key is revoked. Esc Back  Q Quit") || strings.Contains(plain, "Trust once") {
					t.Fatalf("revoked warning or choices changed: %q", plain)
				}
			} else if !strings.Contains(plain, "> Reject (default)    Trust once    Trust and persist") {
				t.Fatalf("reject default or choices changed: %q", plain)
			}
		})
	}
}

func promptValueColumn(t *testing.T, view, value string) int {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if column := strings.Index(line, value); column >= 0 {
			return column
		}
	}
	t.Fatalf("value %q not found: %q", value, view)
	return -1
}

func TestDecideTrustLineReaderOutputAndDecisionsRemainExact(t *testing.T) {
	prompt := app.TrustDecisionPrompt{
		Status: app.HostTrustChanged,
		Host: app.PresentedHost{
			Endpoint:          app.HostEndpoint{CanonicalHost: "host.example", Port: 22},
			RemoteAddress:     "192.0.2.10:22",
			KeyAlgorithm:      "ssh-ed25519",
			FingerprintSHA256: "SHA256:new",
		},
		Known: &app.TrustedHost{FingerprintSHA256: "SHA256:old"},
	}
	wantOutput := "Verify host identity\n\n" +
		"Host: host.example:22\n" +
		"Remote address: 192.0.2.10:22\n" +
		"Algorithm: ssh-ed25519\n" +
		"SHA-256 fingerprint: SHA256:new\n" +
		"Known fingerprint: SHA256:old\n" +
		"WARNING: changed key; this may indicate a possible attack.\n\n" +
		"> Reject (default)    Trust once    Trust and persist\n" +
		"Decision [reject/once/persist] (reject): "

	for _, test := range []struct {
		input string
		want  app.TrustDecision
	}{
		{input: "y\n", want: app.TrustOnce},
		{input: "once\n", want: app.TrustOnce},
		{input: "p\n", want: app.TrustPersist},
		{input: "persist\n", want: app.TrustPersist},
		{input: "\n", want: app.TrustReject},
		{input: "unexpected\n", want: app.TrustReject},
	} {
		var output bytes.Buffer
		decision, err := decideTrust(context.Background(), strings.NewReader(test.input), &output, prompt, true)
		if err != nil || decision != test.want {
			t.Fatalf("input %q decision/error = %q/%v, want %q/nil", test.input, decision, err, test.want)
		}
		if got := output.String(); got != wantOutput {
			t.Fatalf("line-reader output = %q, want %q", got, wantOutput)
		}
	}
}

func assertColonlessPromptLabel(t *testing.T, view, label string) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.HasPrefix(line, label+":") {
			t.Fatalf("label %q retained a colon: %q", label, line)
		}
		if strings.HasPrefix(line, label+" ") {
			return
		}
	}
	t.Fatalf("colonless label %q not found: %q", label, view)
}

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
			if decision, decided := prompt.update(keyPress("p")); !decided || decision != app.TrustPersist {
				t.Fatalf("run %d %s: explicit persist failed", run, status)
			}
			prompt = newTrustPrompt(app.TrustDecisionPrompt{Status: status})
			if decision, decided := prompt.update(keyPress("esc")); !decided || decision != app.TrustReject {
				t.Fatalf("run %d %s: cancel failed", run, status)
			}
		}
		revoked := newTrustPrompt(app.TrustDecisionPrompt{Status: app.HostTrustRevoked})
		if decision, decided := revoked.update(keyPress("y")); decided || decision != app.TrustReject {
			t.Fatalf("run %d revoked: accepted trust", run)
		}
		if decision, decided := revoked.update(keyPress("esc")); !decided || decision != app.TrustReject {
			t.Fatalf("run %d revoked: cancel failed", run)
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
		colored := newTrustPrompt(prompt).view(newStyles(false))
		projection := safeText(unsafeValue, 256)

		if count := strings.Count(view, projection); count != 5 {
			t.Fatalf("projection %q appears %d times, want every dynamic trust value:\n%s", projection, count, view)
		}
		if stripped := ansi.Strip(colored); stripped != view {
			t.Fatalf("unsafe value changed color/plain semantics: color %q, plain %q", stripped, view)
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
