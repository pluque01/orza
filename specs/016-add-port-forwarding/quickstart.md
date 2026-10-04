# Quickstart Validation: Host Port Forwarding

Local, Remote, and Dynamic forwarding commands, the permanent TUI Tunnels panel, and controlled SSH fixtures are implemented. The scenarios below are runnable; automated gates and controlled native Linux/OpenSSH CLI/TUI process/PTY workflows have passed, but the remaining native shell/agent/credential/platform matrix and human/operator evaluation remain pending. Implementation completion is not full release acceptance: [tasks.md](tasks.md) records 61/66 complete and the five pending tasks below.

## Execution Evidence

Evidence date: 2026-10-03, Linux/amd64 with Go 1.26.6 from Nix. This records the verified parent execution and native agent report supplied for reconciliation, checked against current source/tests and the retained native harness; it does not claim these commands were rerun during this documentation-only update. Plain `go` was unavailable at baseline. Existing worktree/staging changes were preserved; no commit, staging, branch change, dependency update, or reviewer-checklist edit was performed by this reconciliation.

| Task / Gate | Execution Evidence | Result and Scope |
|---|---|---|
| T001 baseline | `nix develop --command go version`; `nix develop --command go test ./...` before application changes | Go 1.26.6, linux/amd64; baseline passed. |
| T002 fixture | Controlled multi-transport direct-TCP/session fixture self-tests, `-run ForwardingFixture -count=1`, race `-count=20` | Passed; direct/forwarded channels, listener requests/cancellation, denied/delayed/absent replies and independent transport cleanup now exist in `tests/integration/forwarding_server_test.go` and its self-tests. |
| T003 doubles | Transport/listener/channel/manual-clock self-tests, `-run ForwardingDouble -count=1`, race `-count=100` | Passed; stalled operations, late results and worker convergence are observable without sleeps as the primary synchronization mechanism. |
| T004-T024, T027-T045, T047-T048, T050-T053, T056-T058, T061 | App/SSH/CLI/TUI unit and controlled real-SSH-protocol integration suites, included in full repository tests and races | Passed for the available Linux environment. Tests cover validation, trust-before-secret, pinned revisions, all modes, concurrency, cancellation, independent stop, bounded state, redaction, shell return and terminal restoration. Native gaps are not implied passed. |
| T013 exact timing | `internal/sshclient/forwarding_clock_test.go`, package tests and race detection | Passed: raw socket remains open at 249ms and is forced closed at exactly 250ms; convergence grace remains live at 999ms and fails at exactly 1000ms. Tests join timers/workers and cover every mode's shutdown, blocked protocol cleanup and expired opens. The destination context retains its real 10-second timeout. |
| T036/T040/T046 concurrency and shell | `TestLocalForwardingTwoTunnelsTwoClients`, `TestRemoteForwardingTwoTunnelsTwoClients`, `TestDynamicForwardingTwoTunnelsTwoClients`, management tests and `TestTunnelFailureDuringShellReconcilesOnReturn` | Passed: unchanged responses, two tunnels/two clients per tunnel, independent stop, released ports, unrelated traffic continuity during shell suspension, latest Failed state on return, and normal/nonzero/shell-only-failure recovery. |
| T039/T046 SOCKS fuzz | `nix develop --command go test ./internal/sshclient -run '^$' -fuzz '^FuzzSOCKSRequest$' -fuzztime=30s` | Passed, approximately 1.3 million executions in 30 seconds. The target exists in `internal/sshclient/tunnel_test.go`. |
| T050 unavailable recovery | `TestCredentialSagaRecoveryUnavailablePreservesPendingOperation` in `internal/app/credential_saga_unavailable_test.go` | All 15 cases passed: unavailable is not absent; pending operation/catalog state is preserved, no fallback or false completion occurs, and recovery succeeds when the same backend becomes available. Uses a controlled backend, not a real locked native store. |
| T060 documentation | Current `README.md` CLI/TUI forwarding sections | Complete: mode/DNS direction, endpoint grammar, consent/trust, results, limits, no persistence, scope warnings and shell return are documented. |
| T065 full tests | `nix develop --command go test ./...` | All packages passed after implementation and regression corrections. |
| T065 full races | `nix develop --command go test -race ./...` | All packages passed, not only forwarding packages. |
| T065 vet/staticcheck | `go vet ./...` and `staticcheck ./...` in the Nix environment | Passed. One initial staticcheck ST1005 finding was corrected before the successful final run. |
| T065 Nix checks | `nix flake check --no-update-lock-file --keep-going -L path:.` | All nine current-host checks passed: build, formatting, go-test, race, vet, staticcheck, govulncheck, automation and native-smoke. `path:.` was deliberately used to include the working tree and new untracked source/test files, rather than a Git-filtered source missing them. Checks for other hosts were omitted, not executed or accepted. The flake native-smoke is a version check, not interactive acceptance. |
| T065 vendor | `nix develop --command bash scripts/check-vendor.sh` | Clean; `go.mod`, `go.sum` and `vendor/` unchanged. No vendored SSH modification or new dependency. |
| T065 tree secrets | `nix develop --command gitleaks dir --redact --config .gitleaks.toml .` | Initial scan found two reviewed false positives in Shift+Tab/F2 keyboard-label literals at `internal/tui/actions.go:207` and `internal/tui/tunnels.go:189`. Specific reviewed fingerprints were added to `.gitleaksignore`; final tree scan found no leaks. No broad rule exclusion was added. |
| T065 history secrets | `nix develop --command gitleaks git --redact --config .gitleaks.toml --log-opts=--all .` | 46 commits scanned; no leaks. History scanning is separate from the working-tree scan. |
| T065 application build | `go build -o /tmp/opencode/orza-forwarding ./cmd/orza` in Nix | Passed; native Linux/amd64 binary used by the real OpenSSH harness. |
| T065 full application cross-builds | `CGO_ENABLED=0`, `go build ./cmd/orza` for Linux/arm64, Windows/amd64 and arm64, Darwin/amd64 and arm64 | All five passed. These compile the full application, not just credential packages; they do not execute native tests. Darwin no-cgo uses the unavailable-store fallback. |
| T054 Darwin cgo | Native Keychain bridge source and tests inspected; Linux compiler cannot provide Darwin `-arch`/macOS SDK | Not compiled or natively validated here; both Darwin native release targets still require matching macOS/cgo builders. |
| T062 real OpenSSH and extended native acceptance | Final native agent report, OpenSSH 10.5p1/Linux process/PTY; retained `/tmp/opencode/t062_native_smoke.py` | Final 130/130 checks passed, superseding the initial 65/65 report. Private temporary HOME/XDG catalog, disposable host/client keys and isolated services; keyscan checked against the generated host public key before trust. Actual Local/Remote/Dynamic HTTP responses, two simultaneous clients, two independent Local CLI tunnels, local/remote half-close, explicit IPv6 Local/Remote without IPv4 fallback, SIGINT 130/SIGTERM 143, separate unattended stdout checks and listener reuse within 2 seconds passed. Added interactive CLI/TUI/relaunch checks are detailed below; this is native automated evidence, not human evaluation. |
| T062 real server policy | Disposable sshd configurations and administrative `/proc/net/tcp`/`tcp6` binding observations | `GatewayPorts no`, `clientspecified`, and `yes` matched actual loopback/wildcard bindings while Orza retained Requested/unverified warnings. `AllowTcpForwarding no`, `PermitOpen none`, and `PermitListen none` denial cases plus allowed controls passed; no address/port fallback. Remote listener release was observed after controlled cleanup, not guaranteed for an unreachable server. |
| T025 native Local workflow and all-mode TUI coverage | Actual application processes in native Linux PTYs against real OpenSSH, each Local/Remote/Dynamic mode at both 80x24 and 40x12 | Keyboard `p`, form traversal, Ctrl-S, captured summary and `y` produced Active and correct traffic bytes; `s`, stop summary and `y` produced Stopped and port reuse within 2 seconds; `q` exited 0 and termios was restored. Native Local workflow demonstration closes T025 alongside passing US1/regression suites, without claiming a human operator study or full release acceptance. |
| T063 partial native resize/exit | Native `ioctl` resize from 80x24 to 40x12 and back, and from 40x12 to 80x24 and back, while Local forwarding was live; confirmed quit with a live tunnel | Active count and live traffic survived resize; confirmed live quit closed the listener, exited 0 and restored termios. Empty native PTYs also retained Tree/Details/Tunnels, no Actions panel and safe quit. Native shell handover, agents, real locked/approval-required stores and macOS/Windows execution are not covered. |
| T026 full-process relaunch | Quit a live native TUI with confirmation, then launch a new application process using the SAME HOME/XDG catalog, not a new empty HOME | Relaunch displayed `Tunnels (0 active)` and `No tunnels`; saved host remained at revision 1. A fresh Local tunnel created from that saved host exchanged correct bytes, stopped and released its port; relaunched TUI exited 0 with termios restored. This closes the full-process persistence-oracle gap beyond the manager-restart tests. |
| T059 native interactive CLI | Each Local/Remote/Dynamic CLI mode in a native Linux PTY, known disposable host and key authentication | Cancel-default target summary accepted with `yes` through the masked native terminal reader; termios ECHO was off, no unmasked `yes` was echoed, correct traffic bytes flowed, SIGINT returned 130 and termios/listener cleanup passed. Known-host approval was already established, so no trust prompt occurred. Together with existing all-mode unattended, validation/result and process-local ownership coverage, this closes T059 with native credential/platform blockers explicitly recorded. |
| T059 output/no-UI scope | Separate unattended stdout/stderr capture plus automated CLI I/O, app fake-backend recovery and Linux backend-policy tests | Separate unattended stdout remained empty; combined interactive PTY streams cannot prove stdout/stderr separation, so the interactive output guarantee comes from automated I/O tests. No-UI/unavailable credential cases passed app fake/backend tests, not real native locked/approval-store acceptance. Native agent, D-Bus locked store, macOS/Windows credential/platform checks remain under T049/T054/T055/T063. |
| T066 review | FR-001-FR-020 and SC-001-SC-008 reconciliation below | Evidence review updated with native shell/agent/credential/platform/operator blockers explicitly retained; full-process relaunch is now verified, but no full release-acceptance claim. |

