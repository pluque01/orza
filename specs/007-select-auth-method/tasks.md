---

description: "Task list for direct authentication-method selection"
---

# Tasks: Selección directa del método de autenticación

**Input**: Design documents from `/specs/007-select-auth-method/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/tui-auth-method-selector.md, quickstart.md

**Tests**: Automated tests are required by the feature specification, implementation plan, acceptance criteria, and project constitution. Write each story's tests first and observe them fail before implementation.

**Organization**: Tasks are grouped by user story so the closed selector, dependent controls, and terminal presentation can be implemented and validated as explicit increments.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes different files and has no dependency on an incomplete task
- **[Story]**: Maps work to US1, US2, or US3 from spec.md
- Every task includes exact repository-relative file paths

## Phase 1: Setup (Shared Test Infrastructure)

**Purpose**: Replace direct test mutation of the former authentication text field with one typed fixture boundary shared by selector tests

- [X] T001 Add shared helpers for setting, reading, and snapshotting the selected authentication method, then route existing direct `inputs[fieldAuth]` fixture writes through them in `internal/tui/connection_form_test.go`, `internal/tui/ui_conformance_test.go`, `internal/tui/resize_conformance_test.go`, and `internal/tui/sc007_viewport_acceptance_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the closed typed state used by every user story without changing persistence or authentication services

**CRITICAL**: No user story implementation can begin until this phase is complete.

- [X] T002 Define the ordered Agent/Key/Password inventory, typed selected and baseline method state, create/edit initialization, and selector accessors while retaining `fieldAuth` as the stable focus/error identifier and preserving existing conditional-field behavior in `internal/tui/connection_form.go`

**Checkpoint**: Every form has exactly one valid typed method and existing tests can address it without editing free text.

---

## Phase 3: User Story 1 - Elegir entre métodos visibles (Priority: P1) MVP

**Goal**: Show all three predefined methods in create and edit forms, select the correct initial method, cycle with Left/Right, reject free-text input, and save exactly the selected method.

**Independent Test**: Open new and existing connection forms, verify the three full labels and one selection, traverse both cyclic boundaries, attempt typing/editing/paste, and save each method through create/edit flows without relying on dependent-field behavior beyond supplying valid data.

### Tests for User Story 1

> Write these tests first and ensure they fail before implementation.

- [X] T003 [US1] Add failing SC-001/SC-002 tables covering 30 new-form openings, 10 edit openings per persisted method, exact one-row rendering, unique selection, Left/Right from every option, and 20 complete cycles in each direction in `internal/tui/connection_form_test.go`
- [X] T004 [P] [US1] Add failing public-model keyboard flows that create and edit connections with each selected method and assert the persisted method matches the visible selection in `tests/integration/tui_connection_test.go`
- [X] T005 [US1] Add failing non-mutation cases for three valid names, three invalid names, printable/edit keys, Unicode, control-bearing and malformed paste, plus generic text-field exclusion and typed create/edit request assertions in `internal/tui/connection_form_test.go` and `internal/tui/model_test.go`

### Implementation for User Story 1

- [X] T006 [US1] Replace the Method text-input view with a compact `[Selected]` option row and route only unmodified physical Left/Right to cyclic typed selection while consuming other local input in `internal/tui/connection_form.go`
- [X] T007 [US1] Drive dependent visibility needed for valid Agent/Key/Password saves, form dirtiness, defensive validation, and create/update authentication request values from typed selection and baseline instead of `inputs[fieldAuth]` in `internal/tui/connection_form.go`

**Checkpoint**: US1 independently provides a closed visible selector in both form modes and persists exactly the chosen method.

---

## Phase 4: User Story 2 - Completar solo la configuración aplicable (Priority: P2)

**Goal**: Show, focus, validate, and persist only the selected method's dependent control while retaining hidden drafts safely during the form lifetime.

**Independent Test**: Exercise all nine method transitions after entering Identity and checking Remember; verify immediate visibility and focus fallback, retained drafts on return, selected-method-only validation/request projection, and zero password prompt or credential intent when Remember is hidden under Agent or Key.

### Tests for User Story 2

> Write these tests first and ensure they fail before implementation.

