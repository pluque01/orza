package tui

import (
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func conflictFixture(t *testing.T) conflictState {
	t.Helper()
	state, ok := newConflictState(conflictTypeRevisionChanged, capturedTarget{
		id:          "connection-1",
		revision:    4,
		kind:        app.NodeKindConnection,
		path:        "/team/prod",
		ancestorIDs: []app.NodeID{"team", "root"},
	}, conflictOwnerForm)
	if !ok {
		t.Fatal("new conflict failed")
	}
	return state
}

func TestConflictReloadRechecksOnlyCapturedID(t *testing.T) {
	state := conflictFixture(t)

	missing := state.recheck(nil)
	if missing == nil || missing.kind != conflictTypeMissing || missing.target.id != state.target.id || !missing.blocked {
		t.Fatalf("missing recheck = %+v", missing)
	}
	different := state.recheck(&capturedTarget{id: "current-row", revision: 4})
	if different == nil || different.kind != conflictTypeMissing || different.target.id != "connection-1" {
		t.Fatal("reload retargeted conflict to a different row")
	}
	revised := state.recheck(&capturedTarget{id: "connection-1", revision: 5})
	if revised == nil || revised.kind != conflictTypeRevisionChanged || revised.target.revision != 4 || !revised.blocked {
		t.Fatalf("revised recheck = %+v", revised)
	}
	if resolved := state.recheck(&capturedTarget{id: "connection-1", revision: 4}); resolved != nil {
		t.Fatalf("same-ID revision reload remained conflicted: %+v", resolved)
	}
}

func TestConflictBackUsesNearestExistingAncestor(t *testing.T) {
	state := conflictFixture(t)
	existing := map[app.NodeID]bool{"team": true, "root": true}
	got := state.backSelection(func(id app.NodeID) bool { return existing[id] }, "root")
	if got != "team" {
		t.Fatalf("back selection = %q, want nearest ancestor team", got)
	}
	delete(existing, "team")
	got = state.backSelection(func(id app.NodeID) bool { return existing[id] }, "root")
	if got != "root" {
		t.Fatalf("back selection = %q, want root", got)
	}
}

func TestConflictCancelCompactsButRemainsBlocked(t *testing.T) {
	state := conflictFixture(t).compact()
	if state.detailVisible || !state.blocked || state.target.id != "connection-1" {
		t.Fatalf("compact conflict = %+v", state)
	}
	if reloaded := state.recheck(&capturedTarget{id: "connection-1", revision: 5}); reloaded == nil || reloaded.detailVisible {
		t.Fatal("compact warning was not preserved by unresolved reload")
	}
}

func TestOperationConflictAppearsOnlyAfterCleanupReleasesOwner(t *testing.T) {
	conflict := conflictFixture(t)
	conflict.owner = conflictOwnerOperation
	active, _, _ := startOperation(12, nil, asyncOperationSave, &conflict.target, operationOwnerForm)

	state, ok := active.queueConflict(13, asyncOperationSave, conflict)
	if !ok || state.phase != asyncPhaseCleaningUp {
		t.Fatalf("queued conflict operation = %+v", state)
	}
	active = &state
	if _, release, released := releaseOperation(active); released || release.conflict != nil {
		t.Fatal("conflict became dispatchable before cleanup")
	}
	state, ok = active.finishCleanup(13)
	if !ok || state.phase != asyncPhaseCompleted {
		t.Fatal("conflict operation cleanup did not complete")
	}
	active, release, ok := releaseOperation(&state)
	if !ok || active != nil || release.conflict == nil || release.conflict.owner != conflictOwnerOperation || !release.conflict.blocked {
		t.Fatalf("post-cleanup release = (%+v, %+v, %v)", active, release, ok)
	}
}
