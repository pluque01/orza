# Data Model: Rediseño multipanel de la TUI

> **Superseded viewport fields**: Feature 005 replaces marker-row budgeting with scrollbar geometry while preserving logical-offset restoration.

No se añaden tablas ni datos persistentes. Todos los modelos son estado transitorio de presentación sobre
snapshot, formularios, operaciones y security boundaries existentes.

## LayoutState

| Field | Type | Rules |
|---|---|---|
| `width`, `height` | non-negative integer | Dimensiones reportadas en celdas/filas. |
| `displayMode` | wide / stacked / undersized | Undersized si width <40 o height <12; wide si no y width >=80; stacked en otro caso. |
| `tree`, `detail`, `actions` | optional rectangle | Presentes solo en wide/stacked y dentro del viewport. |
| `modal` | optional rectangle | Acotado; centrado cuando cabe. |
| `reduced` | bool | Wide/stacked con width <80 o height <24; incluye wide-short y no equivale a undersized. |

### Invariants

- Rectángulos no negativos, no solapados y dentro del terminal.
- Wide alinea Tree/Details y coloca Actions debajo; stacked ordena Tree/Details/Actions.
- Undersized no crea rectángulos base ni clampa/muta selection, expansion, viewport, form, focus, modal,
  conflict, operation o pending intent.
- Recuperar 40x12 recalcula rectángulos y revela exactamente el estado preservado.

## ViewportState

Contrato reutilizado por Tree, Details, connection form, Actions/Help, move picker, confirmations y errors.

| Field | Type | Rules |
|---|---|---|
| `offset` | non-negative integer | Clampado al contenido, no al valor persistido. |
| `contentLength` | non-negative integer | Líneas lógicas antes de markers. |
| `availableRows` | non-negative integer | Budget interior después de title/border/active controls. |
| `hasPrevious`, `hasNext` | bool | Derivados de offset y rango visible. |
| `activeLine` | optional index | Selection/field/error/recovery que debe quedar visible. |

### Projection rules

- `↑ more` reserva primera fila si `hasPrevious`; `↓ more` reserva última si `hasNext`.
- Con una/dos filas, active/error/recovery tiene prioridad; marker aparece solo en fila restante.
- Toda línea que excede display width contiene `…` dentro del ancho.
- Markers no reciben foco ni desplazan active/error/recovery.
- Resize preserva offset cuando válido y lo clampa sin cambiar contenido/model bytes.

## FocusOwner

| Value | Allowed input |
|---|---|
| `tree` | Movement, expansion, object actions, Tab. |
| `detail` | Read-only scroll, object actions, Tab/Shift+Tab. |
| `connection_form` | Field navigation, Save, Back/Quit. |
| `modal` | Solo controles del modal kind actual. |

Exactamente uno existe. Modal posee foco cuando activo; closing restaura `openedFrom`. Operation/resize/result
no transfieren foco salvo transición aceptada explícitamente.

`FocusOwner` describe solo las cuatro superficies de aplicación. `SecurityInputState`, cuando existe,
preempte input y operation filtering sin reemplazarlo; al cerrar revela el mismo FocusOwner.

## SecurityInputState

| Field | Type | Rules |
|---|---|---|
| `kind` | trust / secret | Specialized terminal boundary outside ModalState. |
| `target`, `fingerprint`, `status` | optional safe public values | Trust only; no secrets/raw cause. |
| `decision` | pending / accept / cancel | Trust only; pending blocks every application key. |
| `secretLifecycle` | optional masked-input owner | Secret only; raw value never enters presentation model. |
| `preservedFocus` | FocusOwner | Nunca se muta por prompt/resize. |
| `viewport` | ViewportState | Bounded in normal/reduced/undersized shells. |

Specialized verifier/terminal owns this state outside ModalState. Trust Accept must be explicit; Cancel persists
no trust and starts no session. Secret input preserves existing masking/zeroization. Security dispatch precedes
operation filtering. In undersized, input remains pending and hidden behind common Help/safe Quit shell; resize
cannot accept. Quit/cancel waits operation/session cleanup.

## SelectionContext

| Field | Type | Rules |
|---|---|---|
| `nodeID` | stable ID | Identidad; row index nunca autoriza acción. |
| `revision` | revision | Capturada para CAS. |
| `kind` | root / folder / connection | Determina detail/actions. |
| `path` | public bytes/string | Se conserva; projection es terminal-safe. |
| `ancestorIDs` | ordered IDs | Parent → root para fallback determinista. |
| `rowIndex` | derived integer | Solo presentación. |

