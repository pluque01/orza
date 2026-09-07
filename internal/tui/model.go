package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

const (
	minimumWidth  = 80
	minimumHeight = 24
)

type screen int

const (
	screenBrowser screen = iota
	screenConnectionForm
)

type operationKind int

const (
	operationReload operationKind = iota
	operationCreate
	operationUpdate
	operationDeleteScope
	operationDelete
	operationFolderCreate
	operationFolderRename
	operationFolderDeleteScope
	operationFolderDelete
	operationMove
)

type operationResultMsg struct {
	id             uint64
	kind           operationKind
	targetConflict bool
	list           app.ListConnectionsResult
	folder         app.Folder
	connection     app.ConnectionResult
	scope          app.ConnectionDeleteScope
	deleted        app.DeleteConnectionResult
	folderScope    app.FolderDeleteScope
	snapshot       *catalogSnapshot
	err            error
}

type recoveryAction int

const (
	recoveryRetry recoveryAction = iota
	recoveryEdit
)

type recoveryResolvedMsg struct {
	id         uint64
	action     recoveryAction
	connection app.Connection
	err        error
}

type connectionTreeContext struct {
	selectedID app.NodeID
	expanded   map[app.NodeID]struct{}
	viewport   viewportState
	focus      focusOwner
}

type connectionEditState struct {
	treeContext   connectionTreeContext
	target        capturedTarget
	conflict      *conflictState
	recovery      *errorModal
	quitAfterSave bool
}

// Config supplies shared application use cases and deterministic presentation
// boundaries. Width and Height are primarily useful to model tests.
type Config struct {
	Connections ConnectionService
	Folders     FolderService
	Connect     ConnectService
	Terminal    app.Terminal
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	NoColor     bool
	Width       int
	Height      int
	Context     context.Context
}

// Model is the Bubble Tea v2 root model.
type Model struct {
	ctx                  context.Context
	connections          ConnectionService
	folders              FolderService
	connect              ConnectService
	terminal             app.Terminal
	input                io.Reader
	output               io.Writer
	errOutput            io.Writer
	keys                 keyMap
	styles               styles
	noColor              bool
	width                int
	height               int
	screen               screen
	browser              browserModel
	focusOwner           focusOwner
	detailState          detailState
	ownedSelectionID     app.NodeID
	form                 *connectionForm
	connectionEdit       *connectionEditState
	operationID          uint64
	operation            *operationState
	modalRegistry        modalRegistry
	modal                modalState
	status               string
	navigationNotice     navigationNotice
	sessionResult        app.ConnectResult
	sessionErr           error
	sessionRuntime       *sessionRuntime
	securityInput        *securityInputState
	sessionSecret        *sessionSecretRequestMsg
	sessionTrustResponse chan sessionTrustResponse
	credentialInput      *credentialMutationReadyMsg
	pendingSelection     app.NodeID
}

func New(config Config) *Model {
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	width, height := config.Width, config.Height
	if width == 0 {
		width = minimumWidth
	}
	if height == 0 {
		height = minimumHeight
	}
	model := &Model{
		ctx: ctx, connections: config.Connections, folders: config.Folders, connect: config.Connect, terminal: config.Terminal,
		input: config.Stdin, output: config.Stdout, errOutput: config.Stderr,
		keys: newKeyMap(), styles: newStyles(config.NoColor), noColor: config.NoColor, width: width, height: height,
		focusOwner: defaultFocusOwner(), status: "READY",
	}
	model.modalRegistry = newModalRegistry()
	model.browser.setSnapshot(newCatalogSnapshot(rootFolder(), 0), "")
	model.ownedSelectionID = model.browser.selectedID
	model.syncDetail()
	return model
}

func (m *Model) Init() tea.Cmd { return m.reloadCommand() }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.handleSecurityResize()
	case sessionTrustRequestMsg:
		return m, m.handleSessionTrustRequest(msg)
	case sessionSecretRequestMsg:
		return m, m.handleSessionSecretRequest(msg)
	case sessionSecretFinishedMsg:
		return m, m.handleSessionSecretFinished(msg)
	case sessionActivateMsg:
		return m, m.handleSessionActivate(msg)
	case sessionJoinedMsg:
		return m, nil
	case sessionRuntimeClosedMsg:
		return m, nil
	case credentialMutationReadyMsg:
		return m, m.handleCredentialMutationReady(msg)
	case credentialSecretFinishedMsg:
		return m, m.handleCredentialSecretFinished(msg)
	case operationResultMsg:
		return m, m.handleOperation(msg)
	case sessionFinishedMsg:
		return m, m.handleSession(msg)
	case recoveryResolvedMsg:
		return m, m.handleRecoveryResolved(msg)
	case tea.PasteMsg:
		return m, m.handlePaste(msg)
	case tea.PasteStartMsg, tea.PasteEndMsg:
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handlePaste(msg tea.PasteMsg) tea.Cmd {
	if m.modal.isOpen() {
		if m.modal.helpVisible {
			return nil
		}
		switch payload := m.modal.payload.(type) {
		case folderCreatePayload:
			return payload.form.update(msg)
		case folderEditPayload:
			return payload.form.update(msg)
		}
		return nil
	}
	if m.operation != nil {
		return nil
	}
	switch m.screen {
	case screenConnectionForm:
		if m.form != nil && m.form.focusedField() <= fieldIdentity {
			return m.form.update(msg)
		}
	}
	return nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.securityInput != nil && m.securityInput.preemptsApplicationInput() {
		return m, m.handleSecurityInputKey(msg)
	}
	if calculateLayout(m.width, m.height, m.focusedLayoutRegion()).mode == layoutUndersized {
		switch {
		case key.Matches(msg, m.keys.Help):
			m.toggleHelp()
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			if m.operation != nil {
				return m.handleOperationKey(msg)
			}
			if m.screen == screenBrowser && !m.modal.isOpen() {
				return m, tea.Quit
			}
			if msg.String() == "q" {
				msg = tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})
			}
			return m.handleActiveKey(msg)
		default:
			return m, nil
		}
	}
	return m.handleActiveKey(msg)
}

func (m *Model) handleActiveKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Repair an impossible overlap defensively. Normal transitions never expose
	// a completed modal owner beside a newly installed failure modal.
	if failure := m.activeSSHFailure(); m.operation != nil && failure != nil && failure.recovery != recoveryResolving && m.operation.kind != asyncOperationSSHStart {
		m.cancelCurrentOperation(false)
		m.completeOperation(m.operation.id, nil)
	}
	if m.operation != nil {
		return m.handleOperationKey(msg)
	}
	if m.modal.isOpen() {
		return m.handleModalKey(msg)
	}
	if m.screen == screenConnectionForm && m.connectionEdit != nil && m.connectionEdit.conflict != nil {
		return m.handleFormConflictKey(msg)
	}
	if m.screen == screenConnectionForm {
		return m.handleFormKey(msg)
	}
	return m.handleBrowserKey(msg)
}

func (m *Model) handleOperationKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.modal.helpVisible {
		if key.Matches(msg, m.keys.Help, m.keys.Back) {
			m.toggleModalHelp()
		} else {
			m.scrollModal(msg)
		}
		return m, nil
	}
	if m.modal.kind == modalKindHelp {
		return m.handleModalKey(msg)
	}
	descriptor, ok := actionForKey(msg, actionsFor(actionContext{state: actionStateOperation}), m.keys)
	if !ok {
		return m, nil
	}
	switch descriptor.id {
	case actionHelp:
		m.toggleHelp()
	case actionCancel, actionQuit:
		quit := descriptor.id == actionQuit
		state, changed := m.operation.requestCancellation(m.operation.id, quit)
		if changed {
			m.operation = &state
			if m.operation.cancel != nil {
				m.operation.cancel()
			}
		}
	}
	return m, nil
}

func (m *Model) currentHelpLines() []string {
	if m.screen == screenConnectionForm {
		return append([]string{
			"Connection form Help",
			"Tab Next  Shift+Tab/F2 Previous  Ctrl+S Save  Esc Cancel  Ctrl+C Quit",
		}, actionHelpLines(connectionFormActionDescriptors)...)
	}
	return actionHelpLines(m.currentActionDescriptors())
}

