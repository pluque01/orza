# Research: Simplificar etiquetas de panel

## Decision: Use the existing placement-aware renderer

**Rationale**: Structural titles and context labels already have distinct placements in the shared style renderer. Changing that renderer updates regions, modals, details, forms, and prompts consistently without altering call sites or layout code.

**Alternatives considered**:

- Change every surface separately: rejected because behavior would drift across panels and prompts.
- Add a theme configuration system: rejected because the feature removes theme-sensitive styling rather than adding configuration.

## Decision: Use plain titles and bold-only context labels

**Rationale**: Plain title text follows the retained textual focus marker. Bold context labels remain perceptible without relying on a theme's ANSI palette, and their literal text remains the required cue when style is unavailable.

**Alternatives considered**:

- Keep a foreground color: rejected because terminal themes remap ANSI colors differently.
- Keep a background color: rejected because contrast depends on the theme palette.
