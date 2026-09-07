package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestObjectActionInventoriesAreExact(t *testing.T) {
	tests := []struct {
		name      string
		selection actionSelection
		wantKeys  string
		wantIDs   []actionID
	}{
		{"root", actionSelectionRoot, "n/f/r/?/q", []actionID{actionNewConnection, actionNewFolder, actionReload, actionHelp, actionQuit}},
		{"folder", actionSelectionFolder, "n/f/e/m/d/r/?/q", []actionID{actionNewConnection, actionNewFolder, actionEdit, actionMove, actionDelete, actionReload, actionHelp, actionQuit}},
		{"connection", actionSelectionConnection, "c/n/f/e/m/d/r/?/q", []actionID{actionConnect, actionNewConnection, actionNewFolder, actionEdit, actionMove, actionDelete, actionReload, actionHelp, actionQuit}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions := objectActions(actionContext{selection: test.selection, state: actionStateNormal})
			if got := actionKeys(actions); got != test.wantKeys {
				t.Fatalf("keys = %q, want %q", got, test.wantKeys)
			}
			gotIDs := make([]actionID, len(actions))
			for index, descriptor := range actions {
				gotIDs[index] = descriptor.id
			}
			if !slices.Equal(gotIDs, test.wantIDs) {
				t.Fatalf("IDs = %v, want %v", gotIDs, test.wantIDs)
			}
		})
	}
}

func TestObjectActionCanonicalLabelsAndPrioritiesAreStable(t *testing.T) {
	want := map[actionID]struct {
		key      string
		label    string
		category actionCategory
		priority actionPriority
	}{
		actionConnect:       {"c", "Connect", actionCategoryDomain, actionPriorityDomain},
		actionNewConnection: {"n", "New connection", actionCategoryDomain, actionPriorityDomain},
		actionNewFolder:     {"f", "New folder", actionCategoryDomain, actionPriorityDomain},
		actionEdit:          {"e", "Edit", actionCategoryDomain, actionPriorityDomain},
		actionMove:          {"m", "Move", actionCategoryDomain, actionPriorityDomain},
		actionDelete:        {"d", "Delete", actionCategoryDomain, actionPriorityDomain},
		actionReload:        {"r", "Reload", actionCategoryRecovery, actionPriorityRecovery},
		actionHelp:          {"?", "Help", actionCategoryRecovery, actionPriorityRecovery},
		actionQuit:          {"q", "Quit", actionCategoryRecovery, actionPriorityRecovery},
	}

	for _, descriptor := range connectionActionDescriptors {
		expected, ok := want[descriptor.id]
		if !ok {
			t.Fatalf("unexpected descriptor %q", descriptor.id)
		}
		if descriptor.key != expected.key || descriptor.label != expected.label || descriptor.category != expected.category || descriptor.priority != expected.priority {
			t.Errorf("%s = key %q label %q category %d priority %d, want %#v", descriptor.id, descriptor.key, descriptor.label, descriptor.category, descriptor.priority, expected)
		}
	}

	if _, exists := reflect.TypeFor[actionDescriptor]().FieldByName("compactLabel"); exists {
		t.Fatal("actionDescriptor must not define a compact label")
	}
}

func TestConnectionCreationActionsTargetParent(t *testing.T) {
	actions := objectActions(actionContext{selection: actionSelectionConnection, state: actionStateNormal})
	for _, id := range []actionID{actionNewConnection, actionNewFolder} {
		index := slices.IndexFunc(actions, func(descriptor actionDescriptor) bool { return descriptor.id == id })
		if index < 0 {
			t.Fatalf("missing %s", id)
		}
		if got := actions[index].target; got != actionTargetParent {
			t.Errorf("%s target = %d, want parent", id, got)
		}
	}

	for _, selection := range []actionSelection{actionSelectionRoot, actionSelectionFolder} {
		actions := objectActions(actionContext{selection: selection, state: actionStateNormal})
		for _, id := range []actionID{actionNewConnection, actionNewFolder} {
			index := slices.IndexFunc(actions, func(descriptor actionDescriptor) bool { return descriptor.id == id })
			if index < 0 || actions[index].target != actionTargetSelection {
				t.Errorf("selection %d action %s does not target selection", selection, id)
			}
		}
	}
}

func TestOperationAndConflictInventories(t *testing.T) {
	operation := actionsFor(actionContext{state: actionStateOperation, selection: actionSelectionConnection, focus: actionFocusTree})
	if got := actionKeys(operation); got != "Esc/?/q" {
		t.Fatalf("operation keys = %q", got)
	}
	if got := operationLoadingStatus(asyncOperationSSHStart, "server.example:22"); got != "Loading: SSH start — server.example:22" {
		t.Fatalf("loading status = %q", got)
	}
	if got := operationLoadingStatus(asyncOperationInitialLoad, ""); got != "Loading: Initial load — catalog" {
		t.Fatalf("initial loading status = %q", got)
	}

	conflict := actionsFor(actionContext{state: actionStateConflict, selection: actionSelectionFolder, focus: actionFocusTree})
	if got := actionKeys(conflict); got != "r/b/Esc/?/q" {
		t.Fatalf("conflict keys = %q", got)
	}
	wantLabels := []string{"Reload", "Back", "Cancel warning", "Help", "Quit"}
	for index, descriptor := range conflict {
		if descriptor.label != wantLabels[index] || descriptor.priority != actionPriorityRecovery {
			t.Errorf("conflict[%d] = %q priority %d", index, descriptor.label, descriptor.priority)
		}
	}
}