- [X] T008 [US2] Add failing nine-transition visibility, canonical forward/reverse focus, hidden Identity/Remember retention, focus fallback, and selected-method-only validation tables in `internal/tui/connection_form_test.go`
- [X] T009 [P] [US2] Add failing security regressions proving a retained hidden Remember choice cannot request, project, or persist a password for final Agent or Key selections in `internal/tui/embedded_form_security_test.go`
- [X] T010 [P] [US2] Add failing create/update projection cases for applicable Identity, clearing a persisted Key identity, changed-method pointer semantics, hidden-only drafts, and remembered-password removal in `internal/tui/operation_conformance_test.go`
- [X] T011 [P] [US2] Add failing public create/edit keyboard flows across all nine transitions, validation/persistence failures, identity clearing, and hidden Remember safety in `tests/integration/tui_connection_test.go`

### Implementation for User Story 2

- [X] T012 [US2] Complete typed dependent focus order/fallback, hidden Identity/Remember retention, dirty protection, and selected-method-only validation/projection across every transition in `internal/tui/connection_form.go`
- [X] T013 [US2] Gate effective remember intent on final Password selection before starting create/update credential work while preserving the existing remembered-password removal lifecycle in `internal/tui/model.go`

**Checkpoint**: US2 independently proves that only applicable method configuration can be validated, prompted for, or persisted, without losing temporary form drafts.

---

## Phase 5: User Story 3 - Reconocer y operar el selector en cualquier terminal soportada (Priority: P3)

**Goal**: Keep focus, selection, labels, controls, and retained state readable and operable in color/no-color, responsive, overflowing, and undersized terminal states.

**Independent Test**: Traverse the form with keyboard only at 40x12, 79x24, and 80x24 in color and no-color modes; verify complete labels, separate focus/selection markers, contextual Left/Right help, one focus stop, scrollbar compatibility, and exact state restoration after undersized resize.

### Tests for User Story 3

> Write these tests first and ensure they fail before implementation.

- [X] T014 [P] [US3] Add failing exact no-color and ANSI-stripped color assertions for one focus marker, one selected bracket marker, and complete Agent/Key/Password labels in `internal/tui/accessibility_test.go`
- [X] T015 [P] [US3] Add failing 40x12, 79x24, 80x24, overflow-scrollbar, twelve-size resize, and 40x12→39x11→40x12 selector-state restoration cases, including rejected selector input while undersized, in `internal/tui/resize_conformance_test.go` and `internal/tui/undersized_test.go`
- [X] T016 [P] [US3] Add failing contextual Actions/Help assertions for cyclic `Left/Right` method navigation and absence of text-entry, paste, activation, or mouse instructions in `internal/tui/actions_test.go` and `internal/tui/language_test.go`
- [X] T017 [P] [US3] Add failing keyboard-owner tests for printable command keys, shifted arrows, paste boundaries, F1 Help, Ctrl+S Save, Esc Cancel/Discard, Ctrl+C safe exit, and validation, persistence, and conflict-failure preservation with retained drafts while Method is focused in `internal/tui/embedded_form_security_test.go`
- [X] T018 [P] [US3] Add failing selector movement and render cases to the existing 20-run local interaction latency gate without changing its 100 ms target in `internal/tui/performance_acceptance_test.go`
- [X] T019 [US3] Add failing repeated create/edit × Agent/Key/Password × Tab/Shift+Tab/F2, color/no-color, responsive viewport, and failure-preservation acceptance flows in `internal/tui/ui_conformance_test.go` and `internal/tui/resize_conformance_test.go`

### Implementation for User Story 3

- [X] T020 [P] [US3] Add a Method-focus action context and contextual `Left/Right Change method` descriptor without changing non-form action inventories in `internal/tui/actions.go`
- [X] T021 [US3] Wire Method focus into contextual Actions and form Help, keep selector and dependent state unchanged across Help/failure transitions, and include selector navigation in the bounded form footer in `internal/tui/model.go` and `internal/tui/connection_form.go`
- [X] T022 [P] [US3] Register new controlled English selector/help text and expected vocabulary classifications in `internal/tui/testdata/controlled_english.txt` and `internal/tui/language_test.go`

**Checkpoint**: All three stories are independently functional and the selector satisfies terminal-first accessibility, help, responsive, and performance contracts.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Align user documentation and execute the complete repository validation gates