func (m *Model) handleBrowserKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.navigationNotice = navigationNoticeNone
	selectionWasExternallyChanged := m.ownedSelectionID != m.browser.selectedID
	m.syncDetail()
	defer m.syncDetail()
	descriptor, ok := actionForKey(msg, actionsFor(m.actionContext()), m.keys)
	if !ok {
		if selectionWasExternallyChanged && (key.Matches(msg, m.keys.Edit) || key.Matches(msg, m.keys.Move) || key.Matches(msg, m.keys.Delete)) {
			m.rejectRootAction(msg.String())
		}
		return m, nil
	}
	switch descriptor.id {
	case actionShowDetails, actionShowTree:
		if m.focusOwner == focusOwnerDetail {
			m.focusOwner = focusOwnerTree
		} else {
			m.focusOwner = focusOwnerDetail
		}
	case actionUp:
		if m.focusOwner == focusOwnerDetail {
			m.scrollDetail(-1)
			break
		}
		m.browser.move(-1)
		m.ownedSelectionID = m.browser.selectedID
	case actionDown:
		if m.focusOwner == focusOwnerDetail {
			m.scrollDetail(1)
			break
		}
		m.browser.move(1)
		m.ownedSelectionID = m.browser.selectedID
	case actionLeft:
		m.browser.left()
		m.ownedSelectionID = m.browser.selectedID
	case actionRight:
		m.browser.right()
		m.ownedSelectionID = m.browser.selectedID
	case actionHome:
		if m.focusOwner == focusOwnerDetail {
			m.detailState = m.detailState.withOffset(0)
			break
		}
		m.browser.home()
		m.ownedSelectionID = m.browser.selectedID
	case actionEnd:
		if m.focusOwner == focusOwnerDetail {
			m.detailState = m.detailState.withOffset(m.detailMaximumOffset())
			break
		}
		m.browser.end()
		m.ownedSelectionID = m.browser.selectedID
	case actionToggle:
		m.browser.toggle()
		m.ownedSelectionID = m.browser.selectedID
	case actionHelp:
		m.toggleHelp()
	case actionNewFolder:
		if destination := m.browser.destinationFolder(); destination != nil {
			form := newFolderForm(nil, app.ItemSelector{ID: destination.ID})
			form.setDestination(*destination)
			target := capturedTargetFromFolder(*destination)
			target.ancestorIDs = m.ancestorsOf(destination.ID)
			m.openGenericModal(modalKindFolderCreate, &target, folderCreatePayload{form: form})
		}
	case actionNewConnection:
		if destination := m.browser.destinationFolder(); destination != nil {
			form := newConnectionForm(nil)
			form.setDestination(*destination)
			target := capturedTargetFromFolder(*destination)
			target.ancestorIDs = m.ancestorsOf(destination.ID)
			m.openConnectionForm(form, target)
		}
	case actionEdit:
		if selected := m.browser.selectionFolder(); selected != nil {
			form := newFolderForm(selected, app.ItemSelector{})
			target := capturedTargetFromFolder(*selected)
			target.ancestorIDs = m.ancestorsOf(selected.ID)
			m.openGenericModal(modalKindFolderEdit, &target, folderEditPayload{form: form})
		} else if selected := m.browser.selection(); selected != nil {
			m.openConnectionForm(newConnectionForm(selected), m.captureConnectionTarget(*selected))
		}
	case actionMove:
		if selected := m.browser.selectedTreeNode(); selected != nil && !selected.root {
			node := selected.node
			picker := newTreeMovePicker(node, m.browser.snapshot)
			target := capturedTarget{id: node.ID, revision: node.Revision, kind: node.Kind, path: node.Path, ancestorIDs: m.ancestorsOf(node.ID)}
			m.openGenericModal(modalKindMovePicker, &target, movePickerPayload{picker: picker})
		}
	case actionDelete:
		if selected := m.browser.selectionFolder(); selected != nil {
			return m, m.folderDeleteScopeCommand(*selected)
		} else if selected := m.browser.selection(); selected != nil {
			return m, m.deleteScopeCommand(*selected)
		}
	case actionConnect:
		if selected := m.browser.selection(); selected != nil {
			confirmation := newConnectConfirmation(*selected)
			target := m.captureConnectionTarget(*selected)
			m.openGenericModal(modalKindConnectConfirmation, &target, connectConfirmationPayload{confirmation: confirmation})
		}
	case actionReload:
		return m, m.reloadCommand()
	case actionQuit:
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) syncDetail() {
	detail, ok := m.detailState.withTarget(m.browser.snapshot, m.browser.selectedID)
	if !ok {
		m.detailState = detailState{}
		return
	}
	m.detailState = detail
}

func (m *Model) focusedLayoutRegion() layoutRegion {
	if m.focusOwner == focusOwnerDetail || m.focusOwner == focusOwnerConnectionForm {
		return regionDetails
	}
	return regionTree
}

func (m *Model) detailMaximumOffset() int {
	layout := calculateLayout(m.width, m.height, m.focusedLayoutRegion())
	return viewportMaximumOffset(len(m.detailState.content(layout.details.contentWidth())), layout.details.contentHeight())
}

func (m *Model) scrollDetail(delta int) {
	offset := m.detailState.viewport.logicalOffset + delta
	offset = min(max(0, offset), m.detailMaximumOffset())
	m.detailState = m.detailState.withOffset(offset)
}

func (m *Model) rejectRootAction(action string) {
	modal := newErrorModal(action+" root", "/", app.ErrInvalidRequest)
	m.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: modal})
	m.status = "ERROR"
}

func cloneExpanded(expanded map[app.NodeID]struct{}) map[app.NodeID]struct{} {
	copy := make(map[app.NodeID]struct{}, len(expanded))
	for id := range expanded {
		copy[id] = struct{}{}
	}
	return copy
}

func (m *Model) captureConnectionTarget(connection app.Connection) capturedTarget {
	target := capturedTargetFromConnection(connection)
	target.ancestorIDs = m.ancestorsOf(connection.ID)
	return target
}

func (m *Model) ancestorsOf(id app.NodeID) []app.NodeID {
	ancestors := make([]app.NodeID, 0, 8)
	for parent := m.browser.snapshot.parents[id]; parent != ""; parent = m.browser.snapshot.parents[parent] {
		ancestors = append(ancestors, parent)
	}
	return ancestors
}

func (m *Model) openConnectionForm(form *connectionForm, target capturedTarget) {
	m.openConnectionFormWithRecovery(form, target, nil)
}

func (m *Model) openConnectionFormWithRecovery(form *connectionForm, target capturedTarget, recovery *errorModal) {
	m.form = form
	m.screen = screenConnectionForm
	m.connectionEdit = &connectionEditState{
		treeContext: connectionTreeContext{
			selectedID: m.browser.selectedID,
			expanded:   cloneExpanded(m.browser.expanded),
			viewport:   m.browser.viewport,
			focus:      m.focusOwner,
		},
		target:   target.clone(),
		recovery: recovery,
	}
	m.focusOwner = focusOwnerConnectionForm
}

func (m *Model) closeConnectionForm(restoreTree bool) {
	var recovery *errorModal
	if m.connectionEdit != nil {
		recovery = m.connectionEdit.recovery
	}
	if restoreTree && m.connectionEdit != nil {
		context := m.connectionEdit.treeContext
		m.browser.expanded = cloneExpanded(context.expanded)
		m.browser.selectedID = context.selectedID
		m.browser.viewport = context.viewport
		m.browser.rebuildRows()
		m.focusOwner = context.focus
		m.ownedSelectionID = m.browser.selectedID
		m.syncDetail()
	} else {
		m.focusOwner = focusOwnerTree
	}
	m.form = nil
	m.connectionEdit = nil
	m.screen = screenBrowser
	if restoreTree && recovery != nil {
		recovery.recovery = recoveryIdle
		m.installSSHFailure(recovery)
	}
}

func (m *Model) openUnsavedChanges(intent unsavedChangesIntent) {
	if m.form == nil {
		return
	}
	if m.connectionEdit == nil {
		target := capturedTarget{path: m.form.inputs[fieldFolder].Value(), kind: app.NodeKindConnection}
		if m.form.original != nil {
			target = m.captureConnectionTarget(*m.form.original)
		} else if m.form.destination != nil {
			target = capturedTargetFromFolder(*m.form.destination)
		}
		m.connectionEdit = &connectionEditState{
			treeContext: connectionTreeContext{selectedID: m.browser.selectedID, expanded: cloneExpanded(m.browser.expanded), viewport: m.browser.viewport, focus: focusOwnerTree},
			target:      target,
		}
	}
	payload := unsavedChangesPayload{intent: intent, target: m.connectionEdit.target.path}
	if payload.target == "" && m.form.destination != nil {
		payload.target = m.form.destination.Path
	}
	opened, err := m.modal.open(m.modalRegistry, modalOpenRequest{
		kind:       modalKindUnsavedChanges,
		openedFrom: focusOwnerConnectionForm,
		target:     &m.connectionEdit.target,
		payload:    payload,
	})
	if err == nil {
		m.modal = opened
		m.focusOwner = focusOwnerModal
	}
}

