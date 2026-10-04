# Implementation Plan: Host Port Forwarding

**Branch**: `main` (actual checkout; no branch created) | **Feature ID**: `016-add-port-forwarding` | **Date**: 2026-10-03 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/016-add-port-forwarding/spec.md`

## Summary

Add session-only Local, Remote, and Dynamic TCP forwarding on saved connections through a permanent TUI Tunnels panel and foreground CLI commands. Reuse existing trust, credentials, and `golang.org/x/crypto/ssh` rather than invoke OpenSSH. Each tunnel owns a separate transport, listener, client set, and cancellable runtime; shells remain independent and return to the TUI when finished. Remote endpoints are explicitly requested, not verified, with a persistent scope warning.

Planning ends after research and interface/data design. No application code, catalog migration, dependency change, or task list is part of this command.

## Technical Context

**Language/Version**: Go 1.26.0 module, toolchain Go 1.26.6; preserve the pinned Nix toolchain and vendoring.

**Primary Dependencies**: Existing `golang.org/x/crypto v0.56.0`, Bubble Tea v2.0.8, Bubbles v2.1.1, Lip Gloss v2.0.5, Cobra v1.10.2, and standard-library `net`, `io`, `context`, and synchronization. No new dependency planned. A bounded internal SOCKS5 CONNECT parser implements only the approved application-protocol subset, not SSH or cryptography.

**Storage**: Existing SQLite catalog and native credential stores are read/reused through existing services. Tunnel configuration, snapshots, and diagnostics are memory-only. No schema changes or persistent tunnel profiles.

**Testing**: Go unit, race, fuzz, and in-process SSH integration tests; TUI exact geometry/action inventories and golden frames; controlled OpenSSH and native terminal validation. Complete repository gates remain mandatory before merge.

**Target Platform**: Linux, macOS, Windows; amd64 and arm64. VT-capable keyboard-only terminal, full layout 80x24, usable reduced layout 40x12, existing below-minimum protections.

**Project Type**: Terminal application with Cobra CLI, Bubble Tea TUI, application services, and SSH adapters.

**Performance Goals**: Preserve streaming without payload retention; two simultaneous tunnels and two simultaneous clients per tunnel as minimum acceptance. Release locally owned listening ports within 2 seconds after stop/cancel/failure completion. Keep browser updates nonblocking, coalesce runtime state, and use bounded copying buffers. User task completion and comprehension targets remain SC-001/002.

**Constraints**: Trust before secrets; no unattended trust decisions or native credential-store UI; no shell/PTY for forwarding; loopback defaults; no automatic reconnect; no lifetime timeout for healthy idle tunnels; no cross-process management; accepted remote binding scope is unverified. Startup is explicitly cancellable with no new fixed deadline, consistent with current shell startup; do not inherit exec's five-minute command lifetime. A silent server may remain Starting until cancellation, with visible cancel/quit controls.

**Scale/Scope**: One foreground tunnel per CLI invocation. TUI supports up to 16 Starting/Active/Stopping tunnel runtimes; each has at most 64 admitted client-work slots including pending channel opens and incomplete teardown. Remote overload permits one additional rejection/cleanup worker, never unbounded per-overflow goroutines. Keep at most 32 terminal-state snapshots in addition to live tunnels, one latest safe diagnostic per tunnel, and no payload or event history. Admission failure is explicit and must not evict a live tunnel. SOCKS negotiation and destination establishment use 10-second per-client deadlines; established traffic has no idle deadline. Uncooperative SSH open/close convergence triggers failure of that transport after the bounded grace in the runtime contract rather than accumulating workers indefinitely.

## Constitution Check

*GATE: Evaluated before research from the specification and again after design. Both evaluations PASS; no intentional exceptions.*

| Principle or constraint | Before Research | Post-Design Evidence |
|---|---|---|
| I. Secure SSH Operations | PASS: existing trust/authentication rules required | Shared verified-host gate; library handshake; secrets outside rendered models; literal listening addresses; explicit exposure consent; unverified remote scope warning |
| II. Terminal-First Usability | PASS: keyboard, confirmation, cancellation, minimum size required | Permanent Tunnels panel, separate focus key map, 40x12 compact layout, cancel-first confirmations, unchanged security input owners and action legend |
| III. Predictable State and Failure Handling | PASS: explicit lifecycle and cleanup required | Per-tunnel transport owner, session-root lifetime, attempt tokens, independent stop, forced transport close for blocked SSH requests, worker joining and deterministic snapshots |
| IV. Behavior-Focused Testing | PASS: all modes and principal failures required | Protocol fixture, shutdown races, secret canaries, shell coexistence, input/focus/resize contracts, SOCKS fuzzing, native/server-policy validation |
| V. Simplicity and Maintainability | PASS: no unrequested durable state | Existing packages/dependencies; small manager only for required concurrent lifetimes; no pooling, daemon, migration, generic proxy framework, or custom SSH protocol |
| Operational constraints | PASS: safe errors, bounded output, trusted local state | Fixed-size diagnostics and snapshots, literal listening IPs, bounded worker slots, no shell interpolation, no payload logs, README/platform updates before release |
| Workflow and quality gates | PASS: approved spec and applicable checks | Traceability below; formatting, full tests/race/vet/staticcheck/security/CI gates remain required during implementation |

## Project Structure

### Documentation (this feature)

```text
specs/016-add-port-forwarding/
  spec.md
  plan.md
  research.md
  data-model.md
  quickstart.md
  checklists/requirements.md
  contracts/cli.md
  contracts/tui.md
  contracts/runtime.md
