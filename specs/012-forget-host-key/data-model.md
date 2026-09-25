# Data Model: Forget Host Key

## Existing Entity: App-Owned Host Trust

| Field | Meaning | Rules |
|---|---|---|
| `id` | Stable trust-record identity | Existing opaque identifier |
| `canonical_host` | Normalized host namespace | Lowercase, trimmed host identity; paired with port |
| `port` | SSH destination port | Integer from 1 through 65535; paired with host |
| `key_algorithm` | Accepted SSH key algorithm | Existing public metadata |
| `public_key` | Accepted public host key | Existing non-secret key material |
| `fingerprint_sha256` | Displayable host-key fingerprint | Must match the public key |
| `revision` | Trust-record version | Positive; used for conditional deletion |
| `accepted_at` | Time trust was recorded | Existing audit metadata |

**Identity and relationship**: one app-owned host-trust record exists at most once for a canonical host and port. Any number of connections can refer to that endpoint; deleting one connection does not delete its host-trust record.

## New Application Value: Forget Host Key Scope

| Field | Meaning |
|---|---|
| Connection selector and resolved connection | Identifies the operator-selected or command-targeted connection |
| Endpoint | Host and port derived from the resolved connection |
| Trusted host | The app-owned trust record captured for confirmation, including its revision; absent when there is nothing to forget |

**Lifecycle**:

1. Resolve the connection and derive its endpoint.
2. Read app-owned host trust for that endpoint.
3. For the TUI, display the captured endpoint and trust context, then await confirmation.
4. Delete only if the record still matches the captured revision.
5. Report success, idempotent no-op, conflict, or failure without changing standard SSH trust data.

## New Request and Result Values

| Value | Fields | Validation and outcome |
|---|---|---|
| Forget request | Connection selector; optional expected trusted-host revision | Selector must resolve. A supplied revision must be positive. The TUI supplies its captured revision; a CLI call may supply one to detect stale automation input. |
| Forget result | Endpoint; `forgotten`; catalog revision when changed | `forgotten: true` only when a record was deleted. `forgotten: false` means no app-owned record existed. |

## Concurrency Rules

- A trust record replaced or removed after scope capture cannot be deleted by that stale scope.
- A successful deletion increments the catalog revision exactly once.
- A no-op, invalid request, or conflict does not change the catalog revision.
- A matching standard `known_hosts` entry does not affect deletion eligibility and is not changed.
