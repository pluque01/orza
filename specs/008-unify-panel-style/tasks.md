---

description: "Task list for unified panel visual hierarchy"
---

# Tasks: Unified panel visual hierarchy

**Input**: Design documents from `/specs/008-unify-panel-style/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/panel-visual-hierarchy.md, quickstart.md

**Tests**: Automated tests are required by the specification, visual contract, success criteria, and project constitution. Write each story's tests first and observe them fail before implementation.

**Organization**: Tasks are grouped by user story so Details can ship as an MVP, the shared hierarchy can extend to operational surfaces, and responsive behavior can be validated as a separate increment.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes different files and has no dependency on an incomplete task
- **[Story]**: Maps work to US1, US2, or US3 from spec.md
- Every task includes exact repository-relative file paths

## Phase 1: Setup (Shared Test Infrastructure)

**Purpose**: Establish one exhaustive test inventory for the visual surfaces without changing production behavior

- [X] T001 Add reusable feature-008 fixture tables for the three browser regions, Root/Folder/Connection detail kinds, ten registered modal kinds, connection/folder forms, move picker, trust prompt, and secret prompt in `internal/tui/panel_style_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add the semantic presentation primitives and remove punctuation-dependent viewport behavior used by every story

**CRITICAL**: No user story implementation can begin until this phase is complete.

### Foundation Tests

> Write these tests first and ensure they fail before production changes.

- [X] T002 [P] Add failing exact tests for bracket-delimited badges, bright-blue color rendering, muted colonless descriptive labels, unchanged `Warning:`/`Error:` status prefixes, and `ansi.Strip(colored) == plain` semantics in `internal/tui/styles_test.go`
- [X] T003 [P] Add failing tests proving bounded projection no longer parses literal `Target: ` or `ID: ` prefixes and remains correct for ANSI-styled, Unicode-width, invalid UTF-8, bidi-control, and control-bearing rows in `internal/tui/viewport_test.go`

### Foundation Implementation

- [X] T004 Implement the transient Type Badge constraints (`label` non-empty application-controlled non-secret text; `placement` title/content; `active` derived; `accented` color-only) and Display Field/Structured Field Group constraints (`fields` ordered; `labelWidth` maximum display cells; `prefixWidth` existing marker cells; `contentWidth` local; `mode` aligned/stacked/compact-interactive; non-interactive `minimumValueWidth` eight cells) using the existing Lip Gloss and ANSI stack in `internal/tui/styles.go`
- [X] T005 [P] Replace colon-prefix inspection with label-independent safe truncation while preserving ellipsis width and scrollbar projection in `internal/tui/viewport.go`

**Checkpoint**: Shared badges, labels, value layout, and bounded projection are available without changing any domain, storage, SSH, credential, or focus state.

---

## Phase 3: User Story 1 - Recognize a connection at a glance (Priority: P1) MVP

**Goal**: Replace Details `Kind` rows with one content badge and present connection identity as colonless rows sharing one aligned value column.

**Independent Test**: Select Root, Folder, and Connection targets and verify Details shows exactly one matching content badge, never renders `Kind: Connection`, aligns each applicable connection value, preserves ordered direct children/defaults, and exposes no credential or secret data in color or no-color mode.

### Tests for User Story 1

> Write these tests first and ensure they fail before implementation.

- [X] T006 [P] [US1] Add failing SC-001 tables for 30 connection Details presentations plus additional Root/Folder cases, covering matching content badges, zero `Kind` rows, colonless Name/Path/Endpoint/User/Method/Identity labels, one shared value start, optional Identity, default user, direct-child order, and empty state in `internal/tui/detail_test.go`
- [X] T007 [P] [US1] Add failing public-model tests that synchronize selection to Details in color and no-color modes while asserting `[Tree]`, `[Details]`, and `[Actions]` region badges, one distinct selected-content badge, and unchanged region ownership in `tests/integration/tui_tree_test.go`
- [X] T008 [P] [US1] Add failing ANSI-stripped equivalence, untrusted Unicode/control projection, bounded-width, credential-reference, and secret-canary assertions for Details in `internal/tui/accessibility_test.go` and `internal/tui/embedded_form_security_test.go`

### Implementation for User Story 1

- [X] T009 [US1] Remove the `Kind` display field, retain Root/Folder/Connection as controlled content-badge labels, and render ordered detail fields/direct connections through the shared aligned presentation primitives in `internal/tui/detail.go`
- [X] T010 [US1] Pass semantic styles and local content width through Details projection so the `[Details]` region badge and distinct selected-content badge render in the same selection update without changing viewport ownership in `internal/tui/model.go`