func (m *Model) openGenericModal(kind modalKind, target *capturedTarget, payload any) bool {
	opener := m.focusOwner
	if opener == focusOwnerModal {
		return false
	}
	opened, err := m.modal.open(m.modalRegistry, modalOpenRequest{
		kind: kind, openedFrom: opener, target: target, payload: payload,
	})
	if err != nil {
		return false
	}
	m.modal = opened
	m.focusOwner = focusOwnerModal
	return true
}

func (m *Model) closeGenericModal() {
	closed, owner, err := m.modal.close()
	if err != nil {
		return
	}
	m.modal = closed
	m.focusOwner = owner
}

func (m *Model) toggleModalHelp() {
	state, _, err := m.modal.toggleHelp(m.modalControl())
	if err == nil {
		m.modal = state
		m.modal.viewport = newViewportState(0)
	}
}

func (m *Model) toggleHelp() {
	if m.modal.isOpen() {
		if m.modal.kind == modalKindHelp {
			m.closeGenericModal()
		} else {
			m.toggleModalHelp()
		}
		return
	}
	m.openGenericModal(modalKindHelp, nil, helpPayload{lines: wrapHelpLines(m.currentHelpLines(), minimumLayoutWidth-6)})
}

func (m *Model) helpVisible() bool {
	return m.modal.kind == modalKindHelp || m.modal.helpVisible
}

func wrapHelpLines(lines []string, width int) []string {
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = appendWrappedModalLine(wrapped, "", line, width)
	}
	return wrapped
}

func (m *Model) modalControl() any {
	switch payload := m.modal.payload.(type) {
	case folderCreatePayload:
		return payload.form.input.Position()
	case folderEditPayload:
		return payload.form.input.Position()
	case movePickerPayload:
		return payload.picker.selected
	case sshFailurePayload:
		return payload.modal.detailVisible
	default:
		return nil
	}
}

func (m *Model) handleModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.modal.helpVisible {
		if key.Matches(msg, m.keys.Help, m.keys.Back) {
			m.toggleModalHelp()
			return m, nil
		}
		m.scrollModal(msg)
		return m, nil
	}
	_, folderCreate := m.modal.payload.(folderCreatePayload)
	_, folderEdit := m.modal.payload.(folderEditPayload)
	editableFolder := folderCreate || folderEdit
	if m.modal.kind != modalKindHelp && (key.Matches(msg, m.keys.FormHelp) || !editableFolder && key.Matches(msg, m.keys.Help)) {
		m.toggleModalHelp()
		return m, nil
	}
	if m.modal.conflict != nil && m.modal.conflict.blocked {
		switch msg.String() {
		case "esc":
			compact := m.modal.conflict.compact()
			m.modal.conflict = &compact
		case "b":
			selected := m.modal.conflict.backSelection(m.browser.hasNode, m.browser.snapshot.rootID)
			m.closeModal()
			if m.screen == screenConnectionForm {
				m.closeConnectionForm(false)
			}
			m.browser.selectedID = selected
			m.browser.expandAncestors(selected)
			m.browser.rebuildRows()
			m.ownedSelectionID = selected
			m.syncDetail()
			m.status = "READY"
		case "r":
			target := m.modal.conflict.target.clone()
			return m, m.catalogReloadCommand(asyncOperationReload, &target, operationOwnerModal)
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	switch payload := m.modal.payload.(type) {
	case folderCreatePayload:
		return m.handleFolderModalKey(msg, payload.form)
	case folderEditPayload:
		return m.handleFolderModalKey(msg, payload.form)
	case movePickerPayload:
		return m.handleMoveModalKey(msg, payload.picker)
	case deleteConnectionPayload:
		if payload.confirmation.confirmed(msg) {
			scope := payload.confirmation.scope
			return m, m.deleteCommand(scope)
		}
		if key.Matches(msg, m.keys.Back, m.keys.Open) {
			m.closeModal()
		} else {
			m.scrollModal(msg)
		}
	case deleteFolderPayload:
		if payload.confirmation.confirmed(msg) {
			scope := payload.confirmation.scope
			return m, m.folderDeleteCommand(scope)
		}
		if key.Matches(msg, m.keys.Back, m.keys.Open) {
			m.closeModal()
		} else {
			m.scrollModal(msg)
		}
	case connectConfirmationPayload:
		if payload.confirmation.confirmed(msg) {
			connection := payload.confirmation.connection
			recovery := m.activeSSHFailure()
			if recovery != nil {
				m.closeModal()
				return m, m.sessionCommandWithRecovery(connection, recovery)
			}
			return m, m.sessionCommand(connection)
		}
		if key.Matches(msg, m.keys.Back, m.keys.Open) {
			m.closeModal()
		} else {
			m.scrollModal(msg)
		}
	case unsavedChangesPayload:
		return m.handleUnsavedChangesKey(msg)
	case helpPayload:
		if key.Matches(msg, m.keys.Help, m.keys.Back) {
			m.closeModal()
		} else {
			m.scrollModal(msg)
		}
	case operationErrorPayload:
		return m.handleOperationErrorModalKey(msg, payload)
	case sshFailurePayload:
		return m.handleSSHFailureModalKey(msg, payload)
	}
	return m, nil
}

func (m *Model) handleFolderModalKey(msg tea.KeyPressMsg, form *folderForm) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "ctrl+c":
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		m.closeModal()
	case key.Matches(msg, m.keys.Save), key.Matches(msg, m.keys.Open):
		if form.original == nil {
			if m.staleDestination(form.destination) {
				m.modal.recoverableError = "Destination changed. Reload or cancel before saving."
				return m, nil
			}
			request, valid := form.createRequest()
			if valid {
				return m, m.folderCreateCommand(request, form.destination)
			}
		} else if request, valid := form.renameRequest(); valid {
			return m, m.folderRenameCommand(request)
		}
	default:
		return m, form.update(msg)
	}
	return m, nil
}

func (m *Model) handleMoveModalKey(msg tea.KeyPressMsg, picker *movePicker) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		picker.move(-1)
	case key.Matches(msg, m.keys.Down):
		picker.move(1)
	case key.Matches(msg, m.keys.Home):
		picker.selected = 0
		picker.selectEnabled(1)
	case key.Matches(msg, m.keys.End):
		picker.selected = max(0, len(picker.targets)-1)
		picker.selectEnabled(-1)
	case key.Matches(msg, m.keys.Back):
		m.closeModal()
	case key.Matches(msg, m.keys.Open):
		if destination := picker.destination(); destination != nil {
			return m, m.moveCommand(picker.source, *destination)
		}
	}
	return m, nil
}

func (m *Model) handleOperationErrorModalKey(msg tea.KeyPressMsg, payload operationErrorPayload) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back), msg.String() == "b":
		m.closeModal()
	case key.Matches(msg, m.keys.Reload):
		if payload.modal.kind == app.ErrorKindConflict || payload.modal.kind == app.ErrorKindNotFound {
			m.closeGenericModal()
			return m, m.reloadCommand()
		}
		if payload.modal.retry != nil && payload.modal.retry.valid() {
			intent := payload.modal.retry.clone()
			m.closeGenericModal()
			return m, m.retryOperation(intent)
		}
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	default:
		m.scrollModal(msg)
	}
	return m, nil
}

func (m *Model) retryOperation(intent operationRetryIntent) tea.Cmd {
	switch intent.kind {
	case operationRetryCatalog:
		return m.catalogReloadCommandWithRetry(intent.reloadKind, intent.target, intent.owner, intent)
	case operationRetryConnectionDeleteScope:
		return m.deleteScopeCommand(intent.connection)
	case operationRetryFolderDeleteScope:
		return m.folderDeleteScopeCommand(intent.folder)
	case operationRetrySSHStart:
		return m.sessionCommand(intent.connection)
	default:
		return nil
	}
}

