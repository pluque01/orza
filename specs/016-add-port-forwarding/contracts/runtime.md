# Runtime Contract: Ownership and Forwarding

This describes internal service/adapter behavior, not a public network API. Exact Go signatures follow existing ports/types conventions during implementation.

## Operations

| Operation | Contract |
|---|---|
| Start | Accept confirmed request and presentation policy; resolve captured ID/revision; enforce admission; create session-owned runtime before blocking I/O; publish Starting; return stable TunnelID and asynchronous outcome |
| Stop | Target session TunnelID; idempotently cancel and join that runtime; preserve unrelated tunnels; publish final state only after local resources close |
| Retry | Resolve same saved ID and current revision; require fresh review/consent; start new attempt with same TunnelID and higher AttemptID |
| Snapshot | Immutable bounded process-local view; no secrets, payloads, live handles, or I/O; include state version and captured request |
| Dismiss | Remove a terminal-state entry only; cannot stop or remove live resources implicitly |
| Close | Cancel all live/starting runtimes and join; usable from normal exit, root signals, TUI Run failure, and dependency teardown |

All late messages carry TunnelID/AttemptID; a canceled attempt cannot reactivate or alter a new one. Cleanup has one owner with close-once resource semantics. TUI/CLI read only safe snapshots. The manager's lifetime does not depend on a form, active operation, or shell runtime.

## SSH Adapter

Open authenticated transport without creating session channels, PTY, shell, or remote command. Verification precedes agent/password/key access and follows existing policy. Expose owned destination opening, remote listening, transport wait/loss detection, and forced raw close without leaking `*ssh.Client` to UI code.

Local/Dynamic listeners use local standard-library TCP sockets. Remote uses SSH `Listen` and promptly drains forwarded-channel acceptance. Destination handling never blocks the accept loop on one slow destination; admission rejection closes that incoming client. Source endpoints never become trusted UI strings without sanitization.

The vendor remote listener accepts protocol channels before application admission. Maintain at most 64 admitted client-work slots and one serialized overflow rejection/cleanup worker. Do not release a slot on sending channel Close: retain it until copy/read/open workers actually finish. If an expired open or cleanup still has no protocol convergence after 1 second, fail and force-close that unhealthy tunnel transport, leaving other tunnel transports untouched. This does not classify ordinary destination refusal as a tunnel failure. The trusted-server threat assumption and transient vendor protocol buffering must be documented; no strict bound on malicious-server channel internals is claimed and no custom SSH multiplexer is introduced.

Remote listener acceptance supplies only requested address and unverified scope. No `Listener.Addr()`, forwarded-channel metadata, or successful data exchange upgrades it to verified. A generic refusal is a safe forwarding-policy error, not evidence of a more precise raw server reason.

## Resource Limits and Deadlines

- Maximum 16 live TUI runtimes and 64 admitted client-work slots per tunnel, plus one serialized remote-overflow cleanup worker; one CLI runtime. Cap slots include sockets negotiating, destination opens still pending, established copies, and unresolved open/teardown/copy workers.
- Destination establishment timeout is 10 seconds per client. Track the actual channel-open worker behind this deadline; do not release its slot early or lose its result. On eventual success after cancellation, close it immediately.
- Local SOCKS negotiation has a 10-second socket read deadline. Clear it before established traffic. Remote SSH channels do not support deadlines; enforce timeout/stop through owned contexts and closure.
- Healthy tunnel lifetime and established client traffic have no imposed idle timeout. Startup has no new fixed deadline: current production SSH startup is caller-cancellable, not governed by an existing fixed timeout. A silent handshake/remote-listener reply may remain Starting until explicit cancellation; TUI/CLI keep cancellation available. Test cancel convergence without a peer reply. Interactive security input is explicitly cancellable.
- 32 terminal-state snapshots and one latest allowlisted diagnostic per tunnel. Cap details to 256 Unicode code points. Coalesce repeated client failures and state notifications; no per-client event queue or history.