Al desaparecer durante navigation, se selecciona el primer `ancestorID` aún existente o root.

## CapturedTarget

| Field | Type | Rules |
|---|---|---|
| `id`, `revision`, `kind` | stable domain values | Inmutables para interacción/operación. |
| `path` | public value | Target visible seguro. |
| `endpointOrScope` | optional public value | Endpoint SSH o alcance destructivo. |
| `ancestorIDs` | ordered IDs | Permite Back fallback sin row retargeting. |

No contiene username secrets, credentials, key content, passphrases ni raw backend errors.

## ConflictState

| Field | Type | Rules |
|---|---|---|
| `type` | missing / revision_changed | Resultado de same-ID comparison. |
| `target` | CapturedTarget | Nunca se sustituye por selección corriente. |
| `owner` | form / modal / operation | Surface bloqueada. |
| `detailVisible` | bool | Escape lo pone false; compact banner permanece. |
| `blocked` | bool | Siempre true mientras conflicto no se resuelva/abandone. |

Transitions:

- `r` Reload: refresh catalog y recheck same ID, conserva values/focus; no retarget.
- `b` Back: abandona owner y selecciona nearest existing ancestor/root.
- `Esc` Cancel warning: oculta detail, conserva compact banner, blocked state y recovery actions.
- Resolved same ID/revision elimina conflict; missing/changed lo mantiene.

Persistence/network están prohibidos si `blocked`.

## DetailState

| Field | Type | Rules |
|---|---|---|
| `targetID`, `kind` | stable values | Coinciden con SelectionContext. |
| `heading`, `path` | safe projection | Inglés controlado + user value terminal-safe. |
| `fields` | ordered public fields | Connection: name/path/endpoint/user/method/identity reference. |
| `directConnections` | ordered list | Folder/root immediate connection children only. |
| `viewport` | ViewportState | Reset a cero al cambiar target; preserve/clamp en resize. |

Secrets/raw errors no entran. Empty folder muestra mensaje inglés explícito.

## ActionDescriptor

| Field | Type | Rules |
|---|---|---|
| `id`, `binding` | stable values | Binding existente o recovery binding. |
| `label` | controlled English | Única label canónica CLI/TUI; no alternate compact label. |
| `priority` | recovery / primary / navigation / secondary | Orden/budget. |
| `category` | domain / recovery / navigation | Inventarios distintos; no altera applicability. |
| `appliesTo` | predicate | Mismo predicate en Actions, Help y dispatch. |

Normal inventories:

- Root: `n/f/r/?/q`.
- Folder: `n/f/e/m/d/r/?/q`.
- Connection: `c/n/f/e/m/d/r/?/q`.
- `Ctrl+C` aliases `q`; `n/f` on connection targets parent.

Operation inventory: `Esc` Cancel, `?` Help, `q`/`Ctrl+C` Quit only. Conflict inventory: `r` Reload, `b`
Back, `Esc` Cancel warning plus Help/Quit where safe. Unlisted keys are inert.

Domain inventory is the exact root/folder/connection set. Recovery inventory replaces domain actions while
blocked. Navigation controls augment the active region but never count as domain actions.

Precedence: running/cancel_requested/cleaning operation inventory wins. A conflict result first completes and
clears OperationState after cleanup; only the following frame activates ConflictState and its inventory.

## ConnectionEditState

| Field | Type | Rules |
|---|---|---|
| `form` | existing connection form | Values, validation, dirty state, field focus. |
| `mode` | create / edit | Captures destination or target revision. |
| `treeContext` | SelectionContext | Tree visible, not interactive. |
| `viewport` | ViewportState | Keeps field/error/control visible. |
| `conflict` | optional ConflictState | Blocks Save without dropping values. |
| `quitAfterSave` | bool | Solo explicit Save from quit prompt. |
| `formError` | optional safe English error | Persistence/save failure above Save; never creates modal. |

Save error/conflict preserves all state. Quit-after-save occurs only after save commit, reload reconciliation
and operation cleanup.

Canonical focus order: Name, Folder, Host, Port, User, Method, conditional Identity file, conditional Remember
password, Save. Hidden conditional fields do not participate; hiding the focused field returns focus to Method.
Validation errors remain field-local; persistence failures use `formError`.

## ModalState

