---

description: "Task list for proportional scrollbar implementation"
---

# Tasks: Indicador de barra de desplazamiento

**Input**: Design documents from `/specs/005-add-scrollbar-indicator/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/tui-scrollbar.md, quickstart.md

**Tests**: Automated tests are required by the feature specification, acceptance criteria, implementation plan, and project constitution. Write each story's tests first and observe them fail before implementation.

**Organization**: Tasks are grouped by user story so each increment can be implemented and tested independently after the shared viewport foundation is complete.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes different files and has no dependency on an incomplete task
- **[Story]**: Maps work to US1, US2, or US3 from spec.md
- Every task includes exact repository-relative file paths

## Phase 1: Setup (Shared Test Infrastructure)

**Purpose**: Prepare reusable assertions for one-cell scrollbar geometry without changing production behavior

- [X] T001 Add ANSI-stripped panel-row, inner-right-edge, and scrollbar-cell assertion helpers for later story tests in `internal/tui/overflow_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Replace marker-oriented viewport budgeting with shared projection metadata required by every story

**CRITICAL**: No user story implementation can begin until this phase is complete.

- [X] T002 Add failing table tests for full-row content capacity, half-up geometry, effective render ranges, stale offsets, active-line visibility, zero-row inputs, and one-row overflow without marker rows in `internal/tui/viewport_test.go`
- [X] T003 Define projected-section metadata and implement overflow-safe half-up thumb geometry for the effective range matching `data-model.md` in `internal/tui/viewport.go`
- [X] T004 Replace `↑ more`/`↓ more` row budgeting with bounded full-row projection that emits scrollbar geometry while preserving logical offsets, active visibility, and horizontal ellipsis in `internal/tui/viewport.go`

**Checkpoint**: Shared projection exposes marker-free visible ranges and section metadata; user story work can begin.

---

## Phase 3: User Story 1 - Recognize Hidden Content (Priority: P1) MVP

**Goal**: Show a one-column scrollbar adjacent to the right border whenever a scrollable body overflows, hide it when content fits, and remove all visible directional `more` text while keeping Actions non-scrollable.

**Independent Test**: Present fitting and overflowing Tree, Details, form, Help, picker, confirmation, and error content; every overflowing body has a right-edge bar, every fitting body reclaims the column, fixed controls remain outside the track, Actions uses `Hidden actions — ? Help`, and the complete frame contains no `↑ more` or `↓ more`.

### Tests for User Story 1

> Write these tests first and ensure they fail before implementation.

- [X] T005 [P] [US1] Add failing fit/overflow and exact right-border adjacency tests for Tree, Details, and connection form sections in `internal/tui/overflow_test.go`
- [X] T006 [P] [US1] Add failing modal-body track extent tests proving leading notices and trailing recovery/cancel controls stay outside the bar in `internal/tui/modal_layout_test.go`
- [X] T007 [P] [US1] Add failing exact `Hidden actions — ? Help`, non-scrollable Actions, and complete Help action-inventory assertions in `internal/tui/actions_test.go`
- [X] T008 [P] [US1] Add failing complete-frame assertions that directional `more` labels never return through final clipping in `internal/tui/navigation_test.go`

### Implementation for User Story 1

- [X] T009 [P] [US1] Expose Tree and Details projection metadata while preserving the established content width for right-padding composition in `internal/tui/browser.go` and `internal/tui/detail.go`
- [X] T010 [P] [US1] Adapt connection-form content and active-block projection to emit scrollbar metadata only on overflow in `internal/tui/connection_form.go`
- [X] T011 [P] [US1] Return scrollable modal-body metadata separately from fixed controls without changing wrapped content width in `internal/tui/modal.go`
- [X] T012 [P] [US1] Add one-cell track/thumb glyphs, display-width fallbacks, and semantic style roles in `internal/tui/styles.go`
- [X] T013 [US1] Compose projected sections so track/thumb cells replace right padding immediately before the border without changing outer layout rectangles in `internal/tui/model.go`
- [X] T014 [US1] Rename the Actions omission cue and remove directional-label final-frame fallback behavior in `internal/tui/actions.go` and `internal/tui/model.go`
- [X] T015 [US1] Complete the closed Tree/Details/form/Help/picker/confirmation/recoverable-error matrix for fit/overflow, fixed rows, complete Help inventory, and zero directional labels in `internal/tui/overflow_test.go`

**Checkpoint**: US1 independently communicates hidden content on every scrollable body and removes the old labels without changing keyboard behavior.

---

## Phase 4: User Story 2 - Understand Position and Proportion (Priority: P2)

