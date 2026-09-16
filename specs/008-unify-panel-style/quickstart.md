# Quickstart Validation: Estilo visual unificado de paneles

## Prerequisites

- Repository root: `/home/fallen/code/orza`
- Nix with flakes enabled; the pinned development shell supplies Go 1.26.6 and the repository tooling
- A VT-capable terminal at least 40x12 for manual validation
- A disposable catalog containing only synthetic `.invalid` hosts; do not use real credentials, identity files, or SSH sessions

Confirm the pinned development environment from the repository root:

```sh
nix develop -c go version
```

The reported Go version should match the module's Go 1.26 toolchain family.

## Known Tooling Limitation

Direct `go` is unavailable outside `nix develop` on this platform. The Nix-backed invocations below are therefore the supported commands here. T029 records no pass/fail results; command execution and concrete results belong to T030.

## Contract References

- [Data model](data-model.md): transient badges, fields, groups, and responsive transitions
- [Panel visual contract](contracts/panel-visual-hierarchy.md): complete visible, no-color, responsive, and security behavior
- [Research](research.md): technical decisions and rejected alternatives

## Fast Validation

Run the focused primitives, Details, Actions, and Help tests:

```sh
nix develop -c go test ./internal/tui \
  -run '^(TestPanelStyleFixtureInventory|TestStylesTypeBadgeConstraintsAndBrightBlueRendering|TestStylesStructuredFieldGroupUsesDisplayWidthAndLocalThreshold|TestStylesDescriptiveLabelsAreMutedColonlessAndStatusesUnchanged|TestViewportProjectionIsLabelIndependentAndTerminalSafe|TestDetailConnectionPresentationMatrix|TestDetailFolderUsesOrderedImmediateConnectionChildren|TestDetailRootAndFolderEmptyState|TestDetailProjectionMakesUntrustedTextInertAndBounded|TestDetailDirectChildNamesAndEndpointsRemainOrderedSafeAndVisible|TestDetailColorAndNoColorHaveEquivalentSafeBoundedText|TestDetailNeverProjectsCredentialReferencesOrSecretCanaries|TestEmbeddedConnectionFormSecretCanaryNeverProjectsThroughValidationOrResize|TestActionsAndHelpUseSingularBracketedTypeTitles|TestHelpKeepsContextualDescriptorInventoryAndKeys)$' \
  -count=1
```

Run the focused modal, form, move-picker, trust, and secret-prompt tests:

```sh
nix develop -c go test ./internal/tui \
  -run '^(TestModalTitleBadgeInventoryAndANSIParity|TestModalUnknownFallbackAndKnownInvalidPayloadAreSafe|TestModalRemovesDuplicateHeadingsAndKeepsMeaningfulCopy|TestModalStructuredFieldsAreColonlessWithANSIParity|TestModalWarningAndErrorPrefixesRemainUnchanged|TestModalCapturedTargetsWrapCompletelyWithoutDisplacingControls|TestModalControlInventoryAndPriorityRemainFixed|TestConnectionFormControlledBadgeKeepsCatalogPathInSafeStructuredValue|TestConnectionFormCompactRowsUseColonlessMutedLabelsAndBestAlignment|TestConnectionFormFortyByTwelvePreservesAuthMarkersErrorsSaveAndFooter|TestConnectionFormConflictIdentityUsesColonlessStructuredRows|TestFolderFormsReuseModalBadgeAndSharedFieldHierarchy|TestFolderFormSanitizesDynamicValuesBeforeLabelStyling|TestMovePickerReusesModalBadgeAndSharedFieldHierarchy|TestMovePickerSanitizesDynamicValuesBeforeLabelStyling|TestTrustPromptUsesControlledBadgeColonlessFieldsAndColorParity|TestDecideTrustLineReaderOutputAndDecisionsRemainExact|TestUS4TrustUnknownChangedExplicitDecisionMatrixTwentyRuns|TestTrustPromptProjectsEveryDynamicValueBeforeRendering|TestTrustPromptOversizedValuesAreBoundedWithoutChangingSource|TestSecretPromptUsesControlledBadgeColonlessFieldAndColorParity|TestSecretPromptProjectsStoreNameSafelyBeforeRendering|TestUS4PasswordPassphraseNeverRenderRawTwentyRuns|TestUS4SecurityInputOwnershipResizeAndMaskingTwentyRuns|TestSecurityInputProjectsNoSecretOrCredentialData)$' \
  -count=1
```

