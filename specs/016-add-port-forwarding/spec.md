# Feature Specification: Host Port Forwarding

**Feature Branch**: No branch (Git hook not configured)

**Created**: 2026-10-03

**Status**: Draft

**Input**: User description: "Quiero que añadas la funcion para hacer un port-forwarding de un host. Esto debe de funcionar de manera similar a lo disponible con el comando ssh. Para la TUI debe ser intuitivo para el usuario."

## Clarifications

### Session 2026-10-03

- Q: Si el servidor acepta un túnel remoto pero no permite comprobar desde Orza quién puede acceder realmente al puerto, ¿cómo debe actuar la aplicación? → A: Permitir el túnel con una advertencia explícita: el alcance real depende de la configuración del servidor y no está verificado por Orza.
- Q: ¿Qué debe ocurrir con los túneles activos cuando el usuario abre una shell SSH desde la TUI? → A: Mantener los túneles activos durante la shell y volver a la TUI al cerrarla; mientras la shell ocupa el terminal, no se muestran los controles de los túneles.
- Q: ¿Dónde prefieres consultar y gestionar los túneles cuando estás en la TUI sin una shell abierta? → A: En un panel permanente adicional con todos los túneles, visible junto al catálogo. El panel Actions ya no existe y no debe reintroducirse.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Access a Service Through a Local Tunnel (Priority: P1)

As an operator, I select a saved host and forward a local port to a service reachable from that host, so I can use that service without exposing it publicly or opening a remote shell.

**Why this priority**: Local forwarding is the primary service-access workflow and provides a useful standalone capability.

**Independent Test**: Select a saved host, configure a local listening port and a destination service, start the tunnel, access the service through the local port, and stop the tunnel.

**Acceptance Scenarios**:

1. **Given** a selected saved connection, **When** I open its port-forwarding action, **Then** I see Local forwarding selected by default, separate listening and destination fields, and an explanation of which machine listens and which machine reaches the destination.
2. **Given** valid settings and a reachable service, **When** I confirm the host and tunnel summary and complete existing trust and authentication decisions, **Then** the tunnel becomes Active without opening a shell and my local application can exchange data with that service.
3. **Given** an empty required field, an invalid address, or a port outside 1 through 65535, **When** I attempt to start, **Then** the form identifies the invalid field, retains my other values, and starts no tunnel.
4. **Given** an occupied or unauthorized listening port, **When** I start, **Then** I receive an actionable error identifying that endpoint and can edit or retry without restarting Orza.
5. **Given** a draft tunnel form, **When** I cancel and confirm discard if the form is changed, **Then** no connection starts and I return to the selected host without altering its saved settings.

---

### User Story 2 - Monitor and Stop Tunnels Safely (Priority: P1)

As an operator, I can inspect and stop running tunnels while continuing to browse the catalog, so I know which access paths remain open and can close them deliberately.

**Why this priority**: Visible lifecycle and safe shutdown are essential to avoid unnoticed network exposure and stale ports.

**Independent Test**: Start two local tunnels on distinct ports, browse to another host, stop one tunnel, interrupt the other's host connection, and verify the displayed states and released ports.

**Acceptance Scenarios**:

1. **Given** one or more running tunnels, **When** I browse another catalog entry, **Then** forwarding continues and a permanent Tunnels panel remains visible alongside the catalog, showing the active count and session-wide list identifying each tunnel's host, direction, endpoints, and state without opening a separate view.
2. **Given** two active tunnels, **When** I confirm stopping one identified tunnel, **Then** its listener and forwarded connections close and the other tunnel remains usable.
3. **Given** a disconnected host, **When** Orza detects the loss, **Then** its affected tunnels become Failed rather than Active, local listening ports are released, and I can explicitly retry with a new confirmation.
4. **Given** active tunnels, **When** I request application exit, **Then** I see the tunnels that will close and can cancel exit or confirm closing all of them before the application exits.
5. **Given** a running tunnel whose saved host is edited, moved, or deleted, **When** I inspect it, **Then** it still identifies the originally confirmed host and endpoints; a retry uses the current record after confirmation or reports that it no longer exists.
6. **Given** active tunnels in the TUI, **When** I open an interactive SSH shell, **Then** those tunnels continue forwarding while the shell owns the terminal and their controls are temporarily unavailable; closing the shell returns to the TUI with their current states and controls restored, without stopping them solely because the shell ended.
7. **Given** no tunnels in the current TUI session, **When** I browse the catalog, **Then** the Tunnels panel remains visible with a zero active count, a clear empty state, and guidance for starting forwarding on a selected host; no Actions panel appears.
8. **Given** more tunnels than fit in the panel or a terminal resized to 40x12, **When** I focus and navigate the Tunnels panel using the keyboard, **Then** I can reach every tunnel, inspect its endpoints and warnings, and access its stop or recovery controls without losing catalog selection or tunnel state.