At capacity, reject new runtime/client work with a controlled diagnostic/proxy failure. Never terminate another live tunnel to make room. An uncooperative SSH peer that prevents open/close convergence is a transport-level failure; close that affected tunnel after the 1-second grace rather than retain unbounded cleanup state. This is not automatic reconnect or resetting a healthy transport solely for admission capacity.

## SOCKS5 Compatibility

Support version 5 with no-auth method `0x00` only if offered. Return `0xff` when no supported authentication method is offered. Accept only CONNECT (`0x01`), reserved byte zero, and IPv4/domain/IPv6 address types. Reject BIND/UDP ASSOCIATE and invalid destination ports. Domain requests are sent to the SSH host unresolved locally.

Use length-bounded reads, no speculative buffering that loses pipelined payload, and protocol-defined fixed-size responses. Success is returned only after destination channel establishment. Use controlled SOCKS replies: unsupported command `0x07`, unsupported address type `0x08`, policy/admission denial `0x02`, generic establishment failure `0x01`; use more precise network/refusal codes only when a typed safe cause supports them. Success replies use a syntactically valid neutral bound address because SSH does not expose the actual destination-side local socket; do not label that field as verified metadata.

No proxy username/password, TLS, destination ACL, payload inspection, UDP, BIND, or proxy configuration mutation. Non-loopback consent explicitly states that external clients have unauthenticated proxy access. The authenticated SSH transport does not authenticate those local proxy clients.

## Streaming and Teardown

Copy both directions concurrently using bounded buffers. Clean EOF closes that direction's write side where supported and lets reverse traffic finish; do not truncate a response simply because the requester half-closes. Stop/error closes both ends and waits for copy workers.

Immediately on entering Stopping, arm an independent watchdog that forces raw SSH socket closure after at most 250ms, before attempting any channel/listener/graceful transport close. Then deny admission, cancel outstanding work, close tracked local sockets/listener, and initiate best-effort channel and remote-listener closure. SSH channel Close itself may block on protocol writes; the fallback must proceed regardless of any such operation, for every forwarding mode. Continue draining/rejecting remote accept events while vendor bookkeeping unwinds; join owned startup/accept/dial/copy/watch workers. Repeated close is harmless. Do not cancel the watchdog until raw closure and worker convergence are confirmed.

Agent connections/resources acquired during auth must be available for cancellation before blocking list/sign operations; host and secret callbacks must honor cancellation and release input ownership. Authentication cleanup and terminal restoration complete before publishing final lifecycle outcome.

Unattended forwarding selects an immutable no-user-interaction native-store option before bootstrap recovery. Linux never calls Unlock or executes Prompt; inspect Locked collection/items and fail unavailable if access requires approval. macOS uses Security.framework fail-without-UI options on query/mutation/verification, not a skip-protected-items mode that misreports absence. Windows generic credential APIs remain no-UI. Recovery stays enabled under the same policy; inaccessible pending work fails startup without pretending completion or absence. This suppresses native dialogs, not a promise of hard deadlines for all synchronous OS calls.

Once local cleanup has completed, publish Stopped for deliberate cancellation or Failed for runtime/startup error. Local ports must be reusable within the specification's two-second post-completion bound. Remote server release after a broken transport depends on server detection; display that limitation rather than promising immediate verification. SIGKILL cannot provide cleanup guarantees.

## Acceptance Oracles

All three modes must exchange unchanged bytes without session channels or terminal mutation. Test simultaneous clients/tunnels, independent stop, unavailable destination, malformed SOCKS, half-close, stale revision, trust-before-secret, locked native stores/recovery without UI, cancellation during every acquisition stage, delayed replies, blocked channel writes, withheld close replies, accepted remote uncertainty, pending-open saturation, remote flood/shutdown races, and late success after cancellation. Assert owned workers/resources converge, not only that a cancel function was called.

During a TUI shell, manager snapshots and listener cleanup progress without model consumption; on return, only the latest coherent state is rendered. CLI unattended requests neither prompt nor weaken trust. Documentation/native-server checks complement synthetic tests and are release requirements.