**Checkpoint**: US1 independently delivers the requested Details hierarchy and can be demonstrated at 80x24 in color and no-color modes.

---

## Phase 4: User Story 2 - Keep operational surfaces consistent (Priority: P2)

**Goal**: Apply the same badge, colonless-label, and value hierarchy to every existing structured TUI surface while preserving controls, focus, errors, confirmations, and secret boundaries.

**Independent Test**: Open Actions, Help, connection/folder forms, move picker, connection/destructive confirmations, unsaved changes, conflicts, recoverable errors, SSH failure, trust, and secret prompts; verify each type appears once, labels use the shared hierarchy, and all existing actions, targets, errors, masking, and keyboard outcomes remain unchanged.

### Tests for User Story 2

> Write these tests first and ensure they fail before implementation.

- [X] T011 [P] [US2] Add failing known/unknown modal badge-inventory, per-kind ANSI-stripped color/no-color equivalence, duplicate-heading, colonless-field, complete wrapped-target, fixed-control, status-prefix, `[Panel]` fallback, and safe invalid-payload tests in `internal/tui/modal_test.go` and `internal/tui/modal_layout_test.go`
- [X] T012 [P] [US2] Add failing connection-form tests for controlled `[New connection]`/`[Edit connection]` content badges with catalog paths projected only as safe structured values, colonless compact labels, best available value alignment, complete authentication selector, unchanged marker/error/Save priority, colonless conflict identity rows, and public form/confirmation behavior in `internal/tui/connection_form_test.go`, `internal/tui/form_conflict_test.go`, and `tests/integration/tui_connection_test.go`
- [X] T013 [P] [US2] Add failing tests proving folder forms reuse the `Create Folder`/`Edit Folder` modal badge without a duplicate body heading and move pickers retain one type badge, colonless Target/ID/Destination/Source/Name labels, selected/unavailable markers, captured revisions, and unchanged Save/Move/Cancel/Help controls in `internal/tui/folder_test.go` and `internal/tui/modal_conformance_test.go`
- [X] T014 [P] [US2] Add failing trust/secret prompt tests for controlled content badges, per-prompt ANSI-stripped color/no-color equivalence, colonless Host/address/algorithm/fingerprint/Secret labels, unchanged reject default and changed/revoked warnings, exact masking length, and zero secret/credential projection in `internal/tui/trust_prompt_test.go`, `internal/tui/secret_prompt_test.go`, and `internal/tui/security_input_test.go`
- [X] T015 [P] [US2] Add failing Actions/Help tests for ANSI-stripped bracketed type titles, no duplicate body heading, unchanged descriptor inventory/packing/overflow marker/keys, and unchanged captured confirmation targets in `internal/tui/actions_test.go` and `tests/integration/tui_actions_test.go`

### Implementation for User Story 2

- [X] T016 [P] [US2] Render modal titles as badges, remove semantically duplicate body headings, and convert confirmation/error/help payload fields to shared colonless structured rows while preserving full target wrapping and control priority in `internal/tui/modal.go`
- [X] T017 [P] [US2] Render controlled New/Edit connection content badges separately from safely projected catalog paths, then apply muted colonless labels and best-effort aligned compact rows without changing focus, active blocks, authentication options, checkboxes, validation, or footer controls in `internal/tui/connection_form.go`
- [X] T018 [P] [US2] Remove duplicate folder-form body headings in favor of existing modal badges and apply shared colonless structured rows to folder forms and move picker without changing captured targets, revisions, selection, or controls in `internal/tui/folder_form.go` and `internal/tui/move_picker.go`
- [X] T019 [P] [US2] Apply controlled content badges and shared colonless labels after existing safe-value projection while retaining warning/error status punctuation, reject defaults, masking, and secret ownership in `internal/tui/trust_prompt.go` and `internal/tui/secret_prompt.go`
- [X] T020 [US2] Integrate region-title badges, colonless structured form-conflict Target/ID/Revision rows, and non-duplicated embedded form/security headings while preserving modal and focus owners in `internal/tui/model.go`
- [X] T021 [US2] Synchronize all changed/added controlled English badge and label literals with exact production inventory while removing superseded `Kind` and duplicate-heading copies in `internal/tui/testdata/controlled_english.txt` and `internal/tui/language_test.go`

**Checkpoint**: US2 independently demonstrates one visual hierarchy across all operational surfaces with no change to input, persistence, SSH, or credential behavior.