- [X] T023 [P] Document the visible create/edit selector, Agent default, cyclic Left/Right behavior, conditional controls, retained drafts, and exclusion from text/paste fields in `README.md`
- [X] T024 Run every focused, full, race, vet, build, staticcheck, govulncheck, responsive acceptance, and Nix command from `specs/007-select-auth-method/quickstart.md`, recording any platform-only limitation in `specs/007-select-auth-method/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; starts immediately.
- **Foundational (Phase 2)**: Depends on T001 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2 and is the MVP.
- **US2 (Phase 4)**: Depends on US1 typed rendering and request projection because it adds conditional state and credential safety to the same form.
- **US3 (Phase 5)**: Depends on US1 and US2 final selector/dependent behavior before responsive and help acceptance can be conclusive.
- **Polish (Phase 6)**: Depends on all selected user stories.

### User Story Dependency Graph

```text
Setup → Foundation → US1 (P1, MVP) → US2 (P2) → US3 (P3) → Polish
```

### User Story Testability

- **US1**: Independently demonstrates the closed three-option selector, cyclic keyboard choice, input isolation, conditional fields sufficient to save each method, and exact method persistence without requiring US2 safety or US3 responsive acceptance.
- **US2**: Is independently testable as a method-transition and credential-boundary matrix after the selector exists; its technical implementation depends on US1 but its acceptance does not depend on US3.
- **US3**: Is independently testable as a terminal presentation and interaction matrix after selector semantics are complete; it does not change method, request, or credential outcomes.

### Within Each User Story

- Write the story's tests and observe failures before production changes.
- Establish form-level state/rendering before model-level integration.
- Preserve domain, repository, SSH, and credential-service boundaries.
- Pass the independent checkpoint before moving to the next priority in a single-developer flow.

### Parallel Opportunities

- US1 test task T004 can proceed in parallel with the package-level T003/T005 sequence after Foundation.
- US2 test tasks T009–T011 modify different files and can proceed in parallel with T008.
- US3 test tasks T014–T018 modify separate primary test files and can be prepared in parallel before T019 consolidates shared responsive acceptance.
- US3 implementation tasks T020 and T022 modify separate files and can proceed in parallel after their tests.
- Polish documentation T023 can proceed in parallel with completed-code review before T024 runs final gates.

---

## Parallel Example: User Story 1

```text
Task: "T003 Add selector initialization, rendering, cycle, and isolation tests in internal/tui/connection_form_test.go"
Task: "T004 Add public create/edit selection flows in tests/integration/tui_connection_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T008 Add all dependency transition and retention tests in internal/tui/connection_form_test.go"
Task: "T009 Add hidden Remember security regressions in internal/tui/embedded_form_security_test.go"
Task: "T010 Add request projection and credential-lifecycle cases in internal/tui/operation_conformance_test.go"
Task: "T011 Add public dependent-state failure flows in tests/integration/tui_connection_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T014 Add selector accessibility assertions in internal/tui/accessibility_test.go"
Task: "T015 Add selector resize and undersized restoration tests in internal/tui/resize_conformance_test.go and internal/tui/undersized_test.go"
Task: "T016 Add selector Actions/Help assertions in internal/tui/actions_test.go and internal/tui/language_test.go"
Task: "T017 Add selector input-owner and failure preservation tests in internal/tui/embedded_form_security_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001 and T002.
2. Write and fail US1 tests T003–T005.
3. Implement T006–T007.
4. Run the US1 focused form and integration tests.
5. Stop with a demonstrable create/edit selector that rejects free text and persists exactly one selected method.

### Incremental Delivery

1. **Foundation**: Typed closed method inventory and shared test boundary.
2. **US1**: Visible selection, cyclic input, input isolation, and method persistence deliver the MVP.
3. **US2**: Conditional controls, retained drafts, validation/request filtering, and credential safety complete method semantics.
4. **US3**: Focus/selection accessibility, contextual help, responsive restoration, and performance complete terminal quality.
5. **Polish**: README alignment and full quality gates prepare the feature for merge.

### Parallel Team Strategy

1. Complete Setup and Foundation together.
2. Complete stories sequentially because they share `internal/tui/connection_form.go`, `internal/tui/model.go`, and form integration tests.
3. Within each story, parallelize only tasks marked `[P]` after their stated prerequisite.
4. Integrate through public model and responsive acceptance tests before final validation.

---

## Notes

- `[P]` means the task can proceed without editing a file owned by another incomplete task.
- `[US1]`, `[US2]`, and `[US3]` provide direct traceability to the specification.
- Test tasks precede implementation because behavior-focused coverage is mandatory for this user-visible input and state change.
- Do not add a selector dependency, persistent format, new authentication method, fallback authentication, mouse input, or protocol change.
- Use only synthetic non-secret fixtures; no real SSH server, private key, password, or native credential store is required.
- Stop at each story checkpoint and validate that increment independently.
