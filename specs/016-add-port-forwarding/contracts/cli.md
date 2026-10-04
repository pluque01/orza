# CLI Contract: Foreground Tunnels

This is the planned user-facing interface, not an already implemented command. Saved selectors and catalog/security rules remain those of the current CLI.

## Command Shape

```text
orza tunnel local PATH_OR_ID --listen-port PORT --destination HOST:PORT
  [--listen-address IP] [--non-interactive] [--acknowledge-exposure]
orza tunnel remote PATH_OR_ID --listen-port PORT --destination HOST:PORT
  [--listen-address IP] [--non-interactive] [--acknowledge-exposure]
orza tunnel dynamic PATH_OR_ID --listen-port PORT
  [--listen-address IP] [--non-interactive] [--acknowledge-exposure]
```

- Exactly one selector and one mode; one tunnel per foreground process.
- `--listen-port` is required, decimal 1-65535. `--listen-address` defaults to `127.0.0.1`; accepts IP literals or `localhost`, normalized to IPv4 loopback. IPv6 is explicit.
- Local/Remote require `--destination`; Dynamic rejects it. Destination is a nonempty hostname or IP with a numeric port; IPv6 uses `[::1]:8080`.
- Local/Dynamic listen locally and reach destinations from the selected host; Remote listens on the selected host and reaches destinations locally.
- No arbitrary SSH flags, port-zero allocation, background mode, lifetime timeout, or automatic reconnect.
- `--no-color`, `NO_COLOR`, `--help`, and `--version` retain their existing meaning. `--json` is rejected before catalog mutation, credential acquisition, or network activity; no JSON event stream is added.

## Confirmation and Authentication

Without `--non-interactive`, stdin/stdout must be interactive terminals. Show captured catalog path, SSH endpoint, mode, listening machine/address/port, and applicable destination/reaching machine; request cancel-default explicit confirmation before starting. Trust approval remains a separate reject/once/persist decision using the existing gate. Credentials use the existing masked terminal reader, not flags or environment variables.

`--non-interactive` is an explicit choice even when a terminal is available. Resolve/pin the target and perform verification without prompts. Agent, unencrypted key, or existing remembered password can authenticate; missing secrets or unknown/changed identity fail. Revoked identities always fail. Invocation without a terminal and without this flag is usage failure, never an implicit no-prompt approval.

Apply this policy before bootstrap credential recovery, not only in the command handler. Locked or approval-required native credentials must fail unavailable without native OS dialogs; recovery is not skipped or falsely completed. Unsupported `--json` and missing no-terminal policy are rejected by shared command-schema-aware preflight before bootstrap can mutate recovery state. Native access uses fail-without-UI rules on Linux/macOS and existing generic no-UI Windows APIs; preserve ordinary interactive behavior for other invocations.

Non-loopback listening requires `--acknowledge-exposure` in unattended mode. Interactive mode requires a separate explicit warning acknowledgement; the flag may supply that acknowledgement but never approves trust or skips ordinary target confirmation. Dynamic warnings explicitly describe an unauthenticated SOCKS5 proxy accessible to other machines. Changing settings before interactive confirmation invalidates previous consent.

## Output and Lifetime

All controlled progress, readiness, warnings, and failures go to stderr. stdout is empty. Do not echo secrets, raw library errors, client payload, or individual dynamic destination history. Repeated client failures are coalesced/rate-limited rather than emitted as unbounded diagnostics.

Readiness identifies mode, captured target, and listening endpoint. Remote readiness uses `Requested listener` and always displays `Warning: Actual remote listening scope is unverified and depends on server configuration.` This warning also applies to loopback requests. Readiness means listener request accepted and transport established, not destination health.

The process stays foreground until interruption or tunnel-level failure. Destination/proxy-client failures alone do not exit it. `Ctrl-C` stops its listener and all clients; no quit confirmation is required because the invocation explicitly owns one foreground tunnel. Handled process termination joins cleanup before returning. No stdin stream is forwarded to the destination.

Startup has no new fixed deadline; an unresponsive handshake/listener request remains Starting until cancellation. Per-client negotiation/destination wait limits do not impose an idle tunnel lifetime. An SSH peer that withholds protocol progress after an expired open or client cleanup is an unhealthy transport failure, not an ordinary destination refusal.

## Process Results

| Exit | Meaning |
|---|---|
| 0 | Normal explicit owner shutdown without signal or failure, such as controlled embedding/test stop |
| 1 | Internal application failure |
| 2 | Invalid mode/options/endpoints, unsupported JSON, missing explicit no-terminal policy, or missing exposure acknowledgement |
| 3 | Saved connection missing |
| 4 | Captured connection revision conflict |
| 5 | User cancels confirmation/trust/input or owner context is canceled without an OS signal |
| 6 | Host trust, authentication, or required credential unavailable |
| 7 | Catalog or credential-recovery startup failure under existing catalog result policy |
| 10 | Binding, SSH transport, remote-forwarding refusal, or tunnel-level I/O failure |
| 130 / 143 | SIGINT / SIGTERM, with existing root signal precedence after cleanup |

No remote command exit codes are involved. Binding/policy failures include the affected requested endpoint and safe recommendation; they do not fall back to another address or port.

## Examples

```sh
orza tunnel local /lab/host --listen-port 15432 --destination db.internal:5432
orza tunnel remote /lab/host --listen-port 18080 --destination 127.0.0.1:8080
orza tunnel dynamic /lab/host --listen-port 1080
orza tunnel local /lab/host --listen-address ::1 --listen-port 18080 --destination '[::1]:8080'
orza tunnel dynamic /lab/host --listen-address 0.0.0.0 --listen-port 1080 \
  --non-interactive --acknowledge-exposure
```

The last example intentionally exposes an unauthenticated proxy; use it only inside a controlled network with appropriate access controls. CLI processes are not listed or managed by another TUI/CLI process.