| Field | Type | Rules |
|---|---|---|
| `kind` | closed modal kind | Exactly one from list below. |
| `openedFrom` | FocusOwner | Restored on close/cancel. |
| `target` | optional CapturedTarget | Required when target-sensitive. |
| `payload` | kind-specific state | One payload. |
| `viewport` | ViewportState | Universal markers/truncation. |
| `recoverableError` | optional safe English error | Embedded, not replacement. |
| `conflict` | optional ConflictState | Embedded and blocked. |
| `helpVisible` | bool | Inline help when modal already owns focus. |
| `helpOpenedFrom` | optional payload control | Restored by `?` or `Esc` from inline Help. |
| `intent` | optional cancel / quit | Unsaved decision. |

Closed kinds: `folder_create`, `folder_edit`, `move_picker`, `delete_connection`, `delete_folder`,
`connect_confirmation`, `unsaved_changes`, `help`, `operation_error`, `ssh_failure`.

This is exactly ten concrete variants over one slot/renderer. `?` toggles bounded inline Help on a non-Help
kind; `Esc` from inline Help restores `helpOpenedFrom` rather than closing the owner modal.

Connection forms and host trust prompts are not modal kinds. Zero/one modal only. Unknown kind is invalid.

## OperationState

| Field | Type | Rules |
|---|---|---|
| `id` | monotonic operation ID | Solo matching result actualiza. |
| `kind` | initial_load / reload / save / ssh_start | Closed enum. |
| `target` | CapturedTarget or catalog | Public safe target. |
| `ownerSurface` | root / form / modal / trust boundary | Preserved through lifecycle. |
| `phase` | running / cancel_requested / cleaning_up / completed | Explicit transition only. |
| `cancel` | cancellation handle | Owned and joined before restore/quit. |
| `quitIntent` | bool | Quit waits cleanup. |
| `statusLabel` | controlled English | `Loading: <action> — <target>`. |
| `confirmedResult` | optional result | Applied if commit point passed. |

Commit points:

- Initial load/reload: validated snapshot accepted by model.
- Save: service returns post-commit ID/revision.
- SSH start: session result has nonzero `StartedAt` after trust/auth/session setup.

Zero/one operation. Extra mutations inert. Cancel/quit requests cancellation and waits cleanup. Before commit,
restore previous frame; after commit, apply result before restore/quit. Stale/non-owner results are ignored.
Conflict result transitions through completed/cleanup, clears operation owner and then creates ConflictState;
operation and conflict are never simultaneously dispatchable.

Timeout/network interruption are owner results, not new operation kinds. Before active-session commit they
produce recoverable SSH failure; after `StartedAt` they preserve confirmed session result and produce transport
outcome. Both transition `running -> cancel_requested -> cleaning_up -> completed`, close session/transport/auth/
resize resources exactly once, restore terminal, clear OperationState and ignore later duplicate/stale results.

## SSHStreamBoundary

Remote stdout/stderr are ephemeral direct streams to attached terminal writers. They never enter Model,
ViewportState, ModalState, logs or diagnostics. Tests may count bytes and retained memory but cannot retain the
full corpus. Safe technical diagnostics contain controlled copy only and are bounded to 256 visible characters.

## SafeTextProjection

Controlled copy is English. User/backend values are untrusted:

- Underlying model/persistence bytes are unchanged.
- CR/LF, ANSI/control/bidi and invalid sequences are escaped/replaced with visible inert notation.
- Printable Unicode is preserved except display-width truncation with `…`.
- Secrets and raw causes are never inputs.
- Output length is bounded by region and existing diagnostic limits.

## State Transitions

```text
any normal state <-> undersized <-> exact preserved state
application FocusOwner -> security_prompt -> exact preserved FocusOwner
application FocusOwner -> secret_prompt -> exact preserved FocusOwner
tree <-> detail
tree/detail -> connection_form -> tree
tree/detail/form -> modal -> openedFrom
navigation reload -> selected missing -> nearest ancestor/root
form/modal/operation -> conflict -> reload | back | cancel-warning
normal/form/modal -> operation.running -> cancel_requested -> cleaning_up -> restored|quit
operation.running -> owner success|failure | completed+cleanup -> conflict
operation result with non-owner ID -> ignored
connect confirmation -> ssh_start -> trustPrompt? -> active session | ssh_failure
```

Host trust remains an existing specialized security state. It never consumes generic modal ownership and
unknown/changed key cannot cross to active session without explicit accepted decision.
