# Implementation Plan: Simplificar etiquetas de panel

**Branch**: `010-simplify-panel-labels` | **Date**: 2026-09-17 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/010-simplify-panel-labels/spec.md`

## Summary

Render structural titles as plain text after the existing `[*]` or `[ ]` focus marker. Render existing context labels as bold text without a foreground or background color. Keep the no-color text equivalent and retain all current TUI state, layout, controls, safe-text handling, and panel ownership.

## Technical Context

**Language/Version**: Go 1.26 (`toolchain go1.26.6`)

**Primary Dependencies**: Bubble Tea v2.0.8, Lip Gloss v2.0.5, and Charmbracelet ANSI v0.11.7

**Storage**: N/A; existing catalog and credential storage remain unchanged

**Testing**: Go `testing`; renderer, TUI conformance, accessibility, responsive-layout, modal, and integration tests

**Target Platform**: VT-capable terminals on Linux, macOS, and Windows

**Project Type**: Local keyboard-first CLI/TUI application

**Performance Goals**: Preserve existing render performance for selection, focus, modal, and resize updates

**Constraints**: Usable from 40x12; meaning cannot depend on color or bold support; no changed keys, focus order, geometry, SSH behavior, secrets, or persistence

**Scale/Scope**: Shared style renderer and every existing structural-title or context-label surface; no new state or dependencies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

| Principle / Constraint | Evaluation | Result |
|------------------------|------------|--------|
| I. Secure SSH Operations | Presentation-only change; safe text, secrets, host trust, and connection behavior remain unchanged. | PASS |
| II. Terminal-First Usability | Existing keyboard controls, focus markers, minimum size, and no-color textual cues are retained. | PASS |
| III. Predictable State and Failure Handling | No state, lifecycle, error ownership, or asynchronous work changes. | PASS |
| IV. Behavior-Focused Testing | Renderer, no-color, modal, responsive, conformance, and integration contracts cover the revised text and style behavior. | PASS |
| V. Simplicity and Maintainability | One shared style rule changes existing consumers; no package, dependency, persistent state, or theme system is added. | PASS |

No constitutional violations or complexity exceptions are required.

### Post-Design Gate

The transient title and context descriptors retain their existing ownership. The TUI contract fixes plain structural-title text, preserved focus markers, bold colorless context labels, no-color parity, safe dynamic text, and bounded layout. No security, storage, network, lifecycle, dependency, or platform change is introduced. **Result: PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/010-simplify-panel-labels/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/title-context-text.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)
```text
internal/tui/
├── styles.go
├── model.go
├── modal.go
├── detail.go
├── connection_form.go
├── trust_prompt.go
├── secret_prompt.go
├── styles_test.go
├── panel_style_test.go
├── modal_test.go
├── detail_test.go
├── accessibility_test.go
└── responsive_layout_test.go

tests/integration/
├── tui_connection_test.go
└── tui_tree_test.go

README.md
```

**Structure Decision**: Preserve `internal/tui` ownership and adjust the existing shared renderer. Current callers inherit the presentation change without component-specific state or layout changes.

## Complexity Tracking

No violations require justification.

## Implementation Sequence

1. Update shared title and context rendering semantics and no-color output in `internal/tui/styles.go`.
2. Update renderer, modal, detail, accessibility, responsive, conformance, and integration contracts.
3. Update README visual-convention documentation.
4. Run formatting, focused TUI tests, full tests, race detection, vet, and build.
