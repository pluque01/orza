# Research: Hide Connection Actions

## Decision: Retain the fixed lower control region and remove only its panel chrome

**Rationale**: `calculateLayoutWithActions` already reserves the five-row lower region for forms, operations, and conflicts. Keeping that geometry satisfies the clarified requirement not to redistribute the former Actions area. Replacing the titled, bordered rendering with the existing borderless action-legend rendering removes the panel while retaining status and applicable controls.

**Alternatives considered**:

- Reuse the normal browser layout: rejected because it reallocates the lower region to Tree and Details.
- Leave an empty fixed region: rejected because it hides usable status and action controls.
- Remove actions and keyboard bindings: rejected because it would violate keyboard-operable cancellation, quit, and recovery requirements.

## Decision: Reuse the existing action descriptor inventory and dispatch

**Rationale**: The descriptor inventory already drives rendering, Help, applicability, and key dispatch. Rendering the existing operation, conflict, and form descriptors as a borderless legend preserves semantic consistency and avoids duplicate control definitions.

**Alternatives considered**:

- Create a separate startup-only legend inventory: rejected because it risks divergence from keyboard dispatch.
- Render static text for status controls: rejected because it would not adapt to operation or conflict state.

## Decision: Cover presentation behavior across normal, startup, recovery, and minimum-size states

**Rationale**: The behavior is governed by layout state and operation state. Existing unit, conformance, resize, accessibility, and integration tests already model these paths and can verify both title absence and action-legend presence without external dependencies.

**Alternatives considered**:

- Test only SSH startup: rejected because the requirement removes the panel from every user-visible workflow.
- Test only a wide terminal: rejected because the constitution requires minimum-size usability.