**Goal**: Make thumb length and top position represent the final visible fraction and effective range at start, middle, end, and after size/content changes.

**Independent Test**: Evaluate the exact 36 combinations from SC-003 plus 20 bidirectional traversals per surface; geometry follows half-up formulas, remains bounded, uses a one-cell minimum, reaches exact endpoints, and follows active-row-derived ranges.

### Tests for User Story 2

> Write these tests first and ensure they fail before implementation.

- [X] T016 [US2] Expand exact SC-003 coverage to the full 36-case half-up length/top, endpoint, one-cell limitation, clamping, and maximum-integer overflow-safety table in `internal/tui/viewport_test.go`
- [X] T017 [P] [US2] Add failing selected-row and focused-active-block cases proving thumb position uses effective render offset in `internal/tui/browser_test.go` and `internal/tui/connection_form_test.go`
- [X] T018 [P] [US2] Add failing start/middle/end and fit-to-overflow matrices for Tree, Details, form, Help, picker, confirmation, and recoverable-error bodies in `internal/tui/sc007_viewport_acceptance_test.go`

### Implementation for User Story 2

- [X] T019 [US2] Extend effective-range projection for multi-line active blocks and quantized intermediate positions without mutating logical offsets in `internal/tui/viewport.go`
- [X] T020 [US2] Correct oversized active-block effective offsets and expose the actual first rendered row in `internal/tui/connection_form.go`
- [X] T021 [US2] Feed final reflowed content length, visible capacity, and effective offsets into Tree, Details, form, Help, picker, confirmation, and recoverable-error projections in `internal/tui/browser.go`, `internal/tui/detail.go`, `internal/tui/connection_form.go`, and `internal/tui/modal.go`
- [X] T022 [US2] Assert monotonic thumb movement, permitted short-track quantization, and exact top/bottom positions across 20 complete traversals for every closed-inventory surface in `internal/tui/sc007_viewport_acceptance_test.go`

**Checkpoint**: US2 independently proves proportional size and position for every scrollable surface while retaining US1 visibility behavior.

---

## Phase 5: User Story 3 - Preserve Navigation and Legibility (Priority: P3)

**Goal**: Preserve keyboard-only operation, independent container state, active-content visibility, responsive restoration, and distinguishable no-color presentation.

**Independent Test**: Traverse all existing scroll flows using only current keys, resize through supported and undersized modes, and switch color off; focus/selection/logical offsets remain intact, bars stay independent, fixed safety controls remain visible, and track/thumb remain textually distinguishable.

### Tests for User Story 3

> Write these tests first and ensure they fail before implementation.

- [X] T023 [P] [US3] Add failing no-color and ANSI-stripped one-cell track/thumb distinction tests, including fallback glyph widths, in `internal/tui/styles_test.go`
- [X] T024 [P] [US3] Add failing keyboard-only and independent Tree/Details/modal scrollbar state tests in `internal/tui/navigation_test.go` and `internal/tui/modal_recovery_test.go`
- [X] T025 [P] [US3] Add failing SC-005 coverage with 20 repetitions of `40→60→79→80→100→160→80→79→40` at heights 12/24 and `40x12→39x11→40x12`, asserting selection, focus, logical offsets, active visibility, and before-next-input bars in `internal/tui/resize_state_test.go` and `internal/tui/undersized_test.go`

### Implementation for User Story 3

- [X] T026 [US3] Apply distinct one-cell textual track/thumb semantics in color and no-color modes in `internal/tui/styles.go`
- [X] T027 [US3] Preserve per-container logical state and derive independent effective bars through focus changes, content shrink, and resize in `internal/tui/model.go`
- [X] T028 [US3] Keep fixed modal recovery/safety controls visible and bar-free while preserving background viewport state in `internal/tui/modal.go` and `internal/tui/modal_state.go`
- [X] T029 [US3] Extend end-to-end keyboard-only coverage for overflowing Tree, Details, form, Help, picker, confirmation, and recoverable-error flows, asserting no scrollbar focus or mouse dependency in `tests/integration/tui_tree_test.go`

**Checkpoint**: All three stories are independently functional and the new indicator preserves terminal-first usability.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Update broad acceptance gates, documentation, robustness checks, and repository quality gates

