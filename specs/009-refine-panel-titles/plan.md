# Implementation Plan: Refinar títulos de panel

**Branch**: `009-refine-panel-titles` | **Date**: 2026-09-17 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/009-refine-panel-titles/spec.md`

## Summary

Differentiate structural TUI titles from contextual type labels without changing any TUI behavior. Structural titles remain bracketed and use the existing active/inactive foreground treatment without a background. Context labels such as `Connection`, `Folder`, and `Root` omit brackets and retain the shared badge treatment with a dark-blue/white contrast-safe palette. The no-color path continues to render the same semantic text without ANSI styling.

## Technical Context

**Language/Version**: Go 1.26 (`toolchain go1.26.6`)

**Primary Dependencies**: Bubble Tea v2.0.8, Bubbles v2.1.1, Lip Gloss v2.0.5, and Charmbracelet ANSI v0.11.7

**Storage**: N/A; the existing SQLite catalog and native credential stores are unchanged

**Testing**: Go `testing`; renderer, no-color, accessibility, responsive-layout, modal-conformance, and detail tests

**Target Platform**: VT-capable terminals on Linux, macOS, and Windows; amd64 and arm64

**Project Type**: Local keyboard-first CLI/TUI application

**Performance Goals**: Preserve the existing target of at least 19 of 20 selection, focus, Help, confirmation, and resize updates completing within 100 ms on the 1,100-node synthetic catalog

**Constraints**: Usable from 40x12; color-independent semantics; context badge text/background contrast of at least 4.5:1; ANSI and Unicode display-width-safe output; no changed keys, focus order, confirmations, secret projection, persistent data, SSH behavior, or panel geometry

**Scale/Scope**: Shared style renderer, three browser-region titles, modal titles, and existing Root/Folder/Connection plus form and security-prompt context labels; no new state or surfaces

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

| Principle / Constraint | Evaluation | Result |
|------------------------|------------|--------|
| I. Secure SSH Operations | This is presentation-only. Existing safe-text projection, masked secrets, host-trust warnings, and connection behavior remain unchanged. | PASS |
| II. Terminal-First Usability | The keyboard model, focus marker, cancellation controls, 40x12 behavior, and text-first no-color rendering are retained. The contrast requirement directly improves readable terminal output. | PASS |
| III. Predictable State and Failure Handling | No lifecycle state, asynchronous work, error ownership, or recovery flow changes. | PASS |
| IV. Behavior-Focused Testing | Existing exact-rendering and no-color tests will be updated with regression coverage for structural titles, contextual labels, contrast palette selection, and layout bounds. | PASS |
| V. Simplicity and Maintainability | The change is confined to current shared style rendering and its consumers; it introduces no dependency, package, abstraction layer, persistence, or theme system. | PASS |
| Security and operational constraints | Dynamic values remain sanitized and bounded before styling. No command, filesystem, network, protocol, or credential behavior changes. | PASS |
| Documentation and quality gates | The controlled interface-text fixture and README will be reviewed and updated only if they describe the affected visual convention. Formatting, tests, race detection, vet, build, static analysis, vulnerability checks, and Nix validation remain required. | PASS |

No constitutional violations or complexity exceptions are required.

### Post-Design Gate

The design uses only transient presentation descriptors already derived from existing TUI state. The TUI contract preserves focus markers, no-color textual semantics, safe dynamic text, dimensions, and surface ownership while separating structural titles from contextual labels. No storage, network, authentication, lifecycle, dependency, or platform contract changes are introduced. **Result: PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/009-refine-panel-titles/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── title-context-presentation.md
├── checklists/
│   └── requirements.md
└── tasks.md                         # Phase 2 output; not created by this command
```

### Source Code (repository root)
```text
internal/tui/
├── styles.go                         # Shared title and context-label semantics
├── model.go                          # Tree, Details, and Actions region-title composition
├── modal.go                          # Modal structural-title composition
├── detail.go                         # Root, Folder, and Connection context labels
├── connection_form.go                # Existing connection form context label
├── trust_prompt.go                   # Existing trust context label
├── secret_prompt.go                  # Existing secret context label
├── styles_test.go                    # Shared render and color/no-color contracts
├── detail_test.go                    # Context-label behavior by detail target
├── panel_style_test.go               # Cross-surface title and label inventory
├── modal_test.go                     # Modal title presentation
├── modal_conformance_test.go         # Modal focus/no-color behavior
├── accessibility_test.go             # Textual cues without color
└── responsive_layout_test.go         # Bounded title and context placement

README.md                             # Visual convention documentation if user-facing text is affected
```

**Structure Decision**: Preserve the single Go module and existing `internal/tui` ownership. Change the current semantic badge renderer rather than adding a theme abstraction. Each surface retains its current state, focus, wrapping, viewport, and security responsibilities while using the adjusted shared title/context rendering.

## Complexity Tracking

No violations require justification.

## Implementation Sequence

1. Adjust shared title and context-label rendering so placement determines text syntax and background treatment; replace the current low-contrast badge palette with a documented contrast-safe pair.
2. Apply the structural-title contract through existing region and modal title paths, preserving `[Tree]`, `[Details]`, focus markers, sizing, and truncation behavior.
3. Apply the contextual-label contract through detail, connection-form, trust-prompt, and secret-prompt consumers, retaining one type label where current surfaces show one.
4. Update renderer, detail, panel, modal, accessibility, and responsive tests for bracket placement, color/no-color semantic equivalence, contrast palette, and bounded output.
5. Update user-facing documentation only where it describes the changed title/badge convention, then run the documented validation suite.
