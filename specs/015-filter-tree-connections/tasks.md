---

description: "Implementation tasks for connection-tree filtering"
---

# Tasks: Filter Tree Connections

**Input**: Design documents from `/specs/015-filter-tree-connections/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [tree-search.md](./contracts/tree-search.md), [quickstart.md](./quickstart.md)

**Tests**: Required by the project constitution for changed input handling and failure paths. Write each listed test before its associated implementation task.

**Organization**: Tasks are grouped by user story so each delivered increment remains independently testable.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the key bindings and shared transient state needed by all search flows.

- [X] T001 [P] Add `/`, Ctrl+P, and Ctrl+N bindings with help text in `internal/tui/keys.go`
- [X] T002 Define transient search-session state and capture/restore helpers for selection, expansion, viewport, and focus in `internal/tui/model.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Provide the filtered tree projection used by every story without changing the catalog snapshot.

**⚠️ CRITICAL**: Complete this phase before user-story implementation.

- [X] T003 Add failing pure projection tests for connection-only matching, required ancestor folders, natural empty-query rows, and deterministic tree ordering in `internal/tui/browser_test.go`
- [X] T004 Implement filtered-row projection and ancestor expansion derivation from the catalog snapshot in `internal/tui/browser.go`

**Checkpoint**: Search can derive rows from the loaded catalog without persistence, services, or SSH activity.

---

## Phase 3: User Story 1 - Find a Connection in the Tree (Priority: P1) 🎯 MVP

**Goal**: Open a visible query field from the focused Tree panel and show only regex-matching connections with their parent-folder paths and highlighted matched name portions.

**Independent Test**: In a nested catalog, press `/`, enter a partial name and a valid regular expression, and verify only matching connections with their ancestors appear; verify all matches are distinguished and folders never produce results.

### Tests for User Story 1

- [X] T005 [P] [US1] Add model tests for opening search only from Tree focus, query focus, empty-query restoration, and invalid-regex feedback without selection changes in `internal/tui/model_test.go`
- [X] T006 [P] [US1] Add browser rendering tests for all regex match spans, no-color active-result distinction, and bounded long-name output in `internal/tui/browser_test.go`
- [X] T007 [P] [US1] Add end-to-end nested-tree filtering scenarios, including temporarily expanded collapsed ancestors, in `tests/integration/tui_tree_test.go`

### Implementation for User Story 1

- [X] T008 [US1] Route `/` and query-field input ahead of ordinary browser actions, compile case-sensitive regular expressions, and retain the natural tree with actionable feedback on compilation failure in `internal/tui/model.go`
- [X] T009 [US1] Render the visible query field, no-results state, and expression error within the Tree panel layout in `internal/tui/model.go`
- [X] T010 [US1] Render every match span in connection names before selected-row styling and viewport truncation in `internal/tui/browser.go`
- [X] T011 [US1] Add the match-emphasis style while preserving the existing non-color selected-row marker in `internal/tui/styles.go`

**Checkpoint**: User Story 1 is complete when valid and invalid queries meet the interaction contract without mutating the catalog or starting a connection.

---

## Phase 4: User Story 2 - Navigate Filtered Matches (Priority: P2)

**Goal**: Move the active result among matching connections with arrows and Ctrl+P/Ctrl+N while folders remain structural context.

**Independent Test**: With three matching connections in nested folders, use each supported key to move to adjacent matches and confirm navigation clamps at the first and last result.

### Tests for User Story 2

- [X] T012 [P] [US2] Add model tests for Up/Down and Ctrl+P/Ctrl+N moving only among matching connections and clamping at both ends in `internal/tui/model_test.go`
- [X] T013 [P] [US2] Add integration coverage for filtered navigation across parent-folder boundaries in `tests/integration/tui_tree_test.go`

### Implementation for User Story 2

- [X] T014 [US2] Implement filtered-result navigation that ignores structural folders and preserves the active match at either boundary in `internal/tui/model.go`

