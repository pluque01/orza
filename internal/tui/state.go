package tui

import (
	"context"

	"github.com/pluque01/orza/internal/app"
)

func (owner focusOwner) valid() bool {
	return owner >= focusOwnerTree && owner <= focusOwnerModal
}

func defaultFocusOwner() focusOwner { return focusOwnerTree }

type navigationNotice uint8

const (
	navigationNoticeNone navigationNotice = iota
	navigationNoticeMissing
	navigationNoticeRevisionChanged
)

func (notice navigationNotice) text() string {
	switch notice {
	case navigationNoticeMissing:
		return "TARGET NO LONGER EXISTS"
	case navigationNoticeRevisionChanged:
		return "CATALOG CHANGED"
	default:
		return ""
	}
}

type securityInputKind uint8

const (
	securityInputTrust securityInputKind = iota + 1
	securityInputSecret
)

func (kind securityInputKind) valid() bool {
	return kind == securityInputTrust || kind == securityInputSecret
}

// securityInputState preempts application input without becoming an
// application focus owner. Secret bytes remain owned by the terminal adapter.
type securityInputState struct {
	kind           securityInputKind
	preservedFocus focusOwner
	width          int
	height         int
	viewport       viewportState
	trust          *trustPrompt
	secret         *secretPrompt
	suspended      bool
}

func newSecurityInputState(kind securityInputKind, preserved focusOwner) (securityInputState, bool) {
	if !kind.valid() || !preserved.valid() {
		return securityInputState{}, false
	}
	return securityInputState{kind: kind, preservedFocus: preserved}, true
}

func (state securityInputState) preemptsApplicationInput() bool {
	return state.kind.valid() && state.preservedFocus.valid()
}

func (state securityInputState) withViewport(viewport viewportState) securityInputState {
	state.viewport = viewport
	return state
}

func (state securityInputState) close() focusOwner {
	return state.preservedFocus
}

func (state securityInputState) resize(width, height int) securityInputState {
	state.width = width
	state.height = height
	state.suspended = width < minimumLayoutWidth || height < minimumLayoutHeight
	return state
}

func (kind conflictType) valid() bool {
	return kind == conflictTypeMissing || kind == conflictTypeRevisionChanged
}

func (owner conflictOwner) valid() bool {
	return owner >= conflictOwnerForm && owner <= conflictOwnerOperation
}

func newConflictState(kind conflictType, target capturedTarget, owner conflictOwner) (conflictState, bool) {
	if !kind.valid() || !owner.valid() || target.id == "" {
		return conflictState{}, false
	}
	return conflictState{
		kind:          kind,
		target:        target.clone(),
		owner:         owner,
		detailVisible: true,
		blocked:       true,
	}, true
}

// recheck compares only the captured stable ID and revision. A different ID
// is treated as missing rather than becoming a replacement target.
func (state conflictState) recheck(observed *capturedTarget) *conflictState {
	next := state
	next.blocked = true
	switch {
	case observed == nil || observed.id != state.target.id:
		next.kind = conflictTypeMissing
	case observed.revision != state.target.revision:
		next.kind = conflictTypeRevisionChanged
	default:
		return nil
	}
	return &next
}

func (state conflictState) compact() conflictState {
	state.detailVisible = false
	state.blocked = true
	return state
}

func (state conflictState) backSelection(exists func(app.NodeID) bool, root app.NodeID) app.NodeID {
	if exists != nil {
		for _, id := range state.target.ancestorIDs {
			if exists(id) {
				return id
			}
		}
	}
	return root
}

type asyncOperationKind uint8

const (
	asyncOperationInitialLoad asyncOperationKind = iota
	asyncOperationReload
	asyncOperationSave
	asyncOperationSSHStart
)

func (kind asyncOperationKind) valid() bool {
	return kind >= asyncOperationInitialLoad && kind <= asyncOperationSSHStart
}

type operationOwnerSurface uint8

const (
	operationOwnerRoot operationOwnerSurface = iota
	operationOwnerForm
	operationOwnerModal
	operationOwnerTrust
)

func (owner operationOwnerSurface) valid() bool {
	return owner >= operationOwnerRoot && owner <= operationOwnerTrust
}

type asyncOperationPhase uint8

const (
	asyncPhaseRunning asyncOperationPhase = iota
	asyncPhaseCancelRequested
	asyncPhaseCleaningUp
	asyncPhaseCompleted
)

type operationStopReason uint8

const (
	operationStopNone operationStopReason = iota
	operationStopCanceled
	operationStopTimeout
	operationStopNetwork
)

type operationRetryKind uint8

const (
	operationRetryCatalog operationRetryKind = iota + 1
	operationRetryConnectionDeleteScope
	operationRetryFolderDeleteScope
	operationRetrySSHStart
)

// operationRetryIntent is a closed, typed description of the original action.
// It contains only values captured before dispatch and never consults selection.
type operationRetryIntent struct {
	kind       operationRetryKind
	reloadKind asyncOperationKind
	owner      operationOwnerSurface
	target     *capturedTarget
	connection app.Connection
	folder     app.Folder
}

func catalogRetryIntent(kind asyncOperationKind, target *capturedTarget, owner operationOwnerSurface) operationRetryIntent {
	intent := operationRetryIntent{kind: operationRetryCatalog, reloadKind: kind, owner: owner}
	if target != nil {
		copy := target.clone()
		intent.target = &copy
	}
	return intent
}

func connectionDeleteScopeRetryIntent(connection app.Connection) operationRetryIntent {
	return operationRetryIntent{kind: operationRetryConnectionDeleteScope, connection: connection}
}

