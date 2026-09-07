package tui

import "testing"

func TestApplicationFocusHasOneValidDefaultOwner(t *testing.T) {
	owner := defaultFocusOwner()
	if owner != focusOwnerTree || !owner.valid() {
		t.Fatalf("default focus owner = %v, want valid tree", owner)
	}

	owners := []focusOwner{focusOwnerTree, focusOwnerDetail, focusOwnerConnectionForm, focusOwnerModal}
	matches := 0
	for _, candidate := range owners {
		if !candidate.valid() {
			t.Fatalf("declared focus owner %v is invalid", candidate)
		}
		if candidate == owner {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("default owner matches = %d, want exactly one", matches)
	}
	if focusOwner(255).valid() {
		t.Fatal("unknown focus owner is valid")
	}
}

func TestSecurityInputPreemptsAndPreservesApplicationFocus(t *testing.T) {
	for _, kind := range []securityInputKind{securityInputTrust, securityInputSecret} {
		t.Run(map[securityInputKind]string{securityInputTrust: "trust", securityInputSecret: "secret"}[kind], func(t *testing.T) {
			state, ok := newSecurityInputState(kind, focusOwnerDetail)
			if !ok || !state.preemptsApplicationInput() {
				t.Fatal("security input did not become the preemptive owner")
			}

			// Resize changes only the security viewport. Unrelated operation
			// results likewise cannot replace its captured application focus.
			state = state.withViewport(newViewportState(7))
			op, _, ok := startOperation(0, nil, asyncOperationSSHStart, nil, operationOwnerTrust)
			if !ok {
				t.Fatal("start operation failed")
			}
			_, accepted := op.recordCommit(op.id, op.kind, "session")
			if !accepted {
				t.Fatal("matching operation result was not accepted")
			}

			if state.preservedFocus != focusOwnerDetail || state.close() != focusOwnerDetail {
				t.Fatalf("restored focus = %v, want detail", state.close())
			}
			if state.viewport.logicalOffset != 7 {
				t.Fatalf("viewport offset = %d, want 7", state.viewport.logicalOffset)
			}
		})
	}
}
