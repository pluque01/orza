# Research: Host Port Forwarding

**Date**: 2026-10-03 | **Spec**: [spec.md](spec.md)

Read-only investigations covered SSH/application security and CLI/TUI integration. No network server, application build, or acceptance test was executed during research. Findings below refer to current repository code and vendored versions, not assumed upstream APIs.

## 1. Authenticated Transport Without a Shell

**Decision**: Extract a terminal-free dial/config/handshake helper in `internal/sshclient`; add a forwarding runner and request rather than call shell or command runners. Reuse a small shared application security-preparation path.

**Rationale**: `Client.Run` and `RunCommand` in `internal/sshclient/client.go` both acquire an SSH client before `NewSession`. Existing `sshTransport` in `session.go` exposes only session/close; forwarding needs dial/listen/wait capabilities behind adapter tests. `ConnectService` in `internal/app/connect.go` requires a terminal; `CommandService.Run` in `command.go` demonstrates unattended security but requires a command.

**Alternatives considered**: Running `ssh -N` discards existing library/security boundaries. Dummy shell/command sessions violate the requested no-shell workflow. Duplicating trust/credential logic risks inconsistent gates. General transport pooling is not required.

## 2. Security Input and Trust

**Decision**: Preserve `hostVerificationGate.verify`, `verifiedHost` secret gating, existing known-host precedence, revoked-key rejection, single selected authentication method, native remembered-password retrieval, and explicit trust callbacks. Interactive tunnel startup uses existing TUI security owners; CLI input must be cancellation-aware. Unattended mode has no trust or secret prompt callback and selects a no-native-UI credential policy before bootstrap recovery.

**Rationale**: `clientConfig` lazily builds auth after the host-key callback. `internal/tui/session.go`, `secret_prompt.go`, and `internal/terminal/secret_reader.go` keep secret bytes out of the model and preserve prior focus. The synchronous CLI `readLine` trust prompt ignores its context and cannot be blindly copied. Agent RPCs are synchronous: owned closeable agent connections must be exposed early enough for cancellation to unblock them.

**Alternatives considered**: Normal text fields for passwords leak secrets into UI state. Separate forwarding trust policy or changed precedence creates a security regression. Treating missing secrets as permission to prompt in unattended mode breaks automation.

**Native-store finding and decision**: `internal/credential/store_linux.go` calls Unlock and can execute Secret Service prompts from Get/Delete/Create; removing Orza callbacks alone is insufficient. Unattended stores inspect collection/item locks without calling Unlock and reject/dismiss returned prompts without executing them. `store_darwin.go` needs Security.framework fail-without-UI options on query/mutation/verification, using a narrow cgo bridge for constants if necessary. `store_windows.go` uses generic CredRead/Write/Delete without CredUI. The option is immutable and reaches all saga recovery access before `internal/app/bootstrap.go` runs recovery. Unavailable is not NotFound; preserve unresolved durable operations and fail safely. Native call latency is not guaranteed by UI suppression alone, so no unsupported deadline guarantee is made.

## 3. Library Forwarding and Remote Uncertainty

**Decision**: Use vendored `ssh.Client` forwarding operations. Local and Dynamic destinations are passed as names to direct TCP channel opens; Remote destinations are locally dialed. Every accepted remote tunnel has scope `unverified` and a requested endpoint label.

**Rationale**: `vendor/golang.org/x/crypto/ssh/tcpip.go` implements `Dial`, `DialContext`, `Listen`, `tcpip-forward`, `forwarded-tcpip`, and cancellation. `Listen` acceptance has no observable actual bind scope; `Listener.Addr()` is a placeholder. It cannot prove GatewayPorts behavior or loopback-only isolation. Local/Dynamic channel denial may first occur when a client requests a destination, not at listener startup.

**Alternatives considered**: A remote `ss`/`netstat` command is not portable proof of exposure and would require a shell. A custom verification extension is unsupported. Refusing all unverified remote tunnels contradicts the accepted clarification. Reporting the placeholder address as verified is unsafe.

## 4. Cancellation, Backpressure, and Lifetime

**Decision**: One transport per tunnel; explicit raw-socket forced close, worker registry, prompt draining of remote accepts, tracked client sockets/channels, and half-close-aware streaming. Separate startup-attempt cancellation from session-root tunnel lifetime.

