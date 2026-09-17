package tui

import (
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestUS4SecurityInputOwnershipResizeAndMaskingTwentyRuns(t *testing.T) {
	for run := range 20 {
		model := New(Config{Width: 80, Height: 24, NoColor: true})
		model.openGenericModal(modalKindHelp, nil, helpPayload{lines: []string{"modal owner"}})
		state, ok := newSecurityInputState(securityInputSecret, model.focusOwner)
		if !ok || !state.preemptsApplicationInput() || state.preservedFocus != focusOwnerModal {
			t.Fatalf("run %d: invalid security owner", run)
		}
		state.secret = newSecretPrompt(app.SecretPassphrase, "")
		state.secret.setForTest([]byte("canary-secret"))
		if view := state.secret.view(newStyles(true)); strings.Contains(view, "canary-secret") || !strings.Contains(view, "*************") {
			t.Fatalf("run %d: secret was not masked: %q", run, view)
		}
		state = state.resize(39, 11)
		if !state.suspended || !model.modal.isOpen() {
			t.Fatalf("run %d: undersized did not suspend and preserve owner", run)
		}
		state = state.resize(40, 12)
		if state.suspended || state.close() != focusOwnerModal {
			t.Fatalf("run %d: resize recovery changed opener", run)
		}
		state.secret.clear()
		if state.secret.masked != 0 {
			t.Fatalf("run %d: cleanup retained secret", run)
		}
	}
}

func TestSecurityInputProjectsNoSecretOrCredentialData(t *testing.T) {
	const (
		secretCanary     = "raw-secret-canary"
		credentialCanary = "credential-reference-canary"
		promptCanary     = "request-prompt-canary"
	)
	request := app.SecretRequest{
		Kind:       app.SecretPassword,
		Prompt:     promptCanary,
		Credential: credentialCanary,
	}
	prompt := newSecretPrompt(request.Kind, "")
	raw := []byte(secretCanary)
	prompt.setForTest(raw)
	view := prompt.view(newStyles(true))

	for _, canary := range []string{secretCanary, credentialCanary, promptCanary} {
		if strings.Contains(view, canary) {
			t.Fatalf("security presentation projected %q: %q", canary, view)
		}
	}
	if !strings.Contains(view, strings.Repeat("*", len(secretCanary))) {
		t.Fatalf("security presentation lost exact masking length: %q", view)
	}
	for index, value := range raw {
		if value != 0 {
			t.Fatalf("secret byte %d was not wiped: %q", index, raw)
		}
	}
}
