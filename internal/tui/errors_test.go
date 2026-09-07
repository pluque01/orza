package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestSSHFailureModalRendersSafeProjectionMatrix(t *testing.T) {
	tests := []struct {
		category       app.SSHFailureReason
		stage          app.SSHFailureStage
		summary        string
		recommendation string
	}{
		{app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "Operation timed out", "Check connectivity and timeout settings, then retry."},
		{app.SSHFailureAuthenticationDenied, app.SSHFailureStageAuthentication, "Permission denied", "Check the username and credentials, then retry."},
		{app.SSHFailureConnectionRefused, app.SSHFailureStageNetworkConnection, "The endpoint refused the connection", "Check the host, port, and SSH service."},
		{app.SSHFailureHostNotFound, app.SSHFailureStageTargetResolution, "The host could not be resolved", "Check the host name and DNS configuration."},
		{app.SSHFailureNetworkUnreachable, app.SSHFailureStageNetworkConnection, "The network is unreachable", "Check the network, VPN, and routing."},
		{app.SSHFailureHostTrust, app.SSHFailureStageHostTrust, "The host identity was not accepted", "Check the host fingerprint and trust policy."},
		{app.SSHFailureCredentialUnavailable, app.SSHFailureStageCredential, "The local credential is unavailable", "Check the agent, secure store, identity file, or prompt."},
		{app.SSHFailureSSHNegotiation, app.SSHFailureStageSSHNegotiation, "SSH negotiation or session setup failed", "Check SSH, PTY, and shell compatibility."},
		{app.SSHFailureCanceled, app.SSHFailureStageLocalTerminal, "The attempt was canceled", "Retry when ready."},
		{app.SSHFailureUnexpected, app.SSHFailureStageUnknown, "SSH startup failed unexpectedly", "Check the configuration or retry."},
	}
	attempt := app.SSHAttemptTarget{ID: "captured", Revision: 7, Path: "/work/prod", Host: "prod.test", Port: 2202}

	for _, test := range tests {
		t.Run(string(test.category), func(t *testing.T) {
			failure := app.NewSSHStartError(test.category, test.stage, allowedDetail(test.category), errors.New("wrapped-private-canary")).Presentation()
			modal := newSSHFailureModal(attempt, failure)
			view := strings.Join(sshFailureLines(modal, 80), "\n")
			for _, want := range []string{
				"SSH startup failed", test.summary, "Category: " + string(test.category),
				"Path: /work/prod", "Endpoint: prod.test:2202", "Stage: " + string(test.stage),
				"Recommendation: " + test.recommendation,
			} {
				if !strings.Contains(view, want) {
					t.Fatalf("view omitted %q: %q", want, view)
				}
			}
			if modal.detailVisible || strings.Contains(view, "Technical detail:") || strings.Contains(view, "wrapped-private-canary") {
				t.Fatalf("detail was visible or cause leaked by default: %q", view)
			}
			modal.detailVisible = true
			expanded := strings.Join(sshFailureLines(modal, 80), "\n")
			if failure.TechnicalDetail != "" && !strings.Contains(expanded, "Technical detail: "+failure.TechnicalDetail) {
				t.Fatalf("expanded view omitted safe detail: %q", expanded)
			}
			if strings.Contains(expanded, "wrapped-private-canary") {
				t.Fatalf("expanded view exposed wrapped cause: %q", expanded)
			}
		})
	}
}

func TestSSHFailureModalFallbackNoColorAndCapturedTarget(t *testing.T) {
	const canary = "private-error-chain-canary"
	failure := app.NewSSHStartError(app.SSHFailureUnexpected, app.SSHFailureStageUnknown, canary, errors.New(canary)).Presentation()
	modal := newSSHFailureModal(app.SSHAttemptTarget{Path: "/captured", Host: "2001:db8::1", Port: 22}, failure)
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.installSSHFailure(modal)
	view := model.View().Content

	for _, want := range []string{"Category: unexpected", "Path: /captured", "Endpoint: [2001:db8::1]:22", "Stage: unknown", "Check the configuration or retry."} {
		if !strings.Contains(view, want) {
			t.Fatalf("fallback view omitted %q: %q", want, view)
		}
	}
	if strings.Contains(view, canary) || strings.Contains(view, "\x1b[") {
		t.Fatalf("fallback/no-color view exposed unsafe or styled content: %q", view)
	}
}

