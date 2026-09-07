# Quickstart Validation: Gestión de conexiones SSH

**Date**: 2026-07-29

Esta guía valida el diseño de extremo a extremo una vez implementadas las tareas. Los comandos y
resultados normativos se definen en [contracts/cli.md](contracts/cli.md); los datos y revisiones se
definen en [data-model.md](data-model.md).

## Prerequisites

- Linux o macOS con Nix y flakes habilitados para desarrollo y build.
- Un runner Windows, Linux y macOS para la validación nativa final.
- Una terminal de al menos 80x24 para validar la TUI.
- Un servidor SSH de prueba controlado, con host, puerto, usuario y credenciales desechables.
- Credential Manager, Keychain o Secret Service disponible para las pruebas de contraseña recordada.

No usar hosts ni credenciales de producción. El catálogo quickstart usa nombres bajo
`/quickstart-validation` para que pueda eliminarse al terminar.

## 1. Enter the Reproducible Environment

```sh
nix develop
go version
go test ./...
```

Expected:

- Go informa 1.26.5.
- Las dependencias se resuelven desde el entorno fijado.
- Todas las pruebas unitarias terminan correctamente.

## 2. Run Quality Gates

```sh
nix flake check
go test -race ./...
go vet ./...
staticcheck ./...
govulncheck ./...
```

Expected: formato, tests, race detector, análisis estático y vulnerabilidades pasan sin omitir paquetes.
Las pruebas de almacenes nativos que no correspondan al host se excluyen mediante build tags, no se
simulan como si hubiesen pasado.

## 3. Build with Nix

```sh
nix build
./result/bin/orza --version
```

Expected: el build usa el `vendorHash` bloqueado y produce el ejecutable nativo. La matriz de release
también debe producir artefactos `linux-amd64`, `linux-arm64`, `windows-amd64`, `windows-arm64`,
`darwin-amd64` y `darwin-arm64`; cada uno se ejecuta en un runner nativo antes de publicarse.

## 4. Validate Folder and Connection CRUD

```sh
orza folder create /quickstart-validation
orza folder create /quickstart-validation/team
orza connection create /quickstart-validation/team/demo \
  --host "$TEST_SSH_HOST" --port "$TEST_SSH_PORT" --user "$TEST_SSH_USER" --auth agent
orza connection show /quickstart-validation/team/demo
orza connection update /quickstart-validation/team/demo --name demo-renamed
orza connection move /quickstart-validation/team/demo-renamed /quickstart-validation
orza folder list /quickstart-validation
```

Expected:

- Cada mutación muestra ID, ruta y revisión creciente.
- La conexión conserva su ID al renombrarse y moverse.
- La ruta antigua deja de resolver y la nueva aparece una sola vez.
- Ningún comando modifica `~/.ssh/config`.

## 5. Validate Machine Output and Conflicts

```sh
orza --json connection show /quickstart-validation/demo-renamed
orza connection show /quickstart-validation/demo-renamed
```

Anotar la revisión mostrada como `REV`. En una segunda terminal, actualizar la conexión. Después,
intentar desde la primera terminal una actualización con `--if-revision REV`.

Expected:

- JSON contiene `ok`, datos no secretos y revisión del catálogo.
- La actualización obsoleta termina con estado 4.
- El primer cambio confirmado permanece intacto y el error indica recargar.
- Modificaciones simultáneas sobre elementos distintos pueden completarse ambas.

## 6. Validate Host Trust and Agent Authentication

```sh
orza connect /quickstart-validation/demo-renamed
```

Expected on first contact:

- Se muestran host, puerto, algoritmo y huella SHA-256 antes de autenticar.
- Rechazar no persiste confianza ni envía credenciales.
- `trust once` permite solo ese intento.
- `trust and persist` evita repetir la pregunta para la misma clave.
- Una clave cambiada vuelve a advertir y una clave revocada no permite continuar.

Expected during and after session:

- El agente se usa sin copiar claves privadas al catálogo.
- Resize llega al PTY remoto.
- El output se transmite sin crecimiento no acotado.
- Al salir, el terminal recupera eco, cursor y modos; el estado remoto se propaga.

## 7. Validate Key and Password Authentication

Crear una segunda conexión con `--auth key --identity-file PATH` y una tercera con `--auth password`.
No pasar secretos por flags ni variables de entorno.

Expected:

- Una clave cifrada solicita la frase de paso sin eco y nunca la ofrece para guardar.
- Una contraseña puede usarse solo para la sesión.
- Recordarla requiere consentimiento explícito desmarcado por defecto y escribe únicamente en el
  almacén nativo.
- En Linux headless sin Secret Service, se informa que la persistencia no está disponible y la sesión
  todavía puede continuar con contraseña temporal.
- Los detalles y JSON nunca incluyen secretos.

Eliminar la conexión con contraseña recordada o cambiarla a `--auth agent`.

Expected:

- La credencial asociada desaparece del almacén seguro.
- Si el almacén rechaza el borrado, el comando falla y conserva el estado anterior.
- Si se interrumpe el proceso en una fase de saga, el siguiente inicio recupera la operación sin dejar
  una credencial huérfana.

## 8. Validate TUI

```sh
orza
```

Completar con teclado: navegar, crear, editar, mover, eliminar y conectar. Repetir con `NO_COLOR=1`,
redimensionar por encima y por debajo de 80x24 y provocar un conflicto desde otra terminal.

Expected:

- Todas las acciones son visibles y utilizables sin ratón ni color.
- Bajo 80x24 se muestra el mínimo y siguen disponibles ayuda, volver y salir.
- Cancelar formularios o confirmaciones no cambia el catálogo.
- Un conflicto preserva el cambio externo y permite recargar.
- Un fallo antes de abrir SSH restaura una pantalla estable.
- Una sesión abierta libera la TUI, usa el terminal y lo restaura al terminar.

## 9. Validate Failure Paths

Ejecutar la suite de integración que cubre timeout, autenticación fallida, host cambiado, cierre remoto,
interrupción durante escritura, `SQLITE_BUSY`, esquema más nuevo, permisos amplios y almacén bloqueado.

Expected:

- No quedan cambios parciales ni secretos en logs.
- Catálogo incompatible o corrupto deja de aceptar escrituras y conserva el archivo para diagnóstico.
- Todos los retornos cierran red, sesión y agente y restauran el terminal.
- Las pruebas nativas pasan en Windows, Linux y macOS, incluidas named pipe, DACL, Keychain y Secret
  Service según plataforma.

## 10. Cleanup

```sh
orza folder delete /quickstart-validation --recursive --yes
```

Expected: se elimina el subárbol y cualquier contraseña asociada; la raíz y otros elementos permanecen
intactos. Si cambió el alcance desde la confirmación, la eliminación se rechaza con estado 4.