```

`tasks.md` is generated later by `/speckit.tasks`, not by planning.

### Source Code (repository root)

```text
cmd/orza/main.go                    dependency assembly and process signal ownership
internal/credential/                immutable no-native-UI store policy before recovery
internal/app/
  ports.go, types.go, bootstrap.go  forwarding request/runner/service injection
  connect.go, command.go           shared security preparation without behavior drift
  tunnel.go                       proposed forwarding service/session manager
internal/sshclient/
  client.go, session.go, auth.go   terminal-free authenticated transport extraction
  forwarding.go                   proposed listener/dial/copy runtime
  socks.go                        proposed narrow SOCKS5 negotiation
internal/cli/
  root.go, exit.go, output.go      service injection, existing safe results
  tunnel.go                       proposed foreground mode subcommands
internal/tui/
  model.go, state.go, modal_state.go, services.go
  layout.go, actions.go, viewport.go, session.go, run.go
  tunnels.go, tunnel_form.go       proposed panel/form and runtime message integration
tests/integration/
  ssh_server_test.go              existing shell/exec fixture kept compatible
  forwarding_test.go             proposed multi-transport forwarding fixture/tests
README.md                         CLI/TUI support, limitations, exposure guidance
```

**Structure Decision**: Extend existing application ports and adapters rather than introduce another architectural layer. New filenames are responsibilities, not a requirement to split trivial helpers into additional packages. Keep SSH library details inside `internal/sshclient`; the TUI receives snapshots and commands, never transports, payloads, or secret bytes.

## Phase 0: Research

[research.md](research.md) resolves the initial unknowns: terminal-free transport reuse, trust/secret presentation, blocked forwarding cancellation, SOCKS compatibility, lifetime ownership, shell results, live layout geometry, endpoint grammar, CLI confirmation/JSON policy, and resource bounds. Decisions are grounded in current implementation and vendored library behavior. No unresolved research markers remain.

## Phase 1: Design

### Transport and application boundaries

Extract duplicated dial/config/handshake acquisition from `Client.Run` and `RunCommand` into a terminal-free owned transport helper. Preserve existing test seams and trust/authentication ordering. Reuse `hostVerificationGate` and extract only genuinely shared connection/security preparation from Connect/Command services; avoid a third copy or broad service framework. Add a forwarding runner port and request/result types separate from shell and command requests.

`TunnelService` resolves and checks captured ID/revision before network I/O, supplies interactive or non-interactive security callbacks, and starts an owned runtime. The session manager tracks immutable request snapshots and monotonic state versions. One SSH transport per tunnel avoids shared cancellation and global-request locks. Expose safe Start/Stop/Snapshot/Close operations; no generic bus or global daemon.

Before bootstrap, perform flag-aware tunnel preflight for unsupported JSON, explicit unattended mode, and missing terminal policy. Respect `--`, boolean values, and global-option placement using the same Cobra command schema; do not add a naive substring scan. Select an immutable native-store interaction option before creating the store and running credential recovery. Linux unattended mode skips Unlock, checks collection/item Locked state, and never executes returned Prompt operations; macOS uses actual Security.framework fail-without-authentication-UI options through a small native bridge if the pinned wrapper cannot express them; Windows generic credential reads/writes/deletes have no UI fallback. Apply this policy to recovery reads/deletes and verification as well as tunnel authentication. Required locked/approval-protected access fails unavailable without advancing unresolved saga state. Do not skip recovery or eagerly require the store when no credential access is needed.

### Forwarding and cleanup

Local: local TCP accept -> SSH destination channel. Remote: SSH remote listener -> local TCP destination dial. Dynamic: local accept -> bounded SOCKS5 negotiation -> SSH destination channel. Drain remote accepts promptly and handle clients independently. Preserve bidirectional data and half-close where supported.

Cancellation must close the raw SSH socket independently of blocked channel writes, `Listen`, listener `Close`, or graceful transport close. Start an independent 250ms raw-close watchdog immediately on entering Stopping, before any potentially blocking SSH cleanup. Use an owned channel-open worker, not untracked repeated calls to vendor `DialContext`: retain its admission slot through underlying open and teardown convergence even when the client's 10-second wait expires. Allow 1 second of convergence after expired opens or client cleanup; an SSH peer that still withholds protocol progress makes that tunnel transport unhealthy and is failed/closed, unlike an ordinary destination refusal. Late successful connections remain owned and are closed. Full raw close unblocks remaining work. Silent idle partitions have no detection deadline; detected loss closes local listeners.

Stop requested during startup transitions to Stopping and prevents any late activation. Snapshot Stopped/Failed only after local cleanup completes. Remote cancellation is best-effort within the independently armed 250ms grace. The vendor listener accepts protocol channels before application admission, so the 64-slot cap bounds application-owned work, not all transient vendor channel state against a malicious SSH server. Use a single serialized overflow cleanup path, retain teardown slots, and fail the affected transport if close/read workers do not converge within 1 second. A configured, trusted SSH endpoint is the normal threat boundary; document residual transient vendor buffering rather than claim a hard bound the API cannot provide. No custom SSH multiplexer or protocol reimplementation is introduced. Test withheld close replies and vendor accept-queue backpressure under shutdown. Authentication acquisition must have closeable agent resources available before blocking RPCs.

### TUI ownership and design

Attach manager lifetime to the TUI session root, never the short-lived `operation` context that `completeOperation` cancels. Only one foreground startup/security interaction owns terminal input at a time; existing active tunnels continue independently. Cancellation is attempt-scoped. Snapshot updates use bounded/coalesced delivery; the runtime never waits for Bubble Tea to consume every event. Resynchronize after `tea.Exec` returns.

Use the existing visual tokens and border/viewport language. At 80x24 keep Tree left, Details above Tunnels on the right; at 40x12 stack three 3-row panels plus a maximum 3-row borderless legend. A panel's title/count remains visible; a single content row is usable through selection/scrolling, with full inspection in a scrollable modal. At larger sizes distribute space in fixed proportions independent of focus: equal heights for stacked panels, or a 40%/60% left/right split with equal right-column panel heights. Replace the live model's fixed five-row lower control allocation, not just the unused browser calculator. Never render an Actions title or panel.

Keyboard focus is Tree -> Details -> Tunnels. `p` opens forwarding for a selected host; `t` focuses Tunnels; Tunnels owns Enter inspect, `s` stop, `r` retry, and `d` dismiss terminal-state entries. Catalog mutation keys are not active in Tunnels focus. Forms reuse Bubbles editing and existing secret owners. See [contracts/tui.md](contracts/tui.md) for exact behaviors.

Keep `tea.Exec` shell handover and restoration. Change active TUI shell completion from `tea.Quit` to browser resumption with safe shell outcome notice; nonzero shell results and transport failures do not become a later TUI process exit status. `orza connect` remains unchanged and preserves remote exit status. Actual root cancellation/signals still close tunnels and exit rather than resume.

### CLI contract

Add `orza tunnel local|remote|dynamic PATH_OR_ID`, using separate listening address/port fields and a bracket-aware destination endpoint. Interactive invocation confirms the target and uses cancellable trust/secret input. `--non-interactive` explicitly opts into no-prompt behavior including native credential access/recovery; without a terminal it is required. Reject `--json` in pre-bootstrap preflight, not merely in RunE after recovery, rather than invent a streaming JSON contract. Messages go to stderr; stdout remains empty. See [contracts/cli.md](contracts/cli.md).

### Verification and traceability

| Requirements | Planned validation |
|---|---|
| FR-001/002/005/011 | All modes, endpoint parsing, DNS side, byte integrity, two tunnels/clients, admission limits |
| FR-003 | Traffic during active shell; normal/nonzero shell completion returns to browser; direct CLI connect unchanged |
| FR-004/010/016 | Form defaults, empty/overflow panel, exact focus/action dispatch, 80x24/40x12, resize/no-color, golden frames |
| FR-006/007/008 | Exposure choice/reset, pinned revisions, trust-before-secret, revoked/unknown/changed hosts, locked/approval-protected native stores and recovery, redaction |
| FR-009/012/013/014 | Transition/cancel races, independent stop, unavailable destination, silent global reply, blocked channel write, withheld close, accept backpressure, local port reuse, root signals |
| FR-015 | Refusal vs accepted/unverified scope, requested endpoint label, OpenSSH GatewayPorts policy checks |
| FR-017/018/019/020 | Catalog mutations, foreground CLI modes/results, pre-bootstrap rejection/no-UI policy, no persistence, bounded snapshots/errors, documentation |

Extend the controlled SSH fixture: current tests accept session channels only and discard global requests. New fixture must handle direct/forwarded TCP channels and request/cancel remote listeners, including delayed/denied responses and multiple transports. Do not replace shell/exec coverage with tunnel-only fixtures.

Implementation must run `go test ./...`, `go test -race ./...`, `go vet ./...`, repository formatting/static/security gates, and `nix flake check --no-update-lock-file --keep-going -L`. Native platform and controlled OpenSSH checks supplement cross-builds. Quickstart defines end-to-end acceptance and a 10-operator timing/comprehension evaluation for SC-001/002, not a claim that unimplemented commands already run. Post-design PASS refers to design compliance; all implementation and release validation remains outstanding.

## Complexity Tracking

No constitutional violations or exceptions. The only new runtime owner and bounded concurrency exist because independently stoppable concurrent tunnels must survive TUI shell ownership. Per-tunnel transports reject pooling as unnecessary complexity; the narrow SOCKS parser rejects a general proxy dependency because only no-auth CONNECT is in scope.
