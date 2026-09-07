package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type actionID string

const (
	actionConnect       actionID = "connect"
	actionNewConnection actionID = "new_connection"
	actionNewFolder     actionID = "new_folder"
	actionEdit          actionID = "edit"
	actionMove          actionID = "move"
	actionDelete        actionID = "delete"
	actionReload        actionID = "reload"
	actionRetry         actionID = "retry"
	actionHelp          actionID = "help"
	actionQuit          actionID = "quit"
	actionCancel        actionID = "cancel"
	actionBack          actionID = "back"
	actionCancelWarning actionID = "cancel_warning"
	actionSave          actionID = "save"
	actionConfirm       actionID = "confirm"
	actionDiscard       actionID = "discard"
	actionMoveHere      actionID = "move_here"
	actionUp            actionID = "up"
	actionDown          actionID = "down"
	actionHome          actionID = "home"
	actionEnd           actionID = "end"
	actionLeft          actionID = "left"
	actionRight         actionID = "right"
	actionToggle        actionID = "toggle"
	actionShowDetails   actionID = "focus_details"
	actionShowTree      actionID = "focus_tree"
	actionNextField     actionID = "next_field"
	actionPreviousField actionID = "previous_field"
)

type actionCategory uint8

const (
	actionCategoryDomain actionCategory = iota + 1
	actionCategoryRecovery
	actionCategoryNavigation
)

type actionPriority uint8

const (
	actionPriorityStatus     actionPriority = 1
	actionPriorityRecovery   actionPriority = 2
	actionPriorityPrimary    actionPriority = 3
	actionPriorityDomain     actionPriority = 4
	actionPriorityNavigation actionPriority = 5
	actionPrioritySecondary  actionPriority = 6
)

type actionSelection uint8

const (
	actionSelectionRoot actionSelection = iota + 1
	actionSelectionFolder
	actionSelectionConnection
)

type actionState uint8

const (
	actionStateNormal actionState = iota
	actionStateOperation
	actionStateConflict
)

type actionFocus uint8

const (
	actionFocusNone actionFocus = iota
	actionFocusTree
	actionFocusDetails
)

type actionTarget uint8

const (
	actionTargetNone actionTarget = iota
	actionTargetSelection
	actionTargetParent
	actionTargetCatalog
)

type actionContext struct {
	selection actionSelection
	state     actionState
	focus     actionFocus
	canToggle bool
}

type actionDescriptor struct {
	id         actionID
	key        string
	label      string
	category   actionCategory
	priority   actionPriority
	target     actionTarget
	applicable func(actionContext) bool
}

func (descriptor actionDescriptor) appliesTo(context actionContext) bool {
	return descriptor.applicable != nil && descriptor.applicable(context)
}

func (descriptor actionDescriptor) text() string {
	if descriptor.key == "" {
		return descriptor.label
	}
	return descriptor.key + " " + descriptor.label
}

func normalSelection(selection actionSelection) func(actionContext) bool {
	return func(context actionContext) bool {
		return context.state == actionStateNormal && context.selection == selection
	}
}

func stateIs(state actionState) func(actionContext) bool {
	return func(context actionContext) bool { return context.state == state }
}

func focusedOn(focus actionFocus) func(actionContext) bool {
	return func(context actionContext) bool {
		return context.state == actionStateNormal && context.focus == focus
	}
}

func toggleApplies(context actionContext) bool {
	return context.state == actionStateNormal && context.focus == actionFocusTree && context.canToggle
}

var rootActionDescriptors = []actionDescriptor{
	{id: actionNewConnection, key: "n", label: "New connection", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionRoot)},
	{id: actionNewFolder, key: "f", label: "New folder", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionRoot)},
	{id: actionReload, key: "r", label: "Reload", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetCatalog, applicable: normalSelection(actionSelectionRoot)},
	{id: actionHelp, key: "?", label: "Help", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetNone, applicable: normalSelection(actionSelectionRoot)},
	{id: actionQuit, key: "q", label: "Quit", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetNone, applicable: normalSelection(actionSelectionRoot)},
}

var folderActionDescriptors = []actionDescriptor{
	{id: actionNewConnection, key: "n", label: "New connection", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionFolder)},
	{id: actionNewFolder, key: "f", label: "New folder", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionFolder)},
	{id: actionEdit, key: "e", label: "Edit", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionFolder)},
	{id: actionMove, key: "m", label: "Move", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionFolder)},
	{id: actionDelete, key: "d", label: "Delete", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionFolder)},
	{id: actionReload, key: "r", label: "Reload", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetCatalog, applicable: normalSelection(actionSelectionFolder)},
	{id: actionHelp, key: "?", label: "Help", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetNone, applicable: normalSelection(actionSelectionFolder)},
	{id: actionQuit, key: "q", label: "Quit", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetNone, applicable: normalSelection(actionSelectionFolder)},
}

