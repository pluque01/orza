# Implementation Plan: Estilo visual unificado de paneles

**Branch**: `008-unify-panel-style` | **Date**: 2026-09-11 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/008-unify-panel-style/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Introduce one semantic visual hierarchy across Orza's existing TUI surfaces. Replace colon-bearing `Kind` rows with bounded textual type badges, style existing region and modal titles as badges rather than duplicating them, and render structured labels as muted colonless text beside aligned values. Details uses one field per row with a shared value column and stacks non-interactive identity labels above their values only when the local content width cannot preserve a useful value column. Interactive forms keep their compact active field/error block at supported sizes. Apply the same hierarchy to forms, confirmations, Help, Actions, recoverable errors, conflict details, and security prompts while retaining every current focus marker, control, viewport, scrollbar, secret boundary, and outer panel dimension.

## Technical Context

**Language/Version**: Go 1.26 (`toolchain go1.26.6`)

**Primary Dependencies**: Bubble Tea v2.0.8, Bubbles v2.1.1, Lip Gloss v2.0.5, and Charmbracelet ANSI v0.11.7; existing custom panel, modal, form, safe-text, and viewport renderers

**Storage**: N/A for this feature; existing SQLite catalog and native credential stores are unchanged

**Testing**: Go `testing`; table-driven renderer tests, exact visible-text assertions, ANSI-stripped color/no-color equivalence, width/height invariants, model conformance tests, integration tests, fuzz tests, race tests, and build-tagged responsive acceptance tests

**Target Platform**: VT-capable terminals on Linux, macOS, and Windows; amd64 and arm64

**Project Type**: Local keyboard-first CLI/TUI application

**Performance Goals**: At least 19 of 20 selection, focus, Help, confirmation, and resize updates, including rendering, complete within 100 ms on the existing 1,100-node synthetic catalog

**Constraints**: Usable from 40x12; complete layout at 80x24; no meaning conveyed by color alone; output bounded by local panel geometry; display-width-aware alignment for Unicode and ANSI; no altered keys, focus order, confirmations, secret projection, persistent data, SSH behavior, or outer panel geometry

**Scale/Scope**: Three browser regions; Root, Folder, and Connection detail targets; one connection form; ten registered modal kinds; folder forms, move picker, trust and secret prompts, operation/conflict states, color and no-color modes, and the established responsive size matrix

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

| Principle / Constraint | Evaluation | Result |
|------------------------|------------|--------|
| I. Secure SSH Operations | Rendering remains presentation-only. Dynamic values continue through safe-text boundaries; credential references and secret bytes remain excluded or masked. Host-trust decisions and warnings are unchanged. | PASS |
| II. Terminal-First Usability | Existing keyboard ownership, cancel/back/exit controls, focus markers, 40x12 minimum, and color-independent cues remain normative. Narrow non-interactive identity fields stack instead of hiding data; interactive field/error blocks stay compact. | PASS |
| III. Predictable State and Failure Handling | The change adds no lifecycle state or asynchronous work. Existing viewport offsets, active rows, modal payloads, operation states, and recovery controls remain owned by their current components. | PASS |
| IV. Behavior-Focused Testing | The plan updates exact renderer contracts and covers badges, alignment, stacking, all structured surfaces, no-color equivalence, Unicode bounds, secrets, overflow, focus, resize, Help/confirmation performance, and the timed recognition protocol. | PASS |
| V. Simplicity and Maintainability | A small set of semantic style and structured-row helpers replaces repeated label punctuation and styling. No new package, dependency, persistent state, background work, or speculative theme system is introduced. | PASS |
| Security and operational constraints | Untrusted paths, hosts, names, diagnostics, and remote values remain sanitized before styling. Output stays bounded and no command, environment, filesystem, protocol, or credential integration changes. | PASS |
| Documentation and quality gates | User-visible README text and controlled-English inventory are updated with the implementation. Formatting, tests, race detection, vet, build, static analysis, vulnerability checks, and Nix validation remain required. | PASS |

No constitutional violations or complexity exceptions are required.

### Post-Design Gate

