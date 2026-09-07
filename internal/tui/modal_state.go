package tui

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/pluque01/orza/internal/app"
)

type focusOwner uint8

const (
	focusOwnerNone focusOwner = iota
	focusOwnerTree
	focusOwnerDetail
	focusOwnerConnectionForm
	focusOwnerModal
)

func (owner focusOwner) validModalOpener() bool {
	return owner == focusOwnerTree || owner == focusOwnerDetail || owner == focusOwnerConnectionForm
}

type modalKind string

const (
	modalKindClosed              modalKind = ""
	modalKindFolderCreate        modalKind = "folder_create"
	modalKindFolderEdit          modalKind = "folder_edit"
	modalKindMovePicker          modalKind = "move_picker"
	modalKindDeleteConnection    modalKind = "delete_connection"
	modalKindDeleteFolder        modalKind = "delete_folder"
	modalKindConnectConfirmation modalKind = "connect_confirmation"
	modalKindUnsavedChanges      modalKind = "unsaved_changes"
	modalKindHelp                modalKind = "help"
	modalKindOperationError      modalKind = "operation_error"
	modalKindSSHFailure          modalKind = "ssh_failure"
)

func (kind modalKind) valid() bool {
	switch kind {
	case modalKindFolderCreate,
		modalKindFolderEdit,
		modalKindMovePicker,
		modalKindDeleteConnection,
		modalKindDeleteFolder,
		modalKindConnectConfirmation,
		modalKindUnsavedChanges,
		modalKindHelp,
		modalKindOperationError,
		modalKindSSHFailure:
		return true
	default:
		return false
	}
}

type capturedTarget struct {
	id              app.NodeID
	revision        app.Revision
	kind            app.NodeKind
	path            string
	endpointOrScope string
	ancestorIDs     []app.NodeID
}

func (target capturedTarget) clone() capturedTarget {
	target.ancestorIDs = append([]app.NodeID(nil), target.ancestorIDs...)
	return target
}

type conflictType uint8

const (
	conflictTypeMissing conflictType = iota + 1
	conflictTypeRevisionChanged
)

type conflictOwner uint8

const (
	conflictOwnerForm conflictOwner = iota + 1
	conflictOwnerModal
	conflictOwnerOperation
)

type conflictState struct {
	kind          conflictType
	target        capturedTarget
	owner         conflictOwner
	detailVisible bool
	blocked       bool
}

func (conflict conflictState) clone() conflictState {
	conflict.target = conflict.target.clone()
	return conflict
}

// modalRegistry binds each closed modal kind to one exact payload type. It is
// intentionally empty until concrete workflows register their payloads.
type modalRegistry struct {
	payloadTypes map[modalKind]reflect.Type
}

func newModalRegistry() modalRegistry {
	registry := modalRegistry{}
	registry, _ = registerModalPayload[folderCreatePayload](registry, modalKindFolderCreate)
	registry, _ = registerModalPayload[folderEditPayload](registry, modalKindFolderEdit)
	registry, _ = registerModalPayload[movePickerPayload](registry, modalKindMovePicker)
	registry, _ = registerModalPayload[deleteConnectionPayload](registry, modalKindDeleteConnection)
	registry, _ = registerModalPayload[deleteFolderPayload](registry, modalKindDeleteFolder)
	registry, _ = registerModalPayload[connectConfirmationPayload](registry, modalKindConnectConfirmation)
	registry, _ = registerModalPayload[unsavedChangesPayload](registry, modalKindUnsavedChanges)
	registry, _ = registerModalPayload[helpPayload](registry, modalKindHelp)
	registry, _ = registerModalPayload[operationErrorPayload](registry, modalKindOperationError)
	registry, _ = registerModalPayload[sshFailurePayload](registry, modalKindSSHFailure)
	return registry
}

func registerModalPayload[Payload any](registry modalRegistry, kind modalKind) (modalRegistry, error) {
	if !kind.valid() {
		return registry, fmt.Errorf("%w: %q", errModalUnknownKind, kind)
	}
	if _, exists := registry.payloadTypes[kind]; exists {
		return registry, fmt.Errorf("%w: %s", errModalKindRegistered, kind)
	}

	registered := modalRegistry{payloadTypes: make(map[modalKind]reflect.Type, len(registry.payloadTypes)+1)}
	for registeredKind, payloadType := range registry.payloadTypes {
		registered.payloadTypes[registeredKind] = payloadType
	}
	registered.payloadTypes[kind] = reflect.TypeFor[Payload]()
	return registered, nil
}