func (m *Model) handleSSHFailureModalKey(msg tea.KeyPressMsg, payload sshFailurePayload) (tea.Model, tea.Cmd) {
	if payload.confirmation != nil {
		if payload.confirmation.confirmed(msg) {
			connection := payload.confirmation.connection
			recovery := payload.modal
			m.closeModal()
			return m, m.sessionCommandWithRecovery(connection, recovery)
		}
		if key.Matches(msg, m.keys.Back, m.keys.Open) {
			payload.confirmation = nil
			payload.modal.recovery = recoveryIdle
			m.modal.payload = payload
		}
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Detail) && payload.modal.failure.TechnicalDetail != "":
		payload.modal.detailVisible = !payload.modal.detailVisible
	case msg.String() == "b" || key.Matches(msg, m.keys.Back):
		m.backFromFailure(payload.modal)
		m.closeModal()
	case key.Matches(msg, m.keys.Edit) && payload.modal.recovery == recoveryIdle:
		return m, m.resolveRecoveryCommand(recoveryEdit)
	case key.Matches(msg, m.keys.Reload):
		if payload.modal.recovery == recoveryIdle {
			return m, m.resolveRecoveryCommand(recoveryRetry)
		}
		if payload.modal.recovery == recoveryMissing || payload.modal.recovery == recoveryConflict {
			return m, m.reloadCommand()
		}
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	default:
		m.scrollModal(msg)
	}
	return m, nil
}

func (m *Model) scrollModal(msg tea.KeyPressMsg) {
	layout := calculateLayout(m.width, m.height, m.focusedLayoutRegion())
	rect := layout.modalOverlay()
	lines, active := modalContent(m.modal, m.styles, rect.contentWidth(), wrapHelpLines(actionHelpLines(m.currentActionDescriptors()), rect.contentWidth()))
	projection := projectModalViewport(m.modal, lines, rect.contentHeight(), rect.contentWidth(), active)
	maximum := viewportMaximumOffset(projection.contentLength, projection.availableRows)
	switch {
	case key.Matches(msg, m.keys.Up):
		m.modal.viewport = m.modal.viewport.withOffset(max(0, m.modal.viewport.logicalOffset-1))
	case key.Matches(msg, m.keys.Down):
		m.modal.viewport = m.modal.viewport.withOffset(min(maximum, m.modal.viewport.logicalOffset+1))
	case key.Matches(msg, m.keys.Home):
		m.modal.viewport = m.modal.viewport.withOffset(0)
	case key.Matches(msg, m.keys.End):
		m.modal.viewport = m.modal.viewport.withOffset(maximum)
	}
}

func (m *Model) closeModal() {
	m.closeGenericModal()
}

func (m *Model) closeUnsavedChanges() {
	closed, owner, err := m.modal.close()
	if err != nil {
		return
	}
	m.modal = closed
	m.focusOwner = owner
}

func (m *Model) handleUnsavedChangesKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	payload, ok := m.modal.payload.(unsavedChangesPayload)
	if !ok || !payload.valid() {
		return m, nil
	}
	switch msg.String() {
	case "s":
		m.modal.recoverableError = ""
		m.connectionEdit.quitAfterSave = payload.intent == unsavedIntentQuit
		command := m.saveForm()
		if command == nil && m.modal.kind == modalKindUnsavedChanges {
			if m.connectionEdit.conflict != nil {
				conflict, valid := newConflictState(m.connectionEdit.conflict.kind, m.connectionEdit.target, conflictOwnerModal)
				if valid {
					m.modal.conflict = &conflict
				}
			} else {
				m.modal.recoverableError = connectionFormSaveError(app.ErrInvalidRequest)
			}
		}
		return m, command
	case "d":
		m.closeUnsavedChanges()
		quit := payload.intent == unsavedIntentQuit
		m.closeConnectionForm(!quit)
		if quit {
			return m, tea.Quit
		}
	case "esc":
		m.closeUnsavedChanges()
	}
	return m, nil
}

func (m *Model) installFormConflict(kind conflictType) {
	if m.connectionEdit == nil {
		return
	}
	conflict, ok := newConflictState(kind, m.connectionEdit.target, conflictOwnerForm)
	if ok {
		m.connectionEdit.conflict = &conflict
		m.status = "CONFLICT"
	}
}

func (m *Model) handleFormConflictKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	conflict := m.connectionEdit.conflict
	if conflict == nil {
		return m, nil
	}
	switch msg.String() {
	case "r":
		m.pendingSelection = ""
		return m, m.reloadFormConflictCommand()
	case "b":
		selected := conflict.backSelection(m.browser.hasNode, m.browser.snapshot.rootID)
		m.closeConnectionForm(false)
		m.browser.selectedID = selected
		m.browser.expandAncestors(selected)
		m.browser.rebuildRows()
		m.ownedSelectionID = selected
		m.syncDetail()
		m.status = "READY"
	case "esc":
		compact := conflict.compact()
		m.connectionEdit.conflict = &compact
	case "?":
		m.toggleHelp()
	case "q", "ctrl+c":
		if m.form.dirty() {
			m.openUnsavedChanges(unsavedIntentQuit)
		} else {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) handleFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.FormHelp) || m.form.focusedField() > fieldIdentity && key.Matches(msg, m.keys.Help):
		m.toggleHelp()
		return m, nil
	case key.Matches(msg, m.keys.Back):
		if m.form.cancelNeedsConfirmation() {
			m.openUnsavedChanges(unsavedIntentCancel)
		} else {
			m.closeConnectionForm(true)
		}
		return m, nil
	case msg.String() == "ctrl+c" || m.form.focusedField() > fieldIdentity && msg.String() == "q":
		if m.form.dirty() {
			m.openUnsavedChanges(unsavedIntentQuit)
			return m, nil
		}
		return m, tea.Quit
	case key.Matches(msg, m.keys.Save), key.Matches(msg, m.keys.Open):
		return m, m.saveForm()
	case msg.String() == "ctrl+r" || m.form.focusedField() == fieldSave && msg.String() == "r":
		return m, m.reloadCommand()
	default:
		return m, m.form.update(msg)
	}
}

func (m *Model) resolveRecoveryCommand(action recoveryAction) tea.Cmd {
	modal := m.activeSSHFailure()
	if modal == nil || modal.failure == nil || modal.attempt.ID == "" {
		return nil
	}
	target := capturedTarget{id: modal.attempt.ID, revision: modal.attempt.Revision, kind: app.NodeKindConnection, path: modal.attempt.Path, endpointOrScope: modal.attempt.Host}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationReload, &target, operationOwnerModal)
	if !ok {
		return nil
	}
	modal.recovery = recoveryResolving
	capturedID := modal.attempt.ID
	return func() tea.Msg {
		if m.connections == nil {
			return recoveryResolvedMsg{id: id, action: action, err: errUnavailable}
		}
		result, err := m.connections.Get(operationCtx, app.ItemSelector{ID: capturedID})
		if err == nil && result.Connection.ID != capturedID {
			err = app.ErrConflict
		}
		return recoveryResolvedMsg{id: id, action: action, connection: result.Connection, err: err}
	}
}

func (m *Model) handleRecoveryResolved(msg recoveryResolvedMsg) tea.Cmd {
	modal := m.activeSSHFailure()
	if !m.acceptsOperationResult(msg.id, asyncOperationReload) || modal == nil {
		return nil
	}
	quit := m.completeOperation(msg.id, nil)
	m.status = "ERROR"
	if quit {
		return tea.Quit
	}
	if msg.err != nil {
		switch app.ErrorKindOf(msg.err) {
		case app.ErrorKindNotFound:
			modal.recovery = recoveryMissing
		case app.ErrorKindConflict:
			modal.recovery = recoveryConflict
		default:
			modal.recovery = recoveryConflict
		}
		return nil
	}
	if msg.action == recoveryEdit {
		modal.recovery = recoveryEditing
		m.closeGenericModal()
		m.openConnectionFormWithRecovery(newConnectionForm(&msg.connection), m.captureConnectionTarget(msg.connection), modal)
		return nil
	}
	modal.recovery = recoveryConfirming
	if m.modal.kind == modalKindSSHFailure {
		m.modal.payload = sshFailurePayload{modal: modal, confirmation: newRetryConnectConfirmation(modal.attempt, msg.connection)}
		m.modal.viewport = newViewportState(0)
	}
	return nil
}

func (m *Model) activeSSHFailure() *errorModal {
	payload, ok := m.modal.payload.(sshFailurePayload)
	if !ok {
		return nil
	}
	return payload.modal
}

func (m *Model) backFromFailure(modal *errorModal) {
	if modal == nil || modal.failure == nil || modal.attempt.ID == "" {
		return
	}
	if modal.recovery == recoveryResolving {
		m.cancelCurrentOperation(false)
	}
	if modal.recovery != recoveryMissing && m.browser.hasNode(modal.attempt.ID) {
		m.browser.selectedID = modal.attempt.ID
		m.ownedSelectionID = m.browser.selectedID
		m.browser.expandAncestors(modal.attempt.ID)
		m.browser.rebuildRows()
		m.syncDetail()
		m.ownedSelectionID = m.browser.selectedID
		m.status = "READY"
		return
	}
	m.status = "TARGET NO LONGER EXISTS"
}

