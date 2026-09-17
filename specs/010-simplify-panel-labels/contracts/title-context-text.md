# TUI Contract: Plain Titles and Context Labels

## Structural Titles

- Regions and modals render their title as `Tree`, `Details`, `Actions`, or the existing modal name, without title brackets.
- The existing `[*]` or `[ ]` focus marker remains immediately before a region title.

## Context Labels

- Existing context labels such as `Connection`, `Folder`, and `Root` render without brackets or foreground/background color.
- In style-capable terminals, context labels are bold.
- In no-color terminals, context labels retain full text and semantic meaning without ANSI output.

## Invariants

- Dynamic values remain safely projected and bounded.
- Keyboard controls, focus order, modal ownership, geometry, connections, and secret handling do not change.
