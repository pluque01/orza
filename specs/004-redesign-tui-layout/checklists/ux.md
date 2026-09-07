# UX Requirements Quality Checklist: Rediseño multipanel de la TUI

**Purpose**: Formal PR-review gate for completeness, clarity, consistency, measurability and scenario coverage of the multipanel TUI requirements
**Created**: 2026-08-07
**Feature**: [spec.md](../spec.md)

**Note**: This checklist evaluates the quality of the written requirements, not the implementation. Reviewers should record findings inline and require requirement changes for unresolved items.

## Requirement Completeness

- [x] CHK001 Are the required contents of Tree, Details and Actions fully enumerated for root, folder, connection, connection-form and centered-panel contexts? [Completeness, Spec §Definitions, Spec §FR-001, Spec §FR-006–FR-011]
- [x] CHK002 Are all actions that must use the Details form versus a centered panel documented as a closed inventory rather than examples introduced by “at least”? [Completeness, Ambiguity, Spec §FR-012, Spec §FR-015]
- [x] CHK003 Are the available actions and associated keys explicitly specified for every node type and focus owner, including root restrictions and shared-key collisions? [Completeness, Gap, Spec §FR-010–FR-011, Spec §FR-018–FR-019]
- [x] CHK004 Are content-priority requirements complete for every region when width, height or both are below 80x24? [Completeness, Spec §FR-004–FR-005, Spec §FR-019]
- [x] CHK005 Are requirements present for transient states such as initial catalog retrieval, reload in progress, save pending and SSH connection pending, or are those states explicitly excluded? [Completeness, Gap, Spec §User Scenarios, Spec §FR-009, Spec §FR-021–FR-022]

## Requirement Clarity

- [x] CHK006 Is “visually differentiated” defined with required non-color cues for each region and interaction state? [Clarity, Ambiguity, Spec §FR-001, Spec §FR-020]
- [x] CHK007 Are “consistent titles, separation, spacing and markers” specified precisely enough that two reviewers would agree on whether the visual hierarchy satisfies the requirement? [Clarity, Measurability, Spec §FR-020, Spec §SC-008]
- [x] CHK008 Is the meaning of “same visible transition” quantified or tied to an objectively identifiable state boundary? [Clarity, Ambiguity, Spec §FR-009, Spec §SC-001]
- [x] CHK009 Are wide-layout proportions and stacked-layout height allocation defined sufficiently to prevent incompatible interpretations while preserving required content? [Clarity, Gap, Spec §FR-003–FR-005]
- [x] CHK010 Are truncation, wrapping and “additional content” indicators specified for names, paths, endpoints, action labels and Unicode text in every affected region? [Clarity, Spec §FR-019, Spec §Edge Cases]

## Requirement Consistency

- [x] CHK011 Are the spec’s contextual-action requirements consistent with the closed root/folder/connection action matrix in the TUI contract? [Consistency, Spec §FR-010–FR-011, Contract §Contextual action contract]
- [x] CHK012 Is the requirement that Actions remain at the bottom consistent with the reduced-height priority rules and any permitted omission of secondary actions? [Consistency, Spec §Definitions, Spec §FR-004–FR-005]
- [x] CHK013 Are focus-transfer requirements consistent across the clarification, focus-owner definition, connection form, centered panels and restoration after errors? [Consistency, Spec §Clarifications, Spec §FR-002, Spec §FR-012–FR-016, Spec §FR-019, Spec §FR-022]
- [x] CHK014 Are the requirements for preserving current action semantics consistent with the new rule that unavailable shortcuts must have no effect? [Consistency, Spec §FR-010–FR-011, Spec §Assumptions]

## Acceptance Criteria Quality

- [x] CHK015 Can every layout outcome be objectively assessed at all six stated widths and both full/reduced heights without relying on subjective appearance? [Acceptance Criteria, Spec §SC-004–SC-007]
- [x] CHK016 Does the user-study criterion define participant eligibility, task wording, timing start/end, rating question and pass aggregation sufficiently for reproducible results? [Measurability, Gap, Spec §SC-008]
- [x] CHK017 Are the performance expectations for selection, focus, scrolling and resizing tied to a measurable latency target and controlled catalog/environment? [Acceptance Criteria, Gap, Spec §SC-001, Spec §SC-007]
- [x] CHK018 Do the success criteria cover each centered-panel kind and each allowed completion path rather than only representative examples? [Acceptance Criteria, Coverage, Spec §SC-003, Spec §SC-009]

## Scenario Coverage

- [x] CHK019 Are primary requirements complete for browsing each node type, changing focus, editing connections and completing centered-panel actions? [Coverage, Primary Flow, Spec §User Stories 1–4]
- [x] CHK020 Are alternate-flow requirements defined for moving backward through focus, canceling each interaction, switching layouts mid-flow and returning from Details to Tree? [Coverage, Alternate Flow, Spec §Clarifications, Spec §FR-004, Spec §FR-014, Spec §FR-016, Spec §FR-019]
- [x] CHK021 Are exception requirements complete for validation, persistence, stale revision, missing target, SSH failure and insufficient terminal space? [Coverage, Exception Flow, Spec §Edge Cases, Spec §FR-014, Spec §FR-021–FR-022]
- [x] CHK022 Are recovery requirements defined for retry, correction, reload, back, cancel and failed save while preserving the appropriate target, values and focus? [Coverage, Recovery Flow, Spec §FR-014, Spec §FR-016–FR-017, Spec §FR-021–FR-022]
- [x] CHK023 Are non-functional scenarios complete for no-color terminals, keyboard-only use, Unicode width, large catalogs and cross-platform terminal differences? [Coverage, Non-Functional, Spec §Edge Cases, Spec §FR-018–FR-020, Spec §SC-006–SC-008]

