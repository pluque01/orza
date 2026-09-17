# Data Model: Title and Context Presentation

This feature does not add persisted entities. It refines transient display descriptors derived from existing TUI state.

## Structural Title

| Field | Meaning | Validation |
|-------|---------|------------|
| `label` | Controlled name of a window, browser region, or modal | Non-empty, safe controlled text; rendered between `[` and `]` |
| `active` | Whether the owning region has focus | Existing focus ownership only; affects the existing `[*]` or `[ ]` marker |
| `placement` | Structural title classification | Must use the structural-title rendering rule |

**Relationships**: A structural title belongs to one existing region or modal. It is independent of the selected content type.

## Context Label

| Field | Meaning | Validation |
|-------|---------|------------|
| `label` | Controlled type of the displayed target or prompt | Non-empty; must not contain `[` or `]`; rendered without brackets |
| `placement` | Context-label classification | Must use the contextual-label rendering rule |
| `accented` | Whether the terminal supports color styling | Color mode uses the approved contrast-safe foreground/background pair; no-color mode emits the text only |

**Relationships**: A context label describes the existing target or form/prompt content shown within a structural panel. At most one is rendered where the current surface already exposes one.

## Palette Rule

| Field | Meaning | Validation |
|-------|---------|------------|
| `foreground` | Context-label text color | Has at least 4.5:1 contrast against `background` |
| `background` | Context-label fill color | Dark enough to preserve the required contrast with the selected foreground |

**State transitions**: None. All descriptors are recalculated during existing rendering when focus, selection, target type, modal, terminal size, or color capability changes.
