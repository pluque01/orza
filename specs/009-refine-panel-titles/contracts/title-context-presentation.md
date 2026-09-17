# TUI Contract: Title and Context Presentation

## Structural Titles

- Every existing browser-region and modal structural title renders as `[Label]`.
- Structural titles do not have a colored background in any color-capable terminal.
- Existing focus ownership remains visible through the current `[*]` and `[ ]` markers.
- `Tree`, `Details`, `Actions`, and the existing modal title inventory use this rule.

## Context Labels

- Existing context/type labels render as controlled text without `[` or `]`.
- `Connection`, `Folder`, and `Root` use the context-label rule when their current surface renders a type label.
- Existing form and prompt context labels use the same rule; no new labels are introduced.
- In color-capable terminals, context labels use an approved text/background combination with at least 4.5:1 contrast.
- In no-color terminals, context labels render their full text without ANSI styling and retain the same meaning.

## Invariants

- Dynamic values remain safe-text projected and bounded by current panel layout behavior.
- The change does not alter keyboard actions, focus order, modal controls, confirmation flows, scrollbars, panel dimensions, connection behavior, or secret masking.
- When terminal width is limited, existing ANSI-aware truncation and panel-bound rules remain authoritative.

## Verification

- Renderer tests assert structural brackets and absence of title backgrounds.
- Context-label tests assert no brackets, one label where already present, and color/no-color text equivalence after ANSI stripping.
- Accessibility and responsive tests assert the textual cue and bounded rendering.
