# Quickstart: Responsive Multipanel TUI

> **Historical overflow validation**: Directional marker checks in this guide are superseded by the feature-005 scrollbar validation guide.

## Prerequisites

- Go 1.26.5 through `nix develop`.
- Temporary local catalog with no production credentials.
- VT-capable interactive terminal for native checks.
- Feature-002/003 regression suites passing.
- Pinned Charmbracelet dependencies; no Huh or runtime `nexus-tui-builder` dependency.

## Mandatory Automated Gates

### Baseline 2026-08-12

- `nix develop -c go test ./internal/tui`: PASS.
- `nix develop -c go test ./tests/integration -run 'TUI|FolderHierarchy|Terminal|HostKey'`: PASS.
- `nix develop -c go test ./...`: PASS.
- Nix reported transient busy eval-cache messages while parallel shells initialized; commands completed normally.
- Workspace is not a Git worktree. Existing `.gitignore` already covers Go, Nix, local databases, editors and OS files.

### Final automated evidence 2026-08-12

T082 quality gates, rerun after cleanup and the final acceptance-test additions:

- `nix develop -c sh -c 'files=$(gofmt -l cmd internal tests); test -z "$files"'`: PASS.
- `nix develop -c go test ./... -count=1`: PASS, including the Linux amd64 SQLite acceptance test.
- `nix develop -c go test -race ./... -count=1`: PASS. The latency-only SC-011 SQLite test has the standard `!race` build constraint because race instrumentation invalidates its production latency threshold; the functional catalog, model and repository suites still run under race.
- `nix develop -c go vet ./...`: PASS.
- `nix develop -c staticcheck ./...`: PASS.
- `nix develop -c govulncheck ./...`: PASS; zero reachable vulnerabilities. One required module vulnerability is unreachable from this code.
- Parallel Nix shell startup emitted transient busy eval-cache warnings; completed commands returned success.

T083 performance and state-transition evidence on Linux amd64, AMD Ryzen 5 7600X, 12 logical CPUs and
13.36 GB available memory at runner qualification:

- `nix develop -c go test ./internal/tui -run '^$' -fuzz '^FuzzModelStateTransitions$' -fuzztime=30s`: PASS; 22,610 executions, 197 new interesting inputs, 271 total corpus entries.
- `nix develop -c go test ./internal/tui -run '^$' -bench 'Tree|Detail|Layout' -benchtime=20x -count=1`: PASS. `TreeRefresh` 510,336 ns/op; `TreeExpand` 1,007 ns/op; `TreeCollapse` 1,244 ns/op; `DetailDirectChildren` 1,996 ns/op, 712 B/op, 17 allocs/op; `Layout` 18.55 ns/op.
- Selection, 20/20 <=100 ms: 153.812, 155.476, 152.941, 158.171, 238.464, 149.575, 174.431, 178.690, 159.002, 173.960, 177.097, 172.668, 178.400, 265.335, 167.649, 174.011, 208.907, 242.050, 234.496, 173.019 microseconds.
- Focus, 20/20 <=100 ms: 119.797, 131.419, 216.111, 184.541, 157.439, 156.738, 157.159, 171.907, 153.792, 171.626, 155.356, 164.823, 172.839, 167.378, 165.535, 195.882, 191.064, 181.064, 222.984, 188.759 microseconds.
- Scroll, 20/20 <=100 ms: 128.404, 166.216, 123.254, 224.266, 172.298, 391.825, 120.059, 119.988, 120.429, 139.786, 119.938, 120.939, 118.235, 119.568, 153.381, 165.023, 120.879, 125.489, 139.015, 123.375 microseconds.
- Resize, 20/20 <=100 ms: 168.781, 157.770, 168.811, 153.382, 154.343, 134.536, 154.434, 139.005, 170.093, 174.101, 163.170, 142.301, 143.252, 138.073, 147.911, 140.427, 142.501, 140.337, 151.778, 127.833 microseconds.
- Real SQLite reload, 20/20 <=1 s after one 82.834206 ms warm-up: 101.963362, 96.592916, 93.871850, 91.433820, 82.117880, 91.981290, 87.836782, 98.606666, 89.439482, 84.160182, 89.322727, 88.982334, 90.065463, 80.628328, 80.265481, 92.722213, 81.169970, 86.744775, 79.614035, 82.720875 milliseconds. The temporary catalog contained exactly 100 folders, 1,000 connections, depth 10 and no credential-store calls.

