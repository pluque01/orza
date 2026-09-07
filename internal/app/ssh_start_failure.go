package app

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

type SSHFailureReason string

const (
	SSHFailureTimeout               SSHFailureReason = "timeout"
	SSHFailureAuthenticationDenied  SSHFailureReason = "authentication_denied"
	SSHFailureConnectionRefused     SSHFailureReason = "connection_refused"
	SSHFailureHostNotFound          SSHFailureReason = "host_not_found"
	SSHFailureNetworkUnreachable    SSHFailureReason = "network_unreachable"
	SSHFailureHostTrust             SSHFailureReason = "host_trust"
	SSHFailureCredentialUnavailable SSHFailureReason = "credential_unavailable"
	SSHFailureSSHNegotiation        SSHFailureReason = "ssh_negotiation"
	SSHFailureCanceled              SSHFailureReason = "canceled"
	SSHFailureUnexpected            SSHFailureReason = "unexpected"
)

type SSHFailureStage string

const (
	SSHFailureStageTargetResolution  SSHFailureStage = "target_resolution"
	SSHFailureStageNetworkConnection SSHFailureStage = "network_connection"
	SSHFailureStageHostTrust         SSHFailureStage = "host_trust"
	SSHFailureStageSSHNegotiation    SSHFailureStage = "ssh_negotiation"
	SSHFailureStageCredential        SSHFailureStage = "credential"
	SSHFailureStageAuthentication    SSHFailureStage = "authentication"
	SSHFailureStageSessionSetup      SSHFailureStage = "session_setup"
	SSHFailureStageLocalTerminal     SSHFailureStage = "local_terminal"
	SSHFailureStageUnknown           SSHFailureStage = "unknown"
)

type sshFailureCopy struct {
	summary        string
	recommendation string
}

var sshFailureCopies = map[SSHFailureReason]sshFailureCopy{
	SSHFailureTimeout:               {"Operation timed out", "Check connectivity and timeout settings, then retry."},
	SSHFailureAuthenticationDenied:  {"Permission denied", "Check the username and credentials, then retry."},
	SSHFailureConnectionRefused:     {"The endpoint refused the connection", "Check the host, port, and SSH service."},
	SSHFailureHostNotFound:          {"The host could not be resolved", "Check the host name and DNS configuration."},
	SSHFailureNetworkUnreachable:    {"The network is unreachable", "Check the network, VPN, and routing."},
	SSHFailureHostTrust:             {"The host identity was not accepted", "Check the host fingerprint and trust policy."},
	SSHFailureCredentialUnavailable: {"The local credential is unavailable", "Check the agent, secure store, identity file, or prompt."},
	SSHFailureSSHNegotiation:        {"SSH negotiation or session setup failed", "Check SSH, PTY, and shell compatibility."},
	SSHFailureCanceled:              {"The attempt was canceled", "Retry when ready."},
	SSHFailureUnexpected:            {"SSH startup failed unexpectedly", "Check the configuration or retry."},
}

var sshFailureDetails = map[SSHFailureReason]map[string]struct{}{
	SSHFailureTimeout:              {"operation timed out": {}},
	SSHFailureAuthenticationDenied: {"server rejected available authentication methods": {}},
	SSHFailureConnectionRefused:    {"remote endpoint refused connection": {}},
	SSHFailureHostNotFound:         {"host name could not be resolved": {}},
	SSHFailureNetworkUnreachable:   {"network is unreachable": {}},
	SSHFailureHostTrust: {
		"unknown": {}, "changed": {}, "revoked": {}, "rejected": {},
	},
	SSHFailureCredentialUnavailable: {
		"agent": {}, "secure store": {}, "identity file": {}, "secret prompt": {},
	},
	SSHFailureSSHNegotiation: {
		"handshake": {}, "session": {}, "pty": {}, "shell": {},
	},
	SSHFailureCanceled: {"operation canceled": {}},
}

// SSHStartError preserves the original cause for programmatic inspection while
// exposing only controlled diagnostic fields to callers and presenters.
type SSHStartError struct {
	reason SSHFailureReason
	stage  SSHFailureStage
	detail string
	cause  error
}

func NewSSHStartError(reason SSHFailureReason, stage SSHFailureStage, detail string, cause error) *SSHStartError {
	if !validSSHFailureCombination(reason, stage) {
		reason = SSHFailureUnexpected
		stage = SSHFailureStageUnknown
		detail = ""
	}
	return &SSHStartError{
		reason: reason,
		stage:  stage,
		detail: allowedSSHFailureDetail(reason, detail),
		cause:  cause,
	}
}

func (e *SSHStartError) Error() string {
	if e == nil {
		return "SSH startup failed: unexpected at unknown"
	}
	return "SSH startup failed: " + string(e.reason) + " at " + string(e.stage)
}

