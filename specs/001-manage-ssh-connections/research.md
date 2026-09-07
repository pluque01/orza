# Research: Gestión de conexiones SSH

**Date**: 2026-07-29

## Go y compatibilidad multiplataforma

**Decision**: Usar Go 1.26.5, declarar `go 1.26.0` y fijar `toolchain go1.26.5`. Mantener
`CGO_ENABLED=0` salvo en el adaptador nativo de Keychain para macOS, que se construirá en Darwin.

**Rationale**: Go permite producir binarios para Windows, Linux y macOS desde una base de código común.
La versión 1.26 es la versión estable actual y satisface el mínimo requerido por Bubble Tea v2. Los
archivos específicos de plataforma quedarán aislados mediante build tags y tendrán pruebas nativas.

**Alternatives considered**: Rust ofrece binarios portables, pero contradice la preferencia explícita
por Go y aumenta el coste inicial. Go 1.25 ampliaría compatibilidad del toolchain, pero no aporta valor
al tratarse de una aplicación nueva.

## Entorno y construcción con Nix

**Decision**: Usar un flake bloqueado como interfaz principal para el entorno de desarrollo,
comprobaciones y builds. Proporcionar `devShells`, `checks` y paquetes con `buildGo126Module`, un
`vendorHash` fijo y `GOTOOLCHAIN=local`. Construir Linux y Windows desde builders Linux; construir
macOS en builders Darwin. Validar cada sistema en runners nativos.

**Rationale**: Un `flake.lock` y hashes fijos hacen reproducibles las dependencias y herramientas. Nix
no se ejecuta de forma nativa en Windows y el cross-build de Darwin no es fiable desde Linux, por lo
que los builds Nix se distribuyen por plataforma sin presentar el cross-build como sustituto de las
pruebas nativas.

**Alternatives considered**: Scripts de shell o Make simplifican una primera compilación, pero no
capturan versiones ni dependencias. Contenedores no validan el comportamiento real de terminal,
Credential Manager o Keychain.

## CLI y TUI

**Decision**: Usar Cobra v1 para la jerarquía de comandos y Bubble Tea v2 con Bubbles v2 y Lip Gloss
v2 para la interfaz interactiva. Ambos adaptadores invocarán los mismos casos de uso en `internal/app`.
Ejecutar la TUI cuando no se indique un subcomando y stdout/stdin sean terminales; nunca abrirla en un
pipeline.

**Rationale**: Las operaciones CRUD de conexiones y carpetas producen un árbol de comandos suficiente
para justificar Cobra. Bubble Tea aporta un modelo de estado y manejo de resize multiplataforma. Una
capa de aplicación compartida garantiza paridad y evita duplicar reglas de negocio.

**Alternatives considered**: `flag` reduce dependencias, pero exige construir manualmente ayuda,
subcomandos y validación. Una TUI artesanal aumenta el riesgo de limpieza incorrecta del terminal.

## Cliente SSH

**Decision**: Implementar el backend principal con `golang.org/x/crypto/ssh`, `ssh/agent` y
`golang.org/x/term`. Usar `net.Dialer.DialContext`, solicitar PTY remoto, propagar cambios de tamaño y
restaurar el terminal antes de devolver cualquier resultado. En Windows, conectar con el agente
OpenSSH mediante `github.com/Microsoft/go-winio`; en Unix usar `SSH_AUTH_SOCK`.

**Rationale**: Esta pila establecida admite contraseñas proporcionadas por la aplicación, claves con
frase de paso, agente, decisiones estructuradas de host key, cancelación y códigos de salida remotos.
No implementa criptografía ni el protocolo SSH de forma propia. Las dependencias de terminal y agente
se abstraen para probar cancelación y restauración.

**Alternatives considered**: Ejecutar OpenSSH con `os/exec` ofrece mayor compatibilidad con FIDO,
PKCS#11, ProxyJump y `ssh_config`, pero no permite suministrar de forma segura una contraseña guardada
ni controlar de forma uniforme las decisiones de host key. Queda fuera de v1 y puede añadirse después
como backend explícito de compatibilidad.

## Verificación de identidad del host

**Decision**: Verificar primero los archivos estándar `known_hosts` del usuario en modo lectura mediante
`golang.org/x/crypto/ssh/knownhosts`. Mantener las decisiones explícitas de confianza de la aplicación
en una tabla propia. Rechazar siempre claves revocadas; para claves desconocidas o cambiadas mostrar
host, puerto, algoritmo y huella SHA-256, con rechazo por defecto y opciones de confiar una vez o
persistir la decisión.

**Rationale**: Se conserva la confianza existente sin modificar `~/.ssh/config` ni escribir sobre el
`known_hosts` del usuario. Una decisión persistida por la aplicación puede resolver una clave nueva o
reemplazada y se actualiza bajo la misma protección transaccional que el catálogo.

**Alternatives considered**: `InsecureIgnoreHostKey` viola la constitución. Modificar directamente el
`known_hosts` del usuario complica concurrencia, entradas hash y recuperación. Un archivo propio
separado repetiría los problemas de reemplazo atómico en Windows que SQLite ya resuelve.

## Persistencia del catálogo

**Decision**: Usar SQLite en modo rollback journal mediante `modernc.org/sqlite`, con una conexión por
proceso, claves foráneas, `BEGIN IMMEDIATE`, `busy_timeout` acotado, `synchronous=EXTRA`, revisiones por
elemento y una revisión global. Guardar `catalog.db` en el directorio local de datos de la aplicación,
nunca en almacenamiento de red o sincronizado.

