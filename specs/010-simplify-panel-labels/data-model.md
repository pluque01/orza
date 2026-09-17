# Data Model: Text Presentation

No persistent entities are introduced. Existing transient descriptors retain their current fields.

## Structural Title

| Field | Rule |
|-------|------|
| `label` | Controlled panel or modal name rendered without brackets |
| `active` | Existing focus ownership rendered by the preceding `[*]` or `[ ]` marker |

## Context Label

| Field | Rule |
|-------|------|
| `label` | Controlled type name rendered without brackets or color fill |
| `accented` | Uses bold only when styling is available; text remains unchanged otherwise |

**State transitions**: None. Existing rendering recalculates descriptors on focus, selection, modal, resize, and color-capability changes.
