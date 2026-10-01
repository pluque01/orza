# Tree Search Interaction Contract

## Scope

This contract defines the keyboard and visible-state behavior of the connection-tree search. It is an internal TUI interaction contract; it adds no command-line, network, or storage interface.

## Entry

| Precondition | Input | Required result |
|--------------|-------|-----------------|
| Tree panel owns focus and no modal, operation, form, or security prompt preempts input. | `/` | Open a visible search field, capture tree context, and give the field keyboard focus. |
| Any other focus owner or preempting state. | `/` | Do not start tree search. |

## Query Evaluation

| Query state | Required tree content | Required feedback |
|-------------|-----------------------|-------------------|
| Empty | Natural tree | No match highlighting. |
| Valid with matches | Matching connection rows and their complete parent-folder paths | Highlight every matched name portion; retain the normal active-row indicator. |
| Valid with no matches | No connection result rows or standalone folders | Visible no-results message. |
| Invalid regular expression | Natural tree and its current selection | Visible actionable expression error; correction and Escape remain available. |

## Navigation

| Input | Required result |
|-------|-----------------|
| Up Arrow or Ctrl+P | Move to the previous matching connection; remain at the first result if already first. |
| Down Arrow or Ctrl+N | Move to the next matching connection; remain at the last result if already last. |
| Enter with an active matching connection | Close search, restore the natural tree, expand every folder in that connection's path, focus it, and start no action. |
| Escape | Close search and restore the captured selection, expansion, viewport, and focus owner. |

## Presentation Rules

- Ancestor folders needed to reveal a matching connection are temporarily expanded during filtering.
- Folders are contextual and cannot become search results or the active result.
- Match highlighting uses the established differentiated style in color mode; the selected result keeps its existing non-color marker and does not rely on color alone.
- All content continues through the existing safe-text and viewport projection so narrow terminals and long names remain bounded.
