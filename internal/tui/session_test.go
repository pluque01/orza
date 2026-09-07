package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestSessionFailureUsesAttemptCapturedBeforeExecutionAndIgnoresStaleCompletion(t *testing.T) {
	failed := app.SSHAttemptTarget{ID: "captured", Revision: 3, Path: "/before", Host: "before.test", Port: 22}
	mutated := app.SSHAttemptTarget{ID: "other", Revision: 9, Path: "/after", Host: "after.test", Port: 2200}
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", nil).Presentation()
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.operationID = 1
	_, _, _ = model.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerModal)

	updateModel(model, sessionFinishedMsg{id: 1, attempt: mutated, result: app.ConnectResult{Attempt: mutated, Session: app.SSHSessionResult{Failure: &failure}}})
	if model.modal.isOpen() || model.operation == nil {
		t.Fatal("stale session completion changed the model")
	}

	updateModel(model, sessionFinishedMsg{id: 2, attempt: failed, result: app.ConnectResult{Attempt: mutated, Session: app.SSHSessionResult{State: app.SessionFailed, Failure: &failure}}})
	if modal := model.activeSSHFailure(); modal == nil || modal.attempt != failed {
		t.Fatalf("failure attempt = %#v, want immutable %#v", modal, failed)
	}
}

func TestSessionOperationTimeoutNetworkAndActiveCommitMatrix(t *testing.T) {
	attempt := app.SSHAttemptTarget{ID: "captured", Revision: 3, Path: "/server", Host: "server.test", Port: 22}
	for run := 0; run < 20; run++ {
		for _, test := range []struct {
			name   string
			err    error
			active bool
		}{
			{name: "timeout/pre-active", err: context.DeadlineExceeded},
			{name: "network/pre-active", err: errors.New("network interruption")},
			{name: "timeout/post-active", err: context.DeadlineExceeded, active: true},
			{name: "network/post-active", err: errors.New("network interruption"), active: true},
		} {
			t.Run(test.name, func(t *testing.T) {
				model := New(Config{Width: 80, Height: 24, NoColor: true})
				target := capturedTarget{id: attempt.ID, revision: attempt.Revision, kind: app.NodeKindConnection, path: attempt.Path, endpointOrScope: "server.test:22"}
				id, _, _ := model.beginOperationWith(asyncOperationSSHStart, &target, operationOwnerModal)
				result := app.ConnectResult{Attempt: attempt, Session: app.SSHSessionResult{State: app.SessionFailed, Outcome: app.SessionOutcomeTransportFailure}}
				if test.active {
					result.Session.StartedAt = time.Unix(1, 0)
				} else {
					failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", test.err).Presentation()
					result.Session.Failure = &failure
				}
				_, command := model.Update(sessionFinishedMsg{id: id, attempt: attempt, result: result, err: test.err})
				if model.operation != nil {
					t.Fatal("session result did not fully release operation owner")
				}
				if test.active {
					if command == nil || model.sessionResult.Session.StartedAt.IsZero() {
						t.Fatal("post-active result was not retained before quit")
					}
				} else if modal := model.activeSSHFailure(); modal == nil || modal.attempt != attempt {
					t.Fatal("pre-active failure did not preserve captured target")
				}
				frame := model.View().Content
				updateModel(model, sessionFinishedMsg{id: id, attempt: app.SSHAttemptTarget{ID: "duplicate"}, err: errors.New("duplicate")})
				updateModel(model, sessionFinishedMsg{id: id + 1, attempt: app.SSHAttemptTarget{ID: "stale"}, err: errors.New("stale")})
				if got := model.View().Content; got != frame {
					t.Fatal("duplicate or stale session result changed the stable frame")
				}
			})
		}
	}
}

func TestSessionPreCommitCancellationRestoresWithoutDiagnostic(t *testing.T) {
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	attempt := app.SSHAttemptTarget{ID: "captured", Revision: 3, Path: "/server", Host: "server.test", Port: 22}
	target := capturedTarget{id: attempt.ID, revision: attempt.Revision, kind: app.NodeKindConnection, path: attempt.Path, endpointOrScope: "server.test:22"}
	id, _, _ := model.beginOperationWith(asyncOperationSSHStart, &target, operationOwnerModal)
	updateModel(model, keyPress("esc"))
	failure := app.NewSSHStartError(app.SSHFailureCanceled, app.SSHFailureStageNetworkConnection, "", context.Canceled).Presentation()
	updateModel(model, sessionFinishedMsg{id: id, attempt: attempt, result: app.ConnectResult{Attempt: attempt, Session: app.SSHSessionResult{State: app.SessionCanceled, Outcome: app.SessionOutcomeCanceled, Failure: &failure}}, err: context.Canceled})
	if model.operation != nil || model.modal.isOpen() || model.status != "READY" {
		t.Fatalf("pre-commit cancel did not restore frame: operation=%+v modal=%+v status=%q", model.operation, model.modal, model.status)
	}
}