T084 packaging evidence:

- `nix flake check --print-build-logs`: PASS; all 8 x86_64-linux checks passed. Nix reported that incompatible aarch64-darwin, aarch64-linux and x86_64-darwin systems were omitted by flake evaluation; explicit pure-Go builds cover the release boundaries below.
- `CGO_ENABLED=0` `go build ./...`: PASS for linux/amd64, linux/arm64, windows/amd64, windows/arm64, darwin/amd64 and darwin/arm64 through `nix develop`.

T085 automated conformance evidence:

- The focused SC-001-SC-006 and SC-009-SC-017 geometry, modal, conflict, operation, resize, undersized, security-input, retry, stale-result, cleanup, controlled-English and safe-text groups passed, including their internal 20-run products. The complete and race suites in T082 independently reran all non-tagged cells.
- SC-006 `TestSC006QuickstartPrincipalFlowMatrix80x24NoColorTwentyRuns`: PASS. The authoritative US1-US4 matrix contains 74 cells, 1,480 executions and 1,940 textual completion/cancel frames at exactly 80x24 with no ANSI. Every frame identifies its applicable focus, selection, Details information, target, error and Actions semantics using text alone.
- SC-007 `nix develop -c go test -tags acceptance ./internal/tui -run '^TestSC007EveryScaleNodeAcrossTwelveSizesTwentyRuns$' -count=1 -v`: PASS in 404.32 s. The tagged scale gate exercised 1,101 nodes (root + 100 folders + 1,000 connections), exact depth 10, across 12 sizes and 20 runs: 264,240 bounded frames. It verifies Tree/Details visibility, exact shared-viewport projection, applicable previous/next markers and every visible truncation. The routine SC-007 suite additionally passed forms for all three authentication methods, Help, Actions, picker, connect/delete confirmations and operation errors across all 12 sizes x 20 runs, with top/middle/end overflow positions where applicable.
- SC-009 covers the exact closed inventory of ten generic modal kinds and their displayed control aliases, completion, cancellation, failure, retry and conflict paths. SSH retry remains inline and uses a captured typed intent rather than a blind reload.
- SC-012 conflict matrices passed for navigation, forms, target-bearing modals and pending operations, plus 120 operation-to-conflict executions (three applicable operation kinds x missing/revision-changed x 20). Conflicts publish only after cleanup and retain captured IDs/revisions without retargeting.
- SC-013 `TestSC013OperationConformanceMatrix20Runs`: PASS across 560 executions: initial load/reload/save/SSH start x success/failure/cancel/quit/stale/post-cancel/confirmed-before-cleanup x 20. Matching and stale owner results, mutation suppression, cleanup and stable owner restoration passed.
- SC-014 controlled-language tests passed exact manifest equality for canonical terms, application copy, symbolic/lowercase render copy, SSH categories/stages/summaries/recommendations and all source classification. Dynamic folder/connection name, path, host and username bytes remained identical over every size and after the SQLite repository/service reopen round trip.
- SC-015 `TestSC015DirectStreamAndRetentionMatrix20Runs`: PASS. In 20 success runs stdout/stderr each delivered exactly 64 MiB; in 20 injected network-interruption runs each delivered the exact 32 MiB prefix. Deterministic stream patterns and SHA-256 digests prove byte order; writers remained direct and non-retaining, every session was active and terminal-restored, and retained heap growth after GC was 0 bytes, below 1 MiB. Controlled diagnostics are bounded to 256 visible cells. The latency/heap acceptance test is excluded under race while smaller direct-stream regressions remain race-covered.
- SC-016 `TestSC016TransportCleanupLifecycleMatrix20Runs`: PASS across 520 executions: timeout/network interruption x 13 target-resolution, network, trust, authentication, handshake, session, PTY, resize-source, raw, shell and active stream/resize stages x 20. Applicable session, transport, raw connection and authentication resources closed exactly once before terminal restoration; active resize was processed/joined before restore. `TestSessionOperationTimeoutNetworkAndActiveCommitMatrix` additionally passed 20 pre/post-active timeout/network runs with zero operation owner/context/cancel handle and inert duplicate/stale results.
- SC-017 `TestUS5RenderResizeHistoryRetention10000Cycles`: PASS with long Tree, Details, Help and error payloads over the twelve contract sizes; retained heap growth was 0 bytes after 10,000 resize/render cycles, below 1 MiB. The retained-memory threshold is skipped under race instrumentation and runs in the normal gate.
- The integrated trust/password/passphrase matrix passed unknown/changed trust decisions, terminal-owned masked secrets, normal/reduced/undersized recovery, operation cancellation, terminal failure and stale-result isolation. Linux PTY regressions for password/passphrase paste and selection, cancellation, timeout, SIGINT and SIGTERM cleanup also passed. Secret bytes remain outside Model, View, errors and diagnostics.
- T085 COMPLETE. SC-008 is intentionally excluded from this automated task and remains independently pending under T087.

