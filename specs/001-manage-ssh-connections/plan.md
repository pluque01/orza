# Implementation Plan: Gestión de conexiones SSH

**Branch**: `001-manage-ssh-connections` | **Date**: 2026-07-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-manage-ssh-connections/spec.md`

## Summary

Construir una aplicación de terminal para Windows, Linux y macOS que gestione un catálogo jerárquico
de conexiones SSH con paridad funcional entre CLI y TUI. La solución será un proyecto único en Go
1.26: Cobra expondrá comandos, Bubble Tea v2 la interfaz interactiva y ambos usarán los mismos casos de
uso. `x/crypto/ssh` gestionará sesiones seguras, SQLite mantendrá el catálogo transaccional y
adaptadores nativos almacenarán contraseñas fuera del catálogo. Un flake Nix fijará el entorno,
comprobaciones y builds reproducibles.

## Technical Context

**Language/Version**: Go 1.26.5 (`go 1.26.0`, `toolchain go1.26.5`)

**Primary Dependencies**: Cobra v1; Bubble Tea v2, Bubbles v2 y Lip Gloss v2;
`golang.org/x/crypto/ssh`, `golang.org/x/term`, `golang.org/x/sys`;
`modernc.org/sqlite`; `Microsoft/go-winio`; `danieljoos/wincred`; y adaptadores Keychain/Secret
Service de `keybase/go-keychain`

**Storage**: SQLite local (`catalog.db`) para metadatos, jerarquía, confianza de host y operaciones
durables; Windows Credential Manager, macOS Keychain o Linux Secret Service para contraseñas

**Testing**: `go test`, race detector, fuzzing y golden tests; pruebas de integración con SQLite,
servidor SSH controlado, terminal y almacenes nativos; `go vet`, `staticcheck` y `govulncheck`

**Target Platform**: Windows, Linux y macOS en `amd64` y `arm64`, dentro de las versiones soportadas
por Go 1.26; validación nativa obligatoria en los tres sistemas

**Project Type**: Aplicación única de terminal con dos adaptadores de entrada, CLI y TUI

**Performance Goals**: Abrir una carpeta o resolver una ruta conocida en menos de 2 segundos en el
95 % de intentos con 1.000 conexiones; actualizaciones TUI perceptibles en menos de 100 ms para
operaciones locales; I/O de sesión SSH transmitido sin almacenamiento completo en memoria

**Constraints**: Catálogo local y operativo sin red; no modificar `~/.ssh/config`; terminal mínimo
documentado de 80x24 para la TUI; uso completo por teclado; soporte de `NO_COLOR`; secretos ausentes
de logs y catálogo; escrituras obsoletas rechazadas; recursos de terminal y red liberados en todos
los retornos; Nix como interfaz principal de desarrollo y build

**Scale/Scope**: Un usuario local, hasta 1.000 conexiones, 100 carpetas y 10 niveles; una sesión SSH
activa por proceso; varias instancias de gestión concurrentes; sin sincronización, túneles,
transferencias ni ejecución remota no interactiva en v1

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Gate | Pre-Research | Post-Design | Evidence |
|------|--------------|-------------|----------|
| Secure SSH Operations | PASS | PASS | `x/crypto/ssh`, verificación obligatoria de host y secretos en almacenes nativos; nunca se usa una callback insegura. |
| Terminal-First Usability | PASS | PASS | Contratos CLI/TUI, teclado completo, confirmaciones, `NO_COLOR`, resize y fallback bajo 80x24. |
| Predictable State and Failure Handling | PASS | PASS | Estados explícitos, transacciones SQLite, revisiones optimistas, saga de credenciales y restauración del terminal. |
| Behavior-Focused Testing | PASS | PASS | Fronteras inyectables, unitarias, contrato, integración SSH/SQLite/credenciales y matriz nativa de sistemas. |
| Simplicity and Maintainability | PASS | PASS | Proyecto único, capa de aplicación compartida y dependencias limitadas a necesidades multiplataforma justificadas en `research.md`. |
| Security and Operational Constraints | PASS | PASS | Directorios privados, entrada no confiable, sin shell interpolation, salida SSH transmitida y plataformas documentadas. |
| Development Quality Gates | PASS | PASS | Nix ejecuta formato, tests, race, vet, staticcheck y vulnerabilidades antes del build; quickstart define validación manual. |

No se requieren excepciones constitucionales. La revisión posterior al diseño confirma que los
contratos y el modelo preservan todos los gates.

## Project Structure

### Documentation (this feature)

```text
specs/001-manage-ssh-connections/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   └── tui.md
└── tasks.md                 # Se creará con /speckit.tasks
```

### Source Code (repository root)

```text
cmd/
└── orza/
    └── main.go

internal/
├── app/                     # Casos de uso compartidos por CLI y TUI
├── catalog/                 # SQLite, migraciones y control de revisiones
├── cli/                     # Comandos, flags, salida y códigos de proceso
├── credential/              # Interfaz y adaptadores nativos por plataforma
├── domain/                  # Entidades, validación y estados
├── hostkey/                 # Política y persistencia de confianza SSH
├── sshclient/               # Dial, autenticación, PTY y sesión remota
├── terminal/                # Raw mode, resize y restauración por plataforma
└── tui/                     # Modelo Bubble Tea, vistas y keymap

tests/
└── integration/             # Escenarios que cruzan catálogo, SSH, credenciales y terminal

flake.nix
flake.lock
go.mod
go.sum
```

**Structure Decision**: Proyecto Go único con un ejecutable y paquetes `internal`. Las pruebas
unitarias viven junto al paquete probado; `tests/integration` contiene solo pruebas que cruzan varias
fronteras. No se crea `pkg/` porque no existe una API pública reutilizable. Los adaptadores CLI y TUI
dependen de `internal/app`, nunca entre sí.

## Phase 0 Outcome

Todas las decisiones técnicas están resueltas en [research.md](research.md). No quedan incógnitas
técnicas pendientes. Las dependencias no estándar están justificadas por jerarquía de comandos,
interfaz terminal, protocolo SSH establecido, atomicidad multiplataforma o almacenes seguros nativos.

## Phase 1 Design

- [data-model.md](data-model.md) define agregados, restricciones, revisiones y transiciones de sesión
  y credenciales.
- [contracts/cli.md](contracts/cli.md) define comandos, flags, salida, errores y códigos de proceso.
- [contracts/tui.md](contracts/tui.md) define pantallas, keymap, confirmaciones y estados recuperables.
- [quickstart.md](quickstart.md) demuestra build, CRUD, concurrencia, autenticación y restauración en
  los tres sistemas.

## Complexity Tracking

No hay violaciones constitucionales que requieran excepción. SQLite y los adaptadores nativos añaden
dependencias, pero sustituyen implementaciones propias de transacciones, criptografía o almacenamiento
de secretos y están justificados en [research.md](research.md).
