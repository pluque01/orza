# Feature Specification: CLI Command Execution

**Feature Branch**: No branch (Git hook not configured)

**Created**: 2026-09-29

**Status**: Draft

**Input**: User description: "Un script o agente debe poder ejecutar comandos directamente sobre una conexion usando el CLI. Actualmente no puede porque requiere un terminal interactivo."

## Clarifications

### Session 2026-09-29

- Q: ¿Qué formato debe aceptar el CLI para el comando remoto? → A: Texto de shell remoto, con tuberías y redirecciones.
- Q: ¿Qué límite de tiempo debe aplicarse por defecto a cada comando remoto? → A: Límite de 5 minutos, configurable por invocación.
- Q: ¿Debe el comando remoto poder recibir los datos enviados por la entrada estándar del script? → A: Reenviar la entrada estándar del script.
- Q: ¿Cuándo debe recibir el script la salida estándar y de diagnóstico del comando remoto? → A: Entregar la salida conforme llega.
- Q: ¿Cómo debe reflejar el CLI el código de salida del comando remoto? → A: Conservar siempre el código remoto; los fallos locales usan un código documentado, aunque pueda coincidir.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Execute a Remote Command from a Script (Priority: P1)

As an operator or automation agent, I can request execution of a command on a saved connection through the CLI without an interactive terminal, so I can automate remote work using the connection catalog.

**Why this priority**: This is the requested capability and removes the current blocker for scripts and agents.

**Independent Test**: Invoke the CLI with a saved reachable connection and a command that produces output, then verify that the output, error output, and final result are available to the calling script without opening an interactive interface.

**Acceptance Scenarios**:

1. **Given** a saved connection with valid access and a reachable destination, **When** a script requests a remote command through the CLI without an interactive terminal, **Then** the command is executed on the selected destination and its standard output is available to the script.
2. **Given** a remote command that writes diagnostic output, **When** a script executes it through the CLI, **Then** that output is available separately from standard output.
3. **Given** a remote command that completes successfully, **When** the CLI finishes, **Then** it reports success with a successful result to the invoking process.
4. **Given** a remote command that finishes unsuccessfully, **When** the CLI finishes, **Then** it reports failure to the invoking process and preserves the remote command's outcome for script decisions.
5. **Given** a command containing remote shell syntax such as a pipe or redirection, **When** a script executes it through the CLI, **Then** the selected destination interprets that syntax as part of the remote command.
6. **Given** a script sends data through its standard input, **When** it executes a remote command through the CLI, **Then** the command receives that data without an interactive terminal.
7. **Given** a remote command produces output while it is still running, **When** a script executes it through the CLI, **Then** the script receives each standard and diagnostic output channel as it is produced.
8. **Given** a remote command completes, **When** the CLI returns control to the invoking process, **Then** the process receives the command's original exit code; connection and other local failures instead use a documented nonzero code without changing any remote command code.

---

### User Story 2 - Receive Safe, Actionable Automation Failures (Priority: P2)

As an operator or automation agent, I receive a clear failure result when the connection cannot be established or requires an interactive decision, so my automation can stop safely and report what needs attention.

**Why this priority**: Non-interactive use must fail predictably rather than hang, weaken connection security, or hide the reason for failure.

**Independent Test**: Execute a command with an unknown host identity, invalid credentials, an unavailable destination, and a missing connection; verify in each case that the invocation ends without waiting for terminal input, returns failure, and does not expose secrets.

**Acceptance Scenarios**:

1. **Given** a destination whose identity has not been approved or no longer matches the approved identity, **When** a script requests a remote command without an interactive terminal, **Then** the command is not executed and the CLI reports that explicit host-identity approval is required.
2. **Given** invalid or unavailable authentication, **When** a script requests a remote command, **Then** the CLI finishes with an actionable failure result without displaying or recording credentials.
3. **Given** a missing, invalid, or unreachable saved connection, **When** a script requests a remote command, **Then** no remote command is attempted and the CLI reports the affected connection and failure reason.
4. **Given** an interrupted or timed-out command execution, **When** the CLI ends the attempt, **Then** it releases the connection resources and reports failure without leaving the invoking terminal in an unusable state.

### Edge Cases