---

### User Story 3 - Use Remote and Dynamic Forwarding (Priority: P2)

As an operator, I can expose a service reachable from my computer through a port on a saved host, or use that host as a dynamic proxy, so I can handle the other common forwarding workflows available with SSH.

**Why this priority**: These modes extend familiar SSH forwarding behavior while keeping local forwarding the default and simplest path.

**Independent Test**: Separately create a remote tunnel to a local test service and a dynamic tunnel, then verify access from the remote host and from a proxy-configured local application respectively.

**Acceptance Scenarios**:

1. **Given** Remote forwarding selected, **When** I configure and confirm a remote listening endpoint and a destination reachable from my computer, **Then** clients of the remote listening port can exchange data with that destination and the summary explains the direction.
2. **Given** Dynamic forwarding selected, **When** I configure and start a local listening endpoint, **Then** the interface shows no fixed destination fields, explains how to use it as a SOCKS5 proxy, and compatible clients can reach destinations through the selected host.
3. **Given** a listening address outside loopback, **When** I request any forwarding mode, **Then** I must explicitly acknowledge that other machines may access the listener in addition to confirming the host and endpoints.
4. **Given** a host that denies forwarding or reports that it cannot honor the requested remote listening address, **When** I start remote forwarding, **Then** I receive a refusal or unsupported-policy error and Orza does not claim the requested exposure was established.
5. **Given** a destination temporarily refuses a forwarded connection, **When** a client attempts access, **Then** that attempt fails with safe diagnostic context while the listening tunnel remains available for later attempts.
6. **Given** a host that accepts remote forwarding without allowing Orza to verify the actual listening scope, **When** the tunnel becomes Active, **Then** its status explicitly warns that the server accepted the request but its actual listening scope is unverified and depends on server configuration; the requested endpoint is not presented as a verified endpoint.

---

### User Story 4 - Start a Tunnel from the CLI (Priority: P3)

As an operator, I can start the same forwarding modes on a saved host from the CLI and keep the tunnel running in the foreground, so I can use familiar command-line workflows without navigating the TUI.

**Why this priority**: CLI access complements the existing shared catalog while the requested intuitive TUI remains the primary workflow.

**Independent Test**: Start each mode from the CLI using a saved host, verify its readiness and service access, then interrupt the invocation and verify cleanup.

**Acceptance Scenarios**:

1. **Given** valid forwarding settings, **When** I invoke foreground CLI forwarding, **Then** it reports the confirmed host, mode, listening endpoint, and readiness without launching the TUI or a remote shell.
2. **Given** an invocation without an interactive terminal that requires trust approval or credential input, **When** I start forwarding, **Then** it fails without prompting or approving trust; preapproved hosts with available credentials can start without terminal input.
3. **Given** a foreground tunnel, **When** I interrupt it, **Then** the invocation closes its listener and forwarded connections and returns a documented process result.
4. **Given** a non-interactive request for a non-loopback listener without separate exposure acknowledgement, **When** I start forwarding, **Then** it fails without prompting or starting a tunnel; supplying that explicit acknowledgement allows startup if all other prerequisites are satisfied.

### Edge Cases