---

## Phase 5: User Story 3 - Preserve legibility in reduced terminals (Priority: P3)

**Goal**: Stack non-interactive identity rows when their shared value column would have fewer than eight cells, keep interactive rows compact, and preserve bounded output, state, focus, errors, controls, and performance through resize.

**Independent Test**: Exercise Details, forms, Help, confirmations, errors, and prompts across the established responsive matrix and undersized transition with long Unicode/control-bearing values; verify aligned/stacked boundaries, complete identity inventory, compact active field/error blocks, scrollbar geometry, restored state, and the 100 ms target.

### Tests for User Story 3

> Write these tests first and ensure they fail before implementation.

- [X] T022 [P] [US3] Add failing local-width boundary tests for aligned identity rows at eight available value cells, stacked label/value pairs below eight, two-space stacked-value indentation, no omitted fields, and compact interactive rows at 40x12 in `internal/tui/responsive_layout_test.go` and `internal/tui/connection_form_test.go`
- [X] T023 [P] [US3] Add failing 20-run `40->60->79->80->100->160->80->79->40` tests at heights 12/24 plus `40x12->39x11->40x12`, asserting unchanged selection, focus, errors, modal payload, viewport offset, controls, and undersized behavior in `internal/tui/resize_state_test.go` and `internal/tui/undersized_test.go`
- [X] T024 [P] [US3] Add failing SC-005 tables with 100 long/Unicode/control-bearing field cases across Details, forms, confirmations, Help, errors, trust, and secret surfaces, asserting local width/height bounds, no overlap, complete required identity, safe ellipsis/wrapping, and no secret canaries in `internal/tui/overflow_test.go` and `internal/tui/model_fuzz_test.go`
- [X] T025 [P] [US3] Extend the 1,100-node 20-sample acceptance fixture with Help open/render and connection-confirmation open/render operations, requiring at least 19 samples per operation at or below 100 ms in `internal/tui/performance_acceptance_test.go`

### Implementation for User Story 3

- [X] T026 [US3] Complete local-width responsive projection so non-interactive identity groups align with at least eight value cells or stack every label/value pair, while interactive forms keep one compact field row plus visible error and modal controls retain leading/fixed row priority in `internal/tui/detail.go`, `internal/tui/modal.go`, and `internal/tui/connection_form.go`
- [X] T027 [US3] Reconcile browser/modal projection and scrollbar track offsets after stacked rows so resize and undersized restoration preserve logical owners and every frame remains bounded in `internal/tui/model.go` and `internal/tui/viewport.go`

**Checkpoint**: All stories satisfy the minimum-size, overflow, state-restoration, safety, and local-render performance contracts.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Align documentation, gather perceptual acceptance evidence, and execute every repository quality gate

- [X] T028 [P] Document bracketed type badges, colonless secondary labels, aligned/stacked read-only identity rows, compact forms, and color/no-color semantics in `README.md`
- [X] T029 [P] Reconcile actual focused test names, expected outputs, and any platform-only limitations with the runnable commands in `specs/008-unify-panel-style/quickstart.md`
- [X] T030 Execute the complete focused, full, race, vet, build, staticcheck, pinned vulnerability, responsive acceptance, performance, and Nix commands from `specs/008-unify-panel-style/quickstart.md`, recording concrete limitations in `specs/008-unify-panel-style/quickstart.md`
- [ ] T031 Conduct the 20-participant color/no-color 80x24 timed-recognition protocol from `specs/008-unify-panel-style/quickstart.md` and record anonymized frame-level timings and the 19/20 pass decision in `specs/008-unify-panel-style/recognition-results.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; starts immediately.
- **Foundational (Phase 2)**: Depends on T001 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Foundation and is the recommended MVP.
- **US2 (Phase 4)**: Depends on Foundation; it can be implemented without US1, although both consume the same presentation primitives.
- **US3 (Phase 5)**: Depends on US1 and US2 because it validates responsive composition across all implemented surfaces.
- **Polish (Phase 6)**: Depends on all selected user stories; T030 follows T028/T029 and T031 follows the final visual implementation.

### User Story Dependency Graph

```text
Setup -> Foundation -> US1 (P1, MVP) --+
                    -> US2 (P2) -------+-> US3 (P3) -> Polish
