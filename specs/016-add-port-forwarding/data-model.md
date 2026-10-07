# Data Model: Host Port Forwarding

**Spec**: [spec.md](spec.md) | **Design**: [plan.md](plan.md)

All new entities are session-only. Existing catalog schemas and saved credentials are unchanged. Runtime handles are not serialized, exposed through CLI JSON, or placed in rendered TUI state.

## Saved Connection Snapshot

| Field | Meaning |
|---|---|
| ConnectionID | Existing stable catalog ID; selector is resolved before confirmation |
| Revision | Captured expected revision checked before network I/O |
| Path | Captured display path, not a later lookup that could relabel the running target |
| Host / SSHPort / User | Confirmed SSH access target; use existing safe display rules |
| Authentication configuration | Existing selected method and non-secret references, not secret bytes |

Each request captures one snapshot. Moving/editing/deleting the record never redirects a running tunnel. Retry resolves the same ID anew and confirms current values; missing IDs fail. Shared trust and credential access preserve existing catalog revision/security rules.

## Endpoint

| Field | Meaning |
|---|---|
| Host | Listen: canonical IP literal. Destination: hostname or IP literal |
| Port | Integer 1-65535; zero is invalid |
| Role | Listening or destination |
| Machine | Local computer or saved host, derived from mode |
| Display | Joined host/port with IPv6 brackets, sanitized and never shell-interpolated |

Listen defaults to `127.0.0.1`; `localhost` normalizes to that value. IPv6 `::1` is an explicit choice. Parse IPv4-mapped IPv6 consistently for loopback classification. Do not silently replace unavailable IP families, bindings, or ports. Destination names remain unresolved until the appropriate machine dials them. Domain values reject whitespace, control characters, NUL, malformed brackets, and embedded port ambiguity; destination port is always numeric.

## Forwarding Request and Draft

| Field | Meaning |
|---|---|
| Mode | `local`, `remote`, or `dynamic` |
| Connection | Immutable confirmed snapshot |
| Listen | Required endpoint |
| Destination | Required for local/remote; absent for dynamic |
| ExposureAcknowledged | Explicit consent for the exact non-loopback request |
| InteractionPolicy | Interactive callbacks or deliberately selected no-prompt execution |
| AttemptID | Monotonic startup token used to reject stale results |

The editable draft is separate from the confirmed request. Mode defaults to Local. Retain mode-specific destination drafts while editing but validate/use only the selected mode. Changing mode, listening address/port, or selected host clears prior exposure acknowledgement. Canceling a dirty draft offers Discard/Cancel, never a persistent Save action. Start requires field validation, exposure consent where applicable, then target confirmation and revision validation.

## Tunnel Snapshot

| Field | Meaning |
|---|---|
| TunnelID | Monotonic session-local ID; never reused within the session |
| AttemptID / StateVersion | Distinguish canceled starts, explicit retries, and stale notifications |
| Request | Immutable confirmed request, excluding authentication secrets |
| State | Starting, Active, Stopping, Stopped, or Failed |
| ListeningScope | `local_bound` for local/dynamic, `unverified` for accepted remote forwarding |
| RequestedEndpoint | Always retained; remote label is explicitly Requested |
| LocalBoundEndpoint | Actual local listener address, only for local/dynamic; never infer remote verification from `Addr()` |
| Warning | Controlled remote uncertainty or non-loopback/no-auth exposure text |
| Diagnostic | Optional latest safe failure context, not a history |
| CreationOrder | Stable list order, independent of catalog selection |

Starting entries appear immediately, including while trust/authentication is pending. Active count includes only Active; Starting and Stopping remain visibly labeled and included in exit cleanup scope. All entries belong to the current process/session. List selected TunnelID independently of selected catalog ID. A versioned snapshot remains authoritative even while the TUI is suspended for a shell.

## Runtime Ownership

One private runtime per tunnel owns its lifetime context, raw SSH socket/transport, authentication resources, listener, worker registry, pending/established client set, and cleanup completion signal. The session manager owns runtime admission and immutable snapshots; adapters implement I/O. No network resource belongs to the TUI form, renderer, shell, or a startup completion message.

One admitted client-work slot covers negotiation/dial/copy and any unresolved underlying channel-open or teardown worker. At most 64 admitted slots per tunnel; expired waits and close requests do not release slots until their workers actually converge. Remote overload uses at most one additional serialized rejection/cleanup worker, not one goroutine per excess client. Late results remain owned and are closed. If protocol work still cannot converge within 1 second after client timeout/cleanup, fail and force-close that unhealthy SSH transport. This is distinct from ordinary destination refusal, which leaves an otherwise healthy listener Active. Public remote-listener APIs can transiently retain vendor channels before application admission; the application cap is not a guarantee against arbitrary malicious-server protocol buffering. Per-client data is streamed with bounded buffers and never appears in snapshots.

## Lifecycle

| Current State | Event | Next State / Effect |
|---|---|---|
| No runtime | Valid confirmed start admitted | Starting; owns resources before blocking acquisition |
| Starting | Trust/auth complete and listener accepted | Active only if attempt is still live |
| Starting | Error | Failed after cleanup |
| Starting | Cancel or confirmed exit | Stopping -> Stopped after cleanup; no late activation |
| Active | One destination/proxy client fails | Active; close that client, update bounded diagnostic |
| Active | Confirmed stop | Stopping -> Stopped after cleanup |
| Active | Detected transport loss | Stopping -> Failed after cleanup with controlled reason |
| Stopping | Late listener/channel/result | Close it; remain Stopping |
| Stopping | Repeated stop | Idempotent; join same completion |
| Stopped / Failed | Explicit retry | Fresh attempt and request after current-ID/revision confirmation; retain TunnelID, increment AttemptID |
| Stopped / Failed | Dismiss or terminal-snapshot eviction | Remove entry; no active resources exist |
| Any live state | Root cancellation/handled signal | Stop all, join cleanup, then exit |

A retry is refused at the live-runtime limit without changing the existing terminal state. At most 16 live runtimes, plus 32 terminal snapshots. A completing runtime remains visible; when the terminal-snapshot cap is exceeded, evict the oldest completed entry and move selection predictably if that entry was selected. Never evict live tunnels to admit new work.

## Safe Diagnostic

Fields: stable category, operation/stage, captured host identity, requested listening endpoint, applicable fixed destination, controlled recommendation, optional bounded safe detail. Categories include validation, binding, trust, authentication, forwarding_policy, destination, transport, canceled, capacity, and unexpected. Reuse existing broad application error mapping where applicable; do not change published SSH startup identifiers for shell/exec.

Detail is selected from allowlisted values, single-line, stripped of control/ANSI/bidirectional-control characters, at most 256 Unicode code points. Never include raw server/library/OS errors, secret bytes, credential references, identity-file paths, dynamic request payloads, or proxy destination history. Diagnostics persist only in bounded current-session snapshots.
