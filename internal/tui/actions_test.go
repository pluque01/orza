package tui

import (
	"reflect"
	"slices"
	"strings"
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
		{"connection", actionSelectionConnection, "c/x/n/f/e/m/d/r/?/q", []actionID{actionConnect, actionForgetHostKey, actionNewConnection, actionNewFolder, actionEdit, actionMove, actionDelete, actionReload, actionHelp, actionQuit}},
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
		actionForgetHostKey: {"x", "Forget host key", actionCategoryDomain, actionPriorityDomain},
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

func TestBrowserLegendAndHelpUseANSIEquivalentPairStyles(t *testing.T) {
	descriptors := actionsFor(actionContext{selection: actionSelectionConnection, state: actionStateNormal, focus: actionFocusTree, canToggle: true})
	plainLegend := strings.Join(renderBrowserLegend(newStyles(true), descriptors, 120), "\n")
	coloredLegend := strings.Join(renderBrowserLegend(newStyles(false), descriptors, 120), "\n")
	if strings.Contains(plainLegend, "Actions") || !strings.Contains(plainLegend, "  |  ") || ansi.Strip(coloredLegend) != plainLegend {
		t.Fatalf("browser legend is not a borderless ANSI-equivalent set of pairs:\nplain %q\ncolor %q", plainLegend, coloredLegend)
	}

	layout := calculateLayout(80, 24, regionTree)
	state := modalState{kind: modalKindHelp, payload: helpPayload{descriptors: descriptors}}
	plainHelp := renderModalOverlay("", state, layout, newStyles(true), nil)
	coloredHelp := renderModalOverlay("", state, layout, newStyles(false), nil)
	if strings.Count(plainHelp, "[*] Help") != 1 || ansi.Strip(coloredHelp) != plainHelp {
		t.Fatalf("Help title is not a singular ANSI-equivalent label:\nplain %q\ncolor %q", plainHelp, coloredHelp)
	}
	helpLines, _ := modalContent(state, newStyles(true), 200, nil)
	for _, line := range helpLines {
		if strings.TrimSpace(strings.TrimPrefix(line, modalControlsPrefix)) == "Help" {
			t.Fatalf("Help body duplicates its title: %#v", helpLines)
		}
	}
}

func TestModalFixedControlsUseANSIEquivalentPairStyles(t *testing.T) {
	controls := []string{
		modalControlLine("y Confirm"),
		modalControlLine("Enter/Esc Cancel"),
		modalControlLine("? Help"),
		modalControlLine("Ctrl+S/Enter Save  Esc Cancel  F1 Help"),
	}
	plain := strings.Join(packModalControls(controls, 120, 10, newStyles(true)), "\n")
	colored := strings.Join(packModalControls(controls, 120, 10, newStyles(false)), "\n")
	if ansi.Strip(colored) != plain || !strings.Contains(plain, "  |  ") {
		t.Fatalf("modal controls are not ANSI-equivalent separated pairs:\nplain %q\ncolor %q", plain, colored)
	}
	if !strings.Contains(colored, "\x1b[92m") || !strings.Contains(colored, "\x1b[90m") {
		t.Fatalf("modal controls do not emphasize keys and mute actions: %q", colored)
	}

	statusPlain := packModalControls([]string{modalStatusLine("Resolving current target...")}, 80, 1, newStyles(true))[0]
	statusColored := packModalControls([]string{modalStatusLine("Resolving current target...")}, 80, 1, newStyles(false))[0]
	if ansi.Strip(statusColored) != statusPlain || !strings.Contains(statusColored, "\x1b[90m") || strings.Contains(statusColored, "\x1b[92m") {
		t.Fatalf("modal status is not muted plain status:\nplain %q\ncolor %q", statusPlain, statusColored)
	}
}

func TestBrowserHelpUsesOneIndentedGlobalKeyColumn(t *testing.T) {
	descriptors := actionsFor(actionContext{selection: actionSelectionConnection, state: actionStateNormal, focus: actionFocusTree, canToggle: true})
	plain := browserHelpLines(newStyles(true), descriptors, 80)
	colored := browserHelpLines(newStyles(false), descriptors, 80)
	if got := strings.Join(mapANSI(ansi.Strip, colored), "\n"); got != strings.Join(plain, "\n") {
		t.Fatalf("colored Help changed plain-text semantics:\ncolor %q\nplain %q", colored, plain)
	}

	keyColumn := -1
	for _, line := range plain {
		if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, "  ") {
			continue
		}
		key := strings.Fields(line)[0]
		if !slices.ContainsFunc(descriptors, func(descriptor actionDescriptor) bool { return descriptor.key == key }) {
			continue
		}
		column := strings.Index(line, key)
		if keyColumn < 0 {
			keyColumn = column
		} else if column != keyColumn {
			t.Fatalf("Help key %q starts at %d, want global column %d:\n%s", key, column, keyColumn, strings.Join(plain, "\n"))
		}
	}
	if keyColumn != 2 {
		t.Fatalf("Help row indent = %d, want 2:\n%s", keyColumn, strings.Join(plain, "\n"))
	}
	for index, line := range plain {
		if slices.Contains([]string{"Navigation", "Connection", "Management", "Application"}, line) {
			if !strings.Contains(colored[index], "\x1b[") || ansi.Strip(colored[index]) != line {
				t.Fatalf("Help section %q is not a colored ANSI-equivalent title: %q", line, colored[index])
			}
		}
	}
}

