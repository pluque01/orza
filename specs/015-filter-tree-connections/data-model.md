# Data Model: Filter Tree Connections

## Persistent Data

No persistent entity changes are required. The existing catalog snapshot remains authoritative and is not modified by searching.

## Transient Search State

### Search Session

| Field | Description | Validation / Rules |
|-------|-------------|--------------------|
| Query | Current text entered in the visible search field. | Empty input shows the natural tree without match highlighting. |
| Expression status | Whether the query is empty, valid, or invalid. | A valid non-empty query is interpreted as a case-sensitive regular expression. An invalid query leaves the unfiltered tree visible and reports an actionable error. |
| Matching connection IDs | Connection nodes whose names contain at least one match. | Folders and the synthetic root cannot be members. Ordering follows the existing tree order. |
| Match spans | All matched portions of each displayed matching connection name. | Spans identify name characters to distinguish in the rendered row. |
| Filtered rows | The temporary tree projection. | Contains each matching connection and every ancestor folder required to show its path; unrelated nodes are excluded. |
| Pre-search context | Selection ID, folder expansion map, viewport offset, and focus owner captured before opening search. | Restored unchanged on Escape. On Enter, the prior tree is restored and the accepted connection's ancestor folders are expanded. |

## Relationships

- A `Search Session` reads one existing `catalogSnapshot` and produces zero or more matching `Connection` nodes.
- Every visible matching `Connection` has zero or more visible ancestor `Folder` nodes.
- A `Folder` is contextual only and cannot be an active search result.
- One active result is either a matching connection ID or absent when no valid match exists.

## State Transitions

| From | Event | To | Result |
|------|-------|----|--------|
| Inactive | `/` while tree owns focus | Editing, empty query | Capture pre-search context and focus the input. |
| Editing | Query becomes valid and non-empty | Filtering | Rebuild the temporary projection, expand required ancestors, and select a matching connection if available. |
| Editing or Filtering | Query becomes empty | Editing, empty query | Show the natural tree without match highlighting. |
| Editing or Filtering | Query becomes invalid | Invalid query | Preserve natural tree and selection; show validation feedback. |
| Filtering | Up, Down, Ctrl+P, or Ctrl+N | Filtering | Move only among matching connections and clamp at either end. |
| Editing, Filtering, or Invalid query | Escape | Inactive | Restore the exact pre-search context. |
| Filtering with active match | Enter | Inactive | Restore the natural tree, expand the accepted connection's ancestor folders, select it, and perform no connection action. |

## Invariants

- Searching does not persist, mutate, connect to, create, edit, move, or delete a connection or folder.
- The temporary expansion state can only add ancestor folders needed to reveal a match.
- The accepted result must still identify a connection in the loaded snapshot before it is focused.
