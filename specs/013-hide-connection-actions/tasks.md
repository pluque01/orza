# Tasks: Hide Connection Actions

**Input**: Design documents from `specs/013-hide-connection-actions/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/tui-presentation.md`, and `quickstart.md`

**Tests**: Required by the feature specification and project constitution because this changes connection startup, input presentation, recovery, and minimum-size behavior.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated independently.

## Phase 1: Setup

**Purpose**: Establish the exact presentation contract before changing rendering.

- [X] T001 [P] Review and align existing Actions-panel terminology fixtures with `specs/013-hide-connection-actions/contracts/tui-presentation.md` in `internal/tui/language_test.go`
- [X] T002 [P] Capture the 80x24 and 40x12 fixed lower-control-region expectations in `internal/tui/layout_test.go`

---

## Phase 2: Foundational Layout

**Purpose**: Preserve the lower control region without redistributing it, regardless of active TUI state.

- [X] T003 Update shared layout selection to retain the fixed five-row lower control region without an Actions panel in `internal/tui/layout.go`
- [X] T004 Add layout and resize regression coverage for the unredistributed lower control region in `internal/tui/layout_test.go` and `internal/tui/responsive_layout_test.go`

**Checkpoint**: The lower control region is stable at normal and minimum supported sizes before action rendering changes.

---

## Phase 3: User Story 1 - Use the Interface Without an Actions Panel (Priority: P1) MVP

**Goal**: Remove the titled and bordered Actions panel from every TUI state while retaining the applicable borderless action legend.

**Independent Test**: Exercise catalog, form, modal, connection-startup, recovery, and error views; verify that no view renders an Actions title or panel border, while each applicable legend remains visible.

### Tests for User Story 1

- [X] T005 [P] [US1] Add catalog, form, modal, recovery, and error assertions for absent Actions panel chrome and visible legends in `internal/tui/ui_conformance_test.go`
- [X] T006 [P] [US1] Add end-to-end legend-preservation and Actions-panel-absence coverage in `tests/integration/tui_actions_test.go`

### Implementation for User Story 1

- [X] T007 [US1] Render all lower-region action inventories as borderless legends instead of a titled Actions panel in `internal/tui/model.go`
- [X] T008 [US1] Reuse the applicable descriptor inventory for normal, form, operation, conflict, recovery, and error legends in `internal/tui/actions.go`
- [X] T009 [US1] Remove obsolete user-visible Actions panel terminology and update controlled-copy assertions in `internal/tui/language_test.go` and `internal/tui/testdata/controlled_english.txt`

**Checkpoint**: No user-visible state displays an Actions panel, and the action legend remains available for every applicable state.

---

## Phase 4: User Story 2 - Retain Safe Connection Control (Priority: P2)

**Goal**: Preserve visible startup status and keyboard cancellation or quit behavior after changing the lower-region presentation.

**Independent Test**: Start an SSH attempt, including host trust and failure paths, then verify visible status and cancel or quit controls while confirming the panel title is absent.

### Tests for User Story 2

- [X] T010 [P] [US2] Add SSH-startup, cancellation, quit, host-trust, and recovery presentation assertions in `internal/tui/operation_integration_test.go`
- [X] T011 [P] [US2] Extend minimum-size and transition coverage for startup and recovery controls in `internal/tui/undersized_test.go` and `internal/tui/resize_conformance_test.go`

### Implementation for User Story 2

- [X] T012 [US2] Preserve operation status and applicable cancel, quit, trust, and recovery descriptors in the borderless lower-region rendering path in `internal/tui/model.go`
- [X] T013 [US2] Verify action applicability and keyboard dispatch remain aligned with the visible legend during connection startup in `internal/tui/actions.go` and `internal/tui/session.go`

**Checkpoint**: Startup, trust, cancellation, quit, and recovery remain keyboard-operable and visibly explained without an Actions panel.

---

## Phase 5: Polish and Cross-Cutting Validation

**Purpose**: Validate the documented presentation contract across all supported TUI states.

- [X] T014 [P] Reconcile viewport, accessibility, and modal presentation assertions with the panel-free lower region in `internal/tui/accessibility_test.go`, `internal/tui/sc007_viewport_acceptance_test.go`, and `internal/tui/modal_layout_test.go`
- [X] T015 Run the end-to-end validation scenarios from `specs/013-hide-connection-actions/quickstart.md` and execute `nix develop --command go test ./...`

---

## Dependencies and Execution Order

### Phase Dependencies

- Phase 1 has no dependencies.
- Phase 2 depends on T001 and T002 and blocks both user stories because it establishes the fixed lower-region geometry.
- User Story 1 depends on Phase 2.
- User Story 2 depends on the borderless legend rendering from User Story 1.
- Phase 5 depends on both user stories.

### User Story Dependencies

- **US1 (P1)**: Delivers the MVP independently after the foundational layout phase.
- **US2 (P2)**: Builds on US1's panel-free legend rendering to verify connection-specific safety and recovery behavior.

### Parallel Opportunities

- T001 and T002 can run in parallel.
- T005 and T006 can run in parallel after Phase 2.
- T010 and T011 can run in parallel after US1's rendering path is available.
- T014 can run in parallel with final validation preparation after US2 is complete.

## Parallel Example: User Story 1

```text
Task: "Add catalog, form, modal, recovery, and error assertions in internal/tui/ui_conformance_test.go"
Task: "Add end-to-end legend-preservation coverage in tests/integration/tui_actions_test.go"
```

## Implementation Strategy

### MVP First

1. Complete the fixed lower-control-region foundation.
2. Complete US1 to remove Actions panel chrome and preserve legends in all states.
3. Validate the US1 independent test criteria before proceeding.

### Incremental Delivery

1. Deliver US1 as the panel-free presentation change.
2. Deliver US2 as the connection-startup and recovery safety regression layer.
3. Complete cross-cutting viewport, accessibility, and full-suite validation.

## Task Summary

- Total tasks: 15
- US1 tasks: 5
- US2 tasks: 4
- Setup/foundational tasks: 4
- Polish tasks: 2

All tasks use the required checkbox, sequential ID, optional parallel marker, user-story label for story tasks, and exact file-path format.
