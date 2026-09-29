# Feature Specification: Hide Connection Actions

**Feature Branch**: `013-hide-connection-actions`

**Created**: 2026-09-25

**Status**: Draft

**Input**: User description: "cuando se inicia una conexión aparece el panel de actions que ya no debería de existir"

## Clarifications

### Session 2026-09-25

- Q: Cuando se oculta el panel Actions durante el inicio de conexión, ¿debe su espacio redistribuirse entre los paneles restantes? → A: El panel Actions nunca debe aparecer; su espacio no se redistribuye y la leyenda de acciones posibles se conserva.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Use the Interface Without an Actions Panel (Priority: P1)

As an operator, I never see an Actions panel, while continuing to see the action legend appropriate to my current workflow.

**Why this priority**: The panel is redundant with the action legend and should not occupy a permanent interface region.

**Independent Test**: Visit catalog, modal, connection-startup, and recovery views and verify that none displays an Actions panel while each continues to display its applicable action legend.

**Acceptance Scenarios**:

1. **Given** any catalog, modal, startup, or recovery view, **When** the operator views the interface, **Then** no Actions panel is displayed.
2. **Given** a view with available keyboard actions, **When** the operator views the interface, **Then** its applicable action legend remains visible.
3. **Given** a connection startup is waiting for host identity verification, authentication, or network progress, **When** the operator views the interface, **Then** no Actions panel is displayed and the available connection-specific controls remain visible.

---

### User Story 2 - Retain Safe Connection Control (Priority: P2)

As an operator, I can still cancel or quit an in-progress connection without relying on the removed Actions panel.

**Why this priority**: Removing visual clutter must not remove keyboard access to safe recovery actions.

**Independent Test**: During an in-progress connection, trigger cancellation and quit through the displayed connection controls and verify that the application returns to a stable state or exits as requested.

**Acceptance Scenarios**:

1. **Given** a connection startup is in progress, **When** the operator chooses Cancel, **Then** the attempt is canceled and the interface returns to a stable catalog state.
2. **Given** a connection startup is in progress, **When** the operator chooses Quit, **Then** the application completes safe cleanup before exiting.

---

### Edge Cases

- A terminal at the minimum supported size must omit the Actions panel without hiding the action legend, connection status, or safe recovery controls.
- A startup failure, cancellation, or host-trust rejection must not reintroduce the Actions panel.
- Catalog and management workflows must retain their existing action legends without an Actions panel.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST not display an Actions panel in any catalog, modal, connection-startup, session-recovery, or error view.
- **FR-002**: The system MUST preserve the applicable action legend in every view where it was previously available.
- **FR-003**: The system MUST leave the former Actions panel area unredistributed when the panel is removed.
- **FR-004**: The system MUST continue to show connection progress, identity-verification information, and any controls applicable to the current connection-startup state.
- **FR-005**: The system MUST preserve keyboard cancellation and quit behavior during connection startup.
- **FR-006**: The system MUST preserve readable, keyboard-operable startup and recovery states at the documented minimum terminal size.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In 100% of tested catalog, modal, startup, recovery, and error states, the rendered interface contains no Actions panel title or Actions panel region.
- **SC-002**: In 100% of tested views with available actions, the applicable action legend remains visible after the Actions panel is removed.
- **SC-003**: In 100% of tested connection-startup states, users can identify startup status and any available cancel or quit action without an Actions panel.
- **SC-004**: All tested catalog and management workflows retain their existing action legends and keyboard controls.

## Assumptions

- The Actions panel is permanently redundant with the action legend and is removed from all user-visible workflows.
- Removing the panel does not cause its former area to be reassigned to another panel or content region.
- The action legend remains the user-visible source for available keyboard actions; connection-specific modals and status views continue to present their own controls.
- The feature does not change host identity verification, authentication, session lifecycle, or the available connection-control keys.
- The existing documented minimum terminal size remains unchanged.

## Constitution Compliance

- Host identity verification and explicit trust decisions remain visible and unchanged during startup.
- Cancellation and quit remain keyboard-operable while a connection attempt is active.
- Automated coverage will include startup, recovery, and minimum-size behavior.