func (m *Model) saveForm() tea.Cmd {
	if m.connectionEdit != nil && m.connectionEdit.conflict != nil && m.connectionEdit.conflict.blocked {
		m.connectionEdit.quitAfterSave = false
		return nil
	}
	if m.form.original == nil {
		if m.staleDestination(m.form.destination) {
			m.installFormConflict(conflictTypeRevisionChanged)
			return nil
		}
		request, valid := m.form.createRequest()
		if !valid {
			if m.connectionEdit != nil {
				m.connectionEdit.quitAfterSave = false
			}
			return nil
		}
		if m.destinationNameTaken(m.form.destination, request.Name) {
			m.form.errors[fieldName] = "name already exists in this folder"
			m.form.setFocus(fieldName)
			m.status = "ERROR"
			return nil
		}
		destination := m.form.destination
		if destination != nil && destination.Path != m.form.inputs[fieldFolder].Value() {
			destination = nil
		}
		return m.createCommand(request, m.form.remember, destination)
	}
	request, changed := m.form.updateRequest()
	if !changed {
		m.form.errors[fieldSave] = "change at least one field before saving"
		if m.connectionEdit != nil {
			m.connectionEdit.quitAfterSave = false
		}
		return nil
	}
	return m.updateCommand(request, m.form.remember)
}

func (m *Model) destinationNameTaken(destination *app.Folder, name string) bool {
	if destination == nil {
		return false
	}
	id := destination.ID
	if id == syntheticRootID {
		id = m.browser.snapshot.rootID
	}
	for _, childID := range m.browser.snapshot.children[id] {
		if child, exists := m.browser.snapshot.nodes[childID]; exists && child.node.Name == name {
			return true
		}
	}
	return false
}

func (m *Model) staleDestination(destination *app.Folder) bool {
	if destination == nil {
		return false
	}
	node, exists := m.browser.snapshot.nodes[destination.ID]
	return !exists || node.folder == nil || node.node.Revision != destination.Revision || node.node.Path != destination.Path
}

func (m *Model) beginOperationWith(kind asyncOperationKind, target *capturedTarget, owner operationOwnerSurface, retries ...operationRetryIntent) (uint64, context.Context, bool) {
	operation, lastID, ok := startOperation(m.operationID, m.operation, kind, target, owner, retries...)
	if !ok {
		return 0, nil, false
	}
	m.operationID = lastID
	m.operation = operation
	m.operation.ctx, m.operation.cancel = context.WithCancel(m.ctx)
	m.status = "LOADING"
	return operation.id, m.operation.ctx, true
}

func (m *Model) acceptsOperationResult(id uint64, kind asyncOperationKind) bool {
	return m.operation != nil && m.operation.matchesResult(id, kind)
}

func (m *Model) completeOperation(id uint64, commit any) bool {
	if m.operation == nil {
		return false
	}
	state := *m.operation
	if commit != nil {
		var accepted bool
		state, accepted = state.recordCommit(id, state.kind, commit)
		if !accepted {
			return false
		}
		m.operation = &state
	}
	var ok bool
	state, ok = state.beginCleanup(id, state.kind)
	if !ok {
		return false
	}
	m.operation = &state
	state, ok = state.finishCleanup(id)
	if !ok {
		return false
	}
	m.operation = &state
	_, release, ok := releaseOperation(&state)
	if !ok {
		return false
	}
	if m.operation.cancel != nil {
		m.operation.cancel()
	}
	if m.modal.kind == modalKindHelp {
		m.closeGenericModal()
	}
	m.operation = nil
	return release.quitIntent
}

func (m *Model) cancelCurrentOperation(quit bool) {
	if m.operation == nil {
		return
	}
	state, ok := m.operation.requestCancellation(m.operation.id, quit)
	if !ok {
		return
	}
	m.operation = &state
	if m.operation.cancel != nil {
		m.operation.cancel()
	}
}

func (m *Model) reloadCommand() tea.Cmd {
	kind := asyncOperationReload
	if m.browser.revision == 0 {
		kind = asyncOperationInitialLoad
	}
	return m.catalogReloadCommand(kind, nil, operationOwnerRoot)
}

func (m *Model) reloadFormConflictCommand() tea.Cmd {
	if m.connectionEdit == nil || m.connectionEdit.conflict == nil {
		return nil
	}
	target := m.connectionEdit.target.clone()
	return m.catalogReloadCommand(asyncOperationReload, &target, operationOwnerForm)
}

func (m *Model) catalogReloadCommand(kind asyncOperationKind, target *capturedTarget, owner operationOwnerSurface) tea.Cmd {
	return m.catalogReloadCommandWithRetry(kind, target, owner, catalogRetryIntent(kind, target, owner))
}

func (m *Model) catalogReloadCommandWithRetry(kind asyncOperationKind, target *capturedTarget, owner operationOwnerSurface, retry operationRetryIntent) tea.Cmd {
	id, operationCtx, ok := m.beginOperationWith(kind, target, owner, retry)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if m.folders != nil {
			snapshot, err := loadCatalogSnapshot(operationCtx, m.folders)
			return operationResultMsg{id: id, kind: operationReload, snapshot: snapshot, err: err}
		}
		if m.connections == nil {
			return operationResultMsg{id: id, kind: operationReload, err: errUnavailable}
		}
		result, err := m.connections.List(operationCtx, app.ListConnectionsRequest{Folder: app.ItemSelector{Path: "/"}})
		return operationResultMsg{id: id, kind: operationReload, list: result, err: err}
	}
}

func loadCatalogSnapshot(ctx context.Context, folders FolderService) (*catalogSnapshot, error) {
	for attempt := 0; attempt < 2; attempt++ {
		root, err := folders.Get(ctx, app.ItemSelector{Path: "/"})
		if err != nil {
			return nil, err
		}
		snapshot := newCatalogSnapshot(root.Folder, root.CatalogRevision)
		pending := []app.Folder{root.Folder}
		consistent := true
		for len(pending) != 0 {
			current := pending[0]
			pending = pending[1:]
			children, listErr := folders.List(ctx, app.ListChildrenRequest{Folder: app.ItemSelector{ID: current.ID}})
			if listErr != nil {
				return nil, listErr
			}
			if children.CatalogRevision != root.CatalogRevision {
				consistent = false
				break
			}
			if !snapshot.addChildren(current.ID, children) {
				return nil, errors.Join(app.ErrConflict, errors.New("catalog tree contains duplicate or invalid nodes"))
			}
			pending = append(pending, children.Folders...)
		}
		if consistent {
			return &snapshot, nil
		}
	}
	return nil, app.ErrConflict
}

func (m *Model) createCommand(request app.CreateConnectionRequest, remember bool, destination *app.Folder) tea.Cmd {
	target := targetFromFolder(destination)
	owner := operationOwnerForm
	if m.modal.kind == modalKindUnsavedChanges {
		owner = operationOwnerModal
	}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, target, owner)
	if !ok {
		return nil
	}
	var capturedDestination *app.Folder
	if destination != nil {
		copy := *destination
		capturedDestination = &copy
	}
	if remember {
		command := &credentialMutationCommand{ctx: operationCtx, terminal: m.terminal, service: m.connections, folders: m.folders, destination: capturedDestination, create: &request}
		return command.prepare(id, operationCreate)
	}
	return func() tea.Msg {
		if err := verifyDestination(operationCtx, m.folders, capturedDestination); err != nil {
			return operationResultMsg{id: id, kind: operationCreate, targetConflict: true, err: err}
		}
		result, err := m.connections.Create(operationCtx, request)
		return operationResultMsg{
			id: id, kind: operationCreate, connection: result, err: err,
			targetConflict: destinationChangedAfterCreate(operationCtx, m.folders, capturedDestination, err),
		}
	}
}

func (m *Model) updateCommand(request app.UpdateConnectionRequest, remember bool) tea.Cmd {
	var target *capturedTarget
	if m.connectionEdit != nil {
		copy := m.connectionEdit.target.clone()
		target = &copy
	} else {
		target = m.capturedSelectedTarget()
	}
	owner := operationOwnerForm
	if m.modal.kind == modalKindUnsavedChanges {
		owner = operationOwnerModal
	}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, target, owner)
	if !ok {
		return nil
	}
	if remember {
		command := &credentialMutationCommand{ctx: operationCtx, terminal: m.terminal, service: m.connections, update: &request}
		return command.prepare(id, operationUpdate)
	}
	return func() tea.Msg {
		result, err := m.connections.Update(operationCtx, request)
		return operationResultMsg{id: id, kind: operationUpdate, connection: result, err: err}
	}
}

