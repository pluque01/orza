---
description: "Executable implementation tasks for session-only SSH host port forwarding"
---

# Tasks: Host Port Forwarding

**Input**: Design documents from `specs/016-add-port-forwarding/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Required by FR-020 and constitution Principle IV. Add the specified tests before the corresponding behavior, demonstrate failure for the missing behavior, and make them pass before the phase checkpoint. Native/manual validation supplements automated coverage; it must not be reported complete without execution.

**Organization**: Four story phases in priority order. Shared correctness belongs in Foundation; local creation and minimum safe stop/exit form the US1 increment. US2 expands management, recovery, and shell coexistence rather than making US1 depend on a later safe-shutdown task.

## Format: `[ID] [P?] [Story] Description`

- Every executable task has an unchecked checkbox, sequential ID, exact target paths, and a story label only inside a story phase.
- `[P]` means independent file ownership within the stated ready batch, not permission to skip prerequisites. Unmarked tasks are sequential by default.
- New files below are proposed implementation/test files; reuse an existing nearby file when that avoids an unnecessary split, while preserving the task's behavior and verification.

## Path Conventions

All source paths are repository-relative. Existing Go module, vendored dependencies, Nix checks, application services, CLI, and TUI are extended in place. Do not initialize a second module, add a proxy framework, modify vendored SSH code, or introduce tunnel persistence/catalog migrations.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish a reproducible baseline and controlled test support, without accessing production systems.

- [X] T001 Verify the Go 1.26.6/Nix development environment and baseline `go test ./...`, inspect existing worktree changes, and record actual command outcomes or blockers in specs/016-add-port-forwarding/quickstart.md; preserve go.mod, go.sum, vendor/, and flake.nix unless an independently justified correction is required.
- [X] T002 Create an isolated multi-transport SSH fixture in tests/integration/forwarding_server_test.go with stable transport identifiers, direct-tcpip channels, configurable denied/delayed/absent replies, disposable destinations, and deterministic cleanup; reuse session handling from tests/integration/ssh_server_test.go without removing existing shell/exec coverage.
- [X] T003 [P] Add controllable transport/listener/channel/clock test doubles in internal/sshclient/forwarding_test.go that can stall writes, opens, and closes and expose owned-worker convergence; avoid sleeps as the primary synchronization mechanism.

**Checkpoint**: Disposable fixtures and adapter doubles are available; no feature behavior is exposed yet. T002 and T003 may run concurrently after T001.

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared request validation, security, lifecycle, and resource ownership needed by every mode and interface.

- [X] T004 Add shared forwarding types and runner ports in internal/app/types.go and internal/app/ports.go, retaining the exact model constraints "`local`, `remote`, or `dynamic`", "Required endpoint", "Required for local/remote; absent for dynamic", "Immutable confirmed snapshot", "Explicit consent for the exact non-loopback request", and "Interactive callbacks or deliberately selected no-prompt execution"; separate forwarding from SSHSessionRequest/SSHCommandRequest.
- [X] T005 [P] Add endpoint/request table tests in internal/app/tunnel_validation_test.go for "Integer 1-65535; zero is invalid", "Listen: canonical IP literal. Destination: hostname or IP literal", "Listening or destination", "Local computer or saved host, derived from mode", and "Joined host/port with IPv6 brackets, sanitized and never shell-interpolated"; cover invalid domains, mapped IPv6 loopback, consent, and mode-dependent destination presence.
- [X] T006 [P] Add transport-opening and agent-cancellation regression tests in internal/sshclient/client_test.go and internal/sshclient/auth_test.go proving no session/PTY/terminal acquisition, verification before authentication, selected-method-only behavior, raw close on canceled handshake, and unblocking a nonresponsive owned agent connection without changing shell/exec semantics.
- [X] T007 Implement normalized endpoint/request validation in internal/app/tunnel.go using net.ParseIP/SplitHostPort/JoinHostPort: default "127.0.0.1", normalize localhost, keep explicit IPv6, reject whitespace/control/NUL/bracket/port ambiguity, require separate consent for non-loopback, and never silently replace unavailable families/bindings/ports; satisfy T005.
- [X] T008 Extract terminal-free owned authenticated transport acquisition in internal/sshclient/client.go and internal/sshclient/session.go and register closeable auth resources before blocking agent RPCs in internal/sshclient/auth.go; preserve Client.Run/RunCommand behavior and test seams, and satisfy T006.
- [X] T009 Add application security/request tests in internal/app/tunnel_test.go for "Existing stable catalog ID; selector is resolved before confirmation", "Captured expected revision checked before network I/O", "Captured display path, not a later lookup that could relabel the running target", "Confirmed SSH access target; use existing safe display rules", and "Existing selected method and non-secret references, not secret bytes"; assert stale target rejection, revoked/unknown/changed identities, verified-host secret gating, and redaction.
- [X] T010 Implement shared forwarding security preparation in internal/app/tunnel.go with narrowly extracted reuse from internal/app/connect.go and internal/app/command.go; re-resolve captured ID/revision before network I/O, reuse hostVerificationGate and credential policy, preserve existing trust precedence, and satisfy T009 without copying a third trust implementation.
- [X] T011 Add manager/lifecycle/snapshot tests in internal/app/tunnel_manager_test.go for "Monotonic session-local ID; never reused within the session", "Monotonic startup token used to reject stale results", "Distinguish canceled starts, explicit retries, and stale notifications", "Starting, Active, Stopping, Stopped, or Failed", "Immutable confirmed request, excluding authentication secrets", "Optional latest safe failure context, not a history", and "Stable list order, independent of catalog selection"; include late activation, idempotent stop, root close, and final-state-after-cleanup assertions.
- [X] T012 Implement the session-only manager in internal/app/tunnel.go with Start/Stop/Retry/Snapshot/Dismiss/Close, immutable versioned snapshots, 16-live/32-terminal caps, oldest-completed eviction only, retry retaining TunnelID with a fresh AttemptID, Active-only counts, Starting/Stopping included in exit scope, and authoritative/coalesced state independent of UI consumption; satisfy T011.
- [X] T013 Add cancellation/resource-bound tests in internal/sshclient/forwarding_test.go for 64 admitted slots retained through open/copy/teardown convergence, 10-second destination wait, 1-second uncooperative-peer grace, watchdog armed before blocked SSH close, 250ms forced raw-close fallback, late successful results, and no fixed startup or healthy-idle lifetime timeout.
- [X] T014 Implement common owned-runtime cleanup, slot accounting, transport-loss watcher, safe diagnostic mapping, and forced-close watchdog in internal/sshclient/forwarding.go; join owned workers before final state and satisfy T013; preserve ordinary destination-failure isolation, use one latest diagnostic, and enforce "at most 256 Unicode code points" with allowlisted single-line detail stripped of control/ANSI/bidirectional-control characters and no raw errors, secrets, references, identity paths, or payload history.
- [X] T015 Wire the shared forwarding service/runner and manager lifecycle through internal/app/bootstrap.go, internal/tui/services.go, cmd/orza/main.go, and internal/cli/root.go, passing it through RootConfig into tui.Config; add a production-root injection assertion in internal/cli/root_test.go so the actual no-command launcher receives it; keep construction lazy, dependency teardown closeable, TUI session-root lifetime independent of completeOperation's context, and no CLI tunnel command required yet.

**Checkpoint**: T004-T015 pass together, shell/exec regression coverage remains green, and all user stories can use tested transport/security/ownership boundaries. Native unattended credential policy is implemented in US4, not needed to delay the interactive US1 increment.

## Phase 3: User Story 1 - Access a Service Through a Local Tunnel (Priority: P1)

**Goal**: Configure, confirm, start, access, and safely stop a Local tunnel through the TUI without a shell.

**Independent Test**: On a saved disposable host, create a loopback local tunnel, exchange request/response data including two clients, stop it with confirmation, verify port reuse, and recover from invalid/occupied ports without restarting Orza. No Remote/Dynamic or CLI forwarding is needed.

### Tests for User Story 1

- [X] T016 [P] [US1] Add Local adapter tests in internal/sshclient/local_forwarding_test.go for direct-tcpip destination DNS on the host, byte integrity, two simultaneous clients, half-close response preservation, listener conflict, destination refusal without listener death, transport loss, and no session channel/terminal mutation.
- [X] T017 [P] [US1] Add Local form/panel/action tests in internal/tui/tunnel_form_test.go and internal/tui/tunnels_test.go for Local default, field error retention, separate exposure consent, dirty draft Discard/Cancel, cancel-first target/stop/exit confirmation, no Actions panel, and minimum keyboard workflow.
- [X] T018 [P] [US1] Add Local end-to-end/security tests in tests/integration/local_forwarding_test.go using T002 for valid/rejected trust and credentials, startup cancellation/late success, unchanged service responses, occupied/unauthorized binding, secret canaries, and listener reuse within 2 seconds after completion.

### Implementation for User Story 1

- [X] T019 [US1] Implement Local listener/destination-channel forwarding in internal/sshclient/forwarding.go using the owned transport/runtime, bounded two-way copy and CloseWrite where available; keep pending workers/slots owned on timeout, never use an untracked canceled vendor DialContext worker, and satisfy T016.
- [X] T020 [US1] Add a Local forwarding draft in internal/tui/tunnel_form.go with captured host, listen address/port and destination fields, machine-direction guidance, keyboard traversal/help, Ctrl-S Start rather than Save, and exact draft rule "Changing mode, listening address/port, or selected host clears prior exposure acknowledgement"; expose only implemented modes until US3.
- [X] T021 [US1] Integrate confirmation and startup/security ownership in internal/tui/model.go and internal/tui/session.go using existing trust/secret readers, operation tokens, and a separate session-root runtime lifetime; emit Starting immediately, cancel only the current attempt, reject stale results, and preserve drafts/focus on failure without rendered secret bytes.
- [X] T022 [US1] Add the minimum permanent Local Tunnels panel in internal/tui/tunnels.go and extend live layout in internal/tui/layout.go and internal/tui/model.go to Tree-left/Details-over-Tunnels at 80x24 and three 3-row panels plus at most three borderless legend rows at 40x12; include active count, empty guidance, selected row, inspection, confirmed stop, and safe exit without an Actions panel.
- [X] T023 [US1] Add minimum focus-scoped p/start, t/focus, Tab/previous, Enter/inspect, s/stop and safe Quit descriptors/dispatch in internal/tui/actions.go, internal/tui/state.go, and internal/tui/modal_state.go; suppress catalog mutation keys in Tunnels focus and keep printable keys/paste isolated inside form/security owners.
- [X] T024 [US1] Connect successful Local startup and confirmed stop/exit to the TUI manager in internal/tui/model.go and internal/tui/run.go, restore stable browser state, join all owned work on TUI Run failure/root termination, and make T017/T018 pass without depending on later US2 management work.
- [X] T025 [US1] Run the US1 tests plus existing app/SSH/TUI regression suites, manually demonstrate the Local-only independent test, and document results and any incomplete full-feature modes in specs/016-add-port-forwarding/quickstart.md; do not claim full-feature acceptance at this checkpoint.

**Checkpoint**: Safe Local-only MVP works end to end. The permanent panel/basic stop/exit already exists; later phases extend it. Existing shell completion behavior is changed in US2, so this is an incremental development checkpoint, not completion of the full approved specification.

## Phase 4: User Story 2 - Monitor and Stop Tunnels Safely (Priority: P1)

**Goal**: Manage concurrent tunnels across catalog selection, retry and dismiss terminal states, survive a separate shell, and exit deterministically.

**Independent Test**: Start two Local tunnels on distinct ports, change selected/filter-visible hosts, inspect all entries, stop one, cause the other's transport loss, retry through fresh confirmation, and exercise shell return and safe exit. Remote/Dynamic/CLI modes are not prerequisites.

### Tests for User Story 2

- [X] T026 [P] [US2] Add management/catalog-change integration tests in tests/integration/tunnel_management_test.go for two independent tunnels, stopping one without affecting the other, pinned running targets after edit/move/delete, same-ID/current-revision retry, missing-record retry, capacity refusal, bounded terminal eviction, and zero tunnel persistence across relaunch.
- [X] T027 [P] [US2] Extend exact geometry/action/focus/overflow/no-color/undersized preservation tests in internal/tui/layout_test.go, internal/tui/responsive_layout_test.go, internal/tui/actions_test.go, internal/tui/focus_test.go, and internal/tui/resize_state_test.go for permanent panel visibility, independent selections, complete scrollable inspection, and contextual stop/retry/dismiss behavior at 80x24/40x12 and during resize.
- [X] T028 [P] [US2] Add TestTunnelFailureDuringShellReconcilesOnReturn in tests/integration/tunnel_shell_test.go plus session regressions in internal/tui/runtime_convergence_test.go and internal/tui/session_test.go for ongoing traffic during tea.Exec, normal/nonzero shell return, shell-only failure recovery, and fresh failed tunnel state after return.
- [X] T029 [P] [US2] Add manager paused-consumer/exit-race tests in internal/app/tunnel_manager_test.go and internal/tui/tunnel_lifecycle_test.go for coalesced updates, bounded diagnostics during long shells, Starting/Stopping exit scope, Cancel exit preserving work, handled signals bypassing dialogs, stale retries, and worker joins on TUI failure.

### Implementation for User Story 2

- [X] T030 [US2] Extend the permanent panel/inspection in internal/tui/tunnels.go with cross-host creation-order entries, independent TunnelID selection, bounded terminal snapshots, empty/overflow guidance, textual lifecycle/error/warning values, and exact snapshot constraints "Always retained; remote label is explicitly Requested", "Actual local listener address, only for local/dynamic; never infer remote verification from `Addr()`", and "Controlled remote uncertainty or non-loopback/no-auth exposure text".
- [X] T031 [US2] Implement retry/dismiss/inspection and expanded focus-scoped key maps in internal/tui/actions.go and internal/tui/model.go: retry only Stopped/Failed via current saved ID/revision and fresh consent, dismiss only terminal state, restore both catalog/tunnel selection after overlays, and preserve other live runtimes during all recovery actions.
- [X] T032 [US2] Complete exit/stop confirmation scope and draft-versus-live-resource decisions in internal/tui/model.go and internal/tui/modal_state.go; confirmations identify captured tunnel targets and start on Cancel, Starting cancellation never activates late, and explicit Quit with live tunnels cancels or closes all before returning.
- [X] T033 [US2] Change active TUI shell completion in internal/tui/session.go and internal/tui/run.go from tea.Quit to restored browser state with a safe outcome notice and latest manager snapshot; keep root cancellation as exit, do not leak earlier shell status into later TUI exit, and preserve direct CLI connect remote-status behavior in internal/cli/root.go.
- [X] T034 [US2] Update live responsive geometry/legend packing and focused-region sizing in internal/tui/layout.go, internal/tui/model.go, and internal/tui/actions.go plus affected frames under internal/tui/testdata/; ensure one navigable content row per panel at 40x12, full values via inspection, existing below-minimum protections, and no hidden separately opened tunnel list.
- [X] T035 [US2] Harden manager/runtime reconciliation and complete teardown integration in internal/app/tunnel.go, internal/tui/model.go, and internal/tui/run.go so shell suspension never blocks forwarding/error cleanup, evictions move selection predictably, and all tests T026-T029 pass without automatic reconnect or payload history.
- [X] T036 [US2] Run the US2 independent test and shell/selection/layout regression suites, verify Local US1 remains green, and record results in specs/016-add-port-forwarding/quickstart.md; include port reuse and unrelated-tunnel continuity evidence rather than only cancellation calls.

**Checkpoint**: Local tunnels are fully manageable and survive shells. Shell/nonzero process-result regressions and no-color/minimum-size checks are validated before Remote/Dynamic integration.

## Phase 5: User Story 3 - Use Remote and Dynamic Forwarding (Priority: P2)

**Goal**: Add Remote fixed-destination forwarding and a local SOCKS5 CONNECT proxy with honest remote-scope warnings.

**Independent Test**: Verify Remote access to a local test service from the disposable SSH host, and Dynamic access to a host-side service using a proxy client; test remote refusal/unverified acceptance and ordinary destination failure independently of the CLI.

### Tests for User Story 3

- [X] T037 [US3] Extend tests/integration/forwarding_server_test.go with tcpip-forward/cancel requests, forwarded-tcpip connections, accepted-but-unverified bind scope, a withheld-close peer, missing global replies, and inbound flood/transport-close controls while preserving Local and shell fixture behavior.
- [X] T038 [P] [US3] Add Remote adapter tests in internal/sshclient/remote_forwarding_test.go using T003's package-local doubles for request acceptance/refusal, local destination dialing, prompt accept draining, 64 admitted slots plus one serialized overflow cleanup worker, blocked channel/global writes, 250ms forced raw close, and 1-second protocol-convergence failure; real vendor flood/withheld-reply behavior is covered by T040, not imported from another package's _test.go fixture.
- [X] T039 [P] [US3] Add SOCKS unit/fuzz tests in internal/sshclient/socks_test.go with FuzzSOCKSRequest for v5 offered no-auth, CONNECT-only, reserved zero, IPv4/domain/IPv6, ports 1-65535, malformed/truncated requests, unsupported methods/BIND/UDP, bounded replies, pipelined payload preservation, 10-second negotiation deadline, and success only after destination establishment.
- [X] T040 [P] [US3] Add Remote/Dynamic integration tests in tests/integration/remote_dynamic_forwarding_test.go using T037's package-local fixture, with a per-mode matrix of two non-conflicting tunnels, two simultaneous clients per tunnel, unchanged responses, independent stop, destination DNS side, and real vendor accept-queue/flood/withheld-close/global-reply teardown; add separate TUI mode tests in internal/tui/tunnel_modes_test.go for field visibility, draft retention, consent reset, unauthenticated-proxy warnings, and remote acceptance without falsely verified exposure.

### Implementation for User Story 3

- [X] T041 [US3] Implement Remote listening and local destination dialing in internal/sshclient/forwarding.go through existing SSH APIs, with prompt accepts, owned overload cleanup and half-close copy; expose "`local_bound` for local/dynamic, `unverified` for accepted remote forwarding" and never upgrade remote scope from Listener.Addr or channel metadata; satisfy T038.
- [X] T042 [P] [US3] Implement bounded SOCKS5 negotiation in internal/sshclient/socks.go using io.ReadFull, no-auth CONNECT only, protocol reply mapping/neutral bound address, unresolved host-side domains and preserved pipelined bytes; clear local negotiation deadline before traffic and satisfy T039 without introducing dependencies.
- [X] T043 [US3] Integrate Dynamic listener/client routing in internal/sshclient/forwarding.go through the existing owned SSH destination opener and slot budget, distinguishing client protocol/destination failure from tunnel failure and keeping other clients/listeners usable.
- [X] T044 [US3] Enable Local/Remote/Dynamic selection and mode-specific fields in internal/tui/tunnel_form.go, retaining applicable draft values but validating only the selected mode; hide fixed destinations for Dynamic, explain each machine's role, and reset exposure consent on mode/listener/target changes.
- [X] T045 [US3] Add persistent requested-endpoint/unverified-scope and unauthenticated-proxy text to internal/tui/tunnels.go, internal/tui/model.go, and internal/app/tunnel.go; accepted remote requests may become Active with warning even for loopback, while refusals/reported inability fail safely and raw server text never becomes a diagnostic.
- [X] T046 [US3] Run the Remote/Dynamic two-tunnel/two-clients-per-tunnel/independent-stop matrix, blocked-peer/shutdown/half-close tests and `go test ./internal/sshclient -run '^$' -fuzz '^FuzzSOCKSRequest$' -fuzztime=30s`, verify all prior story tests remain green, and record actual outcomes in specs/016-add-port-forwarding/quickstart.md.

**Checkpoint**: All three forwarding modes are independently usable in the TUI. No CLI forwarding or false remote exposure guarantee is needed to demonstrate this story.

## Phase 6: User Story 4 - Start a Tunnel from the CLI (Priority: P3)

**Goal**: Expose all modes as foreground CLI commands with explicit unattended policy, no native dialogs, safe results, and deterministic interruption.

**Independent Test**: Start each mode using a saved host, check stderr readiness and empty stdout, exchange service data, interrupt and verify cleanup/result; separately prove pre-bootstrap validation and locked/unapproved credential/trust failures never prompt.

### Tests for User Story 4

- [X] T047 [P] [US4] Add command/argument/output/result tests in internal/cli/tunnel_test.go for exact contracts/cli.md syntax, one selector/runtime, required numeric listen port, fixed-mode destination/Dynamic rejection, exposure flag, interactive confirmation, explicit unattended choice, unsupported JSON, stderr-only readiness/warnings, and documented exit mappings without remote-command codes.
- [X] T048 [P] [US4] Add pre-bootstrap tests in cmd/orza/main_test.go for global flag placement, explicit boolean values, `--` handling, unsupported JSON and missing no-terminal policy rejected before recovery/acquisition, immutable policy selected before store construction, and SIGINT/SIGTERM 130/143 only after cleanup.
- [ ] T049 [P] [US4] Add native no-UI policy tests in internal/credential/store_linux_test.go, internal/credential/store_darwin_test.go, and internal/credential/store_windows_test.go for locked/approval-required access, Linux never Unlock/Prompt, macOS fail-without-UI rather than skip matches, Windows no CredUI fallback, and verification/mutation access under the same immutable policy.
- [X] T050 [P] [US4] Add unattended recovery/security tests in internal/app/credential_saga_test.go and tests/integration/tunnel_cli_test.go for locked native stores, unavailable-not-absent semantics, pending recovery retained, no prompt callbacks/dialogs, preapproved agent/remembered-secret success, revoked/unknown/changed host failure, and secret/redaction canaries.

### Implementation for User Story 4

- [X] T051 [US4] Add shared Cobra command-schema-aware tunnel preflight in internal/cli/tunnel.go and call it before bootstrap in cmd/orza/main.go; respect global flag positions/booleans/`--`, require explicit --non-interactive without a terminal, reject --json before mutation or secret acquisition, and return an immutable interaction policy rather than a substring scan.
- [X] T052 [US4] Add the immutable native-store no-user-interaction option in internal/credential/store.go and pass it from preflight through cmd/orza/main.go and internal/app/bootstrap.go before credential recovery; keep normal interactive defaults, recovery enabled, and lazy store access unchanged for other commands.
- [X] T053 [P] [US4] Implement Linux no-UI access in internal/credential/store_linux.go with collection/item Locked checks, no Unlock call, and dismissal/rejection of returned Prompt objects without execution for Get/Set/Delete/verification; preserve NotFound versus ErrUnavailable and satisfy Linux T049 coverage.
- [ ] T054 [P] [US4] Implement macOS no-UI query/mutation/verification in internal/credential/store_darwin.go and, only if the pinned wrapper cannot express native constants, a narrow bridge in internal/credential/store_darwin_ui.go; use Security.framework fail-without-authentication-UI, preserve no-cgo unavailable behavior in internal/credential/store_darwin_nocgo.go, and satisfy Darwin T049 coverage.
- [ ] T055 [P] [US4] Propagate the immutable option without native UI fallback in internal/credential/store_windows.go using existing generic credential APIs, preserve native missing/unavailable mapping, and satisfy Windows T049 coverage.
- [X] T056 [US4] Implement local/remote/dynamic foreground subcommands, flag validation, current-target capture/confirmation and service injection in internal/cli/tunnel.go and internal/cli/root.go; interactive trust/input must honor context cancellation, unattended mode supplies no prompt callbacks, and --acknowledge-exposure never approves trust or skips ordinary interactive target review.
- [X] T057 [US4] Implement controlled stderr progress/readiness/coalesced failure output and documented process mappings in internal/cli/tunnel.go, internal/cli/exit.go, and internal/cli/output.go; stdout stays empty, Remote labels Requested listener with scope warning, normal destination failures keep forwarding, and prior shell/exec identifiers/results remain unchanged.
- [X] T058 [US4] Complete foreground owner/root-signal lifecycle in cmd/orza/main.go and internal/app/tunnel.go, join listener/client/agent/native-input cleanup before command return, fail inaccessible recovery without skipping or falsely completing it, and make T047-T050 plus existing CLI/root/bootstrap/security regressions pass.
- [X] T059 [US4] Run all three CLI modes in interactive and unattended conditions and the documented validation/signal/result cases, record platform-specific no-UI evidence or explicit unavailable-platform blockers in specs/016-add-port-forwarding/quickstart.md, and confirm TUI-managed tunnels are not discovered/modified by CLI processes.

**Checkpoint**: Foreground CLI matches the contract, including bootstrap ordering and native-store no-prompt behavior. Existing catalog, shell, and exec commands retain their contracts.

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Complete full-spec documentation, native interoperability, usability measurement, and release-quality gates.

- [X] T060 Update README.md with mode directions, endpoint/DNS grammar, CLI examples/options/results, foreground/session-only lifecycle, safe/non-loopback/no-auth proxy warnings, server policy and requested-not-verified scope, startup cancellation/no fixed deadline, resource limits/threat boundaries, panel/focus bindings and shell return; remove stale Actions-panel and unsupported-port-forwarding claims without broadening unrelated SSH support.
- [X] T061 [P] Audit redaction/no-persistence/bounded-state acceptance in tests/integration/security_test.go and tests/integration/forwarding_security_test.go, including long shell suspension, repeated failures, secret canaries, no payload/destination history, private existing catalog boundaries, and admitted-work versus transient malicious-server vendor buffering assumptions.
- [X] T062 Execute controlled OpenSSH policy/IPv6/half-close/remote-release checks from specs/016-add-port-forwarding/quickstart.md and record actual observations for GatewayPorts/AllowTcpForwarding/PermitOpen/PermitListen, requested-scope warnings, no fallback, and cleanup; never substitute synthetic acceptance for real server policy.
- [ ] T063 Execute native Linux/macOS/Windows terminal, resize, agent, locked credential-store/no-UI recovery, shell handover and handled-signal checks and document actual platform/architecture evidence or blockers in specs/016-add-port-forwarding/quickstart.md; cross-builds alone are not native acceptance.
- [ ] T064 Conduct the 10-operator timing/comprehension evaluation in specs/016-add-port-forwarding/quickstart.md for SC-001/002, with in-app guidance only and timing exclusions; record anonymous results, requiring at least 9/10 under two minutes and at least 9/10 fully correct direction/proxy answers, or explicitly leave blocked pending participants.
- [X] T065 Run repository formatting, `go test ./...`, `go test -race ./...`, `go vet ./...`, `nix flake check --no-update-lock-file --keep-going -L`, `nix develop --command bash scripts/check-vendor.sh`, `nix develop --command gitleaks dir --redact --config .gitleaks.toml .`, and `nix develop --command gitleaks git --redact --config .gitleaks.toml --log-opts=--all .`; perform supported CI cross-builds, preserve unrelated worktree changes (use an isolated working copy if vendor regeneration would overwrite them), fix owned regressions, and record exact outcomes/blockers in specs/016-add-port-forwarding/quickstart.md without bypassing hooks or claiming unrun checks passed.
- [X] T066 Review FR-001 through FR-020 and SC-001 through SC-008 against implementation, contract tests and actual acceptance results, document remaining release blockers and any justified exceptions in specs/016-add-port-forwarding/quickstart.md, and update this file's completed markers only for work genuinely verified; do not change reviewer-owned checklists/requirements.md to imply implementation success.

**Checkpoint**: Feature completion requires implemented story checks and cross-cutting evidence. Missing native environments, real server policy, or operator participants are explicit blockers for the corresponding validation task, not reasons to fabricate completion.

## Dependencies & Execution Order

### Phase Dependencies

```text
Setup (T001-T003)
  -> Foundation (T004-T015)
     -> US1 Local TUI (T016-T025)
        -> US2 management/shell (T026-T036)
        -> US3 remote/dynamic (T037-T046)
           -> US4 foreground CLI (T047-T059)