**Rationale**: `tcpListener.Close` and `Listen` wait synchronously for global replies. `mux.go` serializes reply-waiting global requests. SSH channel deadlines are unsupported. Vendor `DialContext` returns on cancellation while its internal open worker can remain blocked. Remote forwarding-list dispatch can hold its mutex while waiting on a one-entry accept queue. `completeOperation` cancels TUI operation contexts, and `Dependencies.Close` currently only closes the catalog.

**Alternatives considered**: Listener close alone leaves clients and blocked requests alive. A fresh timed `DialContext` for every attacker-controlled request can accumulate hidden workers. Shared shell transports make independent stop unreliable. Payload buffering or an unbounded event queue fails during long shells.

**Resolved policy**: Per-tunnel client slots remain held through unfinished channel opens and cleanup/copy convergence. A 10-second establishment timeout returns safe client failure; permit 1 second for unresolved protocol work to converge, then fail/close that unhealthy tunnel transport rather than accumulate indefinitely. An ordinary destination refusal leaves the listener healthy. On entering Stopping, independently arm a 250ms forced raw-close watchdog before any synchronous SSH close, including channel Close, which writes protocol messages. Late results remain owned. Shutdown joins workers and publishes final state after local cleanup. No automatic keepalive subsystem or fixed idle-partition detection guarantee is introduced.

**Residual library boundary**: `tcpListener.Accept` accepts SSH channels before application admission, and channel Close does not itself end blocked reads without peer cooperation. Bound admitted application work and serialize one extra overflow cleanup worker; retain slots through actual convergence. Withheld close replies trigger unhealthy-transport failure after 1 second. Do not claim that the public listener API strictly bounds all transient vendor protocol state against a malicious server. Continue draining during forced teardown; test withheld-close and blocked-write cases. A custom forwarding multiplexer/fork would add unnecessary SSH protocol ownership and is rejected.

## 5. SOCKS5 Subset and Dependencies

**Decision**: Add a small internal no-auth SOCKS5 CONNECT front end using bounded `io.ReadFull` reads on local TCP connections. No new proxy library.

**Rationale**: No SOCKS server library exists in the module/vendor set. The scope is version 5, offered no-auth method, CONNECT, IPv4/IPv6/domain destinations, and ports 1-65535. Standard-library parsing is sufficient without SSH/cryptographic protocol reimplementation. Send success only after destination establishment; preserve pipelined payload and clear negotiation deadlines before traffic.

**Alternatives considered**: A general SOCKS framework adds features/dependencies without demonstrated need. HTTP proxy, UDP ASSOCIATE, BIND, username/password, and remote dynamic modes are out of scope.

**Resolved safeguards**: Reject malformed lengths/version/reserved bytes/unsupported authentication and commands with protocol replies, not raw server errors. Bound negotiation to 10 seconds; fuzz truncation and request-plus-payload. Non-loopback Dynamic warnings explicitly say this is an unauthenticated proxy. Network access controls remain external.

## 6. Endpoint Grammar and Exposure

**Decision**: Listen address is an IP literal, with `localhost` as an alias normalized to `127.0.0.1`; default is `127.0.0.1`. IPv6 literals are supported explicitly, with no implicit dual-stack promise. Destination is `HOST:PORT`, bracketed for IPv6. Use `net.ParseIP`, `SplitHostPort`, and `JoinHostPort`.

**Rationale**: Hostnames are applicable to destinations, while literal listening IPs remove resolution-dependent exposure classification and remote placeholder ambiguities. Classify loopback using the parsed IP, not spelling. Reject wildcard/empty/non-loopback listen addresses without separate consent. No address/port fallback or privilege elevation.

**Alternatives considered**: OpenSSH's many compressed forwarding grammars are less discoverable and harder to validate. Listening DNS names allow changing resolution to broaden exposure after consent. Port zero adds allocation behavior excluded by the specification.

## 7. CLI Shape and Process Outcomes

**Decision**: `orza tunnel local|remote|dynamic SELECTOR --listen-port PORT [--listen-address IP] [--destination HOST:PORT] [--non-interactive] [--acknowledge-exposure]`. One tunnel per invocation; stderr text readiness/failures; empty stdout; `--json` unsupported.

