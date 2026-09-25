# Tasks: Forget Host Key

**Input**: Design documents from `specs/012-forget-host-key/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/forget-host-key.md`, and `quickstart.md`

**Tests**: Required by the project constitution for host-trust, state-transition, CLI, and TUI behavior.

**Organization**: Tasks are grouped by user story after the shared trust-deletion foundation. The CLI contract is built in the foundation because it is an approved automation interface used by both user-facing flows.

## Phase 1: Setup

**Purpose**: Confirm the existing toolchain and preserve a clean baseline before changing trust behavior.

- [X] T001 Run the existing full test suite and record the baseline result for the Go module in `go.mod`

---

## Phase 2: Foundational Trust Deletion and CLI Contract

**Purpose**: Add the endpoint-scoped, revision-protected app-owned trust operation and its non-interactive automation command. This phase blocks both interactive user stories.

- [X] T002 [P] Add app-level forget-host-key scope, request, result, endpoint/revision validation, and the trusted-host deletion port in `internal/app/types.go` and `internal/app/ports.go`
- [X] T003 [P] Add failing repository cases for successful deletion, canonical-host matching, absent rows, stale revisions, invalid endpoints/revisions, rollback, and catalog-revision changes in `internal/catalog/trusted_host_repository_test.go`
- [X] T004 Implement conditional `trusted_hosts` deletion in the existing immediate transaction flow in `internal/catalog/trusted_host_repository.go`, requiring a positive revision and incrementing the catalog revision exactly once only after success
- [X] T005 [P] Add failing adapter cases for delete request conversion, cloned metadata, and catalog error propagation in `internal/hostkey/policy_test.go`
- [X] T006 Extend the catalog trusted-host adapter with conditional deletion in `internal/hostkey/policy.go`
- [X] T007 [P] Add failing use-case cases for deriving the endpoint from a resolved connection, absent app-owned trust as `forgotten: false`, stale trust conflict, and standard `known_hosts` isolation in `internal/app/host_trust_test.go`
- [X] T008 Implement the dedicated host-trust management service and error classification in `internal/app/host_trust.go`
- [X] T009 Wire the host-trust management service into process dependencies in `internal/app/bootstrap.go` and `cmd/orza/main.go`
- [X] T010 [P] Add CLI command tests for selector validation, no prompt, human and JSON success/no-op output, `--if-revision` conflicts, and error codes in `internal/cli/connection_commands_test.go`
- [X] T011 Implement and register `orza connection forget-host-key PATH_OR_ID [--if-revision REVISION]` with no confirmation input in `internal/cli/connection_mutate.go` and `internal/cli/root.go`
- [X] T012 Update CLI fixtures and interface fakes required by the trusted-host deletion port in `internal/app/connect_test.go`, `internal/cli/connect_command_test.go`, and `tests/integration/security_test.go`

**Checkpoint**: The CLI can idempotently forget only app-owned trust for a connection endpoint; a stale revision or failure does not delete a newer key.

---

## Phase 3: User Story 1 - Forget a Host Key from a Connection (Priority: P1) MVP

**Goal**: Let an operator select a connection in the TUI, confirm forgetting its app-owned host key, and receive a stable success or no-change result.

**Independent Test**: With an app-owned trusted key for a selected connection, trigger the TUI action, press `y`, and verify the endpoint's app-owned trust is removed; repeat after pressing Enter or Escape and verify the record remains.

- [X] T013 [P] [US1] Add TUI service fake and interface tests for loading a forget scope and submitting a revision-pinned forget request in `internal/tui/services.go` and `internal/tui/services_test.go`
- [X] T014 [P] [US1] Add action inventory and dispatch tests proving the action appears only for a selected connection and uses an unused key in `internal/tui/actions_test.go` and `internal/tui/action_dispatch_test.go`
- [X] T015 [P] [US1] Add modal and model tests for confirmation, cancellation, successful deletion, no app-owned key, failure, and stale revision in `internal/tui/confirmation_test.go`, `internal/tui/modal_conformance_test.go`, and `internal/tui/model_test.go`
- [X] T016 [US1] Add the connection-only forget-host-key action identifier, canonical label, descriptor, and key dispatch in `internal/tui/actions.go` and `internal/tui/language_test.go`
- [X] T017 [US1] Add the typed forget-host-key confirmation payload, modal registration, `y` confirmation, and Enter/Escape cancellation handling in `internal/tui/modal_state.go` and `internal/tui/modal.go`
- [X] T018 [US1] Implement the TUI scope-load, revision-pinned forget operation, reload, success/no-change feedback, and recoverable conflict/error states in `internal/tui/model.go` and `internal/tui/services.go`
- [X] T019 [US1] Add end-to-end TUI coverage for forgetting shared endpoint trust without deleting any connection in `tests/integration/tui_connection_test.go`

**Checkpoint**: User Story 1 is independently usable: an operator can forget a selected connection's app-owned host key, cancel safely, and see a stable result.