type modalOpenRequest struct {
	kind             modalKind
	openedFrom       focusOwner
	target           *capturedTarget
	payload          any
	viewport         viewportState
	recoverableError string
	conflict         *conflictState
}

// modalState is the sole generic modal slot. All transition methods use value
// receivers so rejected transitions leave the caller's slot unchanged.
type modalState struct {
	kind             modalKind
	openedFrom       focusOwner
	target           *capturedTarget
	payload          any
	viewport         viewportState
	operationStatus  string
	recoverableError string
	conflict         *conflictState
	helpVisible      bool
	helpOpenedFrom   any
}

var (
	errModalClosed                = errors.New("modal is closed")
	errModalNestedOpen            = errors.New("nested modal is not allowed")
	errModalReplacement           = errors.New("replacing an open modal is not allowed")
	errModalUnknownKind           = errors.New("unknown modal kind")
	errModalKindUnregistered      = errors.New("modal kind has no registered payload")
	errModalKindRegistered        = errors.New("modal kind payload is already registered")
	errModalPayloadType           = errors.New("modal payload has the wrong type")
	errModalInvalidOpener         = errors.New("modal opener must be an application surface")
	errModalInlineHelpUnavailable = errors.New("inline help requires an open non-help modal")
	errModalInlineHelpActive      = errors.New("inline help must restore its payload control before modal close")
)

func (state modalState) open(registry modalRegistry, request modalOpenRequest) (modalState, error) {
	if state.kind != modalKindClosed {
		if state.kind == request.kind {
			return state, errModalNestedOpen
		}
		return state, errModalReplacement
	}
	if !request.kind.valid() {
		return state, fmt.Errorf("%w: %q", errModalUnknownKind, request.kind)
	}
	if !request.openedFrom.validModalOpener() {
		if request.openedFrom == focusOwnerModal {
			return state, errModalNestedOpen
		}
		return state, errModalInvalidOpener
	}
	payloadType, registered := registry.payloadTypes[request.kind]
	if !registered {
		return state, fmt.Errorf("%w: %s", errModalKindUnregistered, request.kind)
	}
	if request.payload == nil || reflect.TypeOf(request.payload) != payloadType || isNilModalPayload(request.payload) {
		return state, fmt.Errorf("%w for %s: want %v, got %T", errModalPayloadType, request.kind, payloadType, request.payload)
	}
	if validator, ok := request.payload.(interface{ validModalPayload() bool }); ok && !validator.validModalPayload() {
		return state, fmt.Errorf("%w for %s: invalid payload", errModalPayloadType, request.kind)
	}

	opened := modalState{
		kind:             request.kind,
		openedFrom:       request.openedFrom,
		payload:          request.payload,
		viewport:         request.viewport,
		recoverableError: request.recoverableError,
	}
	if request.target != nil {
		target := request.target.clone()
		opened.target = &target
	}
	if request.conflict != nil {
		conflict := request.conflict.clone()
		opened.conflict = &conflict
	}
	return opened, nil
}

func isNilModalPayload(payload any) bool {
	value := reflect.ValueOf(payload)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (state modalState) close() (modalState, focusOwner, error) {
	if state.kind == modalKindClosed {
		return state, focusOwnerNone, errModalClosed
	}
	if state.helpVisible {
		return state, focusOwnerNone, errModalInlineHelpActive
	}
	return modalState{}, state.openedFrom, nil
}

// toggleHelp returns the payload control to restore only when inline Help is
// closed. Both '?' and Esc use this transition while Help owns modal input.
func (state modalState) toggleHelp(currentControl any) (modalState, any, error) {
	if state.kind == modalKindClosed || state.kind == modalKindHelp {
		return state, nil, errModalInlineHelpUnavailable
	}
	if state.helpVisible {
		restored := state.helpOpenedFrom
		state.helpVisible = false
		state.helpOpenedFrom = nil
		return state, restored, nil
	}
	state.helpVisible = true
	state.helpOpenedFrom = currentControl
	return state, nil, nil
}

func (state modalState) isOpen() bool {
	return state.kind != modalKindClosed
}

func (state modalState) blocksBackgroundInput() bool {
	return state.isOpen()
}
