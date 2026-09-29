# Presentation Model: Hide Connection Actions

This feature introduces no persisted domain data or new external entities. It changes the presentation contract of existing transient TUI state.

## Existing Presentation State

### Layout State

- **Tree region**: Catalog navigation area.
- **Details region**: Selected-item, form, status, or trust information area.
- **Lower control region**: Fixed five-row region used by form, operation, conflict, recovery, and error states.
- **Action legend**: Borderless key-and-label projection of actions applicable to the current state.
- **Actions panel**: Deprecated titled and bordered projection that this feature removes.

### Action Context

- **Current state**: Normal browsing, form editing, operation, conflict, modal, startup, recovery, or error state.
- **Applicable descriptors**: Existing key-and-label actions valid for the current state.
- **Status**: Optional operation or recovery message displayed with the applicable controls.

## Invariants

- The lower control region keeps its fixed geometry where it is currently reserved.
- The Actions panel title and border are absent in every user-visible state.
- The action legend uses the existing applicable descriptors and therefore remains aligned with keyboard dispatch.
- No catalog, connection, credential, host-trust, or session data is added, modified, or persisted by this feature.