**Checkpoint**: User Stories 1 and 2 work together while the query field retains focus and no folder can become an active search result.

---

## Phase 5: User Story 3 - Exit Search Safely (Priority: P3)

**Goal**: Cancel search or accept the active connection while restoring the natural tree and never initiating a connection operation.

**Independent Test**: Capture a collapsed nested-tree state, cancel a search, then repeat and press Enter on a result; verify exact restoration on Escape and accepted-connection focus without a connection confirmation or session on Enter.

### Tests for User Story 3

- [X] T015 [P] [US3] Add model tests for Escape restoring captured selection, expansion, viewport, and focus, plus Enter selecting a match without an operation in `internal/tui/model_test.go`
- [X] T016 [P] [US3] Add integration tests proving collapsed-folder state restoration and no connection confirmation after accepted search in `tests/integration/tui_tree_test.go`

### Implementation for User Story 3

- [X] T017 [US3] Implement Escape cancellation and Enter acceptance, expanding the accepted connection path without dispatching browser actions in `internal/tui/model.go`

**Checkpoint**: All user stories are independently functional and search has no connection-side effects.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Verify terminal resilience, performance, and the documented end-to-end workflow.

- [X] T018 [P] Add narrow-layout, no-color, and long-match rendering conformance cases in `internal/tui/resize_conformance_test.go`
- [X] T019 [P] Add a 100-connection nested-tree projection performance acceptance case with the 1-second target in `internal/tui/performance_acceptance_test.go`
- [ ] T020 Run the automated and manual validation scenarios in `specs/015-filter-tree-connections/quickstart.md`
- [X] T021 Run formatting, static analysis, full tests, and build for the changed TUI files in `internal/tui/` and `tests/integration/tui_tree_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 has no dependencies.
- Phase 2 depends on T001-T002 and blocks every user story.
- User Story 1 depends on Phase 2 and is the MVP.
- User Story 2 depends on User Story 1's active filtered projection and key-routing flow.
- User Story 3 depends on User Story 1's captured search context and active result.
- Phase 6 depends on all requested user stories.

### User Story Dependencies

```text
Setup (T001-T002)
  -> Foundational projection (T003-T004)
      -> US1: find and render matches (T005-T011)
          -> US2: navigate matches (T012-T014)
          -> US3: cancel and accept safely (T015-T017)
              -> Polish (T018-T021)
```

## Parallel Opportunities

- T001 can proceed in parallel with T002 because they modify different files.
- Within US1, T005, T006, and T007 can be written in parallel before implementation; T011 can proceed in parallel with T008-T010 after the required style interface is agreed.
- Within US2, T012 and T013 can be written in parallel.
- Within US3, T015 and T016 can be written in parallel.
- T018 and T019 can be implemented in parallel because they modify different test files.

### Parallel Example: User Story 1

```text
Task: "T005 Add model tests in internal/tui/model_test.go"
Task: "T006 Add rendering tests in internal/tui/browser_test.go"
Task: "T007 Add integration scenarios in tests/integration/tui_tree_test.go"
```

## Implementation Strategy

### MVP First

1. Complete T001-T004 to establish bindings, captured context, and filtered projection.
2. Complete T005-T011 for User Story 1.
3. Run the User Story 1 tests and confirm valid/invalid expression behavior before adding navigation or acceptance.

### Incremental Delivery

1. Deliver US1 for search, hierarchy preservation, and highlighting.
2. Deliver US2 for keyboard movement among filtered connections.
3. Deliver US3 for deterministic cancellation and selection-only acceptance.
4. Complete cross-cutting terminal, performance, and full-suite validation.

## Notes

- Every task uses the required checkbox, sequential ID, optional parallel marker, story label where applicable, and exact repository path.
- The tasks intentionally reuse the existing snapshot, text field, viewport, and styles; no new dependency, service, storage, or persistence task is required.