func (m *Model) deleteScopeCommand(connection app.Connection) tea.Cmd {
	target := capturedTargetFromConnection(connection)
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, &target, operationOwnerModal, connectionDeleteScopeRetryIntent(connection))
	if !ok {
		return nil
	}
	return func() tea.Msg {
		scope, err := m.connections.DeleteScope(operationCtx, app.ItemSelector{ID: connection.ID})
		if err == nil && (scope.ID != connection.ID || scope.Revision != connection.Revision || scope.Path != connection.Path) {
			err = app.ErrConflict
		}
		return operationResultMsg{id: id, kind: operationDeleteScope, scope: scope, err: err}
	}
}

func (m *Model) deleteCommand(scope app.ConnectionDeleteScope) tea.Cmd {
	target := capturedTarget{id: scope.ID, revision: scope.Revision, kind: app.NodeKindConnection, path: scope.Path, endpointOrScope: scope.Host}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, &target, operationOwnerModal)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		result, err := m.connections.Delete(operationCtx, scope.Request())
		return operationResultMsg{id: id, kind: operationDelete, deleted: result, err: err}
	}
}

func (m *Model) folderCreateCommand(request app.CreateFolderRequest, destination *app.Folder) tea.Cmd {
	target := targetFromFolder(destination)
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, target, operationOwnerModal)
	if !ok {
		return nil
	}
	var capturedDestination *app.Folder
	if destination != nil {
		copy := *destination
		capturedDestination = &copy
	}
	return func() tea.Msg {
		if m.folders == nil {
			return operationResultMsg{id: id, kind: operationFolderCreate, err: errUnavailable}
		}
		if err := verifyDestination(operationCtx, m.folders, capturedDestination); err != nil {
			return operationResultMsg{id: id, kind: operationFolderCreate, err: err}
		}
		result, err := m.folders.Create(operationCtx, request)
		return operationResultMsg{id: id, kind: operationFolderCreate, folder: result.Folder, err: err}
	}
}

func verifyDestination(ctx context.Context, folders FolderService, destination *app.Folder) error {
	if destination == nil || destination.ID == syntheticRootID || folders == nil {
		return nil
	}
	current, err := folders.Get(ctx, app.ItemSelector{ID: destination.ID})
	if err != nil {
		return err
	}
	if current.Folder.ID != destination.ID || current.Folder.Revision != destination.Revision || current.Folder.Path != destination.Path {
		return app.ErrConflict
	}
	return nil
}

func destinationChangedAfterCreate(ctx context.Context, folders FolderService, destination *app.Folder, createErr error) bool {
	kind := app.ErrorKindOf(createErr)
	if kind != app.ErrorKindConflict && kind != app.ErrorKindNotFound {
		return false
	}
	return verifyDestination(ctx, folders, destination) != nil
}

func capturedTargetFromFolder(folder app.Folder) capturedTarget {
	return capturedTarget{id: folder.ID, revision: folder.Revision, kind: app.NodeKindFolder, path: folder.Path}
}

func capturedTargetFromConnection(connection app.Connection) capturedTarget {
	return capturedTarget{
		id:              connection.ID,
		revision:        connection.Revision,
		kind:            app.NodeKindConnection,
		path:            connection.Path,
		endpointOrScope: fmt.Sprintf("%s:%d", connection.Host, connection.Port),
	}
}

func targetFromFolder(folder *app.Folder) *capturedTarget {
	if folder == nil {
		return nil
	}
	target := capturedTargetFromFolder(*folder)
	return &target
}

func (m *Model) capturedSelectedTarget() *capturedTarget {
	if connection := m.browser.selection(); connection != nil {
		target := capturedTargetFromConnection(*connection)
		return &target
	}
	if folder := m.browser.selectionFolder(); folder != nil {
		target := capturedTargetFromFolder(*folder)
		return &target
	}
	return nil
}

func (m *Model) folderRenameCommand(request app.RenameFolderRequest) tea.Cmd {
	var target *capturedTarget
	if m.modal.target != nil {
		copy := m.modal.target.clone()
		target = &copy
	}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, target, operationOwnerModal)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if m.folders == nil {
			return operationResultMsg{id: id, kind: operationFolderRename, err: errUnavailable}
		}
		result, err := m.folders.Rename(operationCtx, request)
		return operationResultMsg{id: id, kind: operationFolderRename, folder: result.Folder, err: err}
	}
}

func (m *Model) folderDeleteScopeCommand(folder app.Folder) tea.Cmd {
	target := capturedTargetFromFolder(folder)
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, &target, operationOwnerModal, folderDeleteScopeRetryIntent(folder))
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if m.folders == nil {
			return operationResultMsg{id: id, kind: operationFolderDeleteScope, err: errUnavailable}
		}
		scope, err := m.folders.DeleteScope(operationCtx, app.ItemSelector{ID: folder.ID})
		if err == nil && (scope.ID != folder.ID || scope.Revision != folder.Revision || scope.Path != folder.Path) {
			err = app.ErrConflict
		}
		return operationResultMsg{id: id, kind: operationFolderDeleteScope, folderScope: scope, err: err}
	}
}

func (m *Model) folderDeleteCommand(scope app.FolderDeleteScope) tea.Cmd {
	target := capturedTarget{id: scope.ID, revision: scope.Revision, kind: app.NodeKindFolder, path: scope.Path}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, &target, operationOwnerModal)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		if m.folders == nil {
			return operationResultMsg{id: id, kind: operationFolderDelete, err: errUnavailable}
		}
		_, err := m.folders.Delete(operationCtx, scope.Request(true))
		return operationResultMsg{id: id, kind: operationFolderDelete, err: err}
	}
}

func (m *Model) moveCommand(source app.Node, destination app.Folder) tea.Cmd {
	target := capturedTarget{id: source.ID, revision: source.Revision, kind: source.Kind, path: source.Path}
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSave, &target, operationOwnerModal)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		expected := source.Revision
		expectedDestination := destination.Revision
		if source.Kind == app.NodeKindFolder {
			if m.folders == nil {
				return operationResultMsg{id: id, kind: operationMove, err: errUnavailable}
			}
			result, err := m.folders.Move(operationCtx, app.MoveFolderRequest{Folder: app.ItemSelector{ID: source.ID}, Destination: app.ItemSelector{ID: destination.ID}, Expected: &expected, ExpectedDestination: &expectedDestination, ExpectedSourcePath: source.Path, ExpectedDestinationPath: destination.Path})
			return operationResultMsg{id: id, kind: operationMove, folder: result.Folder, err: err}
		}
		if m.connections == nil {
			return operationResultMsg{id: id, kind: operationMove, err: errUnavailable}
		}
		result, err := m.connections.Move(operationCtx, app.MoveConnectionRequest{Connection: app.ItemSelector{ID: source.ID}, Destination: app.ItemSelector{ID: destination.ID}, Expected: &expected, ExpectedDestination: &expectedDestination, ExpectedSourcePath: source.Path, ExpectedDestinationPath: destination.Path})
		return operationResultMsg{id: id, kind: operationMove, connection: result, err: err}
	}
}