**Rationale**: `internal/cli/root.go` already injects services. `exec.go` rejects JSON, and existing output guarantees one object, not long-running events. `cmd/orza/main.go` owns signals and preserves 130/143. However bootstrap runs before RunE and may mutate recovery state or open native prompts. Add command-schema-aware pre-bootstrap tunnel validation for JSON/no-terminal policy and select the immutable native-store no-UI policy at that stage. Respect `--`, global flag placement, and explicit booleans. Explicit unattended choice satisfies constitutional deliberate non-interactive behavior without creating `--yes` semantics for trust.

**Alternatives considered**: A single opaque `-L`/`-R`/`-D` grammar harms usability. Implicit trust approval is forbidden. JSON events, detached tunnels, and a global list require new contracts outside scope. Exec's five-minute timeout is not a tunnel lifetime limit.

## 8. TUI Layout, Focus, and Shell Results

**Decision**: Preserve existing theme and catalog; add permanent Tunnels, its own focus/actions, compact responsive geometry, and a full-value inspection modal. Active shell completion restores the browser rather than quitting. Direct CLI connect still returns remote status.

**Rationale**: Live `Model.layout` calls `calculateLayoutWithActions`, reserving five bottom rows even though no visible Actions panel remains. The alternate calculator's tests do not prove live geometry. Bordered panels consume two vertical rows each, so three 3-row panels plus a 3-row legend are the minimum 40x12 layout. `handleSession` in `internal/tui/session.go` currently returns `tea.Quit` after active shells. `tea.Exec` correctly releases/restores the terminal but blocks model updates while the shell runs.

**Alternatives considered**: Restoring Actions violates the user decision. A hidden tunnel-only view violates permanent visibility. Three stacked panels plus five control rows cannot fit 40x12. Letting tunnels depend on UI event consumption causes stalls during the shell. Propagating a previous shell status on later TUI exit misreports unrelated work.

**Resolved bindings**: Existing browser `p`, `t`, and `s` are free. Use `p` forwarding, `t` tunnel focus; focused tunnel `s` stop, Enter inspect, `r` retry, `d` dismiss terminal entries. Dispatch only focused target actions; catalog delete/edit must not activate while a tunnel owns focus.

## 9. Resource Bounds and Observability

**Decision**: 16 live TUI runtimes, 64 admitted active/pending/cleanup slots per tunnel plus one serialized remote-overflow cleanup worker, 32 retained terminal-state snapshots, one safe diagnostic per tunnel, 256-code-point detail ceiling, coalesced state notifications. No payload metrics/history or persistent logs.

**Rationale**: Runtime lifetime is unbounded and may be exposed intentionally. Explicit admission bounds prevent socket, worker, and UI-state growth. Per-client slots include lingering SSH opens; reject excess clients without stopping existing ones. Evict only the oldest completed entry, never live state. User dismissal removes terminal entries, not listeners.

**Alternatives considered**: Unlimited concurrency/history contradicts bounded diagnostics and deterministic cleanup. A throughput dashboard or tracing pipeline has no user requirement. Smaller fixed limits risk failing basic simultaneous-client acceptance.

**Startup deadline decision**: Current `Client.New` uses a zero-value net.Dialer and `handshake` waits for completion or caller cancellation; there is no existing fixed shell startup timeout. Forwarding preserves cancellable startup without adding a fixed deadline. A silent handshake/listener request remains Starting with cancel available; cancellation must converge even if the peer never replies. This is separate from per-client establishment deadlines and healthy tunnel lifetime.

## 10. Validation and Platform Scope

**Decision**: Extend controlled integration support for direct/forwarded channels and global listener requests without removing session tests; supplement with actual OpenSSH and native terminal/agent/signal checks.

**Rationale**: `tests/integration/ssh_server_test.go` currently handles sessions only and discards globals. Existing lifecycle/security, CLI, and TUI focus/resize tests provide regression anchors. `flake.nix` exposes formatting, go-test, race, vet, staticcheck, govulncheck, automation, build, and native-smoke gates. PR CI cross-builds do not prove native Windows/macOS behavior or server binding policy.

**Alternatives considered**: Depending on production hosts is unsafe and flaky. Cross-compilation alone is not terminal/server-policy validation. Existing vendored code contains no local upstream test suite to substitute for Orza acceptance.

All planning unknowns are resolved above. Actual acceptance remains implementation work; see [quickstart.md](quickstart.md) and [contracts/runtime.md](contracts/runtime.md).

The setup script reports a branch-derived feature label, but `git branch --show-current` returned `main`. Feature-directory selection remains `specs/016-add-port-forwarding`; planning does not create or switch branches.