func TestSessionFailureRoutingAndCompletionToModelUpdate(t *testing.T) {
	const canary = "raw-session-cause-canary"
	connection := testConnection("captured", syntheticRootID, "/captured", 3)
	connection.Host = "captured.test"
	connection.Port = 2222
	attempt := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
	failure := app.NewSSHStartError(app.SSHFailureAuthenticationDenied, app.SSHFailureStageAuthentication, "server rejected available authentication methods", errors.New(canary)).Presentation()
	result := app.ConnectResult{Connection: connection, Attempt: attempt, Session: app.SSHSessionResult{State: app.SessionFailed, Outcome: app.SessionOutcomeTransportFailure, Failure: &failure}}
	connectErr := errors.New(canary)
	completed := make(chan time.Time, 1)
	service := ConnectFunc(func(context.Context, app.ConnectRequest) (app.ConnectResult, error) {
		completed <- time.Now()
		return result, connectErr
	})
	model := New(Config{Connect: service, Width: 80, Height: 24, NoColor: true})
	_, _, _ = model.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerModal)
	command := sessionExecCommand{ctx: context.Background(), service: service}
	if err := command.Run(); !errors.Is(err, connectErr) {
		t.Fatalf("session command error = %v", err)
	}
	finishedAt := <-completed
	updateModel(model, sessionFinishedMsg{id: 1, result: command.result, err: command.err})
	if elapsed := time.Since(finishedAt); elapsed >= time.Second {
		t.Fatalf("completion-to-model update took %v", elapsed)
	}
	if model.activeSSHFailure() == nil || model.sessionErr != nil {
		t.Fatalf("pre-active failure did not remain in the model: modal=%#v sessionErr=%v", model.modal, model.sessionErr)
	}
	view := model.View().Content
	if !strings.Contains(view, "/captured") || !strings.Contains(view, "captured.test:2222") || strings.Contains(view, canary) {
		t.Fatalf("pre-active diagnostic used the wrong or unsafe target: %q", view)
	}

	model.closeGenericModal()
	started := result
	started.Session.StartedAt = time.Now()
	started.Session.Failure = nil
	_, _, _ = model.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerModal)
	postActiveErr := errors.New("post-active")
	updateModel(model, sessionFinishedMsg{id: 2, result: started, err: postActiveErr})
	if model.modal.isOpen() || !errors.Is(model.sessionErr, postActiveErr) {
		t.Fatalf("post-active failure was converted to a startup modal: modal=%#v err=%v", model.modal, model.sessionErr)
	}
}

func TestGeneralErrorModalRemainsUnchanged(t *testing.T) {
	modal := newErrorModal("move", "/prod", app.ErrConflict)
	view := strings.Join(operationErrorLines(modal, 80), "\n")
	if strings.Contains(view, "SSH startup failed") || !strings.Contains(view, "Recoverable operation error") || !strings.Contains(view, "r Reload") {
		t.Fatalf("general modal behavior changed: %q", view)
	}
}

func TestGeneralErrorModalMapsApplicationErrorsToControlledTUICopy(t *testing.T) {
	const (
		applicationOperation = "APPLICATION-OPERATION-CANARY"
		applicationSubject   = "APPLICATION-SUBJECT-CANARY"
	)
	target := "/user/\x1b[31mprod\r\nnext"
	kinds := []app.ErrorKind{
		app.ErrorKindInvalid,
		app.ErrorKindNotFound,
		app.ErrorKindConflict,
		app.ErrorKindCanceled,
		app.ErrorKindSecurity,
		app.ErrorKindInternal,
	}

	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			applicationError := &app.UseCaseError{
				Operation: applicationOperation,
				Target:    target,
				Kind:      kind,
				Subject:   applicationSubject,
			}
			modal := newErrorModal("save connection", target, applicationError)
			if modal.message != safeErrorMessage(kind) {
				t.Fatalf("modal message = %q, want controlled TUI copy %q", modal.message, safeErrorMessage(kind))
			}
			if modal.target != target {
				t.Fatalf("captured target bytes = %q, want %q", modal.target, target)
			}

			view := strings.Join(operationErrorLines(modal, 200), "\n")
			if strings.Contains(view, applicationOperation) || strings.Contains(view, applicationSubject) {
				t.Fatalf("application-owned error copy reached rendering: %q", view)
			}
			if strings.ContainsAny(view, "\x1b\r") || !strings.Contains(view, `\x1B[31mprod\r\nnext`) {
				t.Fatalf("target was not projected safely: %q", view)
			}
		})
	}
}

func TestIncompletePreActiveFailureUsesSafeDiagnosticFallback(t *testing.T) {
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.operationID = 3
	_, _, _ = model.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerModal)
	result := app.ConnectResult{
		Attempt: app.SSHAttemptTarget{ID: "captured", Path: "/captured", Host: "captured.test", Port: 22},
		Session: app.SSHSessionResult{State: app.SessionFailed, Outcome: app.SessionOutcomeTransportFailure},
	}
	updateModel(model, sessionFinishedMsg{id: 4, result: result})
	view := model.View().Content
	if model.activeSSHFailure() == nil || !strings.Contains(view, "Category: unexpected") || !strings.Contains(view, "Stage: unknown") || strings.Contains(view, "Recoverable operation error") {
		t.Fatalf("incomplete startup result did not use safe fallback: %q", view)
	}
}

func allowedDetail(category app.SSHFailureReason) string {
	switch category {
	case app.SSHFailureTimeout:
		return "operation timed out"
	case app.SSHFailureAuthenticationDenied:
		return "server rejected available authentication methods"
	case app.SSHFailureConnectionRefused:
		return "remote endpoint refused connection"
	case app.SSHFailureHostNotFound:
		return "host name could not be resolved"
	case app.SSHFailureNetworkUnreachable:
		return "network is unreachable"
	case app.SSHFailureHostTrust:
		return "changed"
	case app.SSHFailureCredentialUnavailable:
		return "agent"
	case app.SSHFailureSSHNegotiation:
		return "handshake"
	case app.SSHFailureCanceled:
		return "operation canceled"
	default:
		return ""
	}
}
