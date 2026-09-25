# Research: Forget Host Key

## Decision: Delete app-owned trust by endpoint and captured trust revision

The deletion targets the existing `trusted_hosts` record identified by canonical host, port, and its captured revision. The repository performs it in the same immediate transaction style as trust persistence and increments the catalog revision once only after a successful deletion.

**Rationale**: Trust is shared by connections with the same endpoint, so connection deletion is not the correct operation. Comparing the trusted-host revision prevents a stale confirmation or command from deleting a replaced key.

**Alternatives considered**:

- Delete trust whenever a connection is deleted: rejected because other connections can share the endpoint.
- Unconditional endpoint deletion: rejected because a newly accepted replacement key could be removed after a stale read.
- Add a schema migration: rejected because endpoint uniqueness and row revisions already exist.

## Decision: Use a dedicated host-trust management use case

The application layer resolves a connection selector, derives its endpoint, obtains a scope with the app-owned trust record when present, and deletes conditionally. The CLI and TUI depend on this use case rather than directly on the catalog repository.

**Rationale**: This keeps endpoint derivation, error classification, and concurrency behavior consistent while preserving the current presentation-to-application boundary.

**Alternatives considered**:

- Add the method to `ConnectionService`: rejected because it couples a shared host-trust resource to a connection aggregate.
- Let CLI and TUI compose repository calls: rejected because it duplicates security and conflict behavior in presentation layers.

## Decision: TUI confirmation and non-interactive CLI command have distinct safety behavior

The TUI action captures the trusted-host revision, displays host and port plus the scope warning, and confirms only on `y`; Enter and Escape cancel. The CLI command is `orza connection forget-host-key PATH_OR_ID [--if-revision REVISION]`, does not read confirmation input, and is suitable for automation.

**Rationale**: The interactive action meets the terminal safety requirement, while the explicitly invoked CLI subcommand satisfies the approved automation requirement. An optional revision gives scripts optimistic concurrency control.

**Alternatives considered**:

- Require `--yes` in the CLI: rejected by the clarified feature decision.
- Omit optimistic concurrency from the CLI: rejected because scripts need a way to reject stale trust state deterministically.

## Decision: Treat an absent app-owned trust record as an idempotent no-op

The scope reports that no app-owned key exists. The TUI shows a stable no-change result; the CLI returns successful structured output with `forgotten: false` and an explanatory human message.

**Rationale**: Repeated automation should not fail merely because the intended end state is already reached. A record that changes after scope capture remains a conflict, not a no-op.

**Alternatives considered**:

- Return not-found as an error: rejected because it makes desired-state automation brittle.
- Treat every stale state as success: rejected because it hides replacement-key races.

## Decision: Keep standard SSH trust sources outside the feature

The feature never writes or deletes `known_hosts` entries. After successful removal, normal trust evaluation remains authoritative: a standard known key can still be accepted and a revoked standard key remains rejected.

**Rationale**: Orza documents standard trust files as read-only sources, and mixing ownership would weaken predictable SSH trust behavior.

**Alternatives considered**:

- Remove matching standard entries: rejected because Orza does not own those files.