### Convergence automated evidence 2026-08-14

- T089-T096 and T101-T108 automated implementation is complete. Runtime regressions cover live resize suspension and fresh-input resume for trust/password/passphrase readers, active Cancel/Help/Quit control during SSH start and remembered-password operations, joined cleanup, safe dynamic projection, read-only target-stable Folder traversal, modal loading controls, standalone modal Help, navigation notices, unbounded consistent snapshot depth, sole `operationState` ownership, stale/duplicate completion rejection, state fuzzing and the required-VT boundary.
- `SHELL=/bin/sh nix shell nixpkgs#go_1_26 --command go test ./...`: PASS. The explicit shell makes the Linux `script(1)` PTY helper independent of the caller's interactive shell.
- `SHELL=/bin/sh nix shell nixpkgs#go_1_26 --command go test -race ./...`: PASS.
- `nix shell nixpkgs#go_1_26 --command go vet ./...`: PASS.
- `nix develop --command staticcheck ./...`: PASS.
- `nix flake check --print-build-logs`: PASS, including build, formatting, normal tests, race, vet, staticcheck, pinned-database govulncheck and native smoke on x86_64-linux.
- `nix flake show --all-systems`: PASS; native cgo Darwin package outputs evaluate for amd64 and arm64, while pure-Go Linux and Windows package outputs evaluate for amd64 and arm64.
- The current external vulnerability database reports reachable `GO-2026-5972` in Go 1.26.5's `encoding/asn1`, fixed in Go 1.26.6. The newest available `nixpkgs-26.05-darwin` revision checked on this date still supplies Go 1.26.5, so the unpinned `nix develop --command govulncheck ./...` gate is BLOCKED pending the patched toolchain; the passing pinned scan predates this advisory and does not waive it.
- T097 implementation and deterministic cancellation/join/prompt-dismissal tests are complete, including race coverage, but T097 remains pending because native Secret Service prompt evidence required by T086 has not been recorded.

Manual and aggregate status:

- T086 PENDING: this runner supplies Linux PTY automation, not complete native Linux, Windows and macOS emulator evidence for trust/password/passphrase rendering, masking, resize suspension and restoration.
- T087 / SC-008 PENDING: the fixed anonymous 10-participant study has not been conducted; `usability-study.md` is intentionally absent rather than pre-approved.
- T088 PENDING: T085 is complete, but constitution review cannot be completed until T086 native evidence and T087/SC-008 evidence are complete. Current automated evidence supports SSH trust, secret isolation, captured-target stability, cleanup, keyboard recovery and quality gates but does not replace those mandatory boundaries.

