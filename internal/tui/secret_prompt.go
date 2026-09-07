package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/pluque01/orza/internal/app"
)

// secretPrompt contains display state only. Secret bytes are owned by the
// terminal adapter and passed directly to the application service.
type secretPrompt struct {
	kind      app.SecretKind
	storeName string
	consent   bool
	masked    int
}

func newSecretPrompt(kind app.SecretKind, storeName string) *secretPrompt {
	return &secretPrompt{kind: kind, storeName: storeName}
}

func (p *secretPrompt) view(style styles) string {
	label := "Private key passphrase"
	if p.kind == app.SecretPassword {
		label = "Password"
	}
	var out strings.Builder
	out.WriteString(style.title.Render(label) + "\n\n")
	out.WriteString("Input is read by the terminal with echo disabled and is not retained by the TUI.\n")
	if p.kind == app.SecretPassword && p.storeName != "" {
		mark := " "
		if p.consent {
			mark = "x"
		}
		out.WriteString("[" + mark + "] Store in " + p.storeName + " (optional, unchecked by default)\n")
	}
	if p.kind == app.SecretPassphrase {
		out.WriteString("Passphrases are never persisted.\n")
	}
	out.WriteString("Secret: " + strings.Repeat("*", p.masked) + "\nEnter Submit  Esc Cancel  F1 Help  Ctrl+C Quit")
	return out.String()
}

func (p *secretPrompt) setForTest(secret []byte) {
	p.masked = utf8.RuneCount(secret)
	for index := range secret {
		secret[index] = 0
	}
}

func (p *secretPrompt) clear() {
	p.masked = 0
}

func (p *secretPrompt) take() []byte {
	p.clear()
	return nil
}
