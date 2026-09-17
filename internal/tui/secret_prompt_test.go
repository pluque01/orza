package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestSecretPromptUsesControlledBadgeColonlessFieldAndColorParity(t *testing.T) {
	fixtures := []struct {
		kind  app.SecretKind
		label string
	}{
		{kind: app.SecretPassword, label: "Password"},
		{kind: app.SecretPassphrase, label: "Private key passphrase"},
	}
	for _, fixture := range fixtures {
		t.Run(string(fixture.kind), func(t *testing.T) {
			prompt := newSecretPrompt(fixture.kind, "system store")
			prompt.setForTest([]byte("a界b"))
			plain := prompt.view(newStyles(true))
			colored := prompt.view(newStyles(false))

			if got := ansi.Strip(colored); got != plain {
				t.Fatalf("color text semantics = %q, want %q", got, plain)
			}
			if strings.Count(plain, fixture.label) != 1 {
				t.Fatalf("controlled badge inventory = %q", plain)
			}
			if !strings.Contains(colored, "\x1b[") {
				t.Fatalf("color prompt has no styling: %q", colored)
			}
			assertColonlessPromptLabel(t, plain, "Secret")
			if strings.Contains(plain, "a界b") || !strings.Contains(plain, "Secret ***") {
				t.Fatalf("secret masking changed: %q", plain)
			}
		})
	}
}

func TestSecretPromptProjectsStoreNameSafelyBeforeRendering(t *testing.T) {
	unsafeStoreName := "store\x1b[31m\ncredential\u202e-canary"
	prompt := newSecretPrompt(app.SecretPassword, unsafeStoreName)
	plain := prompt.view(newStyles(true))
	colored := prompt.view(newStyles(false))
	projection := safeText(unsafeStoreName, 256)

	if !strings.Contains(plain, "Store in "+projection+" (optional, unchecked by default)") {
		t.Fatalf("safe store projection missing: %q", plain)
	}
	if strings.Contains(ansi.Strip(colored), "\x1b[31m") || ansi.Strip(colored) != plain {
		t.Fatalf("store value affected styling or parity: %q", colored)
	}
}

func TestUS4PasswordPassphraseNeverRenderRawTwentyRuns(t *testing.T) {
	for run := range 20 {
		for _, kind := range []app.SecretKind{app.SecretPassword, app.SecretPassphrase} {
			prompt := newSecretPrompt(kind, "store")
			prompt.setForTest([]byte("private-canary"))
			view := prompt.view(newStyles(true))
			if strings.Contains(view, "private-canary") || !strings.Contains(view, strings.Repeat("*", 14)) {
				t.Fatalf("run %d %s view = %q", run, kind, view)
			}
			prompt.clear()
			if prompt.masked != 0 {
				t.Fatalf("run %d %s retained input", run, kind)
			}
		}
	}
}