- Duplicate or overlapping listening addresses and ports, including wildcard addresses, must fail clearly without disrupting an existing tunnel.
- Hostnames, IPv4 addresses, and IPv6 addresses must not be confused with port separators; destination names are resolved from the machine responsible for reaching them.
- A host may permit local forwarding but deny remote forwarding, or enforce remote loopback binding regardless of the requested address.
- A server may accept remote forwarding without revealing its actual listening scope. This does not prevent activation, but both TUI and CLI must show an explicit unverified-scope warning rather than imply verified loopback isolation or external exposure.
- A tunnel may be Active while its destination is unavailable; listener readiness is not a guarantee that every destination request succeeds.
- Cancellation during trust, authentication, or startup must prevent late completion from activating a tunnel.
- Resizing, no-color mode, and narrow terminals must preserve drafts, state, and access to cancel, help, and stop controls.
- An empty or overflowing Tunnels panel must remain usable: empty state explains how to begin, and overflow supports keyboard navigation and inspection. At supported narrow sizes the panel may stack with catalog regions but must not become a hidden, separately opened view.
- Catalog changes between confirmation and startup must require review rather than redirecting forwarding silently.
- Remote network loss can prevent immediate verification of remote listener removal; Orza must report the loss without claiming a remote cleanup guarantee it cannot verify.
- Idle tunnels remain open until explicitly stopped or their transport ends; repeated destination failures must not accumulate unbounded diagnostics.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST offer Local, Remote, and Dynamic forwarding for a saved host, with the direction and destination reachability corresponding to SSH local, remote, and dynamic forwarding respectively.
- **FR-002**: Local forwarding MUST listen on the user's computer and reach the specified destination from the selected host. Remote forwarding MUST listen on the selected host and reach the specified destination from the user's computer. Dynamic forwarding MUST provide a local SOCKS5 proxy whose destinations are reached from the selected host.
- **FR-003**: Forwarding MUST work without requiring a remote shell and MUST preserve existing shell and remote-command access and security rules. Opening a shell from the TUI MUST keep existing tunnels forwarding; tunnel controls are unavailable while the shell owns the terminal. Closing the shell MUST return to the TUI and restore current tunnel states and controls rather than end the application or stop tunnels solely because the shell ended.
- **FR-004**: The TUI MUST provide a discoverable forwarding action for selected connections, default to Local mode, label which machine listens and reaches the destination, and provide contextual explanations and example values without requiring users to know SSH option syntax.
- **FR-005**: The form MUST request mode, listening address, and listening port, plus destination host and port only for Local and Remote modes. It MUST accept hostnames and IPv4/IPv6 endpoints where applicable, require ports from 1 through 65535, reject invalid input before startup, and preserve valid draft values when showing field errors.
- **FR-006**: Listening MUST default to loopback in every mode. Non-loopback binding MUST require an explicit address choice and a separate exposure warning acknowledgement; unavailable bindings MUST fail without falling back to broader access or a different port.
- **FR-007**: Before interactive startup, the system MUST show the saved connection identity, SSH endpoint, mode, listening endpoint, and applicable destination for explicit confirmation. A changed connection revision before network activity MUST require renewed review rather than silently using the new target.
- **FR-008**: Forwarding MUST reuse existing host-identity verification and authentication rules. Unknown or changed identities MUST require explicit approval and revoked identities MUST be rejected. Secret authentication material MUST be masked during entry and MUST NOT be displayed after entry, included in summaries or diagnostics, or recorded by forwarding.
- **FR-009**: The system MUST distinguish Starting, Active, Stopping, Stopped, and Failed states using text. Active MUST mean the listening request was accepted and its forwarding transport is established, not that all destination services are reachable or that a remote listener's actual scope has been verified.
- **FR-010**: The TUI MUST remain usable for catalog browsing while tunnels run and display a permanent additional panel titled Tunnels alongside the catalog regions in the normal browser view. This panel MUST show the active-tunnel count and the current TUI session's tunnel list across hosts, identifying each tunnel by its confirmed host, mode, endpoints, and current state. It MUST remain visible with a clear empty state when there are no tunnels and MUST support keyboard focus, navigation, inspection of complete values and warnings, and stop or recovery actions when its content overflows. The existing action legend MUST remain available, and the removed Actions panel MUST NOT be reintroduced.
- **FR-011**: Users MUST be able to run at least two tunnels on non-conflicting listening endpoints and stop one without closing the other. Each tunnel MUST accept multiple simultaneous client connections.
- **FR-012**: Stopping a tunnel MUST identify its target and request confirmation, then close its listener and existing forwarded client connections. Application exit with active tunnels MUST offer cancel or confirmation identifying the affected tunnels; handled process termination MUST clean up without waiting for confirmation.
- **FR-013**: Startup failure, cancellation, handled termination, and transport loss MUST release locally owned listeners and forwarding resources. Failed tunnels MUST NOT reconnect automatically; explicit retry MUST revalidate the current saved connection and require interactive confirmation where applicable.
- **FR-014**: Errors MUST identify the affected tunnel or request, safely describe validation, binding, trust, authentication, server-policy, transport, or destination failures, and offer the applicable edit, retry, or close action. Individual destination failures MUST NOT stop an otherwise usable listener or other tunnels.
- **FR-015**: Remote forwarding MUST report a server refusal or a reported inability to honor the requested listening scope as a startup failure. If the server accepts the request but its actual listening scope cannot be verified, the tunnel MAY become Active, but both TUI and CLI MUST explicitly warn that the actual scope is unverified and depends on server configuration, and MUST label the displayed endpoint as requested rather than verified. User-facing help MUST explain server binding-policy constraints, that accepting a loopback request does not prove loopback-only exposure, and that a remote endpoint forced to disappear may only be released when the server detects transport loss.
- **FR-016**: All TUI forwarding workflows MUST be keyboard-operable with visible contextual controls for help, cancel, back, and stopping. They MUST remain usable at 40x12, support the full layout at 80x24, preserve state on resize, and convey equivalent state and error meaning without color. At supported narrow sizes, the permanent Tunnels panel MAY stack with the catalog regions and scroll its contents but MUST remain visible in the normal browser view rather than require opening a separate view. Existing modal and below-minimum-size protections remain applicable. Leaving a changed session-only draft MUST offer discard or cancel, with cancel preserving the draft; it MUST NOT suggest that a reusable profile can be saved.
- **FR-017**: Running tunnels MUST retain their confirmed configuration when saved connections change. A removed saved connection MUST prevent retry without silently selecting another host.
- **FR-018**: The CLI MUST provide foreground Local, Remote, and Dynamic forwarding for a saved connection, report readiness and failures, and use documented process results on startup failure, transport loss, and cancellation. Non-interactive invocations MUST neither prompt nor approve trust and MUST fail if required authentication material is unavailable. Non-interactive requests for non-loopback listening MUST supply a separate explicit exposure acknowledgement; otherwise they MUST fail without prompting or starting a tunnel.
- **FR-019**: Tunnel settings and runtime states MUST be session-only, without automatic startup on relaunch. The system MUST NOT record forwarded payloads or persist tunnel diagnostics; diagnostic retention during a session MUST be bounded.
- **FR-020**: User-facing documentation MUST explain direction, destination-name resolution, safe binding, proxy use, supported endpoints, lifecycle, and server restrictions. Automated acceptance coverage MUST include each mode's success path and its principal validation, denial, cancellation, and cleanup failures.