var connectionActionDescriptors = []actionDescriptor{
	{id: actionConnect, key: "c", label: "Connect", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionConnection)},
	{id: actionNewConnection, key: "n", label: "New connection", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetParent, applicable: normalSelection(actionSelectionConnection)},
	{id: actionNewFolder, key: "f", label: "New folder", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetParent, applicable: normalSelection(actionSelectionConnection)},
	{id: actionEdit, key: "e", label: "Edit", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionConnection)},
	{id: actionMove, key: "m", label: "Move", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionConnection)},
	{id: actionDelete, key: "d", label: "Delete", category: actionCategoryDomain, priority: actionPriorityDomain, target: actionTargetSelection, applicable: normalSelection(actionSelectionConnection)},
	{id: actionReload, key: "r", label: "Reload", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetCatalog, applicable: normalSelection(actionSelectionConnection)},
	{id: actionHelp, key: "?", label: "Help", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetNone, applicable: normalSelection(actionSelectionConnection)},
	{id: actionQuit, key: "q", label: "Quit", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetNone, applicable: normalSelection(actionSelectionConnection)},
}

var operationActionDescriptors = []actionDescriptor{
	{id: actionCancel, key: "Esc", label: "Cancel", category: actionCategoryRecovery, priority: actionPriorityRecovery, applicable: stateIs(actionStateOperation)},
	{id: actionHelp, key: "?", label: "Help", category: actionCategoryRecovery, priority: actionPriorityRecovery, applicable: stateIs(actionStateOperation)},
	{id: actionQuit, key: "q", label: "Quit", category: actionCategoryRecovery, priority: actionPriorityRecovery, applicable: stateIs(actionStateOperation)},
}

var conflictActionDescriptors = []actionDescriptor{
	{id: actionReload, key: "r", label: "Reload", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetCatalog, applicable: stateIs(actionStateConflict)},
	{id: actionBack, key: "b", label: "Back", category: actionCategoryRecovery, priority: actionPriorityRecovery, target: actionTargetParent, applicable: stateIs(actionStateConflict)},
	{id: actionCancelWarning, key: "Esc", label: "Cancel warning", category: actionCategoryRecovery, priority: actionPriorityRecovery, applicable: stateIs(actionStateConflict)},
	{id: actionHelp, key: "?", label: "Help", category: actionCategoryRecovery, priority: actionPriorityRecovery, applicable: stateIs(actionStateConflict)},
	{id: actionQuit, key: "q", label: "Quit", category: actionCategoryRecovery, priority: actionPriorityRecovery, applicable: stateIs(actionStateConflict)},
}

var connectionFormActionDescriptors = []actionDescriptor{
	{id: actionCancel, key: "Esc", label: "Cancel", category: actionCategoryRecovery, priority: actionPriorityRecovery},
	{id: actionQuit, key: "Ctrl+C", label: "Quit", category: actionCategoryRecovery, priority: actionPriorityRecovery},
	{id: actionHelp, key: "F1", label: "Help", category: actionCategoryRecovery, priority: actionPriorityRecovery},
	{id: actionSave, key: "Ctrl+S", label: "Save", category: actionCategoryDomain, priority: actionPriorityPrimary},
	{id: actionNextField, key: "Tab", label: "Next", category: actionCategoryNavigation, priority: actionPriorityNavigation},
	{id: actionPreviousField, key: "Shift+Tab/F2", label: "Previous", category: actionCategoryNavigation, priority: actionPriorityNavigation},
}

