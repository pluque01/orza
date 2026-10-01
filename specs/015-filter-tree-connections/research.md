# Research: Filter Tree Connections

## Decision: Keep filtering entirely in the TUI catalog snapshot

**Rationale**: `browserModel` already owns a complete `catalogSnapshot` with parent-child relationships and a deterministic sorted order. Deriving filtered rows from this snapshot avoids a new catalog query, avoids persistence changes, and keeps all folder-path context available.

**Alternatives considered**:

- Querying the catalog service for each keystroke: rejected because it adds asynchronous failure states and latency to a transient local interaction.
- Replacing the snapshot with a filtered snapshot: rejected because the original tree state must be restored exactly on cancellation or acceptance.

## Decision: Capture and restore tree context at search boundaries

**Rationale**: The existing connection-edit flow already captures selected node, expansion map, viewport, and focus owner. The search lifecycle can apply the same pattern so collapsed ancestor folders are expanded only while filtering, Escape restores the prior state, and Enter expands the accepted connection's path.

**Alternatives considered**:

- Keep search-triggered folders expanded after exit: rejected by the accepted clarification.
- Reconstruct the previous state from visible rows: rejected because it cannot reliably preserve collapsed ancestors or the viewport.

## Decision: Compile the query when it changes and treat invalid input as non-filtering

**Rationale**: Standard-library regular expressions provide the requested syntax and partial-name matching without added dependencies. A compile error can be displayed beside the visible input while the unfiltered snapshot and selection remain stable.

**Alternatives considered**:

- Literal substring search only: rejected because regular expressions are required.
- Clear results for an invalid expression: rejected because it makes the existing tree disappear for a recoverable input error.

## Decision: Navigate only matching connection rows while search is active

**Rationale**: Folders remain structural context, not results. Restricting Up, Down, Ctrl+P, and Ctrl+N to matching connection IDs ensures Enter always has a connection to focus and cannot activate a folder.

**Alternatives considered**:

- Navigate all visible rows: rejected because it allows a structural folder to become the active result and makes Enter ambiguous.
- Flatten results: rejected because the specification requires the existing hierarchy.

## Decision: Render match spans before selected-row styling and viewport truncation

**Rationale**: Match highlighting must apply to each regular-expression match while the current selection remains distinguishable in both colored and no-color modes. Rendering styled name segments before the existing viewport projection preserves the current width, overflow, and ANSI-safe behavior.

**Alternatives considered**:

- Highlight the entire connection name: rejected because it does not identify the matching portion.
- Depend on color alone: rejected by the terminal accessibility requirement and the no-color test mode.
