# Research: Refine Action Legend

## Decisions

### Preserve the full action inventory as the source of truth

**Decision**: Keep the existing contextual action descriptors as the shared source for key dispatch and Help, and derive a separate, presentation-only browser legend from them.

**Rationale**: The descriptors already encode which actions apply to the selected object and focus. Filtering that inventory for the compact legend would make hidden secondary shortcuts unavailable or require duplicate applicability logic.

**Alternatives considered**:

- Filter the existing inventory before all rendering: rejected because it would also remove secondary actions from Help and dispatch.
- Define a separate hard-coded key map for the legend: rejected because it would duplicate bindings and drift from dispatch behavior.

### Use the existing structured-field presentation for Help

**Decision**: Represent each Help action as an aligned key/action field and render it with the existing width-aware structured field group.

**Rationale**: Details already establishes the desired muted-label convention, display-cell measurement, aligned columns at practical widths, and stacked fallback on narrow terminals.

**Alternatives considered**:

- Build a dedicated Help table renderer: rejected because it repeats responsive width and colorless rendering behavior.
- Retain raw single-line Help strings: rejected because it cannot align the two fields or distinguish their visual roles.

### Make separation meaningful without color

**Decision**: Render each compact legend entry as a styled key plus muted action and separate neighboring entries with a visible, width-aware delimiter or multi-cell gap.

**Rationale**: Terminal color may be unavailable, and one blank cell is insufficient to identify entry boundaries consistently.

**Alternatives considered**:

- Use color alone: rejected because it violates terminal usability requirements.
- Keep one-space separation: rejected because it does not meet the readability requirement.

### Reclaim the browser-only Actions-panel height

**Decision**: Remove the fixed Actions region from browser layout and allocate the recovered space to the Tree and Details content, while keeping existing form, operation, and conflict controls unchanged unless they share the browser-specific rendering path.

**Rationale**: The feature targets a non-interactive browser panel. Limiting the geometry change prevents unrelated interactive control surfaces from regressing.

**Alternatives considered**:

- Replace every control region with the new legend: rejected because forms and operation states may require their current dedicated controls.
- Leave the empty bordered region in place: rejected because it preserves wasted screen space and the panel appearance the feature removes.

### Test presentation separately from availability and dispatch

**Decision**: Add legend and Help presentation assertions while retaining or strengthening action-dispatch tests for actions omitted from the compact legend.

**Rationale**: The key risk is coupling a visual simplification to behavioral availability. Exact strings, ANSI-stripped output, and display widths are existing project conventions.

**Alternatives considered**:

- Rely on visual snapshots only: rejected because they do not directly prove hidden actions remain executable.
- Test only descriptors: rejected because layout, grouping, styling, and responsive fallbacks are user-visible behavior.
