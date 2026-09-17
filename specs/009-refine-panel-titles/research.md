# Research: Refinar títulos de panel

## Decision: Separate structural-title and context-label rendering by placement

**Rationale**: `styles.regionTitle` already owns region and modal titles, while `styles.contentBadge` owns type/context labels. Both currently delegate to one bracketed, background-styled renderer. Retaining this ownership and changing the shared rendering semantics is the smallest change that covers every specified surface without affecting layout or state.

**Alternatives considered**:

- Restyle every title and label at each call site: rejected because duplicated rules would drift across browser regions, modals, forms, and prompts.
- Add a configurable theme subsystem: rejected because the feature changes one fixed semantic convention and does not add user-selectable themes.
- Remove visual distinction entirely: rejected because context labels must remain prominent.

## Decision: Keep brackets only for structural titles

**Rationale**: Structural titles such as `[Tree]`, `[Details]`, and modal titles orient the stable interface. Context labels such as `Connection`, `Folder`, and `Root` describe the selected content. The rendering contract makes that distinction readable in both color and no-color modes.

**Alternatives considered**:

- Remove brackets from all labels: rejected because it would discard the existing structural navigation cue.
- Retain brackets on context labels: rejected because it makes them indistinguishable from titles and conflicts with the specification.

## Decision: Use a dark background with white foreground for color-mode context labels

**Rationale**: The current black-on-bright-blue ANSI palette has insufficient contrast. A white foreground and dark blue background pair can be selected and verified against the required 4.5:1 contrast threshold, while the no-color path remains plain controlled text.

**Alternatives considered**:

- Keep bright blue and change only the foreground: rejected because the user explicitly rejects the current color treatment and terminal palette variation makes bright colors less dependable.
- Use color only as the cue: rejected because terminal-first accessibility requires textual meaning when color is absent.

## Decision: Preserve current layout and rendering boundaries

**Rationale**: Existing `renderRegionPanelWithScrollbar` handles ANSI display width and truncation. The shared renderer change affects text styling only; it must not modify panel geometry, focus markers, scrollbars, modal controls, connection behavior, or safe dynamic-value handling.

**Alternatives considered**:

- Rework panel layout while changing titles: rejected because it expands scope and risks responsive regressions.
- Add new presentation state: rejected because placement and active state are already represented by current transient badge descriptors.
