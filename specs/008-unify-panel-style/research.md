# Phase 0 Research: Estilo visual unificado de paneles

## Semantic Badge Representation

**Decision**: Render every panel type as a bounded textual badge whose plain form includes visible delimiters, such as `[Connection]`. In color mode, apply a bold bright-blue application accent background with a readable dark foreground to the same text; in no-color mode, emit the identical delimited text without ANSI sequences.

**Rationale**: Orza already uses ANSI color 12 for its title accent and preserves textual markers under `NO_COLOR`. Reusing that accent creates the requested characteristic background without introducing a palette or capability dependency. Delimiters make the badge recognizable when color is unavailable and allow tests to require `ansi.Strip(colored) == plain`.

**Alternatives considered**:

- Background color without textual delimiters: rejected because the type would lose its badge affordance in no-color mode.
- Unicode-only decoration: rejected because display support varies and the application already has portable ASCII semantics.
- A configurable theme or true-color palette: rejected because personalization is out of scope and current 16-color styles cover all supported platforms.

## Title Reuse and Badge Inventory

**Decision**: Keep the existing region/modal title location and focus marker, but render the controlled title as a badge. Do not repeat payload headings that are semantically identical to the title. Add a content badge only where the content type differs from its container: Details shows Root, Folder, or Connection; the connection form and embedded security prompts identify their own content inside Details. Modal kinds retain their current specific controlled names as the badge inventory.

**Rationale**: This implements the clarification that each panel owns a badge while avoiding duplicated `Help`, confirmation, or error headings. Keeping title placement preserves border geometry and overlay composition. A separate Details content badge is necessary because `Details` names a region while `Connection` names the selected entity.

**Alternatives considered**:

- Add a second badge inside every panel: rejected because it duplicates Help, Actions, and modal titles and consumes scarce vertical space.
- Replace `Details` with the selected entity type: rejected because the region's stable navigation identity would change with selection.
- Use one generic `Panel` badge: rejected because it does not communicate the panel type required by the specification.

## Structured Field Layout

**Decision**: Represent controlled label/value content as ordered display fields. Remove terminal colons from descriptive field labels, compute the maximum label width with `ansi.StringWidth`, and align every value to one shared column separated by one space. For non-interactive identity groups, use the aligned layout only when the local row can retain at least eight cells for the value; otherwise emit each label and value on consecutive lines. Interactive forms retain compact rows at every supported size and use the existing bounded component view so a focused field plus its error still fits the minimum Details height. Never omit an identity field solely to fit width.

**Rationale**: One shared value column is the approved Details behavior and provides consistent scanning across structured surfaces. An explicit eight-cell value floor prevents technically aligned but unreadable one-character identity values. Restricting mandatory stacking to non-interactive rows avoids creating a three-line label/value/error block where the 40x12 layout provides only two Details content rows. Interactive components such as the authentication selector keep their own established minimum and truncation contracts.

**Alternatives considered**:

- Keep `Label: value`: rejected by the approved colonless-label requirement.
- Truncate or hide values to preserve two columns: rejected by the narrow-layout clarification.
- Use byte length or formatted-string width: rejected because Unicode and ANSI sequences require display-cell measurements.
- Stack every interactive field: rejected because label, value, and validation error cannot all remain visible in the minimum-height Details region.

## Surface-Specific Composition

**Decision**: Share semantic styles and layout calculation, but let each current renderer compose its own rows. Static Details and modal fields can use a shared field renderer; interactive forms retain their existing `itemSemantics`, input views, active-line indexes, and error blocks while applying shared label styling and best-effort value alignment in compact rows. Actions retain descriptor packing, and Help retains logical help rows and modal viewport ownership.

**Rationale**: The surfaces share appearance but not interaction semantics. A universal renderer would need to understand focus, controls, errors, wrapped targets, selectors, and scroll ownership, increasing coupling and regression risk. A small common primitive plus local composition is the simplest design that preserves behavior.

**Alternatives considered**:

- Replace all surfaces with a new generic panel component: rejected because it would merge unrelated state and control contracts.
- Change only Details: rejected because clarification explicitly includes every structured TUI surface.
- Leave forms and security prompts on the old label style: rejected because it would violate the common hierarchy and produce mixed conventions.

