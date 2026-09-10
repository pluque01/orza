# Data Model: Selección directa del método de autenticación

This feature changes no persistent schema or external data format. It introduces typed transient form state and maps it to the existing connection request model.

## Authentication Option

Represents one member of the closed, ordered selector inventory.

| Field | Type | Rules |
|-------|------|-------|
| `label` | controlled text | Exactly `Agent`, `Key`, or `Password`; displayed in English and never user-editable. |
| `method` | existing authentication enum | Exactly `agent`, `key`, or `password`; unique within the inventory. |
| `order` | integer | `Agent = 0`, `Key = 1`, `Password = 2`; immutable for this feature. |

### Invariants

- The inventory contains exactly three options.
- Labels and values have a one-to-one relationship.
- Option text is application-controlled, non-secret, and independent of catalog content.

## Authentication Selection

Represents the selector state owned by one active connection form.

| Field | Type | Rules |
|-------|------|-------|
| `selectedMethod` | Authentication Option method | Always references exactly one inventory member. |
| `baselineMethod` | Authentication Option method | `agent` for a new form; persisted connection method for an edit form. |
| `focused` | derived boolean | True exactly when the form's stable focus identifier is Method. |

### Validation

- New forms initialize `selectedMethod` and `baselineMethod` to `agent`.
- Edit forms initialize both values from the captured connection.
- Any method received from an external boundary remains subject to existing domain validation.
- Selector input cannot create a fourth value or an empty selection.

### Derived State

- `methodChanged = selectedMethod != baselineMethod`.
- The selected option is bracket-marked in every render.
- The outer form row marker, not the brackets, communicates focus.

## Dependent Draft State

Represents method-specific values retained during one form lifetime.

| Field | Type | Applicable method | Rules |
|-------|------|-------------------|-------|
| `identityFile` | bounded text draft | `key` | Visible and validated only for Key; retained unchanged while hidden. |
| `rememberPassword` | boolean draft | `password` | Visible only for Password; starts unchecked for new/edit forms and is retained while hidden. |

### Invariants

- Agent has no applicable dependent state.
- Key requires a non-empty valid identity path before save.
- Password may save an explicit remember intent; password bytes never enter this model.
- Hidden values may contribute to unsaved-form protection but cannot be validated or projected as configuration for another method.

## Connection Form Authentication State

Aggregates selector and dependent drafts under the existing form owner.

| Field | Type | Rules |
|-------|------|-------|
| `selection` | Authentication Selection | Exactly one per active form. |
| `dependentDraft` | Dependent Draft State | Exists for the form lifetime regardless of current visibility. |
| `focusField` | existing form-field identifier | Method remains one stable focus stop between User and the applicable dependent control or Save. |
| `errors` | existing field/form error map | Method cannot receive a user-generated invalid-value error; applicable domain/save errors remain actionable. |

### Relationships

- One connection form owns one Authentication Selection and one Dependent Draft State.
- One selected method determines zero or one visible dependent control.
- One final selected method maps to one existing create or update request.
- The model/operation owner, not selector state, owns any later secret prompt.

## State Transitions

### Open New Form

1. Create selector inventory.
2. Set selected and baseline method to Agent.
3. Initialize identity text empty and Remember unchecked.
4. Focus remains on Name under existing form rules.

### Open Edit Form

1. Capture the connection and revision under existing rules.
2. Set selected and baseline method to the persisted method.
3. Initialize Identity from persisted data when present; Remember remains an explicit unchecked draft choice.
4. Focus remains on Name.

### Select Previous or Next Method

1. Accept an unmodified physical Left or Right key only while Method is focused.
2. Move one position in the ordered inventory, wrapping at both ends.
3. Preserve both dependent draft values.
4. Recalculate visible fields immediately.
5. If a now-hidden dependent field held focus, transfer focus to Method.
6. Clear only stale validation/form errors governed by existing edit behavior.

### Receive Non-Navigation Input on Method

1. Consume printable, delete, backspace, shifted-arrow, and paste input.
2. Leave selection, dependent drafts, focus, and all other fields unchanged.
3. Preserve outer non-printable form controls such as Help, Save, Cancel, and Quit.

### Project Create Request

| Final method | Authentication value | Identity | Effective remember intent |
|--------------|----------------------|----------|---------------------------|
| Agent | `agent` | empty | false |
| Key | `key` | current visible Identity value | false |
| Password | `password` | empty | retained Remember value |

### Project Update Request

- Emit method only when it differs from the captured connection.
- Emit current Identity only for Key.
- Emit an explicit empty Identity when the original connection used Key and the final applicable identity differs, preserving existing stale-path clearing.
- Effective remember intent is true only for final Password plus checked Remember.
- Hidden-only drafts do not become repository fields or credential intent.

### Validation or Persistence Failure

1. Preserve selected method, baseline, dependent drafts, focus, target, and captured revision.
2. Display the existing field-local or form-level actionable error.
3. Permit correction, retry, cancellation, Help, and safe exit through existing controls.

### Cancel or Discard

1. Discard selector changes and both dependent drafts with the rest of the form.
2. Restore the prior selection/focus owner or exit according to the existing unsaved-change decision.

### Resize and Undersized Recovery

1. Preserve selection, baseline, dependent drafts, focus, errors, and logical viewport offset.
2. Below 40x12, render only the existing undersized surface.
3. On return to a supported size, render the same selected method and applicable control before accepting the next input.
