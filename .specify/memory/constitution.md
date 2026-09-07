<!--
Sync Impact Report
- Version change: template (unversioned) -> 1.0.0
- Modified principles: template placeholders -> I. Secure SSH Operations; II. Terminal-First
  Usability; III. Predictable State and Failure Handling; IV. Behavior-Focused Testing;
  V. Simplicity and Maintainability
- Added sections: Security and Operational Constraints; Development Workflow and Quality Gates
- Removed sections: none
- Follow-up TODOs: none
-->
# Orza Constitution

## Core Principles

### I. Secure SSH Operations
Host identity verification MUST be enabled by default, and unknown or changed host keys MUST require
an explicit user decision. Credentials, private key material, passphrases, and session secrets MUST
never be logged, persisted without explicit user consent, or displayed after entry. SSH
configuration and authentication MUST use established protocol libraries rather than custom
cryptographic or wire protocol implementations. Security-sensitive defaults MAY be relaxed only
through explicit, discoverable user action accompanied by a clear warning. These rules prevent
convenience features from silently weakening the trust boundary of an SSH client.

### II. Terminal-First Usability
Every interactive workflow MUST be fully operable by keyboard and MUST expose a clear way to cancel,
go back, and exit without corrupting state. Destructive or connection-affecting actions MUST
identify their target and request confirmation unless the user has deliberately selected a
documented non-interactive behavior. The interface MUST remain usable at the project's documented
minimum terminal size and MUST provide readable status and error messages without relying on color
alone.
These constraints make the TUI dependable across terminals and accessible during operational work.

### III. Predictable State and Failure Handling
Connection lifecycle states and transitions MUST be explicit, with at most one owner responsible for
each active session. Cancellation, timeout, authentication failure, network interruption, and
process termination MUST release terminal and connection resources deterministically. Errors MUST
include actionable context while excluding secrets, and recoverable failures MUST return control to
a stable screen rather than terminate the application. Deterministic state handling is required
because stale sessions and broken terminal state can disrupt both local and remote operations.

### IV. Behavior-Focused Testing
Changes to parsing, configuration resolution, connection state, input handling, or terminal cleanup
MUST include automated tests covering the changed behavior and its principal failure path. External
SSH servers, clocks, process execution, and terminal capabilities MUST be isolated behind testable
boundaries where practical. A defect fix MUST include a regression test that fails before the fix
unless the behavior cannot be automated; any exception MUST be documented in the change review with
manual verification steps. Testing protects the security and lifecycle guarantees of this
constitution.

### V. Simplicity and Maintainability
Implementations MUST use the smallest design that satisfies the approved specification and these
principles. New dependencies, abstractions, background concurrency, and persistent state MUST have a
specific demonstrated need and MUST be justified during review. Project conventions and the standard
library MUST be preferred when they meet the requirement. Dead code and superseded paths MUST be
removed rather than retained without a concrete compatibility requirement. Simplicity keeps security
and failure behavior understandable enough to review.

## Security and Operational Constraints

- Configuration precedence MUST be deterministic and documented. Invalid or conflicting settings
  MUST fail with an actionable message rather than silently selecting an unsafe fallback.
- Persistent files containing connection metadata MUST use least-privilege filesystem permissions.
  Secret material MUST use operating-system or established SSH-agent facilities where available.
- Commands, host names, user names, paths, and environment-derived values MUST be treated as
  untrusted input. Process invocation MUST avoid shell interpolation unless the specification
  explicitly requires shell semantics and tests cover escaping behavior.
- Session and diagnostic output MUST be bounded or streamed so that long-running connections cannot
  cause unbounded memory growth.
- Supported platforms, terminal capabilities, and SSH compatibility assumptions MUST be stated in
  user-facing documentation before a release depends on them.

## Development Workflow and Quality Gates

Every change MUST reference an approved specification or a clearly stated maintenance objective.
Reviewers MUST verify the applicable constitutional principles, security implications, terminal
cleanup behavior, and user-visible failure paths. Formatting, static analysis, and the complete
automated test suite MUST pass before merge. User-facing behavior changes MUST update relevant
documentation in the same change. Security-sensitive changes MUST document threat assumptions and
manual verification when automated coverage cannot exercise the real SSH or terminal boundary.
Complexity exceptions MUST name the simpler alternative considered and explain why it is
insufficient.

## Governance

This constitution is the highest-priority project governance document. Specifications, plans, and
implementation decisions MUST comply with it; conflicts MUST be resolved in favor of the
constitution.

Amendments MUST be proposed as an explicit constitution change that states the rationale, impact on
existing work, and any required migration. Approval requires review by the project's maintainers and
MUST update the Sync Impact Report and amendment date. Backward-incompatible removals or
redefinitions of principles require a MAJOR version increment; new principles or materially expanded
obligations require a MINOR increment; clarifications and non-semantic refinements require a PATCH
increment.

Each specification and implementation review MUST include a constitution compliance check. Any
intentional exception MUST be documented with its scope, owner, expiration or review date, and
mitigation; silent exceptions are prohibited. Maintainers MUST review the constitution before each
release and whenever a recurring exception indicates that an amendment may be necessary.

**Version**: 1.0.0 | **Ratified**: 2026-07-29 | **Last Amended**: 2026-07-29
