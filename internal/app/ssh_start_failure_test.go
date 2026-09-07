package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSSHStartErrorPresentationMatrix(t *testing.T) {
	tests := []struct {
		reason         SSHFailureReason
		stage          SSHFailureStage
		detail         string
		wantSummary    string
		wantRecommend  string
		wantSafeDetail string
	}{
		{SSHFailureTimeout, SSHFailureStageNetworkConnection, "operation timed out", "Operation timed out", "Check connectivity and timeout settings, then retry.", "operation timed out"},
		{SSHFailureAuthenticationDenied, SSHFailureStageAuthentication, "server rejected available authentication methods", "Permission denied", "Check the username and credentials, then retry.", "server rejected available authentication methods"},
		{SSHFailureConnectionRefused, SSHFailureStageNetworkConnection, "remote endpoint refused connection", "The endpoint refused the connection", "Check the host, port, and SSH service.", "remote endpoint refused connection"},
		{SSHFailureHostNotFound, SSHFailureStageTargetResolution, "host name could not be resolved", "The host could not be resolved", "Check the host name and DNS configuration.", "host name could not be resolved"},
		{SSHFailureNetworkUnreachable, SSHFailureStageNetworkConnection, "network is unreachable", "The network is unreachable", "Check the network, VPN, and routing.", "network is unreachable"},
		{SSHFailureHostTrust, SSHFailureStageHostTrust, "changed", "The host identity was not accepted", "Check the host fingerprint and trust policy.", "changed"},
		{SSHFailureCredentialUnavailable, SSHFailureStageCredential, "identity file", "The local credential is unavailable", "Check the agent, secure store, identity file, or prompt.", "identity file"},
		{SSHFailureSSHNegotiation, SSHFailureStageSSHNegotiation, "handshake", "SSH negotiation or session setup failed", "Check SSH, PTY, and shell compatibility.", "handshake"},
		{SSHFailureCanceled, SSHFailureStageLocalTerminal, "operation canceled", "The attempt was canceled", "Retry when ready.", "operation canceled"},
		{SSHFailureUnexpected, SSHFailureStageUnknown, "password=secret-canary", "SSH startup failed unexpectedly", "Check the configuration or retry.", ""},
	}

	for _, test := range tests {
		t.Run(string(test.reason), func(t *testing.T) {
			cause := errors.New("private-cause-canary")
			failure := NewSSHStartError(test.reason, test.stage, test.detail, cause)
			presentation := failure.Presentation()
			if presentation.Category != test.reason || presentation.Stage != test.stage || presentation.Summary != test.wantSummary || presentation.Recommendation != test.wantRecommend || presentation.TechnicalDetail != test.wantSafeDetail {
				t.Fatalf("Presentation() = %#v", presentation)
			}
			if !errors.Is(failure, cause) {
				t.Fatal("SSHStartError does not unwrap its cause")
			}
			if strings.Contains(failure.Error(), "private-cause-canary") || strings.Contains(failure.Error(), test.detail) {
				t.Fatalf("Error() exposed non-identifier data: %q", failure.Error())
			}
		})
	}
}

func TestSSHFailureStableValues(t *testing.T) {
	reasons := []SSHFailureReason{SSHFailureTimeout, SSHFailureAuthenticationDenied, SSHFailureConnectionRefused, SSHFailureHostNotFound, SSHFailureNetworkUnreachable, SSHFailureHostTrust, SSHFailureCredentialUnavailable, SSHFailureSSHNegotiation, SSHFailureCanceled, SSHFailureUnexpected}
	wantReasons := []string{"timeout", "authentication_denied", "connection_refused", "host_not_found", "network_unreachable", "host_trust", "credential_unavailable", "ssh_negotiation", "canceled", "unexpected"}
	for i := range reasons {
		if string(reasons[i]) != wantReasons[i] {
			t.Fatalf("reason %d = %q, want %q", i, reasons[i], wantReasons[i])
		}
	}

	stages := []SSHFailureStage{SSHFailureStageTargetResolution, SSHFailureStageNetworkConnection, SSHFailureStageHostTrust, SSHFailureStageSSHNegotiation, SSHFailureStageCredential, SSHFailureStageAuthentication, SSHFailureStageSessionSetup, SSHFailureStageLocalTerminal, SSHFailureStageUnknown}
	wantStages := []string{"target_resolution", "network_connection", "host_trust", "ssh_negotiation", "credential", "authentication", "session_setup", "local_terminal", "unknown"}
	for i := range stages {
		if string(stages[i]) != wantStages[i] {
			t.Fatalf("stage %d = %q, want %q", i, stages[i], wantStages[i])
		}
	}
}

