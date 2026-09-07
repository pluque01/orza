# CLI Contract: `orza`

**Date**: 2026-07-29

## Invocation

```text
orza [global flags] <command> [arguments] [flags]
orza
```

Sin comando y con stdin/stdout conectados a una terminal, se abre la TUI. Sin terminal interactiva se
muestra ayuda y se devuelve error de uso; nunca se intenta dibujar una TUI dentro de un pipeline.

## Global Flags

| Flag | Meaning |
|------|---------|
| `--json` | Produce una única respuesta JSON para operaciones de gestión. No se permite con `connect`. |
| `--no-color` | Desactiva color; también se respeta `NO_COLOR`. |
| `--help` | Muestra ayuda sin abrir la TUI. |
| `--version` | Muestra versión y termina correctamente. |

Los secretos nunca se aceptan como flags, argumentos ni variables de entorno. Contraseñas y frases de
paso solo se solicitan desde una terminal sin eco.

## Path Grammar

- `/` representa la raíz.
- `/clientes/acme/prod` es una ruta absoluta.
- No existen `.` ni `..` como segmentos especiales.
- Los nombres no pueden estar vacíos ni contener `/`, NUL o controles.
- Las rutas son sensibles a mayúsculas en todos los sistemas.
- Los comandos que aceptan `PATH_OR_ID` distinguen una ruta por su `/` inicial; un ID es el texto
  hexadecimal canónico mostrado por `show` y `list`.

## Connection Commands

### Create

```text
orza connection create PATH --host HOST [--port PORT] [--user USER]
  --auth agent|key|password [--identity-file PATH] [--remember-password]
```

- `PATH` incluye el nombre final y su carpeta debe existir.
- `--port` vale 22 por defecto.
- `--identity-file` es obligatorio solo con `--auth key`.
- `--remember-password` solo es válido con `--auth password` y una terminal interactiva; muestra un
  consentimiento desmarcado antes de escribir en el almacén seguro.

### Read

```text
orza connection list [FOLDER_PATH_OR_ID]
orza connection show PATH_OR_ID
```

`list` usa `/` por defecto y solo devuelve conexiones hijas directas. `show` nunca devuelve contraseña,
frase de paso, material de clave privada ni contenido de sesión.

### Update and Move

```text
orza connection update PATH_OR_ID [--name NAME] [--host HOST] [--port PORT]
  [--user USER|--clear-user] [--auth agent|key|password]
  [--identity-file PATH|--clear-identity-file] [--remember-password|--forget-password]
  [--if-revision REVISION]

orza connection move PATH_OR_ID DESTINATION_FOLDER [--if-revision REVISION]
```

Al menos un cambio es obligatorio. Opciones mutuamente excluyentes fallan antes de escribir. Si
`--if-revision` no coincide, se devuelve conflicto; si se omite, el comando resuelve y modifica la
última versión dentro de una sola transacción.

### Delete

```text
orza connection delete PATH_OR_ID [--if-revision REVISION] [--yes]
```

Sin `--yes`, identifica nombre, ruta, host y usuario y solicita confirmación. Sin terminal interactiva
la ausencia de `--yes` falla sin modificar datos. Una contraseña asociada se elimina mediante su saga
antes de purgar la conexión.

## Folder Commands

```text
orza folder create PATH
orza folder list [PATH_OR_ID]
orza folder show PATH_OR_ID
orza folder rename PATH_OR_ID NAME [--if-revision REVISION]
orza folder move PATH_OR_ID DESTINATION_FOLDER [--if-revision REVISION]
orza folder delete PATH_OR_ID [--recursive] [--if-revision REVISION] [--yes]
```

- `list` devuelve carpetas y conexiones hijas directas.
- La raíz se puede listar y mostrar, pero no renombrar, mover ni eliminar.
- Una carpeta no vacía requiere `--recursive`; la confirmación enumera el número de carpetas,
  conexiones y credenciales afectadas.
- Antes del commit recursivo se recalculan membresía y revisiones. Cualquier cambio invalida la
  confirmación y devuelve conflicto.

## Connect Command

```text
orza connect PATH_OR_ID
```

Requiere una terminal interactiva y rechaza `--json`. El flujo es:

1. Mostrar ruta, usuario, host y puerto seleccionados.
2. Verificar la identidad antes de enviar credenciales.
3. Para host desconocido o cambiado, mostrar algoritmo y huella SHA-256 y ofrecer `reject`,
   `trust once` o `trust and persist`; `reject` es el valor por defecto.
4. Autenticar mediante el método guardado, solicitando secretos sin eco cuando sea necesario.
5. Restaurar la TUI si el fallo ocurre antes de abrir la sesión y se invocó desde la TUI.
6. Ceder stdin/stdout/stderr a la sesión, propagar resize y restaurar el terminal al finalizar.

Una clave revocada se rechaza sin opción de continuar. Una decisión no se solicita en modo no
interactivo: se falla con la huella e instrucciones para repetir desde una terminal.

## Human Output

- Los resultados de creación, actualización y movimiento muestran ID, ruta y revisión nueva.
- Las listas usan columnas legibles sin depender del color y se adaptan al ancho disponible.
- Los errores se escriben en stderr e incluyen operación, objetivo y acción sugerida, nunca secretos.
- El flujo SSH transmite salida directamente y no la acumula ni reformatea.

## JSON Output

Éxito:

```json
{
  "ok": true,
  "data": {},
  "catalogRevision": 42
}
```

Fallo:

```json
{
  "ok": false,
  "error": {
    "code": "conflict",
    "message": "the connection changed; reload before retrying",
    "target": "/clientes/acme/prod"
  }
}
```

Los objetos de nodo incluyen `id`, `kind`, `name`, `path`, `revision` y datos no secretos aplicables.
Los nombres de campo JSON son estables dentro de la versión mayor de la aplicación.

## Exit Status

| Status | Meaning |
|--------|---------|
| `0` | Operación correcta o sesión remota finalizada con 0. |
| `2` | Uso o validación inválidos. |
| `3` | Elemento no encontrado. |
| `4` | Conflicto de revisión, nombre o alcance confirmado. |
| `5` | Operación cancelada por el usuario. |
| `6` | Decisión de confianza, autenticación o almacén seguro fallidos. |
| `7` | Catálogo no disponible, incompatible o no persistible. |
| `10` | Fallo de transporte o protocolo SSH sin estado remoto. |

Para `connect`, un estado remoto no cero entre 1 y 255 se propaga como estado del proceso. El mensaje
de stderr distingue, cuando exista información, un fallo remoto de uno local aunque el número coincida
con un estado reservado de gestión.

## Compatibility Rules

- Añadir comandos o campos JSON opcionales es compatible.
- Eliminar o reinterpretar comandos, flags, estados o campos existentes requiere una versión mayor.
- Los mensajes humanos pueden cambiar; nombres de comando, JSON y estados son el contrato automatizable.