### Key Entities

- **Saved host connection**: Existing catalog identity and access settings used to establish the tunnel; it is selected but not modified by forwarding.
- **Forwarding request**: Session-only mode, listening address and port, optional fixed destination, confirmed host identity and revision, and explicit exposure acknowledgement where needed.
- **Running tunnel**: A forwarding request with a stable session identity, captured host and endpoints, lifecycle state, and any current safe error context.
- **Forwarded client connection**: One client's data exchange through a tunnel; its completion or destination failure is independent of other client connections.

### Scope Boundaries

**Included**:

- Local and remote fixed-destination TCP forwarding and local dynamic SOCKS5 TCP forwarding, comparable in purpose to `ssh -L`, `ssh -R`, and `ssh -D`.
- Keyboard-first creation, monitoring, cancellation, retry, and stopping in the existing TUI.
- A permanent session-wide Tunnels panel alongside the catalog, preserving the existing action legend without restoring an Actions panel.
- Foreground CLI forwarding using existing saved connections and security rules.
- Multiple concurrent session-only tunnels and safe loopback defaults.

**Excluded**:

- UDP forwarding, Unix-domain-socket forwarding, remote dynamic proxies, and privileged-port elevation.
- Saved tunnel profiles, automatic startup or reconnection, background daemons, and a single CLI invocation that requests both forwarding and a remote shell. Opening a shell separately from the TUI while its tunnels run is included.
- SSH configuration import, jump hosts, agent forwarding, file transfer, and changes to existing authentication methods.
- Payload inspection, traffic history, throughput dashboards, and guarantees about unreachable remote servers beyond locally owned cleanup.
- Shared tunnel discovery or management between separate CLI processes and TUI sessions.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 90% of operators familiar with saved connections but new to Orza forwarding can create a working local tunnel within 2 minutes using only in-app guidance, excluding credential entry and external trust verification.
- **SC-002**: At least 90% of those operators can correctly identify the listening machine and destination-reaching machine for Local and Remote modes and identify Dynamic mode as a proxy in a task evaluation.
- **SC-003**: In 100% of controlled acceptance cases for each mode, client data reaches the intended destination and responses return unchanged, including two concurrent client connections and two concurrent non-conflicting tunnels.
- **SC-004**: In 100% of tested default starts, only loopback access is requested; all explicitly requested non-loopback starts require exposure acknowledgement. Server refusals and reported inability to honor the binding never produce an Active state. Every accepted remote request whose actual listening scope cannot be verified displays an explicit unverified-scope warning and identifies the displayed endpoint as requested, not verified.
- **SC-005**: In 100% of tested stop, startup-cancel, startup-failure, and handled-exit cases, locally owned listening ports can be reused within 2 seconds after completion, and stopping one tunnel leaves unrelated tunnels usable.
- **SC-006**: In 100% of controlled trust, authentication, port-conflict, forwarding-denial, and destination-failure cases, the user sees the affected target and an actionable explanation without secrets and can recover without restarting the application.
- **SC-007**: At both 80x24 and 40x12, with and without color, all primary creation, inspection, and stopping tasks can be completed using only the keyboard without losing draft values during resize. In every tested normal browser view, the Tunnels panel and active count remain visible while browsing any catalog host, including zero-tunnel and overflowing-list cases, and no Actions panel is displayed.
- **SC-008**: In 100% of tested non-interactive CLI starts, approved hosts with available credentials can forward without prompting when all required settings and acknowledgements are supplied; missing exposure acknowledgement or required trust or credential input produces a documented failure without waiting for input.

