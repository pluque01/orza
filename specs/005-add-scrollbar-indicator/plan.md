# Implementation Plan: Indicador de barra de desplazamiento

**Branch**: `005-add-scrollbar-indicator` | **Date**: 2026-08-21 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/005-add-scrollbar-indicator/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Replace row-consuming `↑ more`/`↓ more` indicators in every existing vertically scrollable TUI surface with a proportional, one-column, keyboard-neutral scrollbar adjacent to the container's right border. Extend the existing pure viewport projection with effective-range and scrollbar metadata, replace the existing right-padding cell only while overflow exists, and compose tracks only across scrollable body rows so fixed notices and modal recovery controls remain outside the track. Keep Actions non-scrollable and replace its omission cue exactly with `Hidden actions — ? Help`. No dependency, persistence, mouse support, or keyboard binding is added.

## Technical Context

**Language/Version**: Go 1.26 (`toolchain go1.26.5`)

**Primary Dependencies**: Bubble Tea v2, Lip Gloss v2, Charmbracelet ANSI display-width utilities; existing custom viewport renderer

**Storage**: N/A; scrollbar state is derived in memory and is not persisted

**Testing**: Go `testing`, table-driven unit tests, model/view integration tests, fuzz tests, race tests, and build-tagged acceptance tests

**Target Platform**: VT-capable terminals on Linux, macOS, and Windows; amd64 and arm64

**Project Type**: Local keyboard-first CLI/TUI application

**Performance Goals**: Preserve the existing complete-frame targets of at least 19/20 local navigation, focus, scroll, and resize updates within 100 ms for a catalog of 1,000 connections, 100 folders, and depth 10

**Constraints**: One interior display cell only while overflowing; exact right-edge placement; no mouse input or new keys; no color-only semantics; bounded output; preserve logical offsets, active visibility, Unicode-safe truncation, fixed recovery controls, and the 40x12 minimum layout

**Scale/Scope**: Closed surface inventory of Tree, Details, connection form, Help body, move-picker body, confirmation bodies, and recoverable-error bodies across twelve existing responsive sizes; Actions remains non-scrollable

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

| Principle / Constraint | Evaluation | Result |
|------------------------|------------|--------|
| I. Secure SSH Operations | The feature renders only existing non-secret content and does not alter SSH, trust, credentials, logging, or persistence. Tests must continue to use synthetic non-secret fixtures. | PASS |
| II. Terminal-First Usability | Existing keyboard controls remain authoritative; the indicator receives no focus or input, works without color, adapts from 40x12 upward, and does not obscure cancel/back/quit controls. | PASS |
| III. Predictable State and Failure Handling | Scrollbar geometry is pure derived state. Logical offset, focus, selection, forms, modal ownership, operations, and undersized restoration remain unchanged. | PASS |
| IV. Behavior-Focused Testing | The plan requires formula, viewport, cross-surface, resize, no-color, Unicode, fuzz, performance, and regression coverage, including marker removal. | PASS |
| V. Simplicity and Maintainability | The existing shared custom viewport and style set are extended; no dependency, persistent state, concurrency, or speculative abstraction is introduced. | PASS |
| Bounded output and terminal support | The bar uses one validated display cell inside existing bounds; wrapped content is regenerated at the reduced width and frames remain bounded. | PASS |
| Documentation and quality gates | README and superseded overflow contracts will be updated; formatting, tests, race, vet, static analysis, vulnerability scan, and builds remain required before merge. | PASS |

No constitutional violations or complexity exceptions are required.

### Post-Design Gate

Phase 1 preserves every pre-research result and incorporates the approved specification: tracks cover only scrollable body rows, Actions remains non-scrollable, half-up rounding and short-track quantization are normative, and the closed surface/resize/geometry matrices are explicit. The data model contains only transient projection values, the UI contract preserves keyboard-only control and fixed safety rows, and the quickstart includes routine, participant, performance, and acceptance validation. No security boundary, lifecycle owner, persistent format, new dependency, or platform capability is introduced. **Result: PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/005-add-scrollbar-indicator/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/
│   └── tui-scrollbar.md  # Phase 1 visual and interaction contract
├── validation/
│   └── sc006.md          # Pre-merge participant evidence created during implementation
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/tui/
├── viewport.go                    # Shared range projection and proportional geometry
├── viewport_test.go               # Formula, clamping, width, and short-track tests
├── browser.go                     # Tree source lines and section projection
├── detail.go                      # Width-sensitive Details projection
├── connection_form.go             # Focus-block projection and conditional width
├── modal.go                       # Scrollable body plus fixed modal controls
├── model.go                       # Panel section composition and right-edge rendering
├── styles.go                      # Track/thumb styles and textual glyphs
├── actions.go                     # Non-scrollable hidden-actions cue without `more`
├── overflow_test.go               # Cross-surface scrollbar contract
├── sc007_viewport_acceptance_test.go
├── sc007_full_acceptance_test.go  # Build-tagged scale acceptance
├── resize_state_test.go
├── ui_conformance_test.go
└── model_fuzz_test.go

tests/integration/
└── tui_tree_test.go               # End-to-end keyboard navigation regression

README.md                          # User-facing overflow/no-color behavior
```

**Structure Decision**: Preserve the single Go module and existing `internal/tui` ownership. `viewport.go` owns pure projection and geometry; source renderers own width-sensitive content generation; `model.go` composes projected sections into existing layout rectangles; `styles.go` owns presentation. Existing unit and acceptance suites are adapted in place rather than creating a parallel scrollbar package.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations require justification.

## Implementation Sequence

1. Replace marker-row budgeting with full-row projection and add overflow-safe scrollbar geometry derived from the effective range.
2. Add a projected-section contract that identifies scrollable rows and carries geometry to the right-padding compositor.
3. Adapt Tree, Details, form active blocks, and modal bodies while preserving active-line/error/control visibility and fixed rows.
4. Render track/thumb cells adjacent to the right border through the existing panel and overlay composition, including no-color semantics.
5. Remove all visible directional `more` text, rename the non-scrollable Actions omission cue, and remove the final-frame marker fallback.
6. Replace marker assertions with geometry/right-edge assertions and run responsive, keyboard, resize, Unicode, no-color, fuzz, performance, and acceptance regressions.
7. Update README and prior overflow references that describe the superseded marker behavior.