func folderDeleteScopeRetryIntent(folder app.Folder) operationRetryIntent {
	return operationRetryIntent{kind: operationRetryFolderDeleteScope, folder: folder}
}

func sshStartRetryIntent(connection app.Connection) operationRetryIntent {
	return operationRetryIntent{kind: operationRetrySSHStart, connection: connection}
}

func (intent operationRetryIntent) valid() bool {
	switch intent.kind {
	case operationRetryCatalog:
		return (intent.reloadKind == asyncOperationInitialLoad || intent.reloadKind == asyncOperationReload) && intent.owner.valid()
	case operationRetryConnectionDeleteScope, operationRetrySSHStart:
		return intent.connection.ID != ""
	case operationRetryFolderDeleteScope:
		return intent.folder.ID != ""
	default:
		return false
	}
}

func (intent operationRetryIntent) clone() operationRetryIntent {
	if intent.target != nil {
		copy := intent.target.clone()
		intent.target = &copy
	}
	return intent
}

type operationCommitResult struct {
	value any
}

type operationState struct {
	id              uint64
	kind            asyncOperationKind
	phase           asyncOperationPhase
	ctx             context.Context
	cancel          context.CancelFunc
	target          *capturedTarget
	owner           operationOwnerSurface
	commitResult    *operationCommitResult
	stopReason      operationStopReason
	quitIntent      bool
	pendingConflict *conflictState
	retry           *operationRetryIntent
}

func (state operationState) loadingStatus() string {
	target := "catalog"
	if state.target != nil {
		target = state.target.path
		if state.kind == asyncOperationSSHStart && state.target.endpointOrScope != "" {
			target = state.target.endpointOrScope
		}
	}
	return operationLoadingStatus(state.kind, target)
}

type operationRelease struct {
	commitResult *operationCommitResult
	conflict     *conflictState
	quitIntent   bool
}

// startOperation allocates one monotonically increasing ID and refuses to
// replace any current owner, including one awaiting release after cleanup.
func startOperation(lastID uint64, active *operationState, kind asyncOperationKind, target *capturedTarget, owner operationOwnerSurface, retries ...operationRetryIntent) (*operationState, uint64, bool) {
	if active != nil || lastID == ^uint64(0) || !kind.valid() || !owner.valid() || len(retries) > 1 || len(retries) == 1 && !retries[0].valid() {
		return active, lastID, false
	}
	id := lastID + 1
	state := &operationState{id: id, kind: kind, phase: asyncPhaseRunning, owner: owner}
	if target != nil {
		copy := target.clone()
		state.target = &copy
	}
	if len(retries) == 1 {
		copy := retries[0].clone()
		state.retry = &copy
	}
	return state, id, true
}

func (state operationState) owns(id uint64) bool {
	return state.id != 0 && state.id == id
}

func (state operationState) matchesResult(id uint64, kind asyncOperationKind) bool {
	return state.owns(id) && state.kind == kind && (state.phase == asyncPhaseRunning || state.phase == asyncPhaseCancelRequested)
}

func (state operationState) recordCommit(id uint64, kind asyncOperationKind, value any) (operationState, bool) {
	if !state.matchesResult(id, kind) || state.commitResult != nil {
		return state, false
	}
	state.commitResult = &operationCommitResult{value: value}
	return state, true
}

func (state operationState) requestCancellation(id uint64, quit bool) (operationState, bool) {
	if !state.owns(id) {
		return state, false
	}
	if state.phase == asyncPhaseCancelRequested && quit && !state.quitIntent {
		state.quitIntent = true
		return state, true
	}
	if state.phase != asyncPhaseRunning {
		return state, false
	}
	state.phase = asyncPhaseCancelRequested
	state.stopReason = operationStopCanceled
	state.quitIntent = quit
	return state, true
}

func (state operationState) interrupt(id uint64, kind asyncOperationKind, reason operationStopReason) (operationState, bool) {
	if !state.matchesResult(id, kind) || state.phase != asyncPhaseRunning || (reason != operationStopTimeout && reason != operationStopNetwork) {
		return state, false
	}
	state.phase = asyncPhaseCancelRequested
	state.stopReason = reason
	return state, true
}

func (state operationState) beginCleanup(id uint64, kind asyncOperationKind) (operationState, bool) {
	if !state.matchesResult(id, kind) {
		return state, false
	}
	state.phase = asyncPhaseCleaningUp
	return state, true
}

func (state operationState) queueConflict(id uint64, kind asyncOperationKind, conflict conflictState) (operationState, bool) {
	if !state.matchesResult(id, kind) || !conflict.blocked || state.pendingConflict != nil {
		return state, false
	}
	copy := conflict
	copy.target = conflict.target.clone()
	state.pendingConflict = &copy
	state.phase = asyncPhaseCleaningUp
	return state, true
}

func (state operationState) finishCleanup(id uint64) (operationState, bool) {
	if !state.owns(id) || state.phase != asyncPhaseCleaningUp {
		return state, false
	}
	state.phase = asyncPhaseCompleted
	return state, true
}

// releaseOperation is the only transition that removes operation ownership.
// Its queued conflict is returned only after cleanup has completed.
func releaseOperation(active *operationState) (*operationState, operationRelease, bool) {
	if active == nil || active.phase != asyncPhaseCompleted {
		return active, operationRelease{}, false
	}
	release := operationRelease{quitIntent: active.quitIntent}
	if active.commitResult != nil {
		copy := *active.commitResult
		release.commitResult = &copy
	}
	if active.pendingConflict != nil {
		copy := *active.pendingConflict
		copy.target = active.pendingConflict.target.clone()
		release.conflict = &copy
	}
	return nil, release, true
}