Run the SC-005 overflow matrix and the exact US3 boundary, resize, and undersized checks. Selecting the fuzz target with `-run` executes its deterministic seed corpus; an open-ended fuzz campaign is not part of this quickstart.

```sh
nix develop -c go test ./internal/tui \
  -run '^(TestSC005LongUnicodeControlFieldMatrix|FuzzSC005PanelFieldRendering|TestUS3DetailIdentityRowsUseLocalEightCellValueBoundary|TestConnectionFormFortyByTwelvePreservesAuthMarkersErrorsSaveAndFooter|TestUS5ExactResponsiveGeometryMatrix|TestUS5ResizeSequencePreservesOpaqueState20Runs|TestUS5UndersizedShellAndExactRestoration20Runs|TestUS5SharedViewportUniversalOverflowMatrix|TestUS5CurrentTreeDetailsFormActionsAndHelpUseOverflowContract|TestUS5PickerConfirmationAndErrorOverflowAreBoundedAndDiscoverable)$' \
  -count=1 -v
```

Run the feature's public-model integration tests:

```sh
nix develop -c go test ./tests/integration \
  -run '^(TestTUITreeDetailsDirectChildrenEmptyAndFallback|TestTUITreeDetailsBadgesSynchronizeWithSelectionInColorAndNoColor|TestTUIActionsAndHelpUseSingularBracketedTitles|TestTUIContextualActionsAndCapturedConnectTarget|TestTUIFolderActionInventory|TestTUIConnectionCRUDAndStaleRecovery|TestTUIConnectionFormDetailsCancelAndSelectionTwentyRuns|TestTUISessionFailureRecoveryHandoffAndTerminalRestoration|TestTUIConnectConfirmationCancelStartsZeroNetwork)$' \
  -count=1
```

Expected outcomes:

- Region and modal titles render as delimited type badges in color and no-color modes.
- Details shows `[Connection]`, `[Folder]`, or `[Root]` and never renders `Kind: Connection` or another `Kind` row.
- Structured labels have no trailing colon and values begin in one shared column when enough local width exists.
- Narrow non-interactive identity groups stack every label above its value without omission; interactive fields keep their compact field/error block at 40x12.
- Help, Actions, forms, confirmations, errors, conflicts, trust prompts, and secret prompts apply the same hierarchy without changing controls or focus.
- Color rendering strips to the exact no-color text; dynamic values remain inert and bounded and secret canaries remain absent.

## Complete Automated Validation

Run the repository quality gates:

```sh
nix develop -c sh -c 'files=$(gofmt -l cmd internal tests); test -z "$files"'
nix develop -c go test ./...
nix develop -c go test -race ./...
nix develop -c go vet ./...
nix develop -c go build ./...
nix develop -c staticcheck ./...
nix develop -c bash scripts/check-vulnerabilities.sh pinned
```

Run the reproducible project gate:

```sh
nix flake check --no-update-lock-file --keep-going -L
```

All commands must pass. The formatting gate reports failure without rewriting files. Tests must not require network access, a real SSH server, secret entry, a private key, or a native credential store.

### Recorded Results (2026-09-15)

- Formatting: PASS; `gofmt -l cmd internal tests` produced no paths.
- Full tests: PASS for every package with `go test ./...`.
- Race detector: PASS for every package with `go test -race ./...`.
- Vet, build, and staticcheck: PASS for every package.
- Pinned vulnerability policy: PASS; reported findings were unreachable and no policy-blocking vulnerability was present.
- Responsive acceptance: PASS for 264,240 frames (1,101 nodes x 12 sizes x 20 runs), including the closed-surface traversal matrix.
- Performance: PASS with 20/20 samples below 100 ms for selection, focus, Help, connection confirmation, scroll, and resize. The slowest Help sample was 855.57 microseconds and the slowest connection-confirmation sample was 524.489 microseconds. Reload passed 20/20 below one second, and retained heap growth after 10,000 render/resize cycles was 0 bytes.
- Reproducible project gate: PASS; `nix flake check --no-update-lock-file --keep-going -L` completed all nine `x86_64-linux` checks.

The flake check evaluated native and cross-package outputs available from `x86_64-linux`, but omitted checks for the incompatible `aarch64-darwin`, `aarch64-linux`, and `x86_64-darwin` systems. Those checks require matching builders or an explicit configured `--all-systems` build environment.

## Responsive Acceptance

After the non-build-tagged responsive tests above pass, run the existing build-tagged acceptance suite by its actual test names:

```sh
nix develop -c go test -tags acceptance ./internal/tui \
  -run '^(TestSC007EveryScaleNodeAcrossTwelveSizesTwentyRuns|TestSC007AcceptanceIncludesClosedSurfaceTraversalMatrix)$' \
  -count=1 -v
```

Expected outcomes:

- The `40/60/79/80/100/160` width matrix at heights 12 and 24 remains bounded with unchanged outer geometry.
- Aligned identity fields share an exact value start column; insufficient local width switches the complete non-interactive group to stacked pairs.
- The sequence `40→60→79→80→100→160→80→79→40` repeated 20 times preserves state, focus, selection, errors, modal payload, viewport offset, and priority controls.
- `40x12→39x11→40x12` renders only the undersized surface in the middle and restores the same panel state afterward.
- Scrollbars remain in the established right-padding cell and stacked rows do not displace fixed modal or form recovery controls.

Run the extended 1,100-node performance fixture and the retained-render-history fixture after rendering tests pass:

```sh
nix develop -c go test ./internal/tui \
  -run '^(TestUS5PerformanceAcceptance1100NodeFixture|TestUS5RenderResizeHistoryRetention10000Cycles)$' \
  -count=1 -v
```

`TestUS5PerformanceAcceptance1100NodeFixture` now measures selection, focus, Help open/render, connection-confirmation open/render, scroll, and resize. At least 19 of 20 samples for each local operation must finish within 100 ms; at least 19 of 20 reload samples must finish within one second. `TestUS5RenderResizeHistoryRetention10000Cycles` requires retained heap growth below 1 MiB after 10,000 render/resize cycles.

## Timed Recognition Acceptance

Run the SC-003 recognition protocol with 20 participants who can operate a terminal but have not seen the test frames in advance.

1. Prepare fixed 80x24 frames for Details, Actions, connection form, Help, connection confirmation, destructive confirmation, and recoverable error in both color and `NO_COLOR=1` modes.
2. Randomize frame order independently per participant and alternate which color mode is shown first.
3. For each frame, ask the participant to identify the panel/content type and, when present, the target value, focused field or action, and error or primary action.
4. Start timing when the frame appears and stop when the participant gives the complete answer or five seconds elapse.
5. Record participant identifier, frame, mode, elapsed time, expected terms, supplied terms, and pass/fail in a review attachment without recording real hosts, paths, credentials, or secrets.
6. Pass when at least 19 of 20 participants answer every applicable prompt correctly within five seconds in both color modes.

This protocol validates recognition only; it does not claim broader accessibility conformance.

## Manual Visual Validation

Create an isolated synthetic catalog and start Orza:

```sh
export XDG_DATA_HOME="$(mktemp -d)"
nix develop -c go run ./cmd/orza folder create /visual-check
nix develop -c go run ./cmd/orza connection create /visual-check/production \
  --host prod.example.invalid --port 2222 --user deploy --auth agent
nix develop -c go run ./cmd/orza
```

Validate using only the keyboard:

1. Select `/visual-check/production` and confirm Details shows its own badge plus `[Connection]`, with no `Kind` row.
2. Confirm Name, Path, Endpoint, User, and Method have colonless muted labels and one aligned value column.
3. Move focus between Tree and Details and confirm `[*]`/`[ ]` still communicates ownership independently from badge color.
4. Open Actions and Help and confirm their type appears once, controls are unchanged, and Help still scrolls when necessary.
5. Press `c` without confirming the connection and verify the Connect modal identifies the complete `.invalid` target, uses colonless labels, and keeps Confirm, Cancel, and Help controls visible.
6. Open new/edit connection and folder forms and confirm labels share the hierarchy while text cursor, method selector, checkbox, errors, Save, and focus markers remain clear.
7. Inspect Move, Delete, and unsaved-change surfaces with synthetic catalog paths. Rely on the focused automated view tests for SSH-failure, host-trust, and secret-prompt surfaces so this walkthrough never initiates SSH or requests a real secret.
8. Resize through 40x12, 60x12, 79x24, 80x24, 100x24, and 160x24. Confirm identity rows align when space permits and stack label above value without hiding fields when they do not; interactive rows remain compact and actionable.
9. Resize below 40x12 and back; confirm the undersized screen appears alone and the previous focus/content returns unchanged.
10. Restart with `NO_COLOR=1` and confirm badges retain delimiters, labels and values remain distinguishable, all status/focus markers remain visible, and no ANSI color is emitted.
11. Cancel all open forms and confirmations and verify no catalog or credential change occurred.

Do not confirm an SSH connection. The feature requires no network operation or real secret for validation.