var treeNavigationDescriptors = []actionDescriptor{
	{id: actionUp, key: "Up/k", label: "Move up", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
	{id: actionDown, key: "Down/j", label: "Move down", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
	{id: actionHome, key: "Home/g", label: "First row", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
	{id: actionEnd, key: "End/G", label: "Last row", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
	{id: actionLeft, key: "Left/h", label: "Collapse/parent", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
	{id: actionRight, key: "Right/l", label: "Expand/child", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
	{id: actionToggle, key: "Enter/Space", label: "Toggle", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: toggleApplies},
	{id: actionShowDetails, key: "Tab/Shift+Tab", label: "Details", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusTree)},
}

var detailsNavigationDescriptors = []actionDescriptor{
	{id: actionUp, key: "Up/k", label: "Scroll up", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusDetails)},
	{id: actionDown, key: "Down/j", label: "Scroll down", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusDetails)},
	{id: actionHome, key: "Home/g", label: "First row", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusDetails)},
	{id: actionEnd, key: "End/G", label: "Last row", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusDetails)},
	{id: actionShowTree, key: "Tab/Shift+Tab", label: "Tree", category: actionCategoryNavigation, priority: actionPriorityNavigation, applicable: focusedOn(actionFocusDetails)},
}

func objectActions(context actionContext) []actionDescriptor {
	var inventory []actionDescriptor
	switch context.selection {
	case actionSelectionRoot:
		inventory = rootActionDescriptors
	case actionSelectionFolder:
		inventory = folderActionDescriptors
	case actionSelectionConnection:
		inventory = connectionActionDescriptors
	default:
		return nil
	}
	return applicableDescriptors(inventory, context)
}

func focusedNavigationActions(context actionContext) []actionDescriptor {
	var inventory []actionDescriptor
	switch context.focus {
	case actionFocusTree:
		inventory = treeNavigationDescriptors
	case actionFocusDetails:
		inventory = detailsNavigationDescriptors
	default:
		return nil
	}
	return applicableDescriptors(inventory, context)
}

func actionsFor(context actionContext) []actionDescriptor {
	switch context.state {
	case actionStateOperation:
		return applicableDescriptors(operationActionDescriptors, context)
	case actionStateConflict:
		return applicableDescriptors(conflictActionDescriptors, context)
	case actionStateNormal:
		actions := objectActions(context)
		return append(actions, focusedNavigationActions(context)...)
	default:
		return nil
	}
}

func applicableDescriptors(inventory []actionDescriptor, context actionContext) []actionDescriptor {
	actions := make([]actionDescriptor, 0, len(inventory))
	for _, descriptor := range inventory {
		if descriptor.appliesTo(context) {
			actions = append(actions, descriptor)
		}
	}
	return actions
}

func operationLoadingStatus(kind asyncOperationKind, target string) string {
	var action string
	switch kind {
	case asyncOperationInitialLoad:
		action = "Initial load"
	case asyncOperationReload:
		action = "Reload"
	case asyncOperationSave:
		action = "Save"
	case asyncOperationSSHStart:
		action = "SSH start"
	default:
		return ""
	}
	if target == "" {
		target = "catalog"
	}
	return "Loading: " + action + " — " + target
}

const actionsOverflowMarker = "Hidden actions — ? Help"

// packActions returns exactly rows rows. Status is priority-1 content and is
// kept separate from descriptors; descriptors are packed by priority and then
// canonical stable order with one space between complete descriptors.
func packActions(status string, descriptors []actionDescriptor, width, rows int) []string {
	width = max(0, width)
	rows = max(0, rows)
	packed := make([]string, rows)
	if width == 0 || rows == 0 {
		return packed
	}

	protected := make([]bool, rows)
	row := 0
	if status != "" {
		packed[0] = safeText(status, width)
		protected[0] = true
		row = 1
	}

	ordered := orderedActionDescriptors(descriptors)
	recoveryEnd := 0
	for recoveryEnd < len(ordered) && ordered[recoveryEnd].priority <= actionPriorityRecovery {
		recoveryEnd++
	}

	row, _ = packDescriptorRange(packed, protected, row, rows, width, ordered[:recoveryEnd], true)
	withAll := append([]string(nil), packed...)
	_, allFit := packDescriptorRange(withAll, nil, row, rows, width, ordered[recoveryEnd:], false)
	if allFit {
		return withAll
	}

	markerRow := rows - 1
	if protected[markerRow] {
		packDescriptorRange(packed, nil, row, rows, width, ordered[recoveryEnd:], false)
		return packed
	}

	packDescriptorRange(packed, nil, row, markerRow, width, ordered[recoveryEnd:], false)
	packed[markerRow] = viewportEllipsis(actionsOverflowMarker, width)
	return packed
}

func orderedActionDescriptors(descriptors []actionDescriptor) []actionDescriptor {
	ordered := append([]actionDescriptor(nil), descriptors...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].priority != ordered[right].priority {
			return ordered[left].priority < ordered[right].priority
		}
		return canonicalActionOrder(ordered[left].id) < canonicalActionOrder(ordered[right].id)
	})
	return ordered
}