All desired stories -> Polish and full acceptance (T060-T066)
```

Priority execution is US1 -> US2 -> US3 -> US4. US2 and US3 have independent test oracles after US1; they can develop distinct tests/adapters concurrently, but their shared TUI/model/application integrations must be serialized. US4 can prepare its tests/no-UI platform adapters earlier once Foundation is stable, but complete three-mode CLI acceptance requires US3. Do not treat the graph as permission for conflicting edits.

### Task-Level Ordering

- T001 precedes the T002/T003 fixture batch. T004 establishes ports/types; T005/T006 are independent test batches. T007 follows T005; T008 follows T006. T009/T010 and T011/T012 are test-before-implementation pairs. T013/T014 follow T008/T012 and the adapter doubles. T015 follows all shared implementations.
- US1 T016-T018 test files can be authored together after Foundation; T018 uses the fixture. T019 implements adapter behavior; T020-T024 integrate form, runtime, panel, focus and cleanup in listed order because they share TUI files. T025 is the checkpoint.
- US2 T026-T029 are independent test batches. T030-T035 integrate in order, then T036 validates. Update existing shell-exit tests with browser-resumption oracles; retain direct connect exit tests.
- US3 T037 precedes T040 integration tests that need the remote fixture in the same tests/integration package. T038 uses T003's internal/sshclient-local doubles; T039 uses its own parser test input. T038-T040 use independent test files and can run together after T037. T041 and T042 can run concurrently after their tests; T043 integrates both. T044/T045 share TUI/application files and run sequentially. T046 validates the full per-mode concurrency matrix.
- US4 T047-T050 are independent test batches. T051/T052 establish preflight and immutable store options. Only then T053-T055 can run concurrently by platform. T056/T057/T058 integrate commands/output/lifetime after the required adapters, then T059 validates.
- T060-T064 follow the relevant implemented stories. T061 can be authored independently of README/manual evidence. T065 runs after all code/test changes; T066 reconciles actual evidence last.

### Parallel Opportunities

Ready batches marked `[P]` have different files and no incomplete dependencies inside that batch: T002/T003 after T001; T005/T006 after T004; T016-T018 after Foundation; T026-T029 after US1; T038-T040 after T037; T041/T042 after the US3 tests; T047-T050 after story/service contracts exist; T053-T055 after T052. A batch may include an unmarked lead task that another marked task can accompany; `[P]` is not needed on every first task.

Shared files such as internal/tui/model.go, internal/tui/actions.go, internal/app/tunnel.go, internal/sshclient/forwarding.go, and cmd/orza/main.go require a single writer or serialized integration. The quickstart evidence tasks also share one document and must not be run as simultaneous writers.

## Parallel Example: User Story 1

After Foundation, author T016 Local adapter tests, T017 form/panel tests, and T018 end-to-end tests together in their separate files. Complete those contracts before implementing T019-T024; do not parallelize those shared TUI edits.

## Parallel Example: User Story 2

After US1, author T026 management integration tests, T027 geometry/focus tests, T028 shell coexistence tests, and T029 lifecycle/paused-consumer tests together. T030-T035 remain sequential integration work.

## Parallel Example: User Story 3

After T037, author T038 Remote tests, T039 SOCKS fuzz/tests, and T040 mode acceptance tests together. Once those contracts are available, T041 Remote adapter and T042 SOCKS parser can proceed concurrently in internal/sshclient/forwarding.go and internal/sshclient/socks.go; T043 waits for both.

## Parallel Example: User Story 4

Author T047 CLI contracts, T048 process preflight, T049 native store tests, and T050 unattended recovery/security tests together. After T052, implement T053 Linux, T054 Darwin, and T055 Windows adapters concurrently with disjoint platform files; command integration waits for their completion.

## Implementation Strategy

### MVP First

1. Complete Setup and Foundation with existing shell/exec regressions passing.
2. Complete US1: Local forwarding, permanent minimum panel, confirmation, basic stop and safe exit.
3. Validate US1's independent service-access/port-reuse test and stop at a Local-only development demonstration if desired. Do not advertise all modes or full-spec completion before later phases.

### Incremental Delivery

1. Extend US1 with US2 cross-host management, bounded recovery/inspection, and shell return without tunnel interruption.
2. Add US3 Remote and Dynamic modes with honest server-scope warnings and bounded protocol handling.
3. Add US4 foreground CLI and no-native-UI credential policy selected before bootstrap.
4. Finish documentation, automated/native/server-policy and operator validation. Run the whole repository gates and reconcile completion evidence before release.

### Verification and Traceability

| Requirements / Criteria | Principal Tasks |
|---|---|
| FR-001/002/005; SC-003 | T004-T007, T016/T019/T020, T037-T046, T047/T056 |
| FR-003 | T028/T033/T035/T036, T063 |
| FR-004/010/016; SC-007 | T017/T020-T024, T027/T030-T034, T040/T044/T045 |
| FR-006/007/008; SC-004/006 | T005/T007-T010, T017/T018/T021, T040/T045, T047-T058, T061 |
| FR-009/011/012/013/014; SC-003/005/006 | T011-T014, T016/T018/T019/T024, T026/T029-T036, T038/T041/T043, T048/T058 |
| FR-015; SC-004 | T037/T038/T040/T041/T045/T057/T060/T062 |
| FR-017 | T009/T010/T012/T026/T031 |
| FR-018; SC-008 | T047-T059, T063 |
| FR-019 | T011-T014/T026/T029/T035/T050/T061 |
| FR-020 | Story test tasks, T060-T066 |
| SC-001/002 | T020/T044/T060/T064 |

## Notes

Implementation reconciliation on 2026-10-03: 61 of 66 tasks complete, 5 pending. Completed IDs: T001-T048, T050-T053, T056-T062, T065-T066. Pending IDs: T049, T054, T055, T063, T064. Checkbox completion records implemented and verified work, not task-list generation or full release acceptance; see quickstart.md for execution evidence and residual limitations. The extended native Linux/OpenSSH harness passed 130/130 checks: T025's actual Local TUI workflow is demonstrated by automated native process/PTY execution, not a human operator study; T026's relaunch oracle now uses the same HOME/XDG catalog across full application processes; T059's all-mode interactive/unattended CLI checks are complete with credential/native-platform blockers explicitly recorded as permitted by its wording. T063 still owns the incomplete native shell/agent/locked-store/platform matrix. T035/T058 record implemented reconciliation/teardown and passing available tests, not completion of T049's unexecuted native-platform tests. T065 records the available Linux gates and cross-builds with Darwin cgo/native blockers explicitly retained; T066 completes the evidence review, not the pending acceptance work. Story completion: US1 10/10, US2 11/11, US3 10/10, US4 10/13; Setup 3/3, Foundation 12/12, Polish 5/7. There are 22 `[P]` tasks. Existing task descriptions are preserved; implementation types/tests may live in nearby responsibility files as permitted above. No commit, staging, or branch change was requested or performed for this reconciliation. Reviewer-owned checklists remain read-only.