Code review found and corrected three real bugs with regression tests: copy-error cleanup now keeps the read-convergence deadline armed through worker joins (`copy_forwarding_test.go`); stale `StopAttempt` selection/cancellation is atomic and joins only the captured attempt (`tunnel_manager_test.go` and `tunnel_lifecycle_test.go`); application diagnostics accept only allowlisted values rather than sanitized arbitrary text (`tunnel.go` and its tests). Existing integration expectations were updated for the permanent panel and browser resumption after shells; full tests and races passed afterward.

TUI creation/traffic/inspection/confirmed stop for all three modes at both supported sizes is covered by `TestTunnelFormStartsAndStopsAllModesThroughTUI`, using a fake terminal with real in-process SSH connections. Layout/focus/resize/no-color tests supplement it. The extended harness separately passed actual native Linux process/PTY creation, traffic, confirmed stop and terminal restoration for all three modes at each size, plus live Local resize, confirmed live quit and same-catalog full-process relaunch. These native automated demonstrations close T025/T026; they do not measure human timing/comprehension. Types/runner ports reside in `internal/app/tunnel_types.go`; form/mode and SOCKS tests reuse nearby files rather than every proposed task filename.

Native shell coexistence was intentionally not run to avoid executing the real account's shell startup files. Automated shell coexistence with real in-process SSH fixtures still passed normal/nonzero/shell-only-failure return, traffic continuity and reconciliation; that is not native shell acceptance. Disposable key authentication and known-host approval in this harness do not validate native agents, credential entry/unlock/approval or real native no-UI recovery.