func (e *SSHStartError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *SSHStartError) Reason() SSHFailureReason {
	if e == nil {
		return SSHFailureUnexpected
	}
	return e.reason
}

func (e *SSHStartError) Stage() SSHFailureStage {
	if e == nil {
		return SSHFailureStageUnknown
	}
	return e.stage
}

func (e *SSHStartError) Presentation() SSHFailurePresentation {
	if e == nil {
		e = NewSSHStartError(SSHFailureUnexpected, SSHFailureStageUnknown, "", nil)
	}
	copy := sshFailureCopies[e.reason]
	return SSHFailurePresentation{
		Category:        e.reason,
		Stage:           e.stage,
		Summary:         copy.summary,
		Recommendation:  copy.recommendation,
		TechnicalDetail: e.detail,
	}
}

// NormalizeSSHStartError applies application-level precedence. Adapters should
// construct specific typed failures while they still know the blocking stage.
func NormalizeSSHStartError(stage SSHFailureStage, cause error) *SSHStartError {
	if !validSSHFailureStage(stage) {
		stage = SSHFailureStageUnknown
	}
	if errors.Is(cause, context.Canceled) {
		return NewSSHStartError(SSHFailureCanceled, stage, "operation canceled", cause)
	}
	var normalized *SSHStartError
	if errors.As(cause, &normalized) {
		return normalized
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return NewSSHStartError(SSHFailureTimeout, stage, "operation timed out", cause)
	}
	return NewSSHStartError(SSHFailureUnexpected, stage, "", cause)
}

func validSSHFailureCombination(reason SSHFailureReason, stage SSHFailureStage) bool {
	if !validSSHFailureStage(stage) {
		return false
	}
	switch reason {
	case SSHFailureTimeout, SSHFailureCanceled:
		return true
	case SSHFailureAuthenticationDenied:
		return stage == SSHFailureStageAuthentication
	case SSHFailureConnectionRefused, SSHFailureNetworkUnreachable:
		return stage == SSHFailureStageNetworkConnection
	case SSHFailureHostNotFound:
		return stage == SSHFailureStageTargetResolution
	case SSHFailureHostTrust:
		return stage == SSHFailureStageHostTrust
	case SSHFailureCredentialUnavailable:
		return stage == SSHFailureStageCredential || stage == SSHFailureStageAuthentication
	case SSHFailureSSHNegotiation:
		return stage == SSHFailureStageSSHNegotiation || stage == SSHFailureStageSessionSetup
	case SSHFailureUnexpected:
		return true
	default:
		return false
	}
}

func validSSHFailureStage(stage SSHFailureStage) bool {
	switch stage {
	case SSHFailureStageTargetResolution, SSHFailureStageNetworkConnection, SSHFailureStageHostTrust,
		SSHFailureStageSSHNegotiation, SSHFailureStageCredential, SSHFailureStageAuthentication,
		SSHFailureStageSessionSetup, SSHFailureStageLocalTerminal, SSHFailureStageUnknown:
		return true
	default:
		return false
	}
}

func allowedSSHFailureDetail(reason SSHFailureReason, detail string) string {
	if _, allowed := sshFailureDetails[reason][detail]; !allowed {
		return ""
	}
	if reason == SSHFailureCredentialUnavailable && detail == "secret prompt" {
		return detail
	}
	return sanitizeSSHFailureDetail(detail)
}

func sanitizeSSHFailureDetail(input string) string {
	input = strings.ToValidUTF8(input, "")
	lower := strings.ToLower(input)
	for _, sensitive := range []string{"password", "passphrase", "private key", "credential", "secret"} {
		if strings.Contains(lower, sensitive) {
			return ""
		}
	}

	var cleaned strings.Builder
	cleaned.Grow(len(input))
	for i := 0; i < len(input); {
		r, size := utf8.DecodeRuneInString(input[i:])
		i += size
		if r == '\x1b' {
			if i < len(input) && input[i] == '[' {
				i++
				for i < len(input) {
					next, nextSize := utf8.DecodeRuneInString(input[i:])
					i += nextSize
					if next >= 0x40 && next <= 0x7e {
						break
					}
				}
			} else if i < len(input) {
				_, nextSize := utf8.DecodeRuneInString(input[i:])
				i += nextSize
			}
			continue
		}
		if isBidiControl(r) || unicode.IsControl(r) {
			cleaned.WriteByte(' ')
			continue
		}
		cleaned.WriteRune(r)
	}
	output := strings.Join(strings.Fields(cleaned.String()), " ")
	if ansi.StringWidth(output) > 256 {
		output = ansi.Truncate(output, 256, "…")
	}
	return output
}

func isBidiControl(r rune) bool {
	return r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069'
}
