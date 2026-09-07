# Requirements Readiness Checklist: SSH Startup Error Reasons

**Purpose**: Formal pre-task review of the completeness, clarity, consistency, measurability, and risk coverage of the SSH startup-diagnostic requirements
**Created**: 2026-08-06
**Feature**: [spec.md](../spec.md)

**Note**: This checklist evaluates the quality of the written requirements, not implementation behavior.

## Requirement Completeness

- [x] CHK001 Are mappings from every stable failure category to every applicable lifecycle stage documented, including whether some combinations are invalid? [Completeness, Spec §FR-002, Spec §FR-012]
- [x] CHK002 Are safe summary and recommendation requirements defined for all ten categories rather than only illustrated with examples? [Completeness, Spec §FR-005]
- [x] CHK003 Does the specification define the complete stable stage vocabulary exposed by readable and structured output? [Gap, Spec §FR-005, Spec §FR-007, Spec §FR-012]
- [x] CHK004 Are the conditions under which endpoint and technical detail are considered safely available fully documented? [Completeness, Spec §FR-004, Spec §FR-005]
- [x] CHK005 Is the structured CLI failure schema specified completely, including required fields, optional fields, omission rules, channel, and success-session semantics? [Gap, Spec §FR-011]
- [x] CHK006 Are recovery requirements documented when the captured target is deleted, moved, renamed, or revision-conflicted before back, edit, or retry? [Completeness, Spec §US2-AS2, Spec §FR-010]

## Requirement Clarity

- [x] CHK007 Is the precise boundary between a startup failure and a post-active session result defined with an observable criterion? [Clarity, Spec §FR-001, Spec §FR-016]
- [x] CHK008 Is “causa reconocida” defined sufficiently to determine when a specific category is allowed instead of `unexpected`? [Ambiguity, Spec §FR-003, Spec §FR-007, Spec §FR-012]
- [x] CHK009 Is “detalle técnico sanitizado” defined by an explicit allowlist or equally objective disclosure rule, rather than only a prohibited-secret list? [Ambiguity, Spec §FR-005, Spec §FR-006]
- [x] CHK010 Is the 256-character limit unambiguous about bytes, Unicode code points, or grapheme clusters, including how truncation is represented? [Ambiguity, Spec §FR-005, Spec §SC-003]
- [x] CHK011 Is “endpoint” defined with an exact safe format and an explicit decision about whether usernames or resolved addresses may appear? [Ambiguity, Spec §FR-004, Spec §Key Entities]
- [x] CHK012 Is “terminación solicitada por el usuario” distinguished clearly from process signals, terminal interruption, timeout, and network interruption? [Clarity, Spec §FR-008, Spec §Edge Cases]
- [x] CHK013 Are “modo reducido,” “legibles,” “operables,” and “sin desplazar fuera de vista” defined with objective layout or priority criteria? [Ambiguity, Spec §FR-014]

## Requirement Consistency

- [x] CHK014 Does the six-stage precedence list in FR-012 align with the nine stable stages promised by the plan and CLI contract, including `session_setup`, `local_terminal`, and `unknown`? [Conflict, Spec §FR-012, Plan §Technical Context, Contract CLI §Stable identifiers]
- [x] CHK015 Is the specification’s 256-character limit consistent with the plan’s and contract’s 256-rune limit? [Conflict, Spec §FR-005, Spec §SC-003, Plan §Technical Context, Contract CLI §Human failure output]
- [x] CHK016 Do the requirement to classify user-requested termination as `canceled` and the contract preserving signal exits 130/143 define non-overlapping cases? [Consistency, Spec §FR-008, Contract TUI §Cancellation and lifecycle, Contract CLI §Broad codes and process exits]
- [x] CHK017 Does the assumption that existing exit codes may later prove inadequate align with the plan and contract commitment to preserve their current mappings? [Conflict, Spec §Assumptions, Plan §Summary, Contract CLI §Broad codes and process exits]
- [x] CHK018 Are the language assumptions consistent between the localized human presentation, the allowed `Permission denied` equivalent, and the English stable machine identifiers? [Consistency, Spec §FR-003, Spec §Assumptions, Contract CLI §Stable identifiers]

## Acceptance Criteria Quality

