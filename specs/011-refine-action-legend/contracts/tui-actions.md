# TUI Action Presentation Contract

## Scope

This is an internal user-interface contract. It does not add a network, command-line, file, or public library interface.

## Browser Legend

| Context | Required entries, in display order | Excluded entries |
|---------|------------------------------------|------------------|
| Browser with no connectable connection selected | Move Up, Move Down, New connection, New folder, Quit | All secondary actions and Connect |
| Browser with a connectable connection selected | Move Up, Move Down, Connect, New connection, New folder, Quit | All secondary actions |

Each entry consists of an emphasized key display and a muted action label. Adjacent entries have an unambiguous textual separator or gap even when terminal color is unavailable. The legend itself has no titled border or focusable region.

## Help Modal

- Includes every action applicable to the current selection and focus, including actions shown in the legend.
- Groups actions in this order: navigation, connection, management, application.
- Presents each action as an aligned key column and action-description column when the available width permits; uses the established narrow-width stacked fallback otherwise.
- Applies the same key and action visual treatments as the legend.
- Preserves the existing Help close control and scroll behavior.

## Behavioral Invariants

- An action omitted from the legend remains available through its existing key binding whenever it is contextually applicable.
- An unavailable action does not appear in Help or the legend and does not dispatch.
- Toggling selection between connectable and non-connectable items changes Connect visibility in the next rendered browser state.
- Removing color must not change the key text, action text, group membership, ordering, or entry boundaries.
