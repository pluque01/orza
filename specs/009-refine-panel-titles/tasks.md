---
description: "Task list for refining TUI panel titles and context labels"
---

# Tasks: Refinar títulos de panel

**Input**: Design documents from `/specs/009-refine-panel-titles/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/title-context-presentation.md](contracts/title-context-presentation.md), [quickstart.md](quickstart.md)

**Tests**: Tests are required by the feature's acceptance scenarios and the Orza Constitution. Write the listed regression tests before changing the corresponding renderer behavior.

**Organization**: Tasks are grouped by user story so each increment remains independently verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes a different file and has no incomplete-task dependency.
- **[Story]**: Identifies the user story served by the task.

## Phase 1: Setup

**Purpose**: Establish the current renderer baseline before changing shared styles.

- [X] T001 Run the existing TUI renderer baseline and record current structural-title and context-label expectations in `internal/tui/styles_test.go`, `internal/tui/detail_test.go`, and `internal/tui/panel_style_test.go`

---

## Phase 2: Foundational

**Purpose**: No shared infrastructure is required. Existing `styles` placement metadata and safe rendering boundaries are sufficient for all stories.

**Checkpoint**: Begin each user-story increment from the existing shared `internal/tui/styles.go` renderer without adding dependencies, state, or a theme abstraction.

---

## Phase 3: User Story 1 - Leer títulos de panel con claridad (Priority: P1) MVP

**Goal**: Present every structural region and modal title between brackets without a colored background, while preserving active and inactive focus markers.

**Independent Test**: Open Tree and Details with and without a selected target, then open each modal; all structural titles render as `[Label]` with no background styling and retain the existing `[*]` or `[ ]` focus marker.

### Tests for User Story 1

- [X] T002 [US1] Add failing placement-specific assertions for bracketed structural titles with no ANSI background in `internal/tui/styles_test.go`
- [X] T003 [P] [US1] Add failing Tree, Details, and Actions structural-title render assertions in `internal/tui/panel_style_test.go`
- [X] T004 [P] [US1] Add failing modal structural-title and focus-marker assertions in `internal/tui/modal_test.go`

### Implementation for User Story 1

- [X] T005 [US1] Render `badgePlacementTitle` as bracketed foreground-only text while preserving active/inactive markers in `internal/tui/styles.go`
- [X] T006 [US1] Verify region-title composition consumes the updated structural-title renderer without changing panel geometry in `internal/tui/model.go`
- [X] T007 [US1] Verify modal-title composition consumes the updated structural-title renderer without changing modal controls or focus ownership in `internal/tui/modal.go`

**Checkpoint**: Tree, Details, Actions, and modal titles satisfy the structural-title contract independently.

---

## Phase 4: User Story 2 - Distinguir el contexto destacado (Priority: P2)

**Goal**: Keep existing context/type labels prominent while rendering them without brackets and without creating empty or duplicate labels.

**Independent Test**: Select a root, folder, and connection, then open existing form and security-prompt surfaces; every existing context label appears once without brackets and no new label appears where none existed.

### Tests for User Story 2

- [X] T008 [P] [US2] Add failing Root, Folder, and Connection context-label assertions, including one-label and no-bracket cases, in `internal/tui/detail_test.go`
- [X] T009 [P] [US2] Add failing cross-surface context-label inventory assertions for panels, forms, and prompts in `internal/tui/panel_style_test.go`
- [X] T010 [US2] Add failing content-placement rendering assertions that distinguish contextual labels from structural titles in `internal/tui/styles_test.go`

### Implementation for User Story 2

- [X] T011 [US2] Render `badgePlacementContent` as unbracketed controlled text while retaining its accent treatment in `internal/tui/styles.go`
- [X] T012 [US2] Preserve the single existing target-type label for root, folder, and connection detail content in `internal/tui/detail.go`
- [X] T013 [P] [US2] Preserve the existing unbracketed context-label call site for connection forms in `internal/tui/connection_form.go`
- [X] T014 [P] [US2] Preserve the existing unbracketed context-label call sites for trust and secret prompts in `internal/tui/trust_prompt.go` and `internal/tui/secret_prompt.go`

**Checkpoint**: Context labels satisfy the contract independently, without changing navigation, prompts, form behavior, or data presentation.

---

## Phase 5: User Story 3 - Leer etiquetas coloreadas sin esfuerzo (Priority: P3)

**Goal**: Replace the low-contrast context-label palette and preserve equivalent textual meaning when color is unavailable.

**Independent Test**: Render `Connection` and `Folder` in color and no-color modes; color output uses a text/background pair with at least 4.5:1 contrast, while ANSI-stripped color output equals the no-color label text.

### Tests for User Story 3

- [X] T015 [US3] Add failing contrast-ratio, context ANSI-sequence, and ANSI-stripped/no-color equivalence assertions in `internal/tui/styles_test.go`
- [X] T016 [P] [US3] Add failing color-independent context-label recognition assertions in `internal/tui/accessibility_test.go`
- [X] T017 [P] [US3] Add failing narrow-panel bounds assertions for structural titles and context labels in `internal/tui/responsive_layout_test.go`

### Implementation for User Story 3

- [X] T018 [US3] Replace the bright-blue/black context badge palette with a dark-background/white-foreground pair that meets 4.5:1 contrast in `internal/tui/styles.go`
- [X] T019 [US3] Preserve ANSI-aware truncation and panel-bound behavior for the refined title and context output in `internal/tui/model.go` and `internal/tui/modal.go`

**Checkpoint**: Color mode improves contrast, no-color mode retains full text meaning, and responsive rendering remains bounded.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Complete documentation and repository-level quality gates without broadening the visual scope.

- [X] T020 Update the documented structural-title and context-label convention, including no-color behavior, in `README.md`
- [X] T021 Run formatting on changed renderer and test files in `internal/tui/styles.go`, `internal/tui/styles_test.go`, `internal/tui/detail_test.go`, `internal/tui/panel_style_test.go`, `internal/tui/modal_test.go`, `internal/tui/accessibility_test.go`, and `internal/tui/responsive_layout_test.go`
- [X] T022 Run the focused and full automated validation commands from `specs/009-refine-panel-titles/quickstart.md`
- [X] T023 Run Tree, Details, context-label, modal, no-color, and resize verification through the integration and TUI test scenarios in `specs/009-refine-panel-titles/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Starts immediately.
- **Foundational (Phase 2)**: Records that no blocking infrastructure is needed.
- **US1 (Phase 3)**: Starts after setup and is the MVP.
- **US2 (Phase 4)**: Starts after setup; it may proceed after US1 because both alter `internal/tui/styles.go`, so execute its implementation after T005.
- **US3 (Phase 5)**: Starts after US2 because T018 finalizes the same shared style palette in `internal/tui/styles.go`.
- **Polish (Phase 6)**: Starts after all selected user stories are complete.