All temporary native children/services, keys, configs and scratch/catalog directories from the extended run were cleaned up. Only the harness `/tmp/opencode/t062_native_smoke.py` was retained from that disposable test environment; the built binary remains at `/tmp/opencode/orza-forwarding`. No production server, keys or catalog were used.

## Pending Checklist Tasks

- [ ] **T049**: Linux backend/no-UI policy tests passed, including Locked properties and returned-Prompt rejection. Darwin/Windows policy tests are present but have not been executed on their native platforms; actual locked/approval-required native-store behavior remains pending on the native matrix.
- [ ] **T054**: Darwin fail-without-UI implementation and narrow Security.framework constant bridge exist; Linux lacks the Darwin compiler flags/macOS SDK needed to compile the cgo path. Darwin no-cgo cross-builds do not validate Keychain behavior. Native cgo compilation and tests are required.
- [ ] **T055**: Windows immutable option and generic no-CredUI backend implementation/tests exist; full Windows cross-builds passed, but native Windows test execution and credential behavior are not verified.
- [ ] **T063**: Native Linux all-mode interactive CLI and TUI workflows, live Local resize, signals, terminal restoration, confirmed quit and same-catalog process relaunch passed. The matrix remains incomplete: native shell coexistence was intentionally not run to avoid real-account shell startup files; native-agent authentication, real D-Bus locked/approval-store no-UI/recovery checks and native macOS/Windows execution remain pending. Automated real-SSH-fixture shell and app fake/backend credential tests passed but do not substitute for those native cases. Cross-builds and the flake version smoke do not complete this matrix.
- [ ] **T064**: No ten-operator study was conducted; participants are unavailable. SC-001/SC-002 timing and comprehension results must not be inferred from automated checks or the native agent report.