- [x] CHK019 Is the controlled ten-category matrix defined sufficiently to establish representative causes, stages, interfaces, and expected targets for SC-001? [Measurability, Spec §SC-001]
- [x] CHK020 Is the start point for the one-second diagnostic deadline defined so SC-004 can be measured consistently across cancellation, timeout, and cleanup? [Ambiguity, Spec §FR-013, Spec §SC-004]
- [x] CHK021 Are the canary classes and inspected surfaces in SC-003 complete enough to measure absence of credential references, arbitrary server text, and session secrets as well as the listed values? [Completeness, Spec §FR-006, Spec §SC-003]
- [x] CHK022 Can “objetivo correcto” and “misma causa controlada” be determined objectively when the target changes revision, path, or endpoint during the attempt? [Measurability, Spec §SC-001, Spec §SC-006]
- [x] CHK023 Are objective fallback criteria defined for “sin terminación abrupta” and “sin exposición del error crudo,” including nested or joined causes? [Measurability, Spec §SC-007, Spec §Edge Cases]

## Scenario Coverage

- [x] CHK024 Are primary-flow requirements complete for each of TUI, readable CLI, and structured CLI, including target, category, stage, recommendation, and optional detail? [Coverage, Spec §US1, Spec §US3, Spec §FR-005]
- [x] CHK025 Are alternate-flow requirements defined when safe endpoint or technical detail is unavailable while the remaining diagnostic is still required? [Coverage, Spec §FR-004, Spec §FR-005]
- [x] CHK026 Are exception-flow requirements complete for failures during target resolution, local terminal setup, PTY/session setup, and trust prompting, not only network and authentication? [Coverage, Gap, Spec §FR-012, Spec §Scope Boundaries]
- [x] CHK027 Are recovery requirements explicit about where canceled retry confirmation, canceled edit, and a failed fresh resolution return the user? [Coverage, Gap, Spec §FR-010]
- [x] CHK028 Are structured CLI requirements defined for an interactive trust decision that precedes a later startup failure without making machine-readable output ambiguous? [Coverage, Gap, Spec §FR-011]

## Edge Case Coverage

- [x] CHK029 Is category precedence specified when explicit cancellation, deadline expiry, network failure, and cleanup errors occur concurrently or are joined? [Edge Case, Spec §FR-008, Spec §FR-012]
- [x] CHK030 Are host-trust requirements complete for unknown, changed, revoked, rejected, and user-canceled decisions, including their category and recommendation distinctions? [Edge Case, Spec §FR-015, Spec §Edge Cases]
- [x] CHK031 Are credential-unavailable boundaries specified for absent agent, unavailable secure store, unreadable identity file, missing passphrase, and locally canceled secret input versus remote authentication denial? [Edge Case, Spec §FR-002, Spec §Edge Cases]
- [x] CHK032 Are requirements defined for empty, malformed, control-character, bidirectional-text, ANSI-bearing, multiline, and over-limit candidate details? [Edge Case, Gap, Spec §FR-005, Spec §FR-006]

## Non-Functional Requirements

- [x] CHK033 Are confidentiality requirements explicit for every presentation and retention surface, including in-memory diagnostics, returned errors, logs, structured output, and screenshots or copied text where applicable? [Security, Completeness, Spec §FR-006, Spec §FR-017]
- [x] CHK034 Is the requirement to avoid arbitrary server, operating-system, resolver, and library text stated explicitly as part of the diagnostic trust boundary? [Security, Gap, Spec §FR-006, Spec §Scope Boundaries]
- [x] CHK035 Are accessibility requirements complete for keyboard-only operation, focus ownership, no-color meaning, and the documented minimum terminal size? [Accessibility, Completeness, Spec §FR-010, Spec §FR-014]
- [x] CHK036 Are supported-platform and terminal-capability assumptions documented at the requirements level where platform-specific classification or presentation differences could affect outcomes? [Portability, Gap, Spec §Assumptions]

## Dependencies & Assumptions

- [x] CHK037 Is the assumption that the lifecycle can reliably identify whether a session became active validated or converted into an explicit prerequisite? [Assumption, Spec §Assumptions, Spec §FR-016]
- [x] CHK038 Are compatibility dependencies on existing host-trust, credential, terminal-restoration, catalog-revision, and exit-code contracts identified with the exact guarantees this feature relies on? [Dependency, Spec §Assumptions, Spec §FR-010, Spec §FR-015]

## Ambiguities & Conflicts

- [x] CHK039 Is the apparent tension resolved between excluding retry-policy changes and introducing an explicit retry action with fresh validation and confirmation? [Conflict, Spec §FR-010, Spec §Scope Boundaries]
- [x] CHK040 Is it clear whether every pre-active failure is recoverable in the TUI, or whether categories such as cancellation, local terminal failure, missing target, and security rejection have different available actions? [Ambiguity, Spec §US2-AS1, Spec §FR-010]

## Notes

- Check items off as completed: `[x]`.
- Record requirement findings and links inline; do not use this checklist as an implementation test plan.
- Resolve formal-gate gaps and conflicts before generating implementation tasks.
- Resolution: all findings were incorporated into the approved specification and aligned design artifacts on 2026-08-06; T001 is complete.