func canonicalActionOrder(id actionID) int {
	switch id {
	case actionCancel, actionCancelWarning:
		return 0
	case actionBack:
		return 1
	case actionReload, actionRetry:
		return 2
	case actionQuit:
		return 3
	case actionHelp:
		return 4
	case actionSave:
		return 10
	case actionConfirm:
		return 11
	case actionDiscard:
		return 12
	case actionMoveHere:
		return 13
	case actionConnect:
		return 20
	case actionNewConnection:
		return 21
	case actionNewFolder:
		return 22
	case actionEdit:
		return 23
	case actionMove:
		return 24
	case actionDelete:
		return 25
	case actionUp:
		return 30
	case actionDown:
		return 31
	case actionHome:
		return 32
	case actionEnd:
		return 33
	case actionLeft:
		return 34
	case actionRight:
		return 35
	case actionToggle:
		return 36
	case actionShowDetails, actionShowTree:
		return 37
	case actionNextField:
		return 38
	case actionPreviousField:
		return 39
	default:
		return 100
	}
}

func packDescriptorRange(lines []string, protected []bool, row, limit, width int, descriptors []actionDescriptor, protect bool) (int, bool) {
	for _, descriptor := range descriptors {
		text := truncateActionDescriptor(descriptor, width)
		if text == "" {
			continue
		}
		if row >= limit {
			return row, false
		}
		if lines[row] != "" && ansi.StringWidth(lines[row])+1+ansi.StringWidth(text) > width {
			row++
		}
		if row >= limit {
			return row, false
		}
		if lines[row] == "" {
			lines[row] = text
		} else {
			lines[row] += " " + text
		}
		if protect && protected != nil {
			protected[row] = true
		}
	}
	return row, true
}

func truncateActionDescriptor(descriptor actionDescriptor, width int) string {
	if width <= 0 {
		return ""
	}
	text := descriptor.text()
	if ansi.StringWidth(text) <= width {
		return text
	}
	if descriptor.key == "" {
		return viewportEllipsis(text, width)
	}
	prefix := descriptor.key + " "
	prefixWidth := ansi.StringWidth(prefix)
	if prefixWidth >= width {
		keyWidth := ansi.StringWidth(descriptor.key)
		if keyWidth >= width {
			return ansi.Truncate(descriptor.key, width, "")
		}
		return descriptor.key + ansi.Truncate(safeTextEllipsis, width-keyWidth, "")
	}
	return prefix + ansi.Truncate(descriptor.label, width-prefixWidth, safeTextEllipsis)
}

func actionKeys(descriptors []actionDescriptor) string {
	keys := make([]string, len(descriptors))
	for index, descriptor := range descriptors {
		keys[index] = descriptor.key
	}
	return strings.Join(keys, "/")
}

// actionForKey resolves only bindings present in the supplied descriptor
// inventory. This keeps unavailable keys inert and makes Actions, Help and
// dispatch share the same applicability decision.
func actionForKey(msg tea.KeyPressMsg, descriptors []actionDescriptor, keys keyMap) (actionDescriptor, bool) {
	for _, descriptor := range descriptors {
		var binding key.Binding
		switch descriptor.id {
		case actionConnect:
			binding = keys.Connect
		case actionNewConnection:
			binding = keys.New
		case actionNewFolder:
			binding = keys.NewFolder
		case actionEdit:
			binding = keys.Edit
		case actionMove:
			binding = keys.Move
		case actionDelete:
			binding = keys.Delete
		case actionReload, actionRetry:
			binding = keys.Reload
		case actionHelp:
			binding = keys.Help
		case actionQuit:
			binding = keys.Quit
		case actionCancel, actionCancelWarning:
			binding = keys.Back
		case actionUp:
			binding = keys.Up
		case actionDown:
			binding = keys.Down
		case actionHome:
			binding = keys.Home
		case actionEnd:
			binding = keys.End
		case actionLeft:
			binding = keys.Left
		case actionRight:
			binding = keys.Right
		case actionToggle:
			binding = keys.Toggle
		case actionShowDetails, actionShowTree:
			if key.Matches(msg, keys.Next, keys.Previous) {
				return descriptor, true
			}
			continue
		default:
			continue
		}
		if key.Matches(msg, binding) {
			return descriptor, true
		}
	}
	return actionDescriptor{}, false
}

func actionHelpLines(descriptors []actionDescriptor) []string {
	lines := []string{
		"Help",
		"Paste isolation requires terminal bracketed-paste support.",
		"Without it, input works but pasted bytes cannot be distinguished from typing.",
		"The application never reads the operating system clipboard.",
	}
	for _, descriptor := range descriptors {
		lines = append(lines, descriptor.text())
	}
	return lines
}