## Assumptions

- "Similar to ssh" means the common local, remote, and dynamic TCP forwarding modes rather than complete OpenSSH option or configuration compatibility.
- Users already have a saved host and permission to access it; host-side forwarding permission and destination availability are external dependencies.
- The feature follows the existing English-only interface and current platform and terminal support; this specification does not introduce localization.
- SOCKS5 CONNECT is the supported dynamic-proxy use case. Proxy clients configure themselves using the displayed listening endpoint; no browser or operating-system proxy settings are changed automatically.
- Port zero and automatic port allocation are outside scope; users choose a fixed port from 1 through 65535 without requiring elevated privileges.
- Destination names resolve from the selected host for Local and Dynamic forwarding and from the user's computer for Remote forwarding.
- Session-only settings are the simplest default because persistence was not requested. Closing the application ends its tunnels, and returning to a form does not save a reusable profile.
- A healthy idle tunnel has no automatic lifetime limit. Network-loss detection depends on transport and environment behavior, so no fixed detection deadline is promised.
- Non-loopback warnings do not replace firewall or server access controls; users remain responsible for external network policy.
- For accepted remote forwarding, Orza does not guarantee that the actual listening scope matches the request when the server does not expose verification information. Activation is allowed with an explicit warning; checking the server's actual binding and network policy remains the user's responsibility.
- A shell opened from the TUI is separate from its running tunnels: ending that shell does not end them. Tunnel states may change during the shell, and the TUI displays their latest states on return.
- The current TUI has catalog regions and an action legend, not an Actions panel. Tunnels is a new dedicated permanent panel; its exact geometry and keyboard bindings will be defined during planning within the stated visibility and minimum-size requirements.
- Each foreground CLI forwarding process reports and owns its own forwarding activity in its invoking terminal. The TUI panel lists only tunnels owned by that TUI session, not tunnels started by other processes.

## Constitution Compliance

- Host verification, explicit trust decisions, existing authentication safeguards, and secret exclusion remain mandatory.
- Connection-affecting starts, stops, and interactive exit identify their targets and require explicit confirmation; cancellation, help, and stable recovery remain keyboard-accessible.
- Tunnel lifecycle and cleanup are explicit, and unsupported or denied configurations cannot silently broaden exposure.
- Acceptance coverage includes success and principal failures, with native terminal and real-server policy checks required before release where automated coverage cannot verify them.
- Session-only settings and bounded diagnostics avoid adding unrequested durable state or payload history.
