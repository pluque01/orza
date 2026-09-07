package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

type trustPrompt struct {
	prompt   app.TrustDecisionPrompt
	decision app.TrustDecision
}

const trustPromptValueWidth = 256

func newTrustPrompt(prompt app.TrustDecisionPrompt) *trustPrompt {
	return &trustPrompt{prompt: prompt, decision: app.TrustReject}
}

func (p *trustPrompt) view(style styles) string {
	host := p.prompt.Host
	var out strings.Builder
	out.WriteString(style.warning.Render("Verify host identity") + "\n\n")
	out.WriteString(fmt.Sprintf("Host: %s:%d\n", safeText(host.Endpoint.CanonicalHost, trustPromptValueWidth), host.Endpoint.Port))
	out.WriteString("Remote address: " + safeText(host.RemoteAddress, trustPromptValueWidth) + "\n")
	out.WriteString("Algorithm: " + safeText(host.KeyAlgorithm, trustPromptValueWidth) + "\n")
	out.WriteString("SHA-256 fingerprint: " + safeText(host.FingerprintSHA256, trustPromptValueWidth) + "\n")
	if p.prompt.Status == app.HostTrustChanged {
		if p.prompt.Known != nil {
			out.WriteString("Known fingerprint: " + safeText(p.prompt.Known.FingerprintSHA256, trustPromptValueWidth) + "\n")
		}
		out.WriteString(style.failure.Render("WARNING: changed key; this may indicate a possible attack.") + "\n")
	}
	if p.prompt.Status == app.HostTrustRevoked {
		out.WriteString("\nThis key is revoked. Esc Back  Q Quit")
		return out.String()
	}
	out.WriteString("\n> Reject (default)    Trust once    Trust and persist")
	return out.String()
}

func (p *trustPrompt) update(msg tea.KeyPressMsg) (app.TrustDecision, bool) {
	if p.prompt.Status == app.HostTrustRevoked {
		return app.TrustReject, msg.String() == "esc" || msg.String() == "q" || msg.String() == "ctrl+c"
	}
	switch msg.String() {
	case "y":
		p.decision = app.TrustOnce
		return p.decision, true
	case "p":
		p.decision = app.TrustPersist
		return p.decision, true
	case "esc", "q", "ctrl+c":
		p.decision = app.TrustReject
		return p.decision, true
	default:
		return app.TrustReject, false
	}
}

func decideTrust(ctx context.Context, input io.Reader, output io.Writer, prompt app.TrustDecisionPrompt, noColor bool) (app.TrustDecision, error) {
	if err := ctx.Err(); err != nil {
		return app.TrustReject, err
	}
	view := newTrustPrompt(prompt).view(newStyles(noColor))
	if _, err := fmt.Fprintln(output, view); err != nil {
		return app.TrustReject, err
	}
	if _, err := fmt.Fprint(output, "Decision [reject/once/persist] (reject): "); err != nil {
		return app.TrustReject, err
	}
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && line == "" {
		if ctx.Err() != nil {
			return app.TrustReject, ctx.Err()
		}
		if !errors.Is(err, io.EOF) {
			return app.TrustReject, err
		}
		return app.TrustReject, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "once":
		return app.TrustOnce, nil
	case "p", "persist":
		return app.TrustPersist, nil
	default:
		return app.TrustReject, nil
	}
}
