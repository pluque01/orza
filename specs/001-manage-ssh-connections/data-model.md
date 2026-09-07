# Data Model: Gestión de conexiones SSH

**Date**: 2026-07-29
**Storage**: Catálogo SQLite local y credenciales en el almacén seguro del sistema

## Conventions

- Los IDs son 128 bits aleatorios, inmutables y se muestran como texto hexadecimal canónico.
- Las revisiones comienzan en 1 y aumentan una vez por mutación confirmada.
- Los nombres son UTF-8, sensibles a mayúsculas y exactos en todos los sistemas.
- `/` es el único separador lógico de rutas; nunca se usan separadores nativos del sistema operativo.
- Los timestamps se guardan en UTC para diagnóstico, nunca para detectar conflictos.
- Contraseñas, frases de paso y claves privadas no forman parte del catálogo.

## Catalog Metadata

Representa la identidad y versión global del catálogo.

| Field | Type | Rules |
|-------|------|-------|
| `catalog_id` | ID | Se crea una vez y nunca cambia. |
| `root_id` | ID | Referencia la única carpeta raíz. |
| `catalog_revision` | Integer | Aumenta una vez por transacción de negocio confirmada. |
| `schema_version` | Integer | Debe ser compatible con el binario antes de cualquier escritura. |
| `created_at` | Timestamp | Inmutable. |

`PRAGMA application_id` identifica el archivo y `PRAGMA user_version` refleja `schema_version`. Un
binario que encuentre una versión posterior abre solo para diagnóstico y no escribe.

## Node

Agregado común para carpetas y conexiones. Permite un único espacio de nombres por carpeta.

| Field | Type | Rules |
|-------|------|-------|
| `id` | ID | Clave primaria estable; no se reutiliza. |
| `parent_id` | ID nullable | Debe señalar una carpeta; solo la raíz usa null. |
| `kind` | Enum | `folder` o `connection`; inmutable. |
| `name` | String | No vacío tras trim; único en el padre; sin `/`, NUL ni controles. |
| `revision` | Integer | Control optimista del agregado. |
| `created_at` | Timestamp | Inmutable. |
| `updated_at` | Timestamp | Se actualiza con cada mutación confirmada. |

### Relationships

- Un nodo pertenece a cero o una carpeta; cero solo es válido para la raíz.
- Una carpeta contiene cero o más nodos.
- Un nodo `connection` tiene exactamente un registro `Connection Details`.
- Una eliminación de carpeta no vacía se rechaza salvo operación recursiva explícita.

### Folder Invariants

- Existe exactamente una raíz, con nombre interno no direccionable y sin padre.
- La raíz no puede renombrarse, moverse ni eliminarse.
- Un movimiento no puede elegir como destino el propio nodo ni uno de sus descendientes.
- La confirmación recursiva captura IDs y revisiones del subárbol; la escritura recalcula el conjunto y
  falla si cambió antes de confirmar.

## Connection Details

Datos no secretos de una conexión SSH; su revisión es la del `Node` asociado.

| Field | Type | Rules |
|-------|------|-------|
| `node_id` | ID | Clave primaria y referencia a un nodo `connection`. |
| `host` | String | Obligatorio; hostname o dirección sin opciones de línea de comandos. |
| `port` | Integer | Rango 1-65535; valor inicial 22. |
| `username` | String nullable | Si se omite, usa la identidad local aplicable. |
| `auth_method` | Enum | `agent`, `key` o `password`. |
| `identity_file` | Path nullable | Obligatorio solo para `key`; referencia, nunca contenido. |
| `credential_ref` | ID nullable | Solo para contraseña recordada; opaco y no secreto. |

### Authentication Validation

- `agent`: `identity_file` y `credential_ref` son null.
- `key`: `identity_file` es obligatorio y `credential_ref` es null.
- `password`: `identity_file` es null; `credential_ref` es opcional.
- Cambiar desde `password` con credencial recordada activa una operación durable de eliminación antes
  de confirmar el nuevo método.

## Trusted Host

Decisión persistida por el usuario para una identidad SSH. Complementa, pero no modifica, los archivos
`known_hosts` del entorno.

| Field | Type | Rules |
|-------|------|-------|
| `id` | ID | Clave primaria. |
| `canonical_host` | String | Host normalizado y sin interpolación. |
| `port` | Integer | Distingue destinos con puertos no estándar. |
| `key_algorithm` | String | Algoritmo comunicado por la librería SSH. |
| `public_key` | Bytes | Clave pública serializada, no una clave privada. |
| `fingerprint_sha256` | String | Valor mostrado para confirmar. |
| `revision` | Integer | Evita reemplazos concurrentes silenciosos. |
| `accepted_at` | Timestamp | Momento de la decisión explícita. |