## Acceptance Reconciliation

| Requirements / Criteria | Current Evidence | Residual Limitation |
|---|---|---|
| FR-001/002/005; SC-003 | All-mode validation, correct endpoint/DNS side, unchanged bytes, two tunnels/two clients and independent-stop fixtures; real OpenSSH all-mode HTTP/two-client checks | Real OpenSSH two-tunnel smoke used Local; the complete per-mode concurrency matrix is automated fixture evidence. |
| FR-003 | Separate transports, traffic/cleanup during shell ownership, normal/nonzero/shell-only-failure return and unchanged direct CLI results | Full native interactive shell handover pending T063. |
| FR-004/010/016; SC-007 | Default Local form, contextual guidance, permanent empty/overflow panel, keyboard/focus/inspection/resize/no-color unit tests; all-mode fake-terminal and native Linux PTY TUI creation/traffic/stop at 80x24/40x12, live Local resize and termios restoration | Remaining native platform/shell matrix is T063; human usability remains unmeasured under T064. |
| FR-006/007/008; SC-004/006 | Exact exposure consent, no fallback, captured revision/identity, trust-before-secret, rejection and secret canaries; real binding/policy checks; native masked CLI confirmation on preapproved disposable hosts | Native agent and real locked/approval-store cases pending T049/T054/T055/T063; no native trust/credential prompt acceptance is inferred from known-host/key-based success. |
| FR-009/011/012/013/014; SC-005 | Lifecycle/late-result races, independent stop, destination isolation, worker joins, exact watchdog/grace clocks and port reuse; actual handled signal exits and half-close | Remote release after undetected transport loss remains server-dependent; no SIGKILL or fixed healthy-idle/partition detection guarantee. |
| FR-015; SC-004 | Persistent Requested/unverified labels in TUI/CLI; real GatewayPorts/AllowTcpForwarding/PermitOpen/PermitListen observations | Administrative binding observations belong to this harness, not an Orza verification capability. |
| FR-017 | Captured targets survive edit/move/delete; same-ID current-revision retry and missing-record failure tests | No automatic reconnect or target substitution. |
| FR-018; SC-008 | Schema-aware pre-bootstrap validation, immutable no-UI option, output/results, unattended trust/credential failure and remembered-secret fixture success; all-mode native interactive and unattended CLI traffic/signal/cleanup checks | T059 complete with recorded native credential/platform blockers; agents/real locked-store/native macOS/Windows checks remain pending. PTY streams are combined; stdout separation is established by automated I/O and separate unattended captures. |
| FR-019 | In-memory manager, unchanged catalog bytes, no payload/destination history, bounded diagnostics/snapshots, empty manager restart and same-HOME/XDG full-process relaunch with saved host revision 1 and fresh Local traffic | Full-process relaunch gap closed under T026; vendor transient malicious-server buffering remains outside the admitted-work cap. |
| FR-020 | README coverage, automated all-mode success/principal failures, full quality/security gates, real OpenSSH checks | Native release acceptance is incomplete; review/documentation completion does not waive pending tasks. |
| SC-001/002 | In-app guidance implemented; evaluation procedure below retained | Unmeasured, pending ten operators under T064. |