## Wrapping, Viewports, and Safe Text

**Decision**: Sanitize dynamic text before applying styles, measure rendered display width with Charmbracelet ANSI utilities, and preserve each surface's current disclosure policy. Confirmation targets continue to hard-wrap without truncation; ordinary rows remain bounded by safe truncation/ellipsis; stacked identity fields remain present but long values may wrap or truncate according to their existing surface contract. Replace `viewportTruncateLines` checks for literal `Target: ` and `ID: ` prefixes with label-independent row preparation.

**Rationale**: Removing colons would silently bypass the current prefix special case. Preparing each structured row explicitly avoids parsing already-rendered presentation text and remains correct after styling adds ANSI sequences. Retaining viewport and scrollbar ownership protects fixed controls and active rows.

**Alternatives considered**:

- Strip ANSI and parse rendered lines by label: rejected because presentation text is not a stable data contract.
- Wrap all values everywhere: rejected because extra rows could displace focused form controls and alter established overflow behavior.
- Truncate confirmation targets: rejected because destructive and connection-affecting actions must identify their complete captured target.

## Focus, Errors, and Security Priority

**Decision**: Preserve region ownership markers in their existing exact compact forms, `>` focus, `!` invalid, `*` primary, checkbox/selector markers, `Warning:` and `Error:` status prefixes, and fixed cancel/back/quit controls independently from badge and label colors. Colon removal applies to descriptive field labels, not these status prefixes or punctuation inside values and sentences. Styling never receives secret bytes or credential references; the secret prompt continues to render only mask length.

**Rationale**: Color-independent operation is constitutional and existing tests already enforce marker precedence. Badge styling must not compete with active errors or primary recovery actions. Keeping display helpers downstream of safe public projections prevents visual refactoring from widening the secret boundary.

**Alternatives considered**:

- Encode focus or error through badge color: rejected because color cannot be the only signal.
- Move labels or badges ahead of existing marker slots: rejected because form width and focus tests depend on stable marker ownership.
- Expose credential metadata as a field for visual consistency: rejected because credential references are not user-facing and may be sensitive.

## Testing Strategy

**Decision**: Extend current table-driven and conformance tests rather than introducing a snapshot framework. Test exact plain badge/field rows, ANSI-stripped equivalence, aligned and stacked local widths, compact invalid form fields at 40x12, all registered modal kinds, duplicate-heading removal, every structured surface, long Unicode/control-bearing values, confirmation wrapping, viewport/control priority, resize state preservation, undersized recovery, and secret canaries. Extend the existing 1,100-node 100 ms fixture with Help-open/render and connection-confirmation-open/render samples. Validate the perceptual success criterion with the documented timed 20-participant protocol.

**Rationale**: Existing exact assertions localize behavioral regressions and already cover geometry, focus, security, and model transitions. The repository's `.golden` files are not referenced by active tests and predate the current bordered layout, so adopting them would add a second baseline mechanism without benefit.

**Alternatives considered**:

- Manual visual review only: rejected by the behavior-focused testing principle.
- Golden snapshots for complete frames only: rejected because they obscure which semantic or security contract failed and create broad churn.
- Platform-specific native-terminal tests: rejected because rendering is shared and platform integrations are unchanged; a small manual color/no-color pass remains useful.

## Dependencies, Performance, and Documentation

**Decision**: Keep the pinned Go and Charmbracelet stack, perform deterministic synchronous layout in `View`, and add no cache or retained render history. Extend, rather than merely reuse, the current performance acceptance for Help and confirmation rendering. Update controlled English and README documentation with the new textual hierarchy and `NO_COLOR` behavior.

**Rationale**: Field and panel inventories are small; maximum-label scans are linear in visible fields and independent of catalog size. The existing performance and retained-memory acceptance tests, extended with Help and confirmation samples, are sufficient to detect regressions. Documentation is required for user-visible behavior and supported terminal capabilities.

**Alternatives considered**:

- Cache rendered rows by width: rejected because invalidation complexity is unnecessary at this scale.
- Add a table/layout dependency: rejected because existing ANSI and Lip Gloss APIs already provide styling and display-width primitives.
- Leave documentation unchanged: rejected because the title and field conventions are visible behavior governed by the constitution.
