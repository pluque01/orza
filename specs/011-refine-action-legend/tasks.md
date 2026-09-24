# Tasks: Refine Action Legend

**Input**: Design documents from `specs/011-refine-action-legend/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/tui-actions.md`, `quickstart.md`

**Tests**: Required by the project constitution for input handling and user-visible TUI behavior. Write each listed test before its corresponding implementation and verify it fails for the missing behavior.

**Organization**: Tasks are grouped by user story so each increment remains independently testable.

## Phase 1: Setup

**Purpose**: No project initialization or dependency changes are required. The existing Go module and package-local TUI test suite provide the required infrastructure.

---

## Phase 2: Foundational

**Purpose**: No shared blocking change is needed before story work. Existing contextual action descriptors remain the source of truth for applicability and dispatch; story tasks must not alter that behavior.

**Checkpoint**: User stories may start immediately. Complete them in priority order to deliver the compact legend before the Help redesign.

---

## Phase 3: User Story 1 - Consultar atajos principales (Priority: P1)

**Goal**: Replace the browser Actions panel with a compact, borderless, accessible legend containing only Move Up, Move Down, optional Connect, New connection, New folder, and Quit in the specified order.

**Independent Test**: Render browser contexts for an empty list, folder, and connectable connection. Verify the exact legend set and order, absence of the Actions border, clear no-color pair boundaries, conditional Connect, and continued dispatch of a hidden secondary action.

### Tests for User Story 1

- [X] T001 [P] [US1] Add table-driven legend selection and ordering tests for empty, folder, connectable-connection, Tree-focus, and Details-focus contexts in `internal/tui/actions_test.go`
- [X] T002 [P] [US1] Add browser geometry and responsive-layout regression tests that prove the five-row Actions region is removed while 40x12 layout remains usable in `internal/tui/layout_test.go` and `internal/tui/responsive_layout_test.go`
- [X] T003 [P] [US1] Add rendered-frame and key-dispatch conformance tests proving the Actions border is absent and applicable secondary shortcuts remain executable when omitted from the legend in `internal/tui/ui_conformance_test.go` and `internal/tui/action_dispatch_test.go`

### Implementation for User Story 1

- [X] T004 [US1] Add a presentation-only browser legend selector in `internal/tui/actions.go` that derives Move Up, Move Down, optional Connect, New connection, New folder, and Quit from applicable descriptors in navigation, connection, creation, and exit order without filtering Help or dispatch inventories
- [X] T005 [US1] Add semantic key/action pair rendering in `internal/tui/styles.go` and `internal/tui/actions.go` using an emphasized key, the existing muted descriptive-label treatment for the action, and a visible multi-cell or textual separator that remains unambiguous after ANSI styling is removed
- [X] T006 [US1] Replace the browser-only Actions region with the compact borderless legend in `internal/tui/model.go`, preserving the existing controls for forms, operations, and conflict states
- [X] T007 [US1] Remove the browser Actions-panel height reservation and allocate recovered space to browser content without changing the documented 40x12 minimum behavior in `internal/tui/layout.go`
- [X] T008 [US1] Add ANSI-stripped color equivalence, display-width, and pair-separation coverage for the legend renderer in `internal/tui/styles_test.go`

**Checkpoint**: The compact browser legend is independently functional, contains no secondary actions, adapts Connect to the selection, and leaves existing applicable shortcuts operational.

---

## Phase 4: User Story 2 - Consultar todos los atajos (Priority: P2)

**Goal**: Make Help a complete, ordered, scannable reference of contextual shortcuts using grouped aligned key/action rows with the same visual treatment as the legend.

**Independent Test**: Open Help for each browser context and verify every applicable descriptor occurs exactly once in navigation, connection, management, and application order; confirm aligned rows at normal widths, readable stacked rows when narrow, visible close control, and identical plain text with color disabled.

### Tests for User Story 2

