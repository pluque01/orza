---
description: "Task list for plain panel titles and bold context labels"
---

# Tasks: Simplificar etiquetas de panel

**Input**: Design documents from `/specs/010-simplify-panel-labels/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/title-context-text.md](contracts/title-context-text.md), [quickstart.md](quickstart.md)

**Tests**: Required by the specification and Orza Constitution. Write renderer contracts before the shared style change.

## Phase 1: Setup

**Purpose**: Establish current title and context rendering baselines.

- [ ] T001 Run and record baseline rendering behavior in `internal/tui/styles_test.go`, `internal/tui/panel_style_test.go`, and `internal/tui/detail_test.go`

---

## Phase 2: Foundational

**Purpose**: No new infrastructure is required; existing placement-aware style rendering is the shared foundation.

**Checkpoint**: Use `internal/tui/styles.go` without new state, dependencies, or theme configuration.

---

## Phase 3: User Story 1 - Leer la estructura sin ruido visual (Priority: P1) MVP

**Goal**: Render plain region and modal title text while retaining `[*]` and `[ ]` focus markers.

**Independent Test**: Tree, Details, Actions, and each modal show unbracketed titles with the appropriate existing focus marker.

### Tests for User Story 1

- [ ] T002 [US1] Add failing structural-title text and no-bracket assertions in `internal/tui/styles_test.go`
- [ ] T003 [P] [US1] Add failing Tree, Details, and Actions focus-marker/title assertions in `internal/tui/panel_style_test.go`
- [ ] T004 [P] [US1] Add failing modal title and focus-marker assertions in `internal/tui/modal_test.go`
- [ ] T005 [P] [US1] Update structural-title expectations across flows in `internal/tui/principal_flow_conformance_test.go`, `internal/tui/ui_conformance_test.go`, and `internal/tui/modal_conformance_test.go`

### Implementation for User Story 1

- [ ] T006 [US1] Render `badgePlacementTitle` labels without title brackets while preserving `regionTitle` focus markers in `internal/tui/styles.go`
- [ ] T007 [US1] Verify existing region and modal title composition needs no layout or control changes in `internal/tui/model.go` and `internal/tui/modal.go`

**Checkpoint**: Structural titles alone satisfy the MVP contract.

---

## Phase 4: User Story 2 - Identificar contexto en cualquier tema (Priority: P2)

**Goal**: Render all existing context labels as bold, unbracketed text without foreground or background color.

**Independent Test**: Root, Folder, Connection, form, trust-prompt, and secret-prompt context labels appear once, without brackets or color fill, and have equivalent no-color text.

### Tests for User Story 2

- [ ] T008 [US2] Add failing bold-only ANSI and no-color parity assertions for context labels in `internal/tui/styles_test.go`
- [ ] T009 [P] [US2] Add failing Root, Folder, and Connection context-label assertions in `internal/tui/detail_test.go`
- [ ] T010 [P] [US2] Update form and prompt context-label assertions in `internal/tui/connection_form_test.go`, `internal/tui/trust_prompt_test.go`, and `internal/tui/secret_prompt_test.go`
- [ ] T011 [P] [US2] Update no-color and narrow-layout label assertions in `internal/tui/accessibility_test.go` and `internal/tui/responsive_layout_test.go`
- [ ] T012 [P] [US2] Update end-to-end panel and form label assertions in `tests/integration/tui_connection_test.go` and `tests/integration/tui_tree_test.go`

### Implementation for User Story 2

- [ ] T013 [US2] Render `badgePlacementContent` as bold-only text without foreground or background color in `internal/tui/styles.go`
- [ ] T014 [US2] Verify existing detail, form, trust, and secret renderers inherit the shared context-label behavior in `internal/tui/detail.go`, `internal/tui/connection_form.go`, `internal/tui/trust_prompt.go`, and `internal/tui/secret_prompt.go`

**Checkpoint**: Context labels remain recognizable across terminal themes without color-dependent semantics.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Document the convention and run the required quality gates.

- [ ] T015 Update title, focus-marker, context-label, and no-color documentation in `README.md`
- [ ] T016 Run formatting on changed source and test files in `internal/tui/` and `tests/integration/`
- [ ] T017 Run focused, full, race, vet, and build validation from `specs/010-simplify-panel-labels/quickstart.md`
- [ ] T018 Run the quickstart Tree, Details, context-label, modal, no-color, and resize scenarios in `specs/010-simplify-panel-labels/quickstart.md`

---

## Dependencies & Execution Order

- Setup starts immediately; no foundational infrastructure blocks delivery.
- US1 is the MVP and precedes US2 because both change `internal/tui/styles.go`.
- US2 follows US1 and finalizes shared context-label rendering.
- Polish follows both user stories.

## Parallel Opportunities

- US1 panel, modal, and broad conformance test tasks T003-T005 can run in parallel.
- US2 detail, form/prompt, accessibility/responsive, and integration test tasks T009-T012 can run in parallel.
- README work can run in parallel with final validation after T013.

## Parallel Examples

### User Story 1

```text
Task: "T003 update structural region assertions in internal/tui/panel_style_test.go"
Task: "T004 update modal title assertions in internal/tui/modal_test.go"
Task: "T005 update conformance expectations in internal/tui/principal_flow_conformance_test.go"
```

### User Story 2

```text
Task: "T009 update detail label assertions in internal/tui/detail_test.go"
Task: "T010 update form and prompt assertions in internal/tui/connection_form_test.go"
Task: "T011 update accessibility assertions in internal/tui/accessibility_test.go"
Task: "T012 update integration assertions in tests/integration/tui_tree_test.go"
```

## Implementation Strategy

1. Complete T001-T007 and validate plain structural titles as the MVP.
2. Complete T008-T014 to remove color dependence from context labels.
3. Complete T015-T018 and run the full quality suite.