Phase 1 preserves every pre-research result. The data model contains only transient display descriptors derived from existing state; the TUI contract fixes textual badge semantics, title reuse, colonless descriptive labels, status-prefix preservation, read-only aligned/stacked layouts, compact interactive fields, safe dynamic-value handling, viewport behavior, and the complete surface inventory. The quickstart validates principal visual, responsive, failure, secret-safety, performance, and timed-recognition paths without a real SSH target or credential. No storage, network, authentication, lifecycle, dependency, or platform contract changes. **Result: PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/008-unify-panel-style/
├── plan.md                          # This file (/speckit.plan output)
├── research.md                      # Phase 0 decisions
├── data-model.md                    # Phase 1 transient presentation model
├── quickstart.md                    # Phase 1 validation guide
├── contracts/
│   └── panel-visual-hierarchy.md    # Badge, field, surface, and responsive TUI contract
├── checklists/
│   └── requirements.md              # Specification quality checklist
└── tasks.md                         # Phase 2 output (/speckit.tasks; not created here)
```

### Source Code (repository root)

```text
internal/tui/
├── styles.go                        # Badge, muted-label, value, warning, error, and marker semantics
├── detail.go                        # Type badge and aligned/stacked Details fields
├── model.go                         # Region title composition, embedded details surfaces, and conflict rows
├── modal.go                         # Modal title badges and structured confirmation/error/help rows
├── connection_form.go               # Colonless labels and compact responsive interactive rows
├── folder_form.go                   # Structured folder target and input rows
├── move_picker.go                   # Structured source identity and picker heading
├── trust_prompt.go                  # Host-identity badge and structured verification fields
├── secret_prompt.go                 # Secret-prompt badge and colonless masked field label
├── viewport.go                      # Label-independent bounded projection
├── styles_test.go                   # Badge/label/value and color/no-color semantics
├── detail_test.go                   # Connection/folder/root badge and field-layout behavior
├── connection_form_test.go          # Interactive value alignment and active block preservation
├── modal_test.go                    # Modal badge, structured-row, and duplicate-title contracts
├── modal_layout_test.go             # Fixed controls, viewport, scrollbar, and overlay geometry
├── accessibility_test.go            # Textual cues without color
├── overflow_test.go                 # Bounded output for every structured surface
├── responsive_layout_test.go        # 40/60/79/80/100/160 layout matrix
├── resize_state_test.go             # State preservation across resize/undersized recovery
├── performance_acceptance_test.go   # Existing fixture plus Help/confirmation render samples
└── testdata/
    └── controlled_english.txt        # Canonical changed and added interface text

tests/integration/
├── tui_actions_test.go              # Actions and Help behavior remains unchanged
└── tui_connection_test.go           # Connection form and confirmation visible contract

README.md                            # User-visible panel hierarchy and no-color behavior
```

**Structure Decision**: Preserve the single Go module and existing `internal/tui` ownership. Add semantic rendering primitives beside current styles rather than introducing a component package. Each current surface keeps responsibility for focus, active-line mapping, wrapping, viewport, and security behavior while consuming the shared badge and structured-label conventions. `layout.go`, app/domain models, repositories, SSH services, and credential adapters remain unchanged.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations require justification.

## Implementation Sequence

1. Add color/no-color-safe badge, secondary-label, and structured-field layout primitives with display-width tests and no new dependency.
2. Replace Details `Kind` fields with Root/Folder/Connection badges; align one field per row and stack non-interactive identity label/value pairs only when the local width cannot retain a useful value column.
3. Style browser and modal titles as badges, remove duplicate payload headings, and retain separate content badges only where title and content types differ.
4. Apply colonless descriptive labels and shared value alignment to connection/folder forms, move picker, confirmations, Help, errors, conflict detail, trust prompt, and secret prompt without changing status prefixes, active-line blocks, or control ownership.
5. Replace viewport's literal colon-prefix handling with label-independent bounded projection while preserving full wrapped confirmation targets and existing scrollbar priorities.
6. Update controlled-English and README documentation, then adapt renderer, conformance, integration, accessibility, overflow, resize, Unicode, secret-canary, and performance tests, including Help and confirmation render samples.
7. Run formatting, focused tests, the complete and race suites, vet, build, static analysis, vulnerability checks, and the Nix flake gate.