func TestNormalizeSSHStartErrorPrecedence(t *testing.T) {
	private := errors.New("private-cause-canary")
	normalized := NewSSHStartError(SSHFailureHostTrust, SSHFailureStageHostTrust, "rejected", private)

	tests := []struct {
		name       string
		stage      SSHFailureStage
		cause      error
		wantReason SSHFailureReason
		wantStage  SSHFailureStage
		wantSame   bool
	}{
		{"explicit cancellation wins", SSHFailureStageAuthentication, errors.Join(normalized, context.Canceled), SSHFailureCanceled, SSHFailureStageAuthentication, false},
		{"normalized is preserved", SSHFailureStageUnknown, normalized, SSHFailureHostTrust, SSHFailureStageHostTrust, true},
		{"deadline is timeout", SSHFailureStageNetworkConnection, context.DeadlineExceeded, SSHFailureTimeout, SSHFailureStageNetworkConnection, false},
		{"unknown fallback", SSHFailureStageCredential, private, SSHFailureUnexpected, SSHFailureStageCredential, false},
		{"invalid stage fallback", SSHFailureStage("private-stage"), private, SSHFailureUnexpected, SSHFailureStageUnknown, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NormalizeSSHStartError(test.stage, test.cause)
			if got.Reason() != test.wantReason || got.Stage() != test.wantStage {
				t.Fatalf("NormalizeSSHStartError() = %q/%q", got.Reason(), got.Stage())
			}
			if test.wantSame && got != normalized {
				t.Fatal("normalized error was replaced")
			}
			if strings.Contains(got.Error(), "private") || strings.Contains(got.Presentation().TechnicalDetail, "private") {
				t.Fatalf("normalization exposed cause: %q %#v", got.Error(), got.Presentation())
			}
		})
	}
}

func TestNewSSHStartErrorInvalidValuesUseSafeFallback(t *testing.T) {
	failure := NewSSHStartError(SSHFailureReason("secret-reason"), SSHFailureStage("secret-stage"), "secret-detail", errors.New("secret-cause"))
	if failure.Reason() != SSHFailureUnexpected || failure.Stage() != SSHFailureStageUnknown || failure.Presentation().TechnicalDetail != "" {
		t.Fatalf("invalid values were retained: %#v", failure.Presentation())
	}
	if strings.Contains(failure.Error(), "secret") {
		t.Fatalf("unsafe Error() = %q", failure.Error())
	}
}

func TestSSHStartErrorAcceptsOnlyExhaustiveSafeTechnicalDetails(t *testing.T) {
	allowed := map[SSHFailureReason]map[string]struct{}{
		SSHFailureTimeout:               {"operation timed out": {}},
		SSHFailureAuthenticationDenied:  {"server rejected available authentication methods": {}},
		SSHFailureConnectionRefused:     {"remote endpoint refused connection": {}},
		SSHFailureHostNotFound:          {"host name could not be resolved": {}},
		SSHFailureNetworkUnreachable:    {"network is unreachable": {}},
		SSHFailureHostTrust:             {"unknown": {}, "changed": {}, "revoked": {}, "rejected": {}},
		SSHFailureCredentialUnavailable: {"agent": {}, "secure store": {}, "identity file": {}, "secret prompt": {}},
		SSHFailureSSHNegotiation:        {"handshake": {}, "session": {}, "pty": {}, "shell": {}},
		SSHFailureCanceled:              {"operation canceled": {}},
	}
	if !reflect.DeepEqual(allowed, sshFailureDetails) {
		t.Fatalf("guarded technical-detail inventory differs from production\n got: %#v\nwant: %#v", sshFailureDetails, allowed)
	}
	stages := map[SSHFailureReason]SSHFailureStage{
		SSHFailureTimeout: SSHFailureStageNetworkConnection, SSHFailureAuthenticationDenied: SSHFailureStageAuthentication,
		SSHFailureConnectionRefused: SSHFailureStageNetworkConnection, SSHFailureHostNotFound: SSHFailureStageTargetResolution,
		SSHFailureNetworkUnreachable: SSHFailureStageNetworkConnection, SSHFailureHostTrust: SSHFailureStageHostTrust,
		SSHFailureCredentialUnavailable: SSHFailureStageCredential, SSHFailureSSHNegotiation: SSHFailureStageSessionSetup,
		SSHFailureCanceled: SSHFailureStageLocalTerminal,
	}
	const rawCause = "RAW-CAUSE-password=SECRET-CANARY"
	for reason, details := range allowed {
		for detail := range details {
			failure := NewSSHStartError(reason, stages[reason], detail, errors.New(rawCause))
			presentation := failure.Presentation()
			if presentation.TechnicalDetail != detail {
				t.Fatalf("detail %q for %q was rejected", detail, reason)
			}
			if width := ansi.StringWidth(presentation.TechnicalDetail); width > 256 {
				t.Fatalf("detail %q occupies %d visible cells", detail, width)
			}
			for field, value := range map[string]string{
				"summary": presentation.Summary, "recommendation": presentation.Recommendation, "technical detail": presentation.TechnicalDetail,
			} {
				if strings.Contains(value, rawCause) || strings.Contains(value, "SECRET-CANARY") || strings.Contains(value, "password=") {
					t.Fatalf("%s for %q/%q exposed a secret or raw cause: %q", field, reason, detail, value)
				}
			}
		}
		failure := NewSSHStartError(reason, stages[reason], "untrusted server text", errors.New(rawCause))
		if failure.Presentation().TechnicalDetail != "" {
			t.Fatalf("unlisted detail for %q was accepted", reason)
		}
	}
}