func (m *Model) handleOperation(msg operationResultMsg) tea.Cmd {
	kind := asyncOperationSave
	if msg.kind == operationReload {
		kind = asyncOperationReload
		if m.operation != nil && m.operation.kind == asyncOperationInitialLoad {
			kind = asyncOperationInitialLoad
		}
	}
	if !m.acceptsOperationResult(msg.id, kind) {
		return nil
	}
	operationEndpoint := ""
	var operationTarget *capturedTarget
	var retry *operationRetryIntent
	if m.operation != nil && m.operation.target != nil {
		operationEndpoint = m.operation.target.endpointOrScope
		copy := m.operation.target.clone()
		operationTarget = &copy
	}
	if m.operation != nil && m.operation.retry != nil {
		copy := m.operation.retry.clone()
		retry = &copy
	}
	commit := any(nil)
	if msg.err == nil {
		commit = msg
	}
	quit := m.completeOperation(msg.id, commit)
	m.status = "READY"
	if failure := m.activeSSHFailure(); msg.kind == operationReload && failure != nil {
		if msg.err != nil {
			if app.ErrorKindOf(msg.err) == app.ErrorKindNotFound {
				failure.recovery = recoveryMissing
			} else {
				failure.recovery = recoveryConflict
			}
			m.status = "ERROR"
			return nil
		}
		failure.recovery = recoveryIdle
	}
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			if m.connectionEdit != nil {
				m.connectionEdit.quitAfterSave = false
			}
			m.status = "READY"
			if quit {
				return tea.Quit
			}
			return nil
		}
		if (msg.kind == operationCreate || msg.kind == operationUpdate) && m.form != nil && m.connectionEdit != nil {
			m.connectionEdit.quitAfterSave = false
			switch app.ErrorKindOf(msg.err) {
			case app.ErrorKindNotFound:
				m.installFormConflict(conflictTypeMissing)
			case app.ErrorKindConflict:
				if msg.kind == operationCreate && !msg.targetConflict {
					m.form.setFormError("Save failed. The name may already exist in this folder; choose another name or reload and try again.")
					m.status = "ERROR"
				} else {
					m.installFormConflict(conflictTypeRevisionChanged)
				}
			default:
				m.form.setFormError(connectionFormSaveError(msg.err))
				m.status = "ERROR"
			}
			if m.modal.kind == modalKindUnsavedChanges {
				if m.connectionEdit.conflict != nil {
					conflict, valid := newConflictState(m.connectionEdit.conflict.kind, m.connectionEdit.target, conflictOwnerModal)
					if valid {
						m.modal.conflict = &conflict
					}
				} else {
					m.modal.recoverableError = m.form.formError
				}
			}
			return nil
		}
		if msg.kind == operationReload && m.form != nil && m.connectionEdit != nil && m.connectionEdit.conflict != nil {
			m.form.setFormError("Reload failed safely. The conflict is still blocking Save; try Reload or Back.")
			m.status = "ERROR"
			return nil
		}
		operation, target := operationName(msg.kind), "/"
		if operationTarget != nil && operationTarget.path != "" {
			target = operationTarget.path
		} else if m.form != nil && m.form.original != nil {
			target = m.form.original.Path
		} else if retry == nil {
			if selected := m.browser.selectionNode(); selected != nil {
				target = selected.Path
			}
		}
		failure := newErrorModal(operation, target, msg.err)
		failure.retry = retry
		if m.modal.isOpen() {
			m.modal.recoverableError = failure.message
			if kind := app.ErrorKindOf(msg.err); kind == app.ErrorKindConflict || kind == app.ErrorKindNotFound {
				conflictKind := conflictTypeRevisionChanged
				if kind == app.ErrorKindNotFound {
					conflictKind = conflictTypeMissing
				}
				if m.modal.target != nil {
					if conflict, ok := newConflictState(conflictKind, *m.modal.target, conflictOwnerModal); ok {
						m.modal.conflict = &conflict
					}
				}
			}
		} else {
			m.openGenericModal(modalKindOperationError, operationTarget, operationErrorPayload{modal: failure})
		}
		m.status = "ERROR"
		return nil
	}
	switch msg.kind {
	case operationReload:
		if m.modal.conflict != nil {
			m.modal.recoverableError = ""
		}
		revision := msg.list.CatalogRevision
		if msg.snapshot != nil {
			revision = msg.snapshot.revision
		}
		selectedID := m.browser.selectedID
		pendingSelection := m.pendingSelection
		selectedRevision := app.Revision(0)
		if selected := m.browser.selectionNode(); selected != nil {
			selectedRevision = selected.Revision
		}
		if m.screen == screenConnectionForm && m.browser.revision != 0 && m.browser.revision != revision {
			m.status = "CATALOG CHANGED"
		}
		changed := m.screen == screenConnectionForm && m.browser.revision != 0 && m.browser.revision != revision
		if msg.snapshot != nil {
			m.browser.setSnapshot(*msg.snapshot, m.pendingSelection)
		} else {
			m.browser.setConnectionsPending(msg.list, m.pendingSelection)
		}
		navigationChanged := false
		m.navigationNotice = navigationNoticeNone
		if kind != asyncOperationInitialLoad && m.screen == screenBrowser && pendingSelection == "" && selectedID != "" {
			if !m.browser.hasNode(selectedID) {
				m.navigationNotice = navigationNoticeMissing
				m.status = "TARGET NO LONGER EXISTS"
			} else if m.browser.selectedID == selectedID {
				if selected := m.browser.selectionNode(); selected != nil && selected.Revision != selectedRevision {
					m.navigationNotice = navigationNoticeRevisionChanged
					navigationChanged = true
					m.status = "CATALOG CHANGED"
				}
			}
		}
		m.syncDetail()
		m.ownedSelectionID = m.browser.selectedID
		m.pendingSelection = ""
		m.browser.changed = changed || navigationChanged
		if m.connectionEdit != nil && m.browser.hasNode(m.connectionEdit.treeContext.selectedID) {
			m.browser.selectedID = m.connectionEdit.treeContext.selectedID
			m.browser.expandAncestors(m.browser.selectedID)
			m.browser.rebuildRows()
			m.ownedSelectionID = m.browser.selectedID
			m.syncDetail()
		}
		m.recheckConnectionFormConflict()
		m.recheckModalConflict()
	case operationCreate, operationUpdate:
		quitAfterSave := m.connectionEdit != nil && m.connectionEdit.quitAfterSave
		m.pendingSelection = msg.connection.Connection.ID
		if m.modal.kind == modalKindUnsavedChanges {
			m.closeGenericModal()
		}
		m.closeConnectionForm(false)
		return m.reloadAfterCommit(quit || quitAfterSave)
	case operationDelete:
		m.closeModal()
		m.form, m.screen = nil, screenBrowser
		return m.reloadAfterCommit(quit)
	case operationDeleteScope:
		confirmation := newDeleteConfirmation(msg.scope, operationEndpoint)
		target := capturedTarget{id: msg.scope.ID, revision: msg.scope.Revision, kind: app.NodeKindConnection, path: msg.scope.Path, endpointOrScope: operationEndpoint}
		m.openGenericModal(modalKindDeleteConnection, &target, deleteConnectionPayload{confirmation: confirmation})
	case operationFolderCreate, operationFolderRename:
		m.pendingSelection = msg.folder.ID
		m.closeGenericModal()
		return m.reloadAfterCommit(quit)
	case operationFolderDeleteScope:
		confirmation := newFolderDeleteConfirmation(msg.folderScope)
		target := capturedTarget{id: msg.folderScope.ID, revision: msg.folderScope.Revision, kind: app.NodeKindFolder, path: msg.folderScope.Path}
		m.openGenericModal(modalKindDeleteFolder, &target, deleteFolderPayload{confirmation: confirmation})
	case operationFolderDelete:
		m.closeModal()
		return m.reloadAfterCommit(quit)
	case operationMove:
		if msg.folder.ID != "" {
			m.pendingSelection = msg.folder.ID
		} else {
			m.pendingSelection = msg.connection.Connection.ID
		}
		m.closeGenericModal()
		return m.reloadAfterCommit(quit)
	}
	if quit {
		return tea.Quit
	}
	return nil
}

func (m *Model) reloadAfterCommit(quit bool) tea.Cmd {
	command := m.reloadCommand()
	if quit && m.operation != nil {
		m.operation.quitIntent = true
	}
	return command
}

func (m *Model) observedFormTarget() *capturedTarget {
	if m.connectionEdit == nil {
		return nil
	}
	target := m.connectionEdit.target
	node, exists := m.browser.snapshot.nodes[target.id]
	if !exists || node.node.Kind != target.kind {
		return nil
	}
	observed := target.clone()
	observed.revision = node.node.Revision
	observed.path = node.node.Path
	return &observed
}

func (m *Model) observedModalTarget() *capturedTarget {
	if m.modal.conflict == nil {
		return nil
	}
	target := m.modal.conflict.target
	node, exists := m.browser.snapshot.nodes[target.id]
	if !exists || node.node.Kind != target.kind {
		return nil
	}
	observed := target.clone()
	observed.revision = node.node.Revision
	observed.path = node.node.Path
	return &observed
}

func (m *Model) recheckModalConflict() {
	if m.modal.conflict == nil || m.modal.conflict.owner != conflictOwnerModal {
		return
	}
	next := m.modal.conflict.recheck(m.observedModalTarget())
	m.modal.conflict = next
	if next == nil {
		m.modal.recoverableError = ""
		m.status = "READY"
	} else {
		m.status = "CONFLICT"
	}
}

func (m *Model) recheckConnectionFormConflict() {
	if m.connectionEdit == nil || m.connectionEdit.conflict == nil {
		return
	}
	next := m.connectionEdit.conflict.recheck(m.observedFormTarget())
	m.connectionEdit.conflict = next
	if next == nil {
		m.status = "READY"
	} else {
		m.status = "CONFLICT"
	}
}