### User Story Dependencies

- **US1 (P1)**: Independent visual increment; no dependency on context labels.
- **US2 (P2)**: Independently testable after the shared title path is stable; shares `internal/tui/styles.go` with US1, so serialize style implementation.
- **US3 (P3)**: Independently verifies contrast and no-color semantics after US2 has established the context-label text syntax.

### Parallel Opportunities

- US1 test work in `internal/tui/panel_style_test.go` and `internal/tui/modal_test.go` can run in parallel.
- US2 detail and cross-surface test work can run in parallel; form and prompt call-site verification can run in parallel.
- US3 accessibility and responsive-layout tests can run in parallel.
- Documentation work can run in parallel with final test execution once all renderer changes are complete.

## Parallel Examples

### User Story 1

```text
Task: "T003 add Tree, Details, and Actions title assertions in internal/tui/panel_style_test.go"
Task: "T004 add modal title assertions in internal/tui/modal_test.go"
```

### User Story 2

```text
Task: "T008 add detail context-label assertions in internal/tui/detail_test.go"
Task: "T009 add cross-surface context-label assertions in internal/tui/panel_style_test.go"
Task: "T013 verify connection-form context label in internal/tui/connection_form.go"
Task: "T014 verify prompt context labels in internal/tui/trust_prompt.go and internal/tui/secret_prompt.go"
```

### User Story 3

```text
Task: "T016 add no-color accessibility assertions in internal/tui/accessibility_test.go"
Task: "T017 add narrow-panel title and label bounds assertions in internal/tui/responsive_layout_test.go"
```

## Implementation Strategy

### MVP First

1. Complete T001-T007.
2. Validate Tree, Details, Actions, and modal structural titles independently.
3. Demo the no-background, bracket-preserving structural-title convention before implementing contextual-label changes.

### Incremental Delivery

1. Deliver US1: structural titles have no background.
2. Deliver US2: existing context labels lose brackets while remaining distinct.
3. Deliver US3: context label contrast and no-color equivalence are enforced.
4. Complete cross-cutting documentation and validation.
