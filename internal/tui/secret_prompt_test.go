package tui

import (
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

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
