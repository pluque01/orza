package tui

import "testing"

func TestOperationHasOneMonotonicMatchingOwner(t *testing.T) {
	active, lastID, ok := startOperation(0, nil, asyncOperationReload, nil, operationOwnerRoot)
	if !ok || active.id != 1 || lastID != 1 {
		t.Fatalf("first operation = (%+v, %d, %v), want ID 1", active, lastID, ok)
	}
	unchanged, unchangedID, ok := startOperation(lastID, active, asyncOperationSave, nil, operationOwnerForm)
	if ok || unchanged != active || unchangedID != lastID {
		t.Fatal("a second operation replaced the current owner")
	}
	if !active.matchesResult(1, asyncOperationReload) {
		t.Fatal("owner result did not match")
	}
	if active.matchesResult(0, asyncOperationReload) || active.matchesResult(1, asyncOperationSave) {
		t.Fatal("stale ID or non-owner kind matched")
	}

	state, ok := active.beginCleanup(1, asyncOperationReload)
	if !ok {
		t.Fatal("begin cleanup failed")
	}
	state, ok = state.finishCleanup(1)
	if !ok {
		t.Fatal("finish cleanup failed")
	}
	active = &state
	active, _, ok = releaseOperation(active)
	if !ok || active != nil {
		t.Fatal("completed owner was not released")
	}
	active, lastID, ok = startOperation(lastID, active, asyncOperationSave, nil, operationOwnerForm)
	if !ok || active.id != 2 || lastID != 2 {
		t.Fatalf("second operation ID = %d, want 2", active.id)
	}
}

func TestOperationRejectsStaleAndDuplicateResults(t *testing.T) {
	active, _, _ := startOperation(40, nil, asyncOperationSave, nil, operationOwnerForm)
	state, accepted := active.recordCommit(40, asyncOperationSave, "stale")
	if accepted || state.commitResult != nil {
		t.Fatal("stale result changed operation")
	}
	state, accepted = active.recordCommit(41, asyncOperationSave, "saved")
	if !accepted || state.commitResult == nil || state.commitResult.value != "saved" {
		t.Fatal("matching commit result was not retained")
	}
	duplicate, accepted := state.recordCommit(41, asyncOperationSave, "duplicate")
	if accepted || duplicate.commitResult.value != "saved" {
		t.Fatal("duplicate result replaced confirmed commit")
	}
	state, accepted = state.beginCleanup(41, asyncOperationSave)
	if !accepted || state.matchesResult(41, asyncOperationSave) {
		t.Fatal("cleaning operation still accepts duplicate results")
	}
}

func TestOperationCancelAndQuitWaitForCleanupAndKeepCommit(t *testing.T) {
	active, _, _ := startOperation(7, nil, asyncOperationSave, nil, operationOwnerForm)
	state, ok := active.requestCancellation(8, false)
	if !ok || state.phase != asyncPhaseCancelRequested || state.stopReason != operationStopCanceled {
		t.Fatalf("cancel state = %+v", state)
	}
	state, ok = state.requestCancellation(8, true)
	if !ok || !state.quitIntent {
		t.Fatal("quit intent was not retained while cancellation was pending")
	}
	state, ok = state.recordCommit(8, asyncOperationSave, "post-commit revision")
	if !ok {
		t.Fatal("confirmed result after cancel was discarded before cleanup")
	}
	state, ok = state.beginCleanup(8, asyncOperationSave)
	if !ok || state.phase != asyncPhaseCleaningUp {
		t.Fatal("cancel did not enter cleanup")
	}
	if _, _, released := releaseOperation(&state); released {
		t.Fatal("operation released before cleanup completed")
	}
	state, ok = state.finishCleanup(8)
	if !ok || state.phase != asyncPhaseCompleted {
		t.Fatal("cleanup did not complete")
	}
	active, release, ok := releaseOperation(&state)
	if !ok || active != nil || !release.quitIntent || release.commitResult == nil || release.commitResult.value != "post-commit revision" {
		t.Fatalf("release = (%+v, %+v, %v)", active, release, ok)
	}
}

func TestTimeoutAndNetworkInterruptionUseCleanupPhases(t *testing.T) {
	for _, reason := range []operationStopReason{operationStopTimeout, operationStopNetwork} {
		name := map[operationStopReason]string{operationStopTimeout: "timeout", operationStopNetwork: "network"}[reason]
		t.Run(name, func(t *testing.T) {
			active, _, _ := startOperation(100, nil, asyncOperationSSHStart, nil, operationOwnerModal)
			state, ok := active.interrupt(101, asyncOperationSSHStart, reason)
			if !ok || state.phase != asyncPhaseCancelRequested || state.stopReason != reason {
				t.Fatalf("interruption state = %+v", state)
			}
			state, ok = state.beginCleanup(101, asyncOperationSSHStart)
			if !ok || state.phase != asyncPhaseCleaningUp {
				t.Fatal("interruption did not enter cleanup")
			}
			state, ok = state.finishCleanup(101)
			if !ok || state.phase != asyncPhaseCompleted {
				t.Fatal("interruption cleanup did not complete")
			}
			active, _, ok = releaseOperation(&state)
			if !ok || active != nil {
				t.Fatal("interrupted operation owner was not cleared")
			}
		})
	}
}