Focused loop:

```sh
nix develop -c go test ./internal/tui
nix develop -c go test ./tests/integration -run 'TUI|FolderHierarchy|Terminal|HostKey'
nix develop -c go test ./internal/cli -run 'NoColor|Root'
```

Merge-blocking gates:

```sh
nix develop -c sh -c 'files=$(gofmt -l cmd internal tests); test -z "$files"'
nix develop -c go test ./...
nix develop -c go test -race ./...
nix develop -c go vet ./...
nix develop -c staticcheck ./...
nix develop -c govulncheck ./...
nix flake check --print-build-logs
```

Cross-build pure-Go release boundaries:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build ./...
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...
```

A failed, unavailable or unrun mandatory gate means **not merge-ready**. It may be recorded as pending but
never passed or waived. Native terminal/host-trust evidence is mandatory where portable automation cannot
exercise the real boundary; hardware unavailability leaves release approval pending.

## Fixed Catalogs

### Scale catalog

- 1,000 connections, 100 folders, maximum depth 10.
- Temporary SQLite database, stable names/IDs and no production secrets.
- Include direct and nested connections, empty folders, long Unicode and terminal-control canaries.

### Usability catalog

- Root.
- Folders `Empty` and `Team`.
- Direct connections `Prod` and `Stage` under `Team`.
- Folder `Nested` under `Team` with connection `Deep`.
- `Prod` selected and Tree focused.

## Layout and Resize Matrix

Use complete dimensions:

| Size | Expected mode |
|---|---|
| 40x12 | Stacked minimum |
| 60x16 | Stacked reduced |
| 79x23 | Stacked reduced boundary |
| 80x12 | Wide reduced minimum height |
| 80x24 | Wide complete boundary |
| 100x20 | Wide reduced |
| 100x30 | Wide |
| 160x40 | Wide |

Repeat this exact sequence 20 times in Tree navigation, Details navigation, connection form, each applicable
centered-panel kind and pending operation:

```text
40x12 → 60x16 → 79x23 → 80x24 → 100x30 → 160x40 → 80x24 → 79x23 → 40x12
```

For each transition assert selection, expansion, offsets, text/cursor/selection, focus, panel, conflict,
operation owner and pending intent. Every line/rectangle remains bounded and orientation follows width only.

Additionally transition 20 times through 80x12→80x24→100x20→100x30. Assert all four remain wide, the two
short variants apply reduced priorities, and no active/error/recovery control disappears.

Run the full Cartesian matrix `{40,60,79,80,100,160} × {12,24}` and assert the exact contract formulas for
Tree/Details/Actions rectangles, reduced priority, borders, padding, gutter, descriptor packing and odd-row
assignment. Include `39x12` and `40x11` as independent undersized OR-boundary cases.

Repeat 20 times for navigation, dirty form, confirmation and pending operation:

```text
40x12 → 39x11 → 40x12
```

At 39x11 assert undersized notice/minimum, Help and safe Quit only. Exercise Help, dirty Quit and operation
Quit. On recovery assert exact state restoration and no hidden mutation.

## Overflow and Safe Text Matrix

Exercise Tree, Details, connection form, Actions, Help, move picker, confirmation and error payload with:

- Previous-only, next-only and both-direction overflow.
- One, two and many available content rows.
- Long ASCII, combining characters, CJK, emoji and ZWJ sequences.
- CR/LF, ANSI escape, C0/C1 controls, bidi controls, invalid UTF-8 and oversized values.

Expected:

- `↑ more` only when previous content exists; `↓ more` only when next exists; both when applicable.
- Active selection/field/error/recovery is never displaced.
- Every truncated line includes in-width `…`.
- User bytes remain unchanged in model/persistence; visual controls are inert/escaped.
- No canary or raw cause reaches UI, logs, fixtures or usability evidence.

## Details and Normal Actions

Run root, folder and connection selection cases 20 times each:

- Connection: name, path, endpoint, user, method and allowed identity reference only.
- Folder/root: direct connections only in catalog order; no child folder/nested connection.
- Empty folder: explicit English state.
- Selection updates Details/Actions in same transition.

Exact action inventories:

| Context | Keys |
|---|---|
| Root | `n/f/r/?/q` |
| Folder | `n/f/e/m/d/r/?/q` |
| Connection | `c/n/f/e/m/d/r/?/q` |

Run each displayed action and every inapplicable object-action key 20 times. Assert Actions, Help and dispatch
agree; unlisted keys preserve catalog, selection, focus and operation. `Ctrl+C` aliases `q`; `n/f` on
connection uses parent.

## Form, Modal and Dirty Exit Matrix

Run connection create and edit 20 times each:

- Form remains in Details; Tree remains visible/inert.
- First field focus; full forward/back traversal; resize preservation.
- Validation/persistence/conflict preserves values/focus/target.
- Cancel restores prior selection/expansion.

Traverse exactly Name→Folder→Host→Port→User→Method→Identity file for key auth→Remember password for password
auth→Save and the exact reverse. Hidden conditional controls are skipped; changing Method while a conditional
control is focused returns focus to Method. Validation error is field-local. Save/persistence failure is a
form-level inline error above Save and does not open a modal.

Run Save, Discard and Cancel dirty-Quit choices 20 times each plus 20 save failures. Save exits only after
commit/reconcile/cleanup; failure never exits.

Run each generic modal variant 20 times:

- Folder create and folder edit.
- Move destination.
- Delete connection and delete folder.
- Connect confirmation.
- Unsaved changes.
- Help.
- Recoverable operation error.
- SSH failure.

Assert single slot, centered/bounded rendering, target/scope, explicit destructive acceptance, input isolation,
overflow markers, error/conflict embedding and opener restoration. Request Help during modal-owned operation;
`?` toggles bounded inline Help, `Esc` from Help restores the prior payload control, and no key stacks/replaces.

## Concurrent Target Recovery Matrix

For each applicable Cartesian cell, run 20 times:

- Conflict: missing target and changed revision.
- Surface: navigation, form, modal and operation.
- Recovery: `r` Reload, `b` Back and `Esc` Cancel warning.

Assert navigation nearest-ancestor/root fallback; active target/values/focus preservation; same-ID reload;
Back abandonment/fallback; Cancel leaves compact banner and `r`/`b`/`Esc` recovery visible; persistence/network
blocked; no row-index retargeting.

## Async Operation Matrix

For initial load, reload, save and SSH start, run each case 20 times:

- Success and failure.
- `Esc` cancellation.
- `q`/`Ctrl+C` during operation.
- Stale/duplicate result.
- Result after cancellation request.
- Confirmed result before cleanup completes.
- Conflict result for each applicable operation.
- Help while root/form/modal owns operation.

Assert exact `Loading: <action> — <target>`, last valid frame/initial empty shell, mutation-key suppression,
matching owner ID, cleanup ordering and focus preservation. Before commit restore previous state; after commit
apply confirmed result before restore/exit.

For conflict result, assert operation controls remain authoritative through cleanup, OperationState is removed,
and only the next frame exposes compact conflict plus `r` Reload, `b` Back, `Esc` Cancel warning, Help and Quit.
No frame may dispatch both operation and conflict inventories.

Run the precedence rows 20 times each:

| Point | Expected result |
|---|---|
| Conflict before dispatch | Block I/O; preserve owner/values/focus. |
| Conflict while running before commit | Complete/cleanup operation, then publish conflict. |
| Cancel requested before commit | Cleanup, restore previous frame; publish conflict only from matching owner result. |
| Result confirmed after commit | Reconcile confirmed result before cleanup/restore/quit; do not reclassify as pre-commit conflict. |
| Stale/non-owner completion | Ignore completely. |

Commit points:

- Initial/reload: accepted validated snapshot.
- Save: returned post-commit ID/revision.
- SSH start: nonzero `StartedAt` active result.

## Performance Protocol

Environment: Linux amd64, at least 2 logical CPUs and 4 GiB free, temporary scale catalog, no concurrent
load, one warm-up, monotonic event-to-complete-frame clock. SQLite is included only for reload.

Collect and report all 20 measurements for selection, focus, scroll, resize and reload:

- At least 19/20 selection/focus/scroll/resize frames are ≤100 ms.
- At least 19/20 reload snapshots are visible in ≤1 s.
- Direct-folder projection processes immediate child IDs only.

## SSH Stream and Transport-Failure Protocol

Using counting writers that do not retain the corpus, emit 64 MiB stdout and 64 MiB stderr in 64 KiB chunks.
Repeat success and mid-stream network interruption 20 times. Assert byte-for-byte delivery up to interruption,
no remote corpus in Model/modal/log/diagnostic state, ≤1 MiB retained TUI content, and controlled technical
details ≤256 visible characters without raw causes or secret canaries.

Inject timeout and network interruption before active commit at resolution, dial, host trust, authentication,
session setup and PTY/shell setup; inject both after nonzero `StartedAt` during stream and resize forwarding.
Run every applicable cell 20 times. Assert session, transport, auth and resize forwarding close/join exactly
once, terminal restoration precedes stable failure/return/exit, OperationState is cleared, pre-active failure is
recoverable, active transport result preserves confirmed state, and delayed duplicate completion is inert.

Run existing and new benchmarks/fuzzing:

```sh
nix develop -c go test ./internal/tui -run 'PerformanceAcceptance|Conformance' -count=1 -v
nix develop -c go test ./internal/tui -run '^$' -bench 'Tree|Detail|Layout' -benchtime=20x -count=1
nix develop -c go test ./internal/tui -run '^$' -fuzz '^FuzzModelStateTransitions$' -fuzztime=30s
```

## English Language Corpus

Extract controlled strings from titles, action descriptors, Help, status labels, validation, confirmations,
recoverable errors and SSH failure projections. Assert 100% English and canonical terms: Tree, Details,
Actions, New connection, New folder, Edit, Move, Delete, Connect, Reload, Save, Discard, Cancel, Back, Help,
Quit. User names/paths/hosts/users are not translated and remain byte-identical internally; only safe visual
escaping/truncation is permitted.

Build the authoritative corpus by extracting every application-controlled render literal, action descriptor,
Help entry, status, validation, confirmation and safe error projection from `internal/tui`. Require exact set
equality with human-reviewed `internal/tui/testdata/controlled_english.txt`, which records approved English copy
and canonical concept term. Maintain an explicit allowlist only for user/backend values and structural test
fixtures; fail on missing, extra, duplicate-concept or unclassified controlled copy.

## Principal Flow Matrix

At 80x24 with no color, run each row 20 times using keyboard only:

| Story | Closed principal flows |
|---|---|
| US1 | Select root, populated folder, empty folder and connection; enter/scroll/leave Details. |
| US2 | Execute every displayed root/folder/connection domain action; cancel every action that opens an interaction; cancel and quit each operation kind. |
| US3 | Create Save/Cancel; edit Save/Cancel; validation failure; persistence failure; conflict Reload/Back/Cancel; dirty Quit Save/Discard/Cancel. |
| US4 | Complete and cancel each of ten modal kinds; embedded error; embedded conflict; inline Help open/close; host trust accept/cancel for unknown/changed. |

For every row identify focus, selection, target, error and actions using text only and record completion/cancel
restoration. This is the authoritative meaning of “every principal flow” for SC-006.

## Host-Key Regression

Automated and controlled loopback checks:

- Known matching key proceeds normally.
- Unknown key shows target/fingerprint and requires explicit acceptance.
- Changed key has distinct warning and requires explicit acceptance.
- Cancel opens no active session and persists no trust.
- Retry, reload, resize, operation cancel and stale result never auto-accept.
- Secret/raw diagnostic canaries never appear.

Automate the full specialized security-input boundary with injected verifier/terminal implementations, 20 runs
per applicable cell:

| Input kind | Cases |
|---|---|
| Trust unknown/changed | Accept, Cancel, resize normal↔reduced, undersized suspension/recovery, stale result, operation cancellation, terminal failure. |
| Password | Masked input, Cancel, resize normal↔reduced, undersized suspension/recovery, operation cancellation, terminal failure. |
| Passphrase | Masked input, Cancel, resize normal↔reduced, undersized suspension/recovery, operation cancellation, terminal failure. |

Assert security input dispatch precedes operation/application dispatch, application FocusOwner remains exact,
undersized accepts no trust/secret key, recovery resumes pending input, raw secret never enters model/render/log,
and every cancel/failure completes cleanup. Native checks below complement but do not replace this matrix.

Native terminal verification records target/fingerprint readability, keyboard decision, cancellation and
terminal restoration. Trust and password/passphrase prompts remain outside generic modal enum and preempt
operation/application dispatch while preserving FocusOwner. Exercise normal/reduced reflow; at undersized,
assert security input remains pending and hidden behind minimum/Help/safe Quit until resize. Resize never accepts.

## Test Classification

- New multipanel/layout/focus/overflow/conflict/operation behavior follows red-first tests.
- Existing host-key decisions, paste isolation/limits, credential masking/lifecycle, signals, exit codes and
  terminal cleanup are characterization regressions and must pass at baseline and after implementation.
- New security-input ownership, operation precedence, responsive reflow and undersized suspension are red-first.
- Never weaken preserved behavior to manufacture a failing baseline test.

## Render-History Memory Protocol

Using long Tree/Details/Help/error payloads, execute 10,000 renders and 10,000 resize events cycling the twelve
SC-004 dimensions. Measure retained memory after GC against a warmed baseline. Pass when frame/resize-history
growth is <1 MiB, no prior-frame/event collection exists and every emitted frame stays within current bounds.

## Usability Protocol (SC-008)

Recruit 10 participants who neither contributed to nor saw the redesign. Use usability catalog at 100x30,
`Prod` selected, Tree focused. Start timer on first complete frame; without moderator help, ask participant to
identify selected element, Details information, focused region and two available actions. Stop after all four.

Ask exactly:

> La organización visual permite entender rápidamente el contexto y las acciones disponibles.

Use 1 (totalmente en desacuerdo) to 5 (totalmente de acuerdo). Pass when at least 8/10 finish in ≤10 seconds
and score 4/5. Record protocol, anonymous times/responses and aggregate in `usability-study.md`; no personal data.

## Threat Review and Native Boundaries

Review untrusted catalog/backend/environment/control bytes, async duplicates/stale completion, concurrent
mutation, malicious SSH peer/host keys, secret surfaces and bounded output. Verify no shell/protocol/schema/
telemetry/persistent-UI change.

On Linux, Windows and macOS native artifacts, record OS, architecture, emulator and result at all matrix sizes,
no-color, forms, every modal kind, conflict, loading/cancel, host trust and undersized recovery. Native manual
evidence supplements automated gates and cannot replace or waive them.

## Acceptance

- [TUI contract](contracts/tui.md) and SC-001–SC-017 matrices pass with recorded evidence.
- Existing paste, CAS, credential, SSH diagnostic, host trust, signal/exit and terminal cleanup regressions pass.
- Formatting, complete tests, race, vet, staticcheck, govulncheck, Nix and cross-builds pass.
- Required native and usability evidence is present; no pending/failed/unrun gate is called approved.
