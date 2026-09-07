# CLI Contract: SSH Startup Diagnostics

## Scope

This contract supersedes only the previous restriction that `connect` rejects structured output.
Commands, successful interactive sessions, remote exit propagation, and existing broad error codes remain
unchanged.

## Stable identifiers

Categories: `timeout`, `authentication_denied`, `connection_refused`, `host_not_found`,
`network_unreachable`, `host_trust`, `credential_unavailable`, `ssh_negotiation`, `canceled`, `unexpected`.

Stages: `target_resolution`, `network_connection`, `host_trust`, `ssh_negotiation`, `credential`,
`authentication`, `session_setup`, `local_terminal`, `unknown`.

Identifiers are stable within the current major version; human summaries/recommendations may improve
without changing their meaning.

## Human failure output

A pre-active failure is written through the normal error channel and includes:

```text
error: SSH session could not start
target: /prod
endpoint: prod.example:22
category: timeout
stage: network_connection
recommendation: Check network reachability and timeout settings, then retry.
detail: operation timed out
```

`endpoint` is the captured catalog `host:port` and never includes username, resolved remote address,
credential reference, or identity path. `endpoint` and `detail` are omitted when unavailable. Detail uses
only the vocabulary table in `spec.md`, is single-line, and is at most 256 Unicode code points (255 plus
`…` when truncated). Raw causes are never printed.

## Structured failure output

`orza --json connect SELECTOR` is accepted. On pre-active failure, stderr contains exactly one JSON
object:

```json
{
  "ok": false,
  "error": {
    "code": "transport_failure",
    "message": "Connection timed out",
    "target": "/prod",
    "endpoint": "prod.example:22",
    "category": "timeout",
    "stage": "network_connection",
    "recommendation": "Check network reachability and timeout settings, then retry.",
    "technicalDetail": "operation timed out"
  }
}
```

Rules:

- Existing `ok`, `code`, `message`, and `target` remain present.
- Optional fields are omitted, never null.
- stdout remains the trust prompt/session stream and does not contain the failure object.
- stderr contains exactly one failure object after a pre-active failure, even when stdout previously
  contained an interactive trust prompt.
- No success JSON envelope is emitted after an interactive session.
- Remote exit remains the process status and is not a startup diagnostic.

## Broad codes and process exits

| Category | Existing code/exit |
|---|---|
| `canceled` | `canceled`, exit 5 |
| `host_trust`, `authentication_denied`, `credential_unavailable` | `security_failure`, exit 6 |
| `timeout`, `connection_refused`, `host_not_found`, `network_unreachable`, `ssh_negotiation`, `unexpected` | `transport_failure`, exit 10 |

Catalog target not-found/conflict before SSH startup retain their existing codes/exits and do not receive
startup categories. In-app cancellation uses exit 5; process-terminating signals retain 130/143.

## Target consistency

CLI resolves the selector once, displays its snapshot, and calls connect using captured ID plus expected
revision. A concurrent change fails before network instead of connecting to a different endpoint.

## Safety

No field may contain passwords, passphrases, key material, authentication responses, credential
references, arbitrary server text, or an unclassified `error.Error()`.