## Edge Case Coverage

- [x] CHK024 Are requirements explicit for zero-state variants: empty catalog, root only, empty folder and folder containing only subfolders? [Edge Case, Coverage, Spec §Edge Cases, Spec §FR-007–FR-008]
- [x] CHK025 Are requirements defined for repeatedly crossing the 79/80 boundary while Tree, Details, a dirty form or every centered-panel kind owns focus? [Edge Case, Spec §Edge Cases, Spec §FR-004, Spec §SC-005]
- [x] CHK026 Are minimum usable-height behavior and the point at which content may be omitted specified, including which controls must never disappear? [Edge Case, Gap, Spec §FR-005, Spec §FR-019]
- [x] CHK027 Are requirements complete for long/deep/Unicode content in Tree, Details, Actions, forms, help, confirmations and errors? [Edge Case, Completeness, Spec §Edge Cases, Spec §FR-003, Spec §FR-019]
- [x] CHK028 Are requirements defined for an asynchronous result arriving after selection, focus, layout or panel state has changed? [Edge Case, Gap, Spec §FR-004, Spec §FR-009, Spec §FR-021–FR-022]

## Non-Functional Requirements

- [x] CHK029 Are keyboard accessibility requirements defined for every focus owner, scrolling region, form control, modal choice and recovery path? [Accessibility, Completeness, Spec §FR-002, Spec §FR-013, Spec §FR-016, Spec §FR-018–FR-019]
- [x] CHK030 Are no-color requirements specific enough to distinguish active region, selected row, focused field, invalid field, primary action, warning and failure? [Accessibility, Clarity, Spec §FR-001, Spec §FR-013, Spec §FR-020, Spec §SC-006]
- [x] CHK031 Are confidentiality requirements complete across Details, forms, panels, errors, status, help and any diagnostic text? [Security, Completeness, Spec §FR-006, Spec §FR-022, Spec §Scope Boundaries]
- [x] CHK032 Are target-identification and explicit-confirmation requirements consistently defined for every destructive or connection-affecting action? [Security, Consistency, Spec §FR-015–FR-017, Constitution §II]
- [x] CHK033 Are bounded-memory and bounded-output requirements stated for large detail lists, help, errors and repeated resize events? [Performance, Security, Gap, Spec §FR-019, Spec §SC-007, Constitution §Security and Operational Constraints]

## Dependencies & Assumptions

- [x] CHK034 Is the assumption that existing action semantics and keybindings remain unchanged linked to a canonical inventory that reviewers can assess for compatibility? [Assumption, Dependency, Spec §FR-011, Spec §Assumptions]
- [x] CHK035 Are terminal capability and platform assumptions documented at requirement level, including the expected degradation when VT, color or key-modifier reporting differs? [Dependency, Gap, Spec §Edge Cases, Spec §FR-018–FR-020]
- [x] CHK036 Is the assumption that the existing visual identity will be preserved specific enough to establish which visual tokens and conventions are normative? [Assumption, Ambiguity, Spec §FR-020, Spec §Assumptions]

## Ambiguities & Conflicts

- [x] CHK037 Is optional mouse behavior explicitly excluded or defined, given that only mandatory mouse navigation is excluded? [Ambiguity, Spec §Scope Boundaries]
- [x] CHK038 Is the ordering of direct connections in folder Details specified or explicitly inherited from the catalog ordering contract? [Ambiguity, Gap, Spec §FR-007, Spec §Assumptions]
- [x] CHK039 Is the UI language and terminology policy defined for titles, action labels, errors and help so that copy remains consistent? [Ambiguity, Gap, Spec §FR-001, Spec §FR-010, Spec §FR-022]
- [x] CHK040 Are the requirements and TUI contract aligned on Details scrolling keys, action wrapping, compact labels and centered-panel behavior without introducing plan-only obligations absent from the spec? [Conflict, Traceability, Spec §FR-015–FR-020, Contract §Responsive layout contract, Contract §Details contract, Contract §Centered panel contract]

## Notes

- Check items off as the written requirements are reviewed and corrected: `[x]`.
- Record gaps, conflicts and proposed requirement changes inline.
- This formal gate is intended for peer review before task generation or implementation approval.
- Validation 2026-08-12: 40/40 PASS after three strict review passes against `spec.md`, `contracts/tui.md` and
  constitution. Normative closure added for exhaustive context content, deterministic geometry, Actions packing
  and priority, closed input maps/security precedence, every modal terminal path, no-color cues, Unicode wrap,
  full confirmation identity, terminal fallback, concurrency recovery and retained-memory bounds.