- [X] T030 [P] Replace directional-marker assumptions with the closed surface matrix, exact SC-003/SC-004 geometry, full SC-007 keyboard flows, right-edge placement, and visible-content assertions in `internal/tui/sc007_full_acceptance_test.go`
- [X] T031 [P] Extend malformed/extreme viewport dimension and offset coverage for bounded scrollbar metadata in `internal/tui/model_fuzz_test.go`
- [X] T032 [P] Preserve the 10,000-render/resize retention gate and assert SC-008 has at least 19/20 complete navigation, focus, scroll, and resize frames within 100 ms using conditional right-padding composition in `internal/tui/performance_acceptance_test.go`
- [X] T033 [P] Update keyboard-only overflow, no-color glyphs, Actions omission text, and minimum-size behavior in `README.md`
- [X] T034 Inventory and add feature-005 supersession references for every directional-marker rule in `specs/004-redesign-tui-layout/spec.md`, `specs/004-redesign-tui-layout/plan.md`, `specs/004-redesign-tui-layout/research.md`, `specs/004-redesign-tui-layout/data-model.md`, `specs/004-redesign-tui-layout/contracts/tui.md`, `specs/004-redesign-tui-layout/quickstart.md`, and `specs/004-redesign-tui-layout/tasks.md`
- [X] T035 Run formatting, focused/full/race/acceptance tests, vet, build, staticcheck, govulncheck, and `nix flake check` from `specs/005-add-scrollbar-indicator/quickstart.md`, recording platform-only limitations in `specs/005-add-scrollbar-indicator/quickstart.md`
- [X] T036 Execute the pre-merge SC-006 participant protocol from `specs/005-add-scrollbar-indicator/quickstart.md` and record anonymized aggregate timing and accuracy evidence in `specs/005-add-scrollbar-indicator/validation/sc006.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; starts immediately.
- **Foundational (Phase 2)**: Depends on T001 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2 and is the MVP.
- **US2 (Phase 4)**: Depends on completed US1 rendering because it extends the same source and projection files with effective proportional behavior.
- **US3 (Phase 5)**: Depends on completed US2 geometry because it validates the final bars through keyboard, resize, and no-color flows.
- **Polish (Phase 6)**: Depends on all selected user stories.

### User Story Dependency Graph

```text
Setup → Foundation → US1 (P1, MVP) → US2 (P2) → US3 (P3) → Polish
```

### User Story Independence

- **US1**: Uses foundational proportional geometry to demonstrate valid bar presence/absence and complete directional-label removal without requiring traversal acceptance.
- **US2**: Extends US1 with independently testable effective-offset, exact matrix, and traversal behavior.
- **US3**: Validates the completed US1/US2 bars through independently testable keyboard, resize, no-color, and fixed-row flows.

### Within Each User Story

- Write the story's tests and observe failures before production changes.
- Complete source generation before panel composition that consumes it.
- Complete pure geometry before cross-surface geometry integration.
- Keep fixed rows separate before validating modal safety controls.
- Pass the independent checkpoint before moving to the next priority in a single-developer flow.

### Parallel Opportunities

- US1 test tasks T005–T008 modify different files and can run in parallel.
- US1 source/style tasks T009–T012 modify different files after Phase 2 and can run in parallel.
- After T016 completes, US2 test tasks T017 and T018 can run in parallel because they modify different test files.
- US3 test tasks T023–T025 modify different files and can run in parallel.
- Polish tasks T030–T033 modify different files and can run in parallel.

---

## Parallel Example: User Story 1

```text
Task: "T005 Add Tree, Details, and form right-edge tests in internal/tui/overflow_test.go"
Task: "T006 Add modal fixed-row extent tests in internal/tui/modal_layout_test.go"
Task: "T007 Add Actions omission cue tests in internal/tui/actions_test.go"
Task: "T008 Add final-frame marker-removal tests in internal/tui/navigation_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T017 Add effective-offset cases in internal/tui/browser_test.go and internal/tui/connection_form_test.go"
Task: "T018 Add start/middle/end matrices in internal/tui/sc007_viewport_acceptance_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T023 Add no-color glyph tests in internal/tui/styles_test.go"
Task: "T024 Add keyboard and independent-state tests in internal/tui/navigation_test.go and internal/tui/modal_recovery_test.go"
Task: "T025 Add resize and undersized restoration tests in internal/tui/resize_state_test.go and internal/tui/undersized_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001 and Phase 2.
2. Write and fail US1 tests T005–T008.
3. Implement T009–T015.
4. Run the US1 focused tests and manually inspect fitting/overflow frames.
5. Stop with a demonstrable replacement for directional `more` labels before adding proportional movement refinements.

### Incremental Delivery

