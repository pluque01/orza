# Feature Specification: Forget Host Key

**Feature Branch**: `012-forget-host-key`

**Created**: 2026-09-25

**Status**: Draft

**Input**: User description: "Quiero añadir una opcion que permita olvidar la clave del host. Esta acción debería realizarse tras confirmación. Se podría poner como opción al estar sobre una conexión."

## Clarifications

### Session 2026-09-25

- Q: ¿Debe poder olvidarse la clave únicamente desde la interfaz interactiva al seleccionar una conexión, o también mediante un comando no interactivo? → A: Interfaz y comando no interactivo.
- Q: ¿Debe el comando no interactivo requerir confirmación explícita? → A: No; debe poder ejecutarse directamente para automatización.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Forget a Host Key from a Connection (Priority: P1)

An operator selects a connection and chooses to forget its remembered host key so that the next connection requires a new trust decision if no standard trusted key applies.

**Why this priority**: This lets an operator deliberately revoke app-owned trust after a host rebuild, endpoint reassignment, or suspected key compromise.

**Independent Test**: Select a connection with an app-owned trusted key, confirm the forget action, and verify that the key is no longer treated as trusted for that endpoint.

**Acceptance Scenarios**:

1. **Given** a selected connection whose endpoint has an app-owned trusted key, **When** the operator confirms the forget action, **Then** the application removes that trust record and reports success.
2. **Given** a selected connection whose endpoint has an app-owned trusted key, **When** the operator cancels the confirmation, **Then** the trust record remains unchanged.
3. **Given** a selected connection whose endpoint has no app-owned trusted key, **When** the operator chooses the forget action, **Then** the application explains that there is no app-owned key to forget and makes no change.

---

### User Story 2 - Understand the Security Consequence (Priority: P2)

An operator sees the affected host and port in the confirmation so they can verify the intended target before removing trust.

**Why this priority**: Forgetting a key changes a security decision and must not be applied to the wrong endpoint by mistake.

**Independent Test**: Open the forget action for a selected connection and verify that its confirmation identifies the exact host and port, offers confirm and cancel choices, and can be completed entirely by keyboard.

**Acceptance Scenarios**:

1. **Given** a selected connection, **When** the operator starts the forget action, **Then** the confirmation identifies the connection endpoint and warns that future connections may request host-key trust again.
2. **Given** the confirmation is displayed, **When** the operator cancels or leaves it, **Then** the application returns to a stable connection-focused screen without changing trust.

---

### Edge Cases

- The endpoint's trust record is removed by another active application session between opening the confirmation and confirming it; the application reports that no change was made and returns to a stable screen.
- The catalog cannot be updated or the removal cannot be verified; the application reports an actionable error and retains the existing trust record.
- Multiple connections share the same host and port; forgetting the key from one connection removes the shared app-owned trust, so future connections through any of them require the normal trust evaluation.
- A matching or revoked key remains in the user's standard SSH trust data; the action does not modify that external trust data, and its normal effect continues to apply.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The application MUST expose a "Forget host key" action while a connection is selected and a non-interactive command that targets a connection.
- **FR-002**: The interactive action MUST require an explicit confirmation before removing app-owned trust.
- **FR-003**: The confirmation MUST identify the target host and port and state that the action removes only app-owned trust for that endpoint.
- **FR-004**: The confirmation MUST clearly provide confirm and cancel choices, and both MUST be usable with the keyboard.
- **FR-005**: On confirmation, the application MUST remove the app-owned host-key trust record for the selected connection's host and port.
- **FR-006**: On cancellation, interruption, or leaving the confirmation, the application MUST not alter the trust record.
- **FR-007**: If no app-owned trust record exists for the selected endpoint, the application MUST make no change and present an understandable result.
- **FR-008**: If the trust record changes or disappears before confirmation can be applied, the application MUST make no unintended change and report the outcome.
- **FR-009**: If removal fails, the application MUST report an actionable error and preserve trust unless removal was verified as successful.
- **FR-010**: The action MUST not create, modify, or remove entries in the user's standard SSH trust files.
- **FR-011**: After a successful removal, the next connection to that host and port MUST use the normal trust evaluation and request an explicit decision when no standard trusted key applies.
- **FR-012**: The non-interactive command MUST remove app-owned trust without an interactive or explicit confirmation step, so it can be used in automation.

### Key Entities

- **App-owned host trust**: A remembered host-key decision owned by the application and associated with a canonical host and port.
- **Connection endpoint**: The host and port of the selected connection; it determines which app-owned host trust is affected.
- **Forget confirmation**: The explicit operator decision that authorizes removing app-owned trust for one endpoint.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can remove an app-owned host key from a selected connection in no more than three keyboard actions after opening the connection actions.
- **SC-002**: 100% of confirmed forget operations remove trust only for the selected host and port, without modifying standard SSH trust files.
- **SC-003**: 100% of cancelled, interrupted, or failed forget operations leave the existing app-owned trust unchanged unless success was explicitly reported.
- **SC-004**: In usability testing, at least 95% of operators correctly identify the affected host and port before confirming the action.

## Assumptions

- App-owned trust is shared by all connections that use the same canonical host and port; it is not owned by an individual connection.
- The action is available in the interactive connection-focused interface with confirmation and through a non-interactive command intended for automation without confirmation.
- Forgetting app-owned trust does not override matching or revoked keys from the user's standard SSH trust data.
- The existing connection action pattern supplies the appropriate confirmation and error presentation conventions.