---

## Phase 4: User Story 2 - Understand the Security Consequence (Priority: P2)

**Goal**: Make the TUI confirmation unambiguous about its endpoint, ownership boundary, future trust prompt, and keyboard controls.

**Independent Test**: Open the forget confirmation for a selected connection and assert it shows `host:port`, says it removes only app-owned trust, warns that future connections may request trust again, and exposes `y`, Enter, and Escape controls at supported terminal sizes.

- [X] T020 [P] [US2] Add confirmation-content assertions for host, port, app-owned scope, future trust behavior, and keyboard controls in `internal/tui/confirmation_test.go` and `internal/tui/modal_conformance_test.go`
- [X] T021 [P] [US2] Add narrow-terminal and modal-layout assertions that preserve the forget confirmation target and cancellation controls in `internal/tui/modal_layout_test.go` and `internal/tui/undersized_test.go`
- [X] T022 [US2] Render the endpoint-specific ownership warning and consistent action controls in the forget-host-key modal in `internal/tui/modal.go`
- [X] T023 [US2] Add conflict and cancellation recovery coverage so a stale or interrupted confirmation returns to a stable connection-focused state in `internal/tui/conflict_conformance_test.go` and `internal/tui/resize_conformance_test.go`

**Checkpoint**: User Story 2 is independently verifiable: the security consequence is readable, target-specific, keyboard-operable, and safe under cancellation, resize, and conflict.

---

## Phase 5: Polish and Cross-Cutting Validation

**Purpose**: Validate public guidance, trust boundaries, and complete behavior across supported interfaces.

- [X] T024 [P] Document the non-interactive `connection forget-host-key` command and the app-owned versus `known_hosts` trust boundary in `README.md`
- [X] T025 [P] Add integration coverage that standard `known_hosts` entries remain unchanged while app-owned trust is forgotten in `tests/integration/security_test.go`
- [X] T026 Run the quickstart CLI and TUI validation scenarios from `specs/012-forget-host-key/quickstart.md`
- [X] T027 Run formatting, static analysis, and the complete test suite for the affected Go module in `go.mod`

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1**: Starts immediately.
- **Phase 2**: Starts after the baseline and blocks both user stories. T002 and T003 can begin together; T004 follows T003. T005 follows T002; T006 follows T005. T007 follows T002; T008 follows T007. T009 follows T008. T010 follows T009; T011 follows T010. T012 follows the extended port and its callers.
- **Phase 3 (US1)**: Starts after Phase 2. T013 through T015 can proceed in parallel. T016 through T018 follow their corresponding tests and T018 depends on T016 and T017. T019 follows T018.
- **Phase 4 (US2)**: Starts after the US1 modal exists. T020 and T021 can proceed in parallel; T022 follows them; T023 follows T022.
- **Phase 5**: Starts after both user stories are complete. T024 and T025 can proceed in parallel. T026 and T027 follow all implementation and test changes.

### User Story Dependencies

- **US1 (P1)**: Depends only on the foundational endpoint trust operation and delivers the MVP.
- **US2 (P2)**: Builds on the US1 confirmation modal to harden its target disclosure, responsive behavior, and recovery guarantees.

## Parallel Opportunities

### Foundational Phase

```text
T002: app DTOs and trusted-host port in internal/app/types.go and internal/app/ports.go
T003: repository tests in internal/catalog/trusted_host_repository_test.go
```

After T002:

```text
T005: adapter tests in internal/hostkey/policy_test.go
T007: host-trust use-case tests in internal/app/host_trust_test.go
```

### User Story 1

```text
T013: TUI service boundary tests in internal/tui/services.go and internal/tui/services_test.go
T014: action inventory and dispatch tests in internal/tui/actions_test.go and internal/tui/action_dispatch_test.go
T015: modal and model tests in internal/tui/confirmation_test.go, internal/tui/modal_conformance_test.go, and internal/tui/model_test.go
```

### User Story 2

```text
T020: confirmation-content tests in internal/tui/confirmation_test.go and internal/tui/modal_conformance_test.go
T021: responsive modal tests in internal/tui/modal_layout_test.go and internal/tui/undersized_test.go
```

## Implementation Strategy

### MVP First

1. Complete the endpoint-scoped repository, application, and CLI foundation in Phase 2.
2. Complete Phase 3 so a selected connection can forget app-owned host trust through a confirmed TUI action.
3. Validate the US1 independent test, including cancellation and stale revision behavior.

### Incremental Delivery

1. Phase 2 provides a safe, scriptable automation path.
2. Phase 3 provides the operator-facing forget action and establishes the functional MVP.
3. Phase 4 completes the target disclosure, responsive layout, and recovery quality required for the interactive security action.
4. Phase 5 documents and validates all trust boundaries without modifying standard SSH files.

## Format Validation

All 27 tasks use the required checkbox, sequential task ID, optional `[P]` marker only for parallel work, story labels for user-story tasks, and exact repository file paths.