**Rationale**: El volumen objetivo es pequeño, pero CRUD jerárquico, operaciones de subárbol, procesos
simultáneos y recuperación tras fallos requieren transacciones reales. SQLite aporta bloqueo y
recuperación probados en los tres sistemas. El driver elegido es Go puro y evita la complejidad de CGo
para el catálogo.

**Alternatives considered**: Un JSON plano es legible, pero necesita bloqueo, fsync, reemplazo atómico
y recuperación específicos por plataforma; `os.Rename` no ofrece la garantía requerida en Windows.
SQLite con CGo complica los builds cruzados y no aporta ventajas funcionales para este caso.

## Conflictos y migraciones

**Decision**: Asignar IDs aleatorios estables y una revisión incremental a cada elemento. Cada
modificación recibe la revisión observada y actualiza solo si coincide. Las rutas se resuelven dentro
de la transacción; los movimientos de carpetas validan ciclos con una consulta recursiva. Usar
`PRAGMA application_id` y `user_version`, migraciones ordenadas y rechazo en modo solo lectura ante un
esquema más nuevo.

**Rationale**: Las revisiones distinguen conflictos reales de escrituras no relacionadas y cumplen el
rechazo de ediciones obsoletas sin bloquear todas las instancias. IDs inmutables permiten renombrar y
mover sin perder referencias a credenciales o decisiones de confianza.

**Alternatives considered**: Last-write-wins contradice la aclaración. Bloquear una única instancia
impide usos legítimos desde varias terminales. Timestamps no son fiables como control de concurrencia.

## Credenciales seguras

**Decision**: Definir una interfaz interna `CredentialStore` y adaptadores con build tags: Windows
Credential Manager mediante `danieljoos/wincred`, macOS Keychain mediante `keybase/go-keychain` y Linux
Secret Service mediante el cliente D-Bus de `keybase/go-keychain/secretservice`. No habrá fallback a
archivo, variables de entorno ni catálogo. Cada contraseña usa un ID opaco, requiere consentimiento
explícito y se verifica después de guardar o eliminar.

**Rationale**: Los adaptadores nativos evitan los problemas de prompts no cancelables y borrados
ambiguos de abstracciones genéricas. En Linux headless, la ausencia de D-Bus o Secret Service se
comunica como almacenamiento seguro no disponible. El dominio depende solo de una interfaz pequeña y
puede probar todos los fallos con un fake en memoria.

**Alternatives considered**: `zalando/go-keyring` reduce código, pero su backend Linux no ofrece la
cancelación y verificación estrictas requeridas para el ciclo de vida acordado. `99designs/keyring`
está poco mantenido y puede caer en backends de archivo si no se restringe correctamente.

## Coordinación entre catálogo y credenciales

**Decision**: Modelar altas, sustituciones y borrados mediante una saga durable `credential_operations`.
Una conexión pasa por estados transitorios que conservan la referencia y permiten reanudar o compensar
en el siguiente arranque. El borrado verifica primero la revisión, registra la intención, elimina y
verifica la credencial y finalmente purga o actualiza la conexión. Nunca se descarta una operación
incompleta.

**Rationale**: Ningún mecanismo proporciona una transacción distribuida entre SQLite y los almacenes
del sistema. La saga evita secretos huérfanos y hace explícita la recuperación tras fallos entre ambos
recursos. Los errores antes de eliminar restauran el estado activo; después de eliminar, la
recuperación completa la eliminación o restaura la credencial cuando todavía dispone del secreto en
memoria.

**Alternatives considered**: Actualizar solo uno de los dos recursos puede dejar una referencia rota o
un secreto huérfano. Persistir una segunda copia de la contraseña para garantizar rollback tras un
crash violaría el límite de seguridad; por ello, tras un crash se prioriza completar una eliminación
ya iniciada.

## Estrategia de pruebas

**Decision**: Usar `testing` de Go con pruebas unitarias junto a cada paquete, pruebas golden para
vistas a tamaños fijos, fuzzing de rutas y validadores, `go test -race`, y pruebas de integración para
SQLite, servidor SSH controlado, almacenes nativos y restauración del terminal. Ejecutar smoke tests
nativos en Windows, Linux y macOS.

**Rationale**: Las fronteras externas se inyectan para probar fallos deterministas. Solo los runners
nativos pueden validar ACL, named pipes, Keychain, Secret Service y semántica real de consola.

**Alternatives considered**: Basarse únicamente en mocks no prueba el límite operativo exigido por la
constitución. Un framework adicional de assertions no es necesario para la primera versión.

## References

- [Go releases](https://go.dev/doc/devel/release)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Cobra](https://cobra.dev/)
- [Nixpkgs Go support](https://nixos.org/manual/nixpkgs/stable/#sec-language-go)
- [Go SSH](https://pkg.go.dev/golang.org/x/crypto/ssh)
- [Go known hosts](https://pkg.go.dev/golang.org/x/crypto/ssh/knownhosts)
- [Go terminal support](https://pkg.go.dev/golang.org/x/term)
- [SQLite atomic commit](https://sqlite.org/atomiccommit.html)
- [SQLite transactions](https://sqlite.org/lang_transaction.html)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [Windows Credential Manager](https://learn.microsoft.com/en-us/windows/win32/api/wincred/)
- [Apple Keychain](https://support.apple.com/guide/keychain-access/what-is-keychain-access-kyca1083/mac)
- [Secret Service specification](https://specifications.freedesktop.org/secret-service-spec/latest/)