func TestBrowserHelpNarrowFallbackRetainsIndentAndNoColorSemantics(t *testing.T) {
	descriptors := actionsFor(actionContext{selection: actionSelectionConnection, state: actionStateNormal, focus: actionFocusTree, canToggle: true})
	plain := browserHelpLines(newStyles(true), descriptors, 12)
	colored := browserHelpLines(newStyles(false), descriptors, 12)
	if got := strings.Join(mapANSI(ansi.Strip, colored), "\n"); got != strings.Join(plain, "\n") {
		t.Fatalf("colored narrow Help changed plain-text semantics:\ncolor %q\nplain %q", colored, plain)
	}
	for _, line := range plain {
		if strings.TrimSpace(line) == "" || slices.Contains([]string{"Navigation", "Connection", "Management", "Application"}, line) {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("narrow Help row lost section indent: %q", line)
		}
	}
}

func mapANSI(transform func(string) string, lines []string) []string {
	result := make([]string, len(lines))
	for index, line := range lines {
		result[index] = transform(line)
	}
	return result
}

func TestBrowserLegendAndHelpSelectApplicableDescriptorsInOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		context actionContext
		legend  []actionID
	}{
		{"root tree", actionContext{selection: actionSelectionRoot, state: actionStateNormal, focus: actionFocusTree}, []actionID{actionUp, actionDown, actionSearch, actionNewConnection, actionNewFolder, actionQuit}},
		{"folder details", actionContext{selection: actionSelectionFolder, state: actionStateNormal, focus: actionFocusDetails}, []actionID{actionUp, actionDown, actionNewConnection, actionNewFolder, actionQuit}},
		{"connection tree", actionContext{selection: actionSelectionConnection, state: actionStateNormal, focus: actionFocusTree}, []actionID{actionUp, actionDown, actionSearch, actionConnect, actionNewConnection, actionNewFolder, actionQuit}},
	} {
		t.Run(test.name, func(t *testing.T) {
			descriptors := actionsFor(test.context)
			got := browserLegendDescriptors(descriptors)
			ids := make([]actionID, len(got))
			for index, descriptor := range got {
				ids[index] = descriptor.id
			}
			if !slices.Equal(ids, test.legend) {
				t.Fatalf("legend IDs = %v, want %v", ids, test.legend)
			}
			help := strings.Join(browserHelpLines(newStyles(true), descriptors, 80), "\n")
			for _, descriptor := range descriptors {
				if !actionHelpContains(browserHelpLines(newStyles(true), descriptors, 80), descriptor.key, descriptor.label) {
					t.Fatalf("Help omitted descriptor %q %q:\n%s", descriptor.key, descriptor.label, help)
				}
			}
			for _, group := range []string{"Navigation", "Connection", "Management", "Application"} {
				if strings.Contains(help, group) && strings.Count(help, group) != 1 {
					t.Fatalf("Help group %q is not unique:\n%s", group, help)
				}
			}
			if navigation, management, application := strings.Index(help, "Navigation"), strings.Index(help, "Management"), strings.Index(help, "Application"); navigation > management || management > application {
				t.Fatalf("Help group order is unstable:\n%s", help)
			}
		})
	}
}

func actionHelpContains(lines []string, key, label string) bool {
	want := append([]string{key}, strings.Fields(label)...)
	for _, line := range lines {
		if slices.Equal(strings.Fields(ansi.Strip(line)), want) {
			return true
		}
	}
	return false
}

func TestHelpKeepsContextualDescriptorInventoryAndKeys(t *testing.T) {
	descriptors := actionsFor(actionContext{selection: actionSelectionConnection, state: actionStateNormal, focus: actionFocusTree, canToggle: true})
	const wantKeys = "c/x/n/f/e/m/d/r/?/q/Up/k/Down/j/Home/g/End/G/Left/h/Right/l/Enter/Space/Tab/Shift+Tab//"
	if got := actionKeys(descriptors); got != wantKeys {
		t.Fatalf("contextual keys = %q, want %q", got, wantKeys)
	}

	want := []string{
		"c Connect",
		"x Forget host key",
		"n New connection",
		"f New folder",
		"e Edit",
		"m Move",
		"d Delete",
		"r Reload",
		"? Help",
		"q Quit",
		"Up/k Move up",
		"Down/j Move down",
		"Home/g First row",
		"End/G Last row",
		"Left/h Collapse/parent",
		"Right/l Expand/child",
		"Enter/Space Toggle",
		"Tab/Shift+Tab Details",
		"/ Filter",
		modalControlLine("?/Esc Close"),
	}
	state := modalState{kind: modalKindHelp, payload: helpPayload{lines: actionHelpLines(descriptors)}}
	got, _ := modalContent(state, newStyles(true), 200, nil)
	if !slices.Equal(got, want) {
		t.Fatalf("Help lines = %#v, want exact contextual inventory %#v", got, want)
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
	if got := actionKeys(tree); got != "Up/k/Down/j/Home/g/End/G/Left/h/Right/l/Tab/Shift+Tab//" {
		t.Fatalf("tree navigation = %q", got)
	}
	tree = focusedNavigationActions(actionContext{state: actionStateNormal, focus: actionFocusTree, canToggle: true})
	if got := actionKeys(tree); got != "Up/k/Down/j/Home/g/End/G/Left/h/Right/l/Enter/Space/Tab/Shift+Tab//" {
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
		"x Forget host key n New connection",
		"f New folder e Edit m Move d Delete",
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
		"Hidden actions — ? Help",
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