La combinación `(canonical_host, port)` tiene una sola decisión activa. Una clave marcada como revocada
por una fuente estándar se rechaza siempre. `trust once` vive solo en memoria y no crea este registro.

## Credential Operation

Saga durable que coordina SQLite con el almacén seguro del sistema sin guardar el secreto.

| Field | Type | Rules |
|-------|------|-------|
| `id` | ID | Identifica una operación recuperable. |
| `connection_id` | ID | Conexión afectada. |
| `expected_revision` | Integer | Revisión que autorizó la operación. |
| `operation` | Enum | `save`, `replace`, `remove`, `delete_connection`. |
| `old_ref` | ID nullable | Credencial vigente antes de empezar. |
| `new_ref` | ID nullable | Credencial nueva preparada para guardar. |
| `phase` | Enum | `prepared`, `secret_changed`, `catalog_changed`, `cleanup_pending`. |
| `target_auth_method` | Enum nullable | Método final para un cambio de autenticación. |
| `target_identity_file` | Path nullable | Referencia final si cambia a `key`. |
| `created_at` | Timestamp | Permite diagnosticar operaciones pendientes. |

### State Transitions

```text
save:    prepared -> secret_changed -> catalog_changed -> removed
replace: prepared -> secret_changed -> catalog_changed -> cleanup_pending -> removed
remove:  prepared -> secret_changed -> catalog_changed -> removed
delete:  prepared -> secret_changed -> catalog_changed -> removed
```

- Si el almacén rechaza una eliminación y la credencial sigue presente, se elimina la saga y se
  conserva el agregado anterior.
- Si el secreto cambió pero falla SQLite, se intenta compensar y la saga permanece si la compensación
  no se puede verificar.
- Al arrancar, las sagas pendientes se reanudan antes de permitir nuevas mutaciones sobre la conexión.
- Una recuperación nunca crea una copia persistente adicional de la contraseña.

## SSH Session

Entidad efímera, no persistida. Solo un propietario controla cada sesión activa.

| Field | Type | Rules |
|-------|------|-------|
| `connection_id` | ID | Snapshot de la conexión usada al iniciar. |
| `state` | Enum | Estado del ciclo de vida. |
| `started_at` | Timestamp nullable | Se fija al abrir el shell remoto. |
| `outcome` | Enum nullable | `success`, `remote_failure`, `transport_failure`, `canceled`. |
| `remote_exit_status` | Integer nullable | Presente si el servidor lo proporciona. |

### State Transitions

```text
idle -> resolving -> verifying_host -> authenticating -> opening_pty -> active -> closing
closing -> succeeded | failed | canceled
resolving | verifying_host | authenticating | opening_pty -> failed | canceled
```

En todos los estados terminales se cierran sesión, cliente, conexión y agente, y se restaura el terminal
antes de devolver el resultado.

## Transaction Rules

1. Cada escritura comienza con una transacción inmediata y un timeout acotado.
2. Se vuelve a comprobar la versión de esquema dentro de la transacción.
3. Las rutas se resuelven dentro de esa misma transacción.
4. Cada nodo afectado se compara con su `expected_revision` cuando el llamador lo observó previamente.
5. Un mismatch devuelve conflicto y nunca se reintenta como last-write-wins.
6. Restricciones de nombre, padre, tipo, ciclo y raíz se validan antes de mutar.
7. Se actualizan agregado y detalles, luego revisiones de elementos y revisión global.
8. Solo se devuelven nuevas revisiones después de un commit correcto.

## Storage Location and Permissions

| Platform | Local data directory |
|----------|----------------------|
| Linux | `$XDG_DATA_HOME/orza`, o `~/.local/share/orza` |
| macOS | `~/Library/Application Support/orza` |
| Windows | `%LOCALAPPDATA%\orza` |

En Linux y macOS el directorio usa `0700` y el catálogo `0600`; se rechazan symlinks, propietario
incorrecto y permisos amplios. En Windows se crea una DACL protegida para el usuario actual y se
verifica antes de abrir. El catálogo se rechaza en rutas de red o sincronizadas cuando sean detectables.

## Indexes and Limits

- Índice único `(parent_id, name)` para el espacio de nombres compartido.
- Índices por `parent_id`, `kind` y `(canonical_host, port)`.
- Profundidad funcional validada: 10 niveles.
- Escala funcional validada: 1.000 conexiones y 100 carpetas.
- Las consultas recursivas tienen límite defensivo y detectan ciclos como corrupción.