No requirement is waived and no synthetic result is represented as native/human acceptance. T025 is complete for the actual native Local workflow demonstration by automated process/PTY execution alongside existing suites, not a human operator study. T026's full-process relaunch oracle is complete using the same HOME/XDG catalog. T059 is complete for all-mode interactive/unattended CLI checks with credential/native-platform blockers recorded as its wording permits; T063 retains the full native matrix. T035/T058 remain complete for implemented reconciliation/teardown and passing available suites, while T049 retains unexecuted native-platform tests. T062/T065 are complete for the executed controlled-server and available-host quality gates with limitations recorded; T066 is the completed evidence review, not completion of every success criterion.

## Prerequisites

- Go 1.26.6 or the repository's Nix development environment; all examples run from the repository root.
- Disposable SSH server on `127.0.0.1:2222`, a dedicated user `tunnel-test`, and an available SSH agent containing only a disposable test key.
- Server permits TCP forwarding. For loopback-only manual tests use `GatewayPorts no`; compare a separately controlled server with `GatewayPorts clientspecified` and, if available, forced wildcard policy.
- A non-sensitive HTTP service reachable from the server at `127.0.0.1:8080`, and another reachable from the client at `127.0.0.1:8081`. Confirm direct access from the respective machines before testing tunnels.
- `curl` for HTTP/proxy checks; VT-capable terminal supporting resize to 80x24 and 40x12. No production hosts, credentials, or data.

The host user and ports above are fixture values, not Orza defaults. `SSH_AUTH_SOCK` is the standard agent endpoint, not a place to put secret material. See [contracts/cli.md](contracts/cli.md), [contracts/tui.md](contracts/tui.md), and [data-model.md](data-model.md) for interface and scope.

## Build and Automated Gates

```sh
nix develop
go build -o /tmp/opencode/orza-forwarding ./cmd/orza
go test ./...
go test -race ./...
go vet ./...
nix flake check --no-update-lock-file --keep-going -L path:.
```

The temporary output parent `/tmp/opencode` exists in this workspace; in other environments use an existing private scratch directory. `path:.` includes new untracked implementation/tests in the flake source; the successful run covered only this host's nine checks, not other hosts. Formatting/staticcheck/vulnerability/automation gates passed as exposed by that flake. The SOCKS parser fuzz target is implemented; rerun it with:

```sh
go test ./internal/sshclient -run '^$' -fuzz '^FuzzSOCKSRequest$' -fuzztime=30s
```

`FuzzSOCKSRequest` and the multi-transport direct/forwarded TCP fixtures are current tests. They support listener requests/cancellation, denied or absent replies and transport loss. Never use a production server as the automated fixture. For the separate dependency/secrets gates:

```sh
nix develop --command bash scripts/check-vendor.sh
nix develop --command gitleaks dir --redact --config .gitleaks.toml .
nix develop --command gitleaks git --redact --config .gitleaks.toml --log-opts=--all .
```

## Save a Disposable Host and Approve Trust

```sh
/tmp/opencode/orza-forwarding folder create /forwarding-lab
/tmp/opencode/orza-forwarding connection create /forwarding-lab/host \
  --host 127.0.0.1 --port 2222 --user tunnel-test --auth agent
/tmp/opencode/orza-forwarding connect /forwarding-lab/host
```

Confirm the displayed host fingerprint through a separate trusted channel before persisting trust. Exit the shell normally. Do not put passwords or passphrases in commands, files, environment variables, or test logs.

## Local Forwarding

In terminal A:

```sh
/tmp/opencode/orza-forwarding tunnel local /forwarding-lab/host \
  --listen-port 18080 --destination 127.0.0.1:8080
```

Confirm the host/tunnel summary. Expect stderr Active/readiness and local listening address `127.0.0.1:18080`, no shell, and empty stdout. In terminal B:

```sh
curl --fail http://127.0.0.1:18080/
```

Expect the server-side HTTP response. Run two clients simultaneously; both succeed. Stop terminal A with Ctrl-C; expect exit 130, local listener released, and subsequent access refused. Repeat with `--listen-address ::1` and bracketed IPv6 client URL if both endpoints support IPv6; no IPv4 fallback is allowed.

## Remote Forwarding

On the client:

```sh
/tmp/opencode/orza-forwarding tunnel remote /forwarding-lab/host \
  --listen-port 18081 --destination 127.0.0.1:8081
```

Expect Active, `Requested listener`, and an explicit unverified remote-scope warning even for loopback. From the server's own terminal:

```sh
curl --fail http://127.0.0.1:18081/
```

Expect the client-side service response. Compare actual server listening addresses using server administration tools in this disposable environment; Orza must never claim these addresses were verified through SSH acceptance. A server denying the request produces failure, not Active. A server accepting but changing bind policy remains Active with the warning and requested label.

## Dynamic Forwarding

```sh
/tmp/opencode/orza-forwarding tunnel dynamic /forwarding-lab/host --listen-port 1080
```

From another local terminal:

```sh
curl --fail --socks5-hostname 127.0.0.1:1080 http://127.0.0.1:8080/
```

Expect the server-side response; SOCKS resolves domain targets on the selected host. Use a name resolvable only on the disposable host to prove DNS placement. Verify malformed/truncated requests and unsupported methods/BIND/UDP fail without terminating the listener. A stalled SOCKS negotiation closes after 10 seconds; established idle traffic has no such deadline.

## Unattended and Exposure Safety

After trust approval with agent access:

```sh
/tmp/opencode/orza-forwarding tunnel local /forwarding-lab/host \
  --listen-port 18080 --destination 127.0.0.1:8080 --non-interactive </dev/null
```

Expect no prompts and successful foreground readiness. Missing `--non-interactive` without a terminal is usage failure. Unknown/changed trust or unavailable credentials fail without waiting for input or approving trust.

The following intentionally omits consent and must fail before networking:

```sh
/tmp/opencode/orza-forwarding tunnel dynamic /forwarding-lab/host \
  --listen-address 0.0.0.0 --listen-port 1080 --non-interactive
```

Expected exit 2. In a controlled isolated network only, add `--acknowledge-exposure` and verify that warnings identify unauthenticated proxy exposure. `--json`, port zero, absent destination for fixed modes, destination supplied for Dynamic, and malformed IPv6 are also validation failures. No test should silently choose a different port.

## TUI and Shell Coexistence

```sh
/tmp/opencode/orza-forwarding
```

