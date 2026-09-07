# Implementation Plan: Motivos de error al iniciar SSH

**Branch**: `003-show-ssh-errors` (sin rama Git) | **Date**: 2026-07-31 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/003-show-ssh-errors/spec.md`

## Summary

Normalizar todo fallo anterior a una sesión SSH activa en un diagnóstico seguro y tipado con una de diez
categorías estables, etapa, resumen, recomendación y detalle técnico allowlisted. El adaptador SSH creará
el diagnóstico donde todavía conoce etapa y causa; la capa de aplicación preservará identidad, revisión y
cadena causal; TUI y CLI consumirán la misma proyección segura. La TUI añadirá detalle desplegable y
reintento mediante nueva resolución/confirmación. `connect --json` quedará permitido para emitir un único
diagnóstico estructurado en stderr cuando el inicio falle, sin cambiar salidas remotas ni códigos actuales.

## Technical Context

**Language/Version**: Go 1.26.5 (`go 1.26.0`, `toolchain go1.26.5`)

**Primary Dependencies**: Biblioteca estándar (`context`, `errors`, `net`, `os`, `syscall`); paquetes
existentes `golang.org/x/crypto/ssh`, Bubble Tea v2, Cobra v1 y contratos de `internal/app`; sin nuevas dependencias

**Storage**: Sin cambios; los diagnósticos viven solo durante el intento actual y no se persisten

**Testing**: `go test`, race detector, fuzzing del sanitizador, tests de modelo/adaptadores, servidor SSH
controlado, pruebas de paridad CLI/TUI, `go vet`, `staticcheck`, `govulncheck` y checks Nix

**Target Platform**: Windows, Linux y macOS en `amd64`/`arm64`; clasificación de red basada en causas
tipadas por plataforma, nunca en textos localizados

**Project Type**: Aplicación única de terminal con adaptadores CLI y TUI sobre los mismos casos de uso

**Performance Goals**: Diagnóstico visible/emitido en menos de un segundo monotónico/inyectado desde que
`Connect` retorna con cleanup/restore completo, sin segunda operación de red; clasificación y sanitización
con memoria acotada por un solo error

**Constraints**: Diez categorías y nueve etapas estables; detalle allowlisted de una línea y máximo 256
puntos de código Unicode (runes), truncado a 255 más `…`;
ningún secreto ni causa cruda visible/logged/persistida; cancelación distinta de deadline; target capturado;
reintento revalida y confirma antes de red; códigos CLI y resultados post-activos sin cambios

**Scale/Scope**: Un intento SSH activo por proceso; matriz de diez categorías, tres presentaciones (TUI,
CLI legible y CLI estructurada), una modal recuperable y sin migraciones de catálogo

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Gate | Pre-Research | Post-Design | Evidence |
|------|--------------|-------------|----------|
| Secure SSH Operations | PASS | PASS | Solo campos allowlisted llegan a presentación; causas siguen encapsuladas y canarios cubren vistas, JSON, errores y logs. |
| Terminal-First Usability | PASS | PASS | La modal identifica target y ofrece `r`, `e`, `d`, `Esc` y `q` por teclado, con contenido prioritario sin color. |
| Predictable State and Failure Handling | PASS | PASS | Se separan cancelación/deadline, pre/post-activo y target capturado; retry vuelve por resolución y confirmación CAS. |
| Behavior-Focused Testing | PASS | PASS | Matriz tipada, fallos principales, fuzz sanitizer, servidor SSH controlado y paridad de presentadores están aislados. |
| Simplicity and Maintainability | PASS | PASS | Un modelo en `internal/app`, sin dependencia nueva ni parser de textos; el rechazo no tipado de x/crypto se reconoce por agotamiento posterior a un método realmente ofrecido. |
| Security and Operational Constraints | PASS | PASS | Salida acotada, sin shell, sin persistencia y con capacidades/plataformas documentadas. |
| Development Quality Gates | PASS | PASS | Quickstart exige formato, suite, race, vet, staticcheck, vulnerabilidades, Nix y cross-builds. |

No se requieren excepciones constitucionales. La causa original se preserva para composición interna,
pero `Error()` y los presentadores nunca la formatean. El rechazo de autenticación no tipado de x/crypto se
convierte en sentinel privado cuando un método realmente ofrecido fue rechazado y el callback queda
agotado; los fallos locales del callback tienen prioridad y no se parsea texto remoto/localizado.

## Project Structure

### Documentation (this feature)

```text
specs/003-show-ssh-errors/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   └── tui.md
└── tasks.md                 # Lista ejecutable generada por /speckit.tasks
```

### Source Code (repository root)

```text
internal/
├── app/
│   ├── ssh_start_failure.go       # Categorías, etapas, precedencia y proyección segura
│   ├── ssh_start_failure_test.go
│   ├── connect.go                 # Target capturado y fallback pre-activo
│   └── types.go                   # Descriptor seguro del intento/resultado
├── sshclient/
│   ├── client.go                  # Clasificación en dial/host/handshake/auth
│   ├── auth.go                    # Fallos locales frente a rechazo remoto
│   ├── session.go                 # Frontera pre-activa/post-activa
│   ├── network_error_unix.go      # errno tipado Unix
│   └── network_error_windows.go   # errno tipado Windows
├── tui/
│   ├── session.go                 # Snapshot de intento y transición de fallo
│   ├── errors.go                  # Modal de diagnóstico y detalle oculto
│   ├── model.go                   # retry/edit/detail/back por ID capturado
│   ├── keys.go
│   └── modal.go                   # Confirmación fresca y comparación anterior/actual
└── cli/
    ├── connect.go                 # JSON permitido y ConnectRequest revisionado
    ├── exit.go                    # Diagnóstico conservando code/exit
    └── output.go                  # Presentación humana/estructurada segura

cmd/orza/main_test.go               # Contrato de stderr y proceso
tests/integration/                  # Servidor SSH y paridad CLI/TUI
README.md                           # Categorías, recuperación y salida estructurada
```

**Structure Decision**: Se mantienen las capas actuales. `internal/app` define el contrato neutral que
`sshclient` puede producir porque ya depende de aplicación; los presentadores dependen del mismo modelo y
no del adaptador SSH. No se crea paquete compartido adicional ni se amplía `ErrorKind`, que sigue
representando errores generales de gestión.

## Phase 0 Outcome

Todas las decisiones están resueltas en [research.md](research.md). No quedan incógnitas técnicas. La
clasificación usa tipos/sentinels y etapa registrada; los textos del SO o servidor nunca deciden la
categoría. `--json connect` queda habilitado únicamente para diagnóstico pre-activo en stderr; stdout
continúa siendo el stream interactivo y no se emite envelope de éxito.
`target_resolution` representa DNS/resolución de nombre posterior a la confirmación, no lookup de catálogo;
not-found/conflict de catálogo conserva el contrato de gestión existente.

## Phase 1 Design

- [data-model.md](data-model.md) define diagnóstico, target, modal y sus transiciones.
- [contracts/cli.md](contracts/cli.md) fija categorías/etapas, JSON, stderr y códigos compatibles.
- [contracts/tui.md](contracts/tui.md) fija contenido, toggle, retry/edit/back y vistas estrechas.
- [quickstart.md](quickstart.md) define la matriz reproducible, canarios, servidor controlado y gates.

## Complexity Tracking

No hay violaciones constitucionales. El wrapper tipado y las ayudas de red por plataforma son necesarios
para evitar clasificación por textos localizados. Se rechaza sanitizar `err.Error()` arbitrario porque no
puede garantizar ausencia de secretos.