1. **Foundation**: Shared marker-free projection and section metadata.
2. **US1**: Visible right-edge bars, conditional width, fixed-row extent, and Actions text deliver the MVP.
3. **US2**: Proportional length/position and effective-offset behavior add spatial context.
4. **US3**: Keyboard, independent state, resize restoration, and no-color semantics complete terminal-first quality.
5. **Polish**: Scale, fuzz, performance, documentation, and repository gates prepare the feature for merge.

### Parallel Team Strategy

1. Complete Setup and Foundation together.
2. Complete US1, US2, and US3 sequentially because they share `browser.go`, `detail.go`, `connection_form.go`, `modal.go`, `styles.go`, `model.go`, and navigation tests.
3. Within each phase, parallelize only tasks explicitly marked `[P]` after their stated prerequisite.
4. Integrate stories through the cross-surface and acceptance matrices before Phase 6 validation.

---

## Notes

- `[P]` means the task can proceed in parallel without editing a file owned by another incomplete task.
- `[US1]`, `[US2]`, and `[US3]` provide direct story traceability.
- Test tasks precede implementation because behavior-focused coverage is mandatory for this user-visible TUI change.
- Do not add a scrollbar to Actions, mouse handling, new keyboard bindings, persistent state, or new dependencies.
- Use only synthetic non-secret fixtures; no real SSH server or credential store is required.
- Stop at any story checkpoint to validate that increment independently.

---

## Phase 7: Convergence

- [X] T037 CRITICAL remove legacy directional-marker constants from `internal/tui/viewport.go` and replace test references with test-local forbidden literals in `internal/tui/viewport_test.go`, `internal/tui/browser_test.go`, `internal/tui/confirmation_test.go`, `internal/tui/connection_form_test.go`, `internal/tui/modal_recovery_test.go`, `internal/tui/overflow_test.go`, and `internal/tui/sc007_viewport_acceptance_test.go` per Constitution V and FR-012 (contradicts)
- [X] T038 Implement ANSI display-width validation for preferred `│`/`█` scrollbar glyphs with `|`/`#` fallbacks in `internal/tui/styles.go` and add invalid-width fallback coverage in `internal/tui/styles_test.go` per Contract §Visual Vocabulary and FR-004 (missing)
- [X] T039 Add 20 complete forward/reverse traversal cases with exact start/middle/end geometry for Tree, Details, form, Help, picker, confirmation, and recoverable-error surfaces in `internal/tui/sc007_viewport_acceptance_test.go` and `internal/tui/sc007_full_acceptance_test.go` per SC-003, SC-004, and SC-007 (partial)
- [X] T040 Add rendered-row fit/overflow coverage for all seven surfaces, including penultimate-cell track/thumb placement, restored right padding, full body-track extent, and track-free fixed controls in `internal/tui/overflow_test.go` and `internal/tui/modal_layout_test.go` per FR-002, FR-003, and FR-004 (partial)
- [X] T041 Extend keyboard-only integration flows through overflowing Tree, Details, connection form, Help, move picker, confirmation, and recoverable-error surfaces in `tests/integration/tui_tree_test.go`, asserting active-content visibility and no scrollbar focus or mouse dependency per SC-007 (partial)
- [X] T042 Run the exact SC-005 width sequence 20 times at heights 12 and 24 plus `40x12→39x11→40x12`, asserting selection, focus, logical/effective offsets, active visibility, bar geometry, and restoration before further input in `internal/tui/resize_state_test.go`, `internal/tui/resize_conformance_test.go`, and `internal/tui/undersized_test.go` per SC-005 (partial)

---

## Phase 8: Convergence

- [X] T043 Separate modal `leadingFixedLines`, scrollable body, and trailing controls in `internal/tui/modal.go`, deduct fixed rows from body capacity, start the track after leading notices, and add rendered overflow coverage for operation-status, conflict, and recoverable-error notices in `internal/tui/modal_layout_test.go` per FR-004, FR-011, and US3/AC5 (partial)
- [X] T044 Drive all seven closed-inventory surfaces through their existing keyboard controls and assert surface-local scrollbar geometry, active-content visibility, focus ownership, and background-container independence in `internal/tui/sc007_viewport_acceptance_test.go`, `internal/tui/sc007_full_acceptance_test.go`, and `tests/integration/tui_tree_test.go` per SC-001, SC-007, US3/AC1, and US3/AC2 (partial)
- [X] T045 Align scrollbar eligibility with composed interior geometry so one content cell beside the separate right-padding/bar cell remains valid, route truly insufficient widths through existing reduced behavior, and add width 0/1/2 projection and rendering coverage in `internal/tui/viewport.go`, `internal/tui/connection_form.go`, `internal/tui/viewport_test.go`, and `internal/tui/overflow_test.go` per FR-018 (partial)
