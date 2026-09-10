# Phase 0 Research: Selección directa del método de autenticación

## Selector Ownership and Representation

**Decision**: Keep authentication selection inside `connectionForm` as a typed `app.AuthMethod` plus a typed baseline. Retain `fieldAuth` as the stable row/focus identifier, but stop using its generic text-input slot as the source of truth.

**Rationale**: A typed value guarantees exactly one valid method and removes cursor, selection, arbitrary-text, and parse-error states that do not belong to a closed selector. Keeping the existing field identifier preserves viewport projection, validation maps, and conditional focus order with a small change surface.

**Alternatives considered**:

- Keep the existing text input and reject edits: rejected because invalid method, cursor, and selection states remain representable and make input isolation fragile.
- Remove the indexed input slot and redesign all form storage: rejected because loops, snapshots, tests, and error handling would change without user value.
- Add a reusable selector package or third-party widget: rejected because there is one fixed three-option control and no popup, search, or reusable list behavior.

## Option Inventory and Rendering

**Decision**: Define one ordered inventory mapping display labels `Agent`, `Key`, and `Password` to existing lowercase `app.AuthMethod` values. Render all options on one row; enclose exactly the selected label in brackets while the existing `>` row marker continues to identify focus.

**Rationale**: Fixed inventory and order implement the closed domain directly. Brackets survive no-color rendering and distinguish selection from focus. The compact row fits the existing Details content width at the 40x12 minimum without truncating any option name.

**Alternatives considered**:

- Color-only highlighting: rejected because selection must remain clear in no-color mode.
- One focusable control per method: rejected because the specification requires one focus stop.
- Dropdown or modal list: rejected because all options must remain simultaneously visible and additional open/close state would complicate terminal operation.

## Keyboard and Paste Boundary

**Decision**: While Method owns focus, unmodified physical Left/Right changes the typed selection cyclically and synchronizes dependent controls immediately. Tab, Shift+Tab/F2, Ctrl+S, Esc, F1, and Ctrl+C retain their existing form-level behavior. All remaining printable, editing, shifted-arrow, and paste messages are consumed without mutation or command dispatch.

**Rationale**: This preserves the documented method-cycling controls while removing free-text input. Explicit consumption prevents pasted or printable command-like bytes from changing another field or escaping to browser actions.

**Alternatives considered**:

- Accept `h`/`l` aliases: rejected because they are printable input in form controls and current method cycling uses physical arrow keys only.
- Use Enter or Space for a second activation step: rejected because selection already changes directly and the approved specification requires no separate confirmation.
- Forward unknown input to the former text field: rejected because it reintroduces edit and paste behavior.

## Dependent State and Focus

**Decision**: Derive `Identity file` and `Remember password` visibility from the typed method. Retain their transient values when hidden; restore them if their method is selected again. If a transition hides the focused dependent control, move focus to Method in the same update.

**Rationale**: The behavior follows the approved clarification, prevents accidental in-form data loss, and preserves the current stable focus sequence. Hidden values remain local drafts and do not become selected-method configuration.

**Alternatives considered**:

- Clear dependent values on every method change: rejected by the approved clarification and because exploratory switching would destroy user input.
- Keep focus on an invisible row: rejected because keyboard ownership would become ambiguous.
- Show both dependent controls disabled: rejected because the specification requires only applicable controls to be visible.

## Validation, Dirty State, and Request Projection

**Decision**: Treat the typed selected method as always valid, retain domain validation as a defensive boundary, and compare it with a typed baseline for dirty detection. Hidden dependent edits continue to make the form dirty for cancel/quit protection, but create/update request projection validates and emits only data applicable to the final method. Preserve the existing explicit empty identity update when changing away from `Key`.

**Rationale**: Separating draft dirtiness from persisted projection prevents data loss warnings from disappearing while guaranteeing that hidden fields do not leak into catalog writes. Existing app/domain invariants remain the final authority.

**Alternatives considered**:

- Ignore hidden draft changes for dirty detection: rejected because cancel or quit could silently discard retained user input.
- Persist hidden identity data: rejected because non-key authentication forbids identity data.
- Remove domain validation because the UI is closed: rejected because non-TUI callers and defensive boundaries still require validation.

## Credential Intent Safety

**Decision**: Compute effective remember intent as selected method equals `Password` and the retained Remember control is checked. Only that effective intent may trigger password prompting or credential persistence; changing a remembered connection away from Password continues to use the existing credential-removal lifecycle.

**Rationale**: The current form retains the remember boolean even when hidden. Passing it unconditionally can request a password for Agent or Key, violating both FR-011 and the constitution's explicit-consent boundary. Gating at the form/model handoff keeps the retained draft behavior without changing credential services.

**Alternatives considered**:

- Clear Remember when leaving Password: rejected by the approved retention clarification.
- Let the application service reject inconsistent intent: rejected because it can still cause an unnecessary secret prompt before rejection.
- Change credential storage semantics: rejected because storage behavior is outside scope.

## Help, Actions, and Documentation

**Decision**: Add a contextual `Left/Right` method-change descriptor when Method owns focus, describe cyclic selection in form Help, and update README wording so the selector is excluded from text-field paste behavior. Keep controlled application text in English.

**Rationale**: Terminal-first controls must be discoverable where they apply. Documentation currently mentions cycling but broadly says all connection fields accept paste, which would contradict the new closed control.

**Alternatives considered**:

- Rely only on README: rejected because in-application contextual help is required.
- Show selector keys for every form field: rejected because Actions space is limited and the keys do not apply elsewhere.
- Add localization: rejected because the existing English-only contract remains in force.

## Test and Platform Strategy

**Decision**: Adapt existing package-local form tests and conformance suites, then add focused tests for initialization, exact rendering, all cyclic transitions, input isolation, dependent retention, create/update projection, hidden-remember gating, failure preservation, no-color output, resize, and undersized recovery. Use synthetic catalog data and no real SSH server or credential.

**Rationale**: The behavior is deterministic and can be covered at existing form, model, and app boundaries. Linux, macOS, and Windows share the TUI logic, while unchanged native credential implementations do not require platform-specific integration for this feature.

**Alternatives considered**:

- Manual TUI checks only: rejected by the behavior-focused testing principle.
- Snapshot-only tests: rejected because focused state assertions localize failures and prove request/secret boundaries that rendering snapshots cannot.
- Native credential-store tests for every platform: rejected because the storage integrations and protocol are unchanged; effective intent can be verified before that boundary.

## Dependency and Performance Decision

**Decision**: Keep Go 1.26.6 and the pinned Bubble Tea/Bubbles/Lip Gloss stack; add no production dependency. Render and move over the constant three-option inventory synchronously in the existing update cycle.

**Rationale**: The work is constant-size and uses current rendering and display-width facilities. It cannot materially affect catalog-scale performance and remains within the existing 100 ms local interaction target.

**Alternatives considered**:

- Add a component framework or selector dependency: rejected because it increases supply-chain and state complexity for a fixed control.
- Perform asynchronous selection updates: rejected because no I/O or expensive computation exists and same-frame dependency updates are required.