func operationName(kind operationKind) string {
	switch kind {
	case operationReload:
		return "reload catalog"
	case operationCreate:
		return "create connection"
	case operationUpdate:
		return "save connection"
	case operationDeleteScope, operationDelete:
		return "delete connection"
	case operationFolderCreate:
		return "create folder"
	case operationFolderRename:
		return "rename folder"
	case operationFolderDeleteScope, operationFolderDelete:
		return "delete folder"
	case operationMove:
		return "move catalog item"
	default:
		return "catalog operation"
	}
}

func (m *Model) View() tea.View {
	layout := calculateLayout(m.width, m.height, m.focusedLayoutRegion())
	if layout.mode == layoutUndersized {
		content := m.undersizedView()
		view := tea.NewView(fitContent(content, m.width, m.height))
		view.AltScreen = true
		view.WindowTitle = "Orza"
		return view
	}

	content := m.browserShell(layout)
	if m.modal.isOpen() {
		background := m.browserShell(layout)
		modal := m.modal
		if m.operation != nil {
			modal.operationStatus = m.operation.loadingStatus()
		}
		content = renderModalOverlay(background, modal, layout, m.styles, wrapHelpLines(actionHelpLines(m.currentActionDescriptors()), layout.modalOverlay().contentWidth()))
	}
	view := tea.NewView(fitContent(content, m.width, m.height))
	view.AltScreen = true
	view.WindowTitle = "Orza"
	return view
}

func (m *Model) browserShell(layout layoutState) string {
	treeProjection := m.browser.projectTree(m.styles, layout.tree.contentWidth(), layout.tree.contentHeight())
	tree := treeProjection.lines

	var details []string
	detailScrollbar := scrollbarGeometry{}
	detailTrackStart := 0
	if m.securityInput != nil && m.securityInput.trust != nil {
		details = strings.Split(m.securityInput.trust.view(m.styles), "\n")
	} else if m.securityInput != nil && m.securityInput.secret != nil {
		details = strings.Split(m.securityInput.secret.view(m.styles), "\n")
	} else if m.screen == screenConnectionForm && m.form != nil {
		if conflict := m.formConflictLines(layout.details.contentWidth()); len(conflict) != 0 {
			details = append(details, conflict...)
		}
		formHeight := max(1, layout.details.contentHeight()-len(details))
		detailTrackStart = len(details)
		projection := m.form.projectAt(m.styles, layout.details.contentWidth(), formHeight)
		details = append(details, projection.lines...)
		detailScrollbar = projection.scrollbar
	} else {
		detailRows := layout.details.contentHeight()
		if notice := m.navigationNotice.text(); notice != "" {
			details = append(details, m.styles.warningMessage(safeText(notice, layout.details.contentWidth())))
			detailRows--
		}
		detailTrackStart = len(details)
		projection := m.detailState.project(max(0, detailRows), layout.details.contentWidth())
		details = append(details, projection.lines...)
		detailScrollbar = projection.scrollbar
	}

	context := m.actionContext()
	status := ""
	if m.operation != nil {
		context.state = actionStateOperation
		status = m.operation.loadingStatus()
	}
	var actions []string
	if m.screen == screenConnectionForm && m.operation == nil {
		if m.connectionEdit != nil && m.connectionEdit.conflict != nil {
			context.state = actionStateConflict
			actions = packActions(status, actionsFor(context), layout.actions.contentWidth(), layout.actions.contentHeight())
		} else {
			actions = packActions(status, connectionFormActionDescriptors, layout.actions.contentWidth(), layout.actions.contentHeight())
		}
	} else {
		actions = packActions(status, actionsFor(context), layout.actions.contentWidth(), layout.actions.contentHeight())
	}
	treePanel := renderRegionPanelWithScrollbar(m.styles.regionTitle("Tree", m.focusOwner == focusOwnerTree), tree, layout.tree, m.styles, treeProjection.scrollbar, 0)
	detailPanel := renderRegionPanelWithScrollbar(m.styles.regionTitle("Details", m.focusOwner == focusOwnerDetail || m.focusOwner == focusOwnerConnectionForm), details, layout.details, m.styles, detailScrollbar, detailTrackStart)
	actionsPanel := renderRegionPanel(m.styles.regionTitle("Actions", false), actions, layout.actions)

	if layout.mode == layoutWide {
		left, right := strings.Split(treePanel, "\n"), strings.Split(detailPanel, "\n")
		base := make([]string, layout.tree.height)
		for index := range base {
			base[index] = left[index] + strings.Repeat(" ", wideGutterWidth) + right[index]
		}
		return strings.Join(append(base, strings.Split(actionsPanel, "\n")...), "\n")
	}
	return treePanel + "\n" + detailPanel + "\n" + actionsPanel
}

func (m *Model) actionContext() actionContext {
	context := actionContext{state: actionStateNormal}
	if m.focusOwner == focusOwnerDetail {
		context.focus = actionFocusDetails
	} else {
		context.focus = actionFocusTree
	}
	selected := m.browser.selectedTreeNode()
	if selected == nil {
		return context
	}
	switch {
	case selected.root:
		context.selection = actionSelectionRoot
	case selected.folder != nil:
		context.selection = actionSelectionFolder
		context.canToggle = len(m.browser.snapshot.children[selected.node.ID]) != 0
	case selected.connection != nil:
		context.selection = actionSelectionConnection
	}
	return context
}

func (m *Model) currentActionDescriptors() []actionDescriptor {
	if m.operation != nil {
		return actionsFor(actionContext{state: actionStateOperation})
	}
	if m.connectionEdit != nil && m.connectionEdit.conflict != nil || m.modal.conflict != nil {
		return actionsFor(actionContext{state: actionStateConflict})
	}
	return actionsFor(m.actionContext())
}

func (f *connectionForm) projectAt(style styles, width, height int) viewportProjection {
	f.setDimensions(width, height)
	return f.project(style)
}

func (m *Model) formConflictLines(width int) []string {
	if m.connectionEdit == nil || m.connectionEdit.conflict == nil {
		return nil
	}
	conflict := m.connectionEdit.conflict
	kind := "captured connection changed"
	if conflict.kind == conflictTypeMissing {
		kind = "captured connection is missing"
	}
	lines := []string{m.styles.warning.Render("Warning: Save blocked: " + kind + ".")}
	if conflict.detailVisible {
		lines = append(lines,
			"Target: "+conflict.target.path,
			fmt.Sprintf("ID: %s  Revision: %d", conflict.target.id, conflict.target.revision),
		)
	}
	lines = append(lines, "r Reload  b Back  Esc Cancel warning")
	return viewportTruncateLines(lines, width)
}

func renderRegionPanel(title string, content []string, rect layoutRect) string {
	return renderRegionPanelWithScrollbar(title, content, rect, styles{}, scrollbarGeometry{}, 0)
}

func renderRegionPanelWithScrollbar(title string, content []string, rect layoutRect, style styles, scrollbar scrollbarGeometry, trackStart int) string {
	if rect.width <= 0 || rect.height <= 0 {
		return ""
	}
	if rect.width == 1 {
		return strings.Repeat("│\n", max(0, rect.height-1)) + "│"
	}

	title = viewportEllipsis(title, max(0, rect.width-2))
	top := "┌" + title + strings.Repeat("─", max(0, rect.width-2-ansi.StringWidth(title))) + "┐"
	bottom := "└" + strings.Repeat("─", max(0, rect.width-2)) + "┘"
	lines := make([]string, rect.height)
	lines[0] = top
	if rect.height > 1 {
		lines[rect.height-1] = bottom
	}
	contentWidth := rect.contentWidth()
	for row := 1; row < rect.height-1; row++ {
		line := ""
		if row-1 < len(content) {
			line = viewportEllipsis(content[row-1], contentWidth)
		}
		rightCell := " "
		trackRow := row - 1 - trackStart
		if scrollbar.visible && trackRow >= 0 && trackRow < scrollbar.trackHeight {
			rightCell = style.scrollbarCell(scrollbar.thumbAt(trackRow))
		}
		lines[row] = "│ " + line + strings.Repeat(" ", max(0, contentWidth-ansi.StringWidth(line))) + rightCell + "│"
	}
	return strings.Join(lines, "\n")
}

func (m *Model) undersizedView() string {
	lines := []string{
		"Terminal too small",
		"Required minimum: 40x12",
		"? Help  q Quit",
	}
	if m.helpVisible() {
		lines = []string{
			"Help",
			"Resize to at least 40x12.",
			"? Close  q Quit",
		}
	}
	return strings.Join(lines, "\n")
}

func fitContent(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for index := range lines {
		if ansi.StringWidth(lines[index]) > width {
			lines[index] = viewportEllipsis(lines[index], width)
		}
	}
	return strings.Join(lines, "\n")
}