1. At 80x24 verify Tree, Details, permanent Tunnels, and borderless legend; no Actions panel. Empty Tunnels includes count zero and start guidance.
2. Select the saved host, press `p`, enter a local request, and use Ctrl-S Start; confirm the summary and required trust/input decisions. Start a second tunnel on another port.
3. Tab through Tree/Details/Tunnels. Changing hosts or catalog filtering does not hide either tunnel. Enter inspects full endpoints/warnings, `s` asks to stop the selected live tunnel, and catalog delete/edit keys cannot accidentally target a host while Tunnels has focus.
4. Resize to 40x12; all three panels remain visible with a usable selected row and at most three legend rows. Inspect full values via scrollable inspection; resize back without losing either selection or drafts. Repeat with `--no-color`.
5. Focus catalog, open a shell with `c`, and run client traffic through both tunnels from another terminal while the shell owns the terminal. End the shell normally and with a nonzero status in separate runs; each returns to TUI and leaves tunnels intact.
6. Rerun the implemented automated integration test `TestTunnelFailureDuringShellReconcilesOnReturn`, which uses the in-process forwarding fixture to close one identified tunnel transport while a separate shell runs. It passed assertions for current Failed state after shell return, released local ports, and uninterrupted traffic on the other tunnel. This uses a fake terminal with real SSH connections; no native interactive fixture-control service is assumed.
7. Stop one tunnel; its client connections close, while the other continues. Quit with the other active; Cancel preserves it, confirmation closes it. SIGINT/SIGTERM cleanup completes without a dialog and returns 130/143.

## Failure and Cleanup Matrix

Implemented automated coverage exercises occupied/unavailable listening endpoints, destination refusal, forwarding denial, revision changes before startup, deleted-record retry, cancellation during trust/auth/global request, late listener/channel success, silent remote replies, blocked channel writes, withheld close replies, pending-open capacity, inbound remote flood during teardown, and half-close response preservation. Admitted-work/serialized-overflow bounds, repeated failures, shell suspension, pre-bootstrap JSON rejection and unavailable recovery passed controlled tests. Repeat actual locked native credential-store/recovery rejection without dialogs on native systems, including Linux Secret Service and macOS approval-required items; backend doubles do not complete that acceptance. A silent startup remains Starting until explicit cancellation; implemented cancellation-convergence tests passed.

For every repeated stop/cancel/failure case, verify locally owned listeners are closed and reusable within 2 seconds after completion and all application-owned workers join. A dead remote server may retain its listener until it detects transport loss; document that limitation rather than asserting verified immediate release. Actual OpenSSH policy checks passed as recorded above. The remaining native Linux/macOS/Windows terminal/agent/credential matrix still requires separate execution; cross-build success is insufficient.

## Operator Evaluation for SC-001 and SC-002

Recruit 10 operators familiar with saved SSH connections but new to Orza forwarding. Give each a saved, preapproved disposable host and a running server-side test service. Permit only in-app guidance; do not coach or provide this quickstart during the measured task.

Ask each participant to create a working local tunnel and demonstrate a service response. Start timing when they begin from the selected host; pause for credential entry or external fingerprint verification if needed, then resume. At least 9 of 10 must complete in at most 2 minutes for SC-001.

Afterward ask, without coaching: which machine listens in Local mode, which machine reaches its destination, the same two questions for Remote mode, and what Dynamic mode provides. Score a participant successful only if all answers correctly identify the local/remote machines and Dynamic as a proxy through the selected host. At least 9 of 10 must succeed for SC-002. Record anonymous counts/timing and observed usability failures, not credentials or traffic. Implementation is complete for this workflow, but this evaluation remains unperformed under T064; neither the native automated report nor unit tests are operator results.

## Clean Up

Stop all Orza tunnel processes and exit the TUI before removing the disposable catalog subtree:

```sh
/tmp/opencode/orza-forwarding folder delete /forwarding-lab --recursive --yes
```

Remove only the test server/services and disposable key from the test environment. Do not alter real SSH configuration or delete application catalog files to clean up a tunnel test.