```

### User Story Testability

- **US1**: Independently proves Root/Folder/Connection content badges, connection identity alignment, no `Kind` row, color/no-color equivalence, safe dynamic text, and no secret projection in Details.
- **US2**: Independently proves badges and colonless structured labels across forms, Actions, Help, modals, errors, conflicts, trust, and secret prompts while all controls and security semantics remain unchanged.
- **US3**: Independently proves the eight-cell aligned/stacked identity threshold, compact interactive rows, full responsive/undersized restoration, bounded Unicode output, and 100 ms local rendering after US1/US2 surfaces exist.

### Within Each User Story

- Write the story's tests and observe failures before production changes.
- Implement shared or local rendering before model-level integration.
- Preserve current action inventories, focus owners, active-line mappings, captured targets, and secret boundaries.
- Pass the independent checkpoint before moving to the next priority in a single-developer flow.

### Parallel Opportunities

- Foundation tests T002/T003 can run in parallel; implementations T004/T005 affect separate production files after their corresponding tests fail.
- US1 tests T006-T008 modify separate primary test files and can run in parallel.
- US2 tests T011-T015 can run in parallel; implementations T016-T019 affect separate production surfaces and can run in parallel before T020/T021 integration.
- US3 tests T022-T025 can run in parallel before T026/T027 integrate responsive behavior.
- Documentation tasks T028/T029 can run in parallel after visual behavior stabilizes.

---

## Parallel Example: User Story 1

```text
Task: "T006 Add the 30-case Details badge and alignment matrix in internal/tui/detail_test.go"
Task: "T007 Add public Details selection and no-color flows in tests/integration/tui_tree_test.go"
Task: "T008 Add Details accessibility and secret-boundary regressions in internal/tui/accessibility_test.go and internal/tui/embedded_form_security_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T011 Add every-modal visual contract test in internal/tui/modal_test.go and internal/tui/modal_layout_test.go"
Task: "T012 Add safe connection-form badge, compact-row, conflict, and integration tests in internal/tui/connection_form_test.go, internal/tui/form_conflict_test.go, and tests/integration/tui_connection_test.go"
Task: "T013 Add folder badge-reuse and move-picker hierarchy tests in internal/tui/folder_test.go and internal/tui/modal_conformance_test.go"
Task: "T014 Add trust and secret prompt visual/security tests in internal/tui/trust_prompt_test.go, internal/tui/secret_prompt_test.go, and internal/tui/security_input_test.go"
Task: "T015 Add Actions/Help inventory and captured-target tests in internal/tui/actions_test.go and tests/integration/tui_actions_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T022 Add aligned/stacked boundary and compact-form tests in internal/tui/responsive_layout_test.go and internal/tui/connection_form_test.go"
Task: "T023 Add repeated resize and undersized restoration tests in internal/tui/resize_state_test.go and internal/tui/undersized_test.go"
Task: "T024 Add the 100-case Unicode, bounds, and secret matrix in internal/tui/overflow_test.go and internal/tui/model_fuzz_test.go"
Task: "T025 Add Help and confirmation render samples in internal/tui/performance_acceptance_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001-T005.
2. Write and fail US1 tests T006-T008.
3. Implement T009-T010.
4. Run the focused Details, accessibility, security, and integration tests.
5. Stop with a demonstrable Details panel that uses content badges and aligned colonless identity rows in color and no-color modes.

### Incremental Delivery

1. **Foundation**: Semantic badges, descriptive labels, field layout, and label-independent bounded projection.
2. **US1**: Details visual hierarchy delivers the MVP.
3. **US2**: Forms, Actions, Help, modals, errors, and security prompts adopt the same hierarchy.
4. **US3**: Responsive stacking, compact form behavior, overflow, restoration, and performance complete terminal quality.
5. **Polish**: Documentation, complete quality gates, and timed-recognition evidence prepare the feature for merge.

### Parallel Team Strategy

1. Complete Setup and Foundation together.
2. After Foundation, one developer can implement US1 while another implements US2 using the stable primitives.
3. Start US3 only after both surface groups are present, while preparing its independent tests in parallel.
4. Integrate through full responsive, security, performance, and repository gates before recognition acceptance.

---

## Notes

- `[P]` means the task can proceed without editing a file owned by another incomplete task.
- `[US1]`, `[US2]`, and `[US3]` provide direct traceability to the specification.
- Automated test tasks precede production changes; T031 is the explicit manual evidence gate for SC-003.
- Do not add a theme dependency, cache, retained frame history, new action, mouse behavior, persistent format, SSH behavior, or credential behavior.
- Use only synthetic non-secret fixtures; no real SSH server, private key, password, or native credential store is required.
- Stop at each story checkpoint and validate that increment independently.