func TestFocusedNavigationDescriptors(t *testing.T) {
	tree := focusedNavigationActions(actionContext{state: actionStateNormal, focus: actionFocusTree})
	if got := actionKeys(tree); got != "Up/k/Down/j/Home/g/End/G/Left/h/Right/l/Tab/Shift+Tab" {
		t.Fatalf("tree navigation = %q", got)
	}
	tree = focusedNavigationActions(actionContext{state: actionStateNormal, focus: actionFocusTree, canToggle: true})
	if got := actionKeys(tree); got != "Up/k/Down/j/Home/g/End/G/Left/h/Right/l/Enter/Space/Tab/Shift+Tab" {
		t.Fatalf("expandable tree navigation = %q", got)
	}
	details := focusedNavigationActions(actionContext{state: actionStateNormal, focus: actionFocusDetails})
	if got := actionKeys(details); got != "Up/k/Down/j/Home/g/End/G/Tab/Shift+Tab" {
		t.Fatalf("details navigation = %q", got)
	}
}

func TestPackActionsUsesPriorityAndGreedyRows(t *testing.T) {
	actions := objectActions(actionContext{selection: actionSelectionConnection, state: actionStateNormal})
	got := packActions("", actions, 35, 3)
	want := []string{
		"r Reload q Quit ? Help c Connect",
		"n New connection f New folder",
		"e Edit m Move d Delete",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("packed = %#v, want %#v", got, want)
	}
	for _, line := range got {
		if width := ansi.StringWidth(line); width > 35 {
			t.Fatalf("line %q has width %d", line, width)
		}
	}
}

func TestPackActionsPreservesStatusAndRecoveryBeforeOverflowMarker(t *testing.T) {
	context := actionContext{selection: actionSelectionConnection, state: actionStateNormal, focus: actionFocusTree, canToggle: true}
	got := packActions("Conflict: target changed", actionsFor(context), 24, 3)
	want := []string{
		"Conflict: target changed",
		"r Reload q Quit ? Help",
		actionsOverflowMarker,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("packed overflow = %#v, want %#v", got, want)
	}

	operation := actionsFor(actionContext{state: actionStateOperation})
	got = packActions(operationLoadingStatus(asyncOperationSave, "/prod/server"), operation, 40, 3)
	want = []string{"Loading: Save — /prod/server", "Esc Cancel q Quit ? Help", ""}
	if !slices.Equal(got, want) {
		t.Fatalf("packed operation = %#v, want %#v", got, want)
	}
}

func TestPackActionsTruncatesSingleDescriptorInWidth(t *testing.T) {
	descriptor := actionDescriptor{id: actionNewConnection, key: "n", label: "New connection", category: actionCategoryDomain, priority: actionPriorityDomain}
	got := packActions("", []actionDescriptor{descriptor}, 8, 1)
	if got[0] != "n New c…" || ansi.StringWidth(got[0]) != 8 {
		t.Fatalf("truncated descriptor = %q width %d", got[0], ansi.StringWidth(got[0]))
	}
	if got := packActions("", []actionDescriptor{descriptor}, 2, 1); got[0] != "n…" {
		t.Fatalf("two-cell descriptor = %q, want key prefix and ellipsis", got[0])
	}
}

func TestUnavailableActionsAreAbsent(t *testing.T) {
	tests := []struct {
		name    string
		context actionContext
		absent  []actionID
	}{
		{"root", actionContext{selection: actionSelectionRoot, state: actionStateNormal}, []actionID{actionConnect, actionEdit, actionMove, actionDelete}},
		{"folder", actionContext{selection: actionSelectionFolder, state: actionStateNormal}, []actionID{actionConnect}},
		{"operation", actionContext{selection: actionSelectionConnection, state: actionStateOperation, focus: actionFocusTree, canToggle: true}, []actionID{actionConnect, actionNewConnection, actionNewFolder, actionEdit, actionMove, actionDelete, actionReload, actionUp}},
		{"conflict", actionContext{selection: actionSelectionConnection, state: actionStateConflict, focus: actionFocusTree, canToggle: true}, []actionID{actionConnect, actionNewConnection, actionNewFolder, actionEdit, actionMove, actionDelete, actionUp}},
		{"invalid selection", actionContext{state: actionStateNormal}, []actionID{actionConnect, actionNewConnection, actionNewFolder, actionReload, actionHelp, actionQuit}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions := actionsFor(test.context)
			for _, absent := range test.absent {
				if slices.ContainsFunc(actions, func(descriptor actionDescriptor) bool { return descriptor.id == absent }) {
					t.Errorf("unavailable action %s is present in %v", absent, actionKeys(actions))
				}
			}
		})
	}
}