- The requested connection name or path does not identify exactly one saved connection.
- The requested command has no output, produces large output, or produces output on both standard and diagnostic channels.
- The destination closes the connection before the command completes.
- The command requests interactive input even though the invocation has no interactive terminal.
- The saved connection uses authentication that normally requires user input, such as a password or key passphrase not available to the invoking environment.
- The invoking process is canceled while a command is running.
- A host identity approval prompt would be required during non-interactive execution.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow a script or agent to select a saved connection and request execution of a remote command through the CLI without requiring an interactive terminal.
- **FR-002**: The system MUST execute the requested command only on the saved connection selected by the invocation and identify that connection in any resulting error.
- **FR-003**: The system MUST deliver the remote command's standard output to the invoking process as it is produced.
- **FR-004**: The system MUST deliver the remote command's diagnostic output separately from standard output as it is produced.
- **FR-005**: The system MUST communicate a successful or failed result through the CLI's process result so scripts and agents can make decisions without parsing human-oriented text.
- **FR-006**: When the remote command completes unsuccessfully, the system MUST preserve that unsuccessful outcome in the CLI result.
- **FR-007**: The system MUST complete a non-interactive invocation without waiting for terminal input. If the remote command requests input that cannot be supplied, the system MUST end the invocation with an actionable failure result.
- **FR-008**: The system MUST continue to verify host identity by default. When identity approval or changed-identity confirmation is required, a non-interactive invocation MUST not execute the remote command and MUST report that an explicit approval is required.
- **FR-009**: The system MUST report missing connections, invalid connection data, authentication failures, unreachable destinations, cancellations, and interrupted executions with actionable context that excludes credentials and other secrets.
- **FR-010**: The system MUST release connection and terminal resources when command execution succeeds, fails, is canceled, or is interrupted.
- **FR-011**: The system MUST preserve the existing interactive connection-start behavior; requesting non-interactive command execution MUST not require or launch the interactive interface.
- **FR-012**: The system MUST not persist command content, command output, credentials, private key material, passphrases, or session secrets solely as a result of non-interactive command execution.
- **FR-013**: The system MUST bound or stream command output so a long-running or high-output command cannot consume unbounded local memory.
- **FR-014**: The system MUST accept the remote command as shell text, including pipes and redirections, and pass that text to the selected destination without local shell interpretation.
- **FR-015**: The system MUST end a remote command that has not completed within 5 minutes by default, report an actionable timeout failure, and allow the invoking script or agent to set a different limit for that invocation.
- **FR-016**: The system MUST forward the invoking process's standard input to the remote command without requiring terminal interaction; when no input is supplied, the remote command MUST receive a closed input stream.
- **FR-017**: The system MUST return the remote command's original exit code to the invoking process when the command completes. Connection, authentication, identity-verification, cancellation, timeout, and other local failures MUST use a documented nonzero exit code; this code MAY also be returned by a remote command.

### Key Entities

- **Remote command request**: A request from a script or agent that identifies one saved connection and the command to run on its destination without an interactive terminal.
- **Command execution result**: The observable completion state of a remote command request, including success or failure, standard output, diagnostic output, and the remote command's outcome.
- **Saved connection**: A catalog entry that identifies an SSH destination and its non-secret access configuration. It is selected for a request but is not modified by command execution.

### Scope Boundaries

**Included**:

- Non-interactive execution of one requested command on one saved connection through the CLI.
- Standard output, diagnostic output, and process results suitable for scripts and agents.
- Remote shell command text, including pipes and redirections.
- Forwarding of the invoking process's standard input to the remote command.
- Safe non-interactive failure behavior for connection, authentication, identity-verification, cancellation, and remote-command failures.

**Excluded**:

- Interactive remote shell sessions as part of this command-execution flow.
- Changes to connection catalog management, authentication methods, or host-identity approval workflows.
- File transfer, port forwarding, command scheduling, multi-host execution, and persistent command history.
- Automatically approving unknown or changed host identities during non-interactive execution.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In 100% of tested invocations from a process without an interactive terminal, a command on a reachable saved connection starts without requiring terminal input.
- **SC-002**: In 100% of tested successful and unsuccessful remote commands, the invoking script can distinguish the result through the CLI process result and receives the command's standard and diagnostic output in their respective channels.
- **SC-003**: In at least 95% of 100 consecutive executions of a command that immediately returns on a reachable saved connection, the invoking process receives the final result within 5 seconds, excluding destination-side command duration.
- **SC-004**: In 100% of tested unknown-host, changed-host, authentication-failure, unavailable-destination, cancellation, and missing-connection cases, the invocation terminates without waiting for interactive input and reports actionable context without secrets.
- **SC-005**: In 100% of tested interrupted, canceled, successful, and failed executions, the invoking terminal remains usable and no connection resources remain active after completion.
- **SC-006**: At least 90% of operators in a scripted-task evaluation can execute a command on a saved connection and correctly determine its outcome without opening the interactive interface.
- **SC-007**: In 100% of tested commands that exceed their configured time limit, the invocation ends within 10 seconds of that limit, reports timeout failure, and releases its connection resources.
- **SC-008**: In 100% of tested commands that emit output before completion, the invoking process receives each output channel before the command completes.
- **SC-009**: In 100% of tested completed commands, the CLI process result uses the remote command's original exit code; in 100% of tested local failures, it uses the documented local-failure code without changing remote command codes that may have the same value.

## Assumptions

- A saved connection already exists and uses the catalog and trust settings established by the existing product.
- The invoking script or agent has permission to run the CLI and access any non-interactive authentication material required by the saved connection.
- A command that needs interactive input is outside the supported non-interactive flow and fails rather than prompting.
- Data intentionally supplied by a script through standard input is forwarded to the remote command; interactive input is never requested.
- Host-identity approval continues to require an explicit user decision through the existing approved workflow; automation does not bypass it.
- The feature executes a command on one selected connection per invocation and does not change the saved connection.
- Remote shell syntax is interpreted only by the selected destination; the local CLI does not interpret the command text.
- Each command has a 5-minute default time limit, and scripts or agents that need longer or shorter execution may set a limit for that invocation.
- The documented local-failure exit code may coincide with a code returned by a remote command; preserving the remote command's original code takes precedence.
- The CLI's exact command spelling and argument syntax will be defined during planning without changing the required behavior.

## Constitution Compliance

- Host identity verification remains enabled, and non-interactive execution rejects any trust decision that cannot be made explicitly.
- Secrets are neither displayed nor persisted, and error output excludes them.
- Completion, cancellation, interruption, and failure release resources deterministically and return an observable result to the invoking process.
- Automated coverage will include successful execution and principal failure paths without relying on a manually operated terminal.
