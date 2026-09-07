# Data Model: Motivos de error al iniciar SSH

No se añaden tablas ni datos persistentes. Los modelos son estado transitorio del intento y contratos
seguros entre SSH, aplicación y presentación.

## SSHStartError

Wrapper interno que conserva semántica y causa sin conceder permiso de presentación.

| Field | Type | Rules |
|---|---|---|
| `reason` | `SSHFailureReason` | Exactamente uno de los diez valores estables. |
| `stage` | `SSHFailureStage` | Etapa bloqueante conocida o `unknown`. |
| `trustStatus` | optional trust status | Solo para host trust; unknown/changed/revoked cuando se conoce. |
| `detail` | optional safe string | Valor/template enumerado, una línea, máximo 256 puntos de código Unicode (255 más `…` al truncar). |
| `cause` | unexported error | Disponible por `Unwrap`; nunca se formatea en salida. |

### Stable reasons

`timeout`, `authentication_denied`, `connection_refused`, `host_not_found`, `network_unreachable`,
`host_trust`, `credential_unavailable`, `ssh_negotiation`, `canceled`, `unexpected`.

### Stable stages

`target_resolution`, `network_connection`, `host_trust`, `ssh_negotiation`, `credential`,
`authentication`, `session_setup`, `local_terminal`, `unknown`.

`target_resolution` es resolución de nombre de red después de confirmar el objetivo. Lookup, ausencia o
conflicto del catálogo conserva los errores de gestión existentes y no crea `SSHStartError`.

### Invariants

- `Error()` contiene solo reason/stage controlados.
- `Unwrap()` conserva la causa para `errors.Is/As`.
- Un wrapper exterior no cambia reason/stage ya normalizados.
- Solo se crea para fallos pre-activos.

## SSHFailurePresentation

Proyección sin causa consumida por todos los adaptadores.

| Field | Required | Rules |
|---|---|---|
| `category` | yes | Identificador estable de reason. |
| `stage` | yes | Identificador estable o `unknown`. |
| `summary` | yes | Texto seguro no vacío controlado por aplicación. |
| `recommendation` | yes | Acción breve controlada por categoría. |
| `technicalDetail` | no | Campo allowlisted/sanitizado; omitido si vacío. |

## SSHAttemptTarget

Snapshot público/no secreto del objetivo del intento.

| Field | Type | Rules |
|---|---|---|
| `id` | `app.NodeID` | ID estable capturado. |
| `revision` | `app.Revision` | Usado por confirmación/CAS. |
| `path` | string | Ruta capturada mostrada; nunca selección actual fallback. |
| `host` | string | Host público validado del catálogo. |
| `port` | uint16 | Puerto validado. |

No incluye username, credential reference, identity contents ni secretos.

## SSHFailureModalState

| Field | Type | Rules |
|---|---|---|
| `attempt` | `SSHAttemptTarget` | Inmutable para el fallo mostrado. |
| `failure` | `SSHFailurePresentation` | Siempre segura. |
| `detailVisible` | bool | Empieza false; `d` alterna sin I/O. |
| `recovery` | idle/resolving/confirming/editing/missing/conflict | Estado explícito del flujo. |
| `currentTarget` | optional target | Solo después de resolver retry/edit; no reemplaza attempt. |

### State transitions

- `attempting -> failure/idle`: fallo pre-activo crea modal con detalle oculto.
- `failure/idle -> failure/idle`: `d` alterna solo `detailVisible`.
- `failure/idle -> resolving`: `r` o `e` resuelve el ID capturado sin red SSH.
- `resolving -> confirming`: retry encontró target actual y presenta confirmación fresca.
- `resolving -> editing`: edit encontró target y abre formulario actual.
- `resolving -> missing|conflict`: conserva attempt y ofrece back/reload.
- `confirming -> attempting`: solo `y`, con ID/revisión actual, permite red.
- `confirming -> failure/idle`: cancelar confirmación no inicia red.
- `failure/idle -> browser`: `Esc` selecciona attempt si existe o fallback estable.
- `failure/idle -> quit`: `q`/Ctrl+C según contrato existente.

## CLIErrorDiagnostic

Extensión opcional del error estructurado actual.

| Field | Required | Rules |
|---|---|---|
| existing `code/message/target` | yes | Compatibilidad preservada. |
| `endpoint` | no | `host:port` capturado cuando disponible. |
| `category` | yes for startup failure | Stable reason. |
| `stage` | yes for startup failure | Stable stage. |
| `recommendation` | yes for startup failure | Texto controlado. |
| `technicalDetail` | no | Omitido si vacío. |

## Classification transition

`raw failure -> explicit cancel | existing normalized | blocking stage + recognized typed cause | unexpected`

Antes de clasificar se verifica que `StartedAt` sea cero y no exista remote status. Deadline mapea a
`timeout`; cancelación manejada en aplicación a `canceled`; una señal que termina el proceso conserva
130/143. Resultados post-activos no entran en este modelo.