- [X] T009 [P] [US2] Add table-driven Help grouping, stable ordering, complete descriptor coverage, and aligned-or-stacked key/action row tests in `internal/tui/actions_test.go`
- [X] T010 [P] [US2] Add Help modal narrow-width, scrolling, and fixed-close-control regression cases for grouped action rows in `internal/tui/modal_layout_test.go`
- [X] T011 [P] [US2] Add color and ANSI-stripped equivalence tests proving Help key/action rows use the same semantic styles as the browser legend in `internal/tui/styles_test.go`

### Implementation for User Story 2

- [X] T012 [US2] Categorize applicable action descriptors and build Help rows in navigation, connection, management, and application order in `internal/tui/actions.go`, ensuring every applicable descriptor appears exactly once
- [X] T013 [US2] Render Help action rows through the existing structured key/action field presentation with aligned normal-width columns and the established narrow stacked fallback in `internal/tui/actions.go` and `internal/tui/styles.go`
- [X] T014 [US2] Integrate grouped Help rows into the existing Help payload and modal without displacing its close control or changing contextual availability in `internal/tui/model.go` and `internal/tui/modal.go`
- [X] T015 [US2] Update visible-frame conformance expectations so secondary actions are required in Help rather than the compact legend in `internal/tui/ui_conformance_test.go`

**Checkpoint**: Help is independently complete and readable, including secondary operations absent from the legend, while preserving modal navigation and action dispatch behavior.

---

## Phase 5: Polish & Cross-Cutting Validation

**Purpose**: Confirm the two increments work together and satisfy the terminal usability and maintenance gates.

- [X] T016 Run formatting for changed TUI sources under `internal/tui/` and resolve resulting formatting changes
- [X] T017 Run the focused and full automated validation commands documented in `specs/011-refine-action-legend/quickstart.md`
- [X] T018 Perform the empty-list, folder, connectable-connection, narrow-terminal, Help, secondary-shortcut, and no-color manual checks documented in `specs/011-refine-action-legend/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1: No work required; the repository already provides the necessary test and TUI infrastructure.
- Phase 2: No new foundational artifact; it records the invariant that action applicability and dispatch stay separate from presentation.
- Phase 3 (US1): Can begin immediately and is the MVP.
- Phase 4 (US2): Can begin after T005 establishes the shared key/action presentation; complete after US1 to preserve a single visual treatment.
- Phase 5: Depends on completion of US1 and US2.

### User Story Dependencies

- **US1 (P1)**: No dependency on another story. Delivers the compact legend and preserved dispatch behavior.
- **US2 (P2)**: Reuses the semantic pair presentation from US1 but is independently testable through the Help modal.

### Task Dependencies

- T001-T003 should fail before T004-T007 change browser behavior.
- T004 and T005 precede T006; T007 follows the browser-shell change; T008 validates the completed legend renderer.
- T009-T011 should fail before T012-T014 change Help behavior.
- T012 and T013 precede T014; T015 updates end-to-end expectations after Help integration.
- T016-T018 run after both stories are complete.

## Parallel Opportunities

### User Story 1

```text
T001: Legend selection and order tests in internal/tui/actions_test.go
T002: Browser layout tests in internal/tui/layout_test.go and internal/tui/responsive_layout_test.go
T003: UI and dispatch conformance tests in internal/tui/ui_conformance_test.go and internal/tui/action_dispatch_test.go
```

These tests touch separate files and can be prepared in parallel. T004 and T005 can proceed once their test expectations are defined, but T006 must wait for both.

### User Story 2

```text
T009: Help grouping tests in internal/tui/actions_test.go
T010: Help modal geometry tests in internal/tui/modal_layout_test.go
T011: Help color-equivalence tests in internal/tui/styles_test.go
```

These tests touch separate files and can be prepared in parallel. T012 and T013 define the data and presentation needed by T014.

## Implementation Strategy

### MVP First

1. Complete T001-T008 for US1.
2. Run the US1 independent test criteria.
3. Demonstrate a borderless compact legend with conditional Connect and preserved secondary shortcut dispatch.

### Incremental Delivery

1. Deliver US1 as the compact, accessible main-screen legend.
2. Deliver US2 as the complete grouped Help reference.
3. Complete T016-T018 to validate the combined behavior across widths and color modes.
