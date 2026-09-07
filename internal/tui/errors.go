package tui

import (
	"fmt"

	"github.com/pluque01/orza/internal/app"
)

type errorModal struct {
	operation     string
	target        string
	message       string
	kind          app.ErrorKind
	attempt       app.SSHAttemptTarget
	failure       *app.SSHFailurePresentation
	detailVisible bool
	recovery      recoveryState
	retry         *operationRetryIntent
}

type recoveryState int

const (
	recoveryIdle recoveryState = iota
	recoveryResolving
	recoveryConfirming
	recoveryEditing
	recoveryMissing
	recoveryConflict
)

func newErrorModal(operation, target string, err error) *errorModal {
	kind := app.ErrorKindOf(err)
	message := safeErrorMessage(kind)
	return &errorModal{operation: operation, target: target, message: message, kind: kind}
}

func newSSHFailureModal(attempt app.SSHAttemptTarget, failure app.SSHFailurePresentation) *errorModal {
	return &errorModal{
		operation: "start SSH session",
		target:    attempt.Path,
		attempt:   attempt,
		failure:   &failure,
	}
}

func safeErrorMessage(kind app.ErrorKind) string {
	switch kind {
	case app.ErrorKindInvalid:
		return "The request is invalid. Correct the highlighted fields and retry."
	case app.ErrorKindNotFound:
		return "The connection no longer exists. Reload the catalog."
	case app.ErrorKindConflict:
		return "The connection changed. Your stale version was never overwritten."
	case app.ErrorKindCanceled:
		return "The operation was canceled."
	case app.ErrorKindSecurity:
		return "Host trust, authentication, or the secure store rejected the operation."
	default:
		return "The operation failed safely. No catalog changes were assumed."
	}
}

func connectionFormSaveError(err error) string {
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		return "Save failed. Correct the highlighted fields and try again."
	case app.ErrorKindCanceled:
		return "Save was canceled. Your changes are still available."
	case app.ErrorKindSecurity:
		return "Save failed because the credential store rejected the request. Review the password choice and try again."
	default:
		return "Save failed safely. Your changes were not discarded; try again or cancel."
	}
}

func targetOf(connection *app.Connection) string {
	if connection == nil {
		return "/"
	}
	return fmt.Sprintf("%s (%s:%d)", connection.Path, connection.Host, connection.Port)
}
