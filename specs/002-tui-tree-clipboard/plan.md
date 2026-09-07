# Implementation Plan: Árbol contextual y pegado en TUI

**Branch**: `002-tui-tree-clipboard` (sin rama Git) | **Date**: 2026-07-30 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-tui-tree-clipboard/spec.md`

## Summary

Sustituir el navegador limitado a una carpeta por una proyección en memoria de todo el catálogo que
muestre raíz, carpetas y conexiones como lista vertical indentada. La selección y expansión se
guardarán por `app.NodeID`, de modo que las acciones sean contextuales y sobrevivan a recargas y
mutaciones. Los formularios procesarán `tea.PasteMsg` como una operación atómica de texto, sin acceder
al portapapeles del sistema. La lectura de secretos conservará la frontera `terminal.Terminal`, pero
usará un editor no-echo basado en el lector de eventos de Ultraviolet para distinguir pegado, teclas y
selección, con restauración determinista del terminal y transferencia temporal controlada del resultado.

## Technical Context

**Language/Version**: Go 1.26.5 (`go 1.26.0`, `toolchain go1.26.5`)

**Primary Dependencies**: Bubble Tea v2.0.8, Bubbles v2.1.1, Lip Gloss v2.0.5;
`github.com/charmbracelet/ultraviolet` ya transitiva y fijada, que pasa a dependencia directa para la
lectura segura de eventos en prompts secretos; `golang.org/x/term` y servicios existentes de `internal/app`

**Storage**: Sin cambios; SQLite mantiene el catálogo y selección/expansión son solo estado de sesión

**Testing**: `go test`, race detector, tests de modelo y componentes, lectores deterministas, PTY Unix,
compilación cruzada Windows, benchmarks del árbol y validación manual nativa

**Target Platform**: Windows, Linux y macOS en `amd64` y `arm64`; terminal interactivo con soporte de
secuencias VT y bracketed paste para las garantías completas de pegado

**Project Type**: Aplicación única de terminal; cambios limitados a los adaptadores TUI y terminal

**Performance Goals**: Tras un warm-up, al menos 19 de 20 recargas, expansiones y contracciones de un
árbol de 1.000 conexiones, 100 carpetas y 10 niveles en menos de un segundo sobre el runner de referencia
Linux amd64 (2 CPU lógicas, 4 GiB libres, SQLite temporal local, 80x24 y sin carga concurrente)

**Constraints**: Operación completa por teclado y sin color; terminal mínimo 80x24 con fallback legible;
exactamente una selección visible; payload indivisible; separadores de línea definidos eliminados y
otros controles rechazados; máximo 4.096 runes normales/4.096 bytes secretos; secretos nunca renderizados
ni registrados y persistidos solo por `Remember password` en el almacén seguro; confirmación con revisión
esperada antes de red; restauración y cancelación de lectores antes de salida 130/143 por señales
capturables; CAS atómico de revisión/ruta de padre/origen/destino; handles o readers fallback Windows
no certificables fallan antes de raw mode

**Scale/Scope**: Un árbol local de hasta 1.101 nodos y profundidad 10; ocho controles normales y tres
tipos de prompt secreto enumerados en el spec; sin cambios a CLI ni modelo persistente

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Gate | Pre-Research | Post-Design | Evidence |
|------|--------------|-------------|----------|
| Secure SSH Operations | PASS | PASS | El prompt posee el secreto hasta cleanup, lo transfiere temporalmente a autenticación y solo `Remember password` autoriza persistencia segura; nunca usa logger ni clipboard del SO. |
| Terminal-First Usability | PASS | PASS | El contrato define teclado, marcadores ASCII, rutas completas, confirmación de conexión/destructivas y cancelación para árbol, formularios y secretos. |
| Predictable State and Failure Handling | PASS | PASS | Identidad/revisión capturadas, fallback determinista y cleanup del lector/raw mode antes de salida 130/143 por señales. |
| Behavior-Focused Testing | PASS | PASS | Tests cubren árbol, payloads de control, selección, Unicode, no filtrado de secretos y fallos/cancelación/restauración del terminal. |
| Simplicity and Maintainability | PASS | PASS | Se reutilizan servicios y tipos existentes; Ultraviolet ya está fijada por Bubble Tea y evita implementar un parser VT/Windows propio. |
| Security and Operational Constraints | PASS | PASS | No cambia persistencia ni ejecución; límites y capacidades del terminal quedan documentados y el árbol permanece acotado por la escala aprobada. |
| Development Quality Gates | PASS | PASS | El quickstart incluye formato, tests, race, vet, checks Nix, builds cruzados y validación manual nativa. |

No se requieren excepciones constitucionales. La complejidad adicional del editor secreto es necesaria
porque `term.ReadPassword` no distingue pegado de teclas ni soporta cursor/selección, y el text input de
Bubble Tea expondría secretos a su loop y trazas. La confirmación previa a conectar y el cleanup ante
señales satisfacen los gates constitucionales de acciones connection-affecting y process termination.

## Project Structure

### Documentation (this feature)

```text
specs/002-tui-tree-clipboard/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tui.md
├── validation/                # Evidencia de rendimiento, quality gates y runners nativos
└── tasks.md                 # Se creará con /speckit.tasks
```

### Source Code (repository root)

```text
cmd/
└── orza/
    └── main.go               # Contexto raíz y propiedad de señales

internal/
├── app/
│   ├── types.go             # Revisiones/rutas esperadas en requests contextuales
│   ├── connect.go           # Comparación antes de cualquier I/O de red
│   ├── connections.go       # Validación de CAS de padre/destino
│   └── folders.go           # Validación de CAS de padre/origen/destino
├── catalogrepo/
│   ├── connection_repository.go # CAS transaccional de conexión y rutas
│   └── folder_repository.go     # CAS transaccional de carpeta y ancestros
├── terminal/
│   ├── terminal.go          # Frontera ReadSecret existente
│   ├── terminal_unix.go     # Estado y restauración Unix
│   ├── terminal_windows.go  # Estado y restauración Windows
│   ├── secret_editor.go     # Buffer, cursor, selección, máscara y política de pegado
│   ├── secret_reader.go     # Adaptación de eventos Ultraviolet y ciclo de cleanup
│   └── secret_reader_windows.go # Certificación fail-closed del reader nativo
└── tui/
    ├── model.go             # Carga asíncrona del snapshot y acciones contextuales
    ├── browser.go           # Índices del árbol, filas visibles y renderizado
    ├── keys.go              # Contrato de navegación/expansión
    ├── text_field.go        # Selección y pegado atómico sobre textinput
    ├── connection_form.go   # Campos y destino contextual
    ├── folder_form.go       # Nombre editable
    ├── move_picker.go       # Destinos jerárquicos existentes
    ├── modal.go             # Confirmación de conexión y acciones destructivas
    └── *_test.go            # Modelo, formularios, pegado, Unicode y benchmarks

tests/
└── integration/             # Catálogo real y pruebas PTY/terminal aplicables

README.md                    # Árbol, teclas, pegado y capacidades soportadas
go.mod                       # Ultraviolet pasa de transitiva a directa
go.sum
```

**Structure Decision**: Se conserva el proyecto Go único y sus fronteras actuales. El árbol y los
campos normales pertenecen a `internal/tui`; el editor secreto pertenece a `internal/terminal` y no
expone su contenido al modelo Bubble Tea. `ConnectRequest` añade una revisión esperada opcional para que
la TUI vincule confirmación y conexión sin cambiar la CLI. No se añade capa ni esquema persistente.

## Phase 0 Outcome

Todas las decisiones técnicas están resueltas en [research.md](research.md). La carga inicial usa los
servicios `FolderService.Get/List` existentes con recorrido por niveles y construye índices locales. El
benchmark de aceptación se ejecuta antes de cerrar US1; si menos de 19/20 mediciones califican se añade
una consulta agregada en `internal/app/folders.go`, `internal/catalogrepo/folder_repository.go` y la
frontera TUI antes de repetir el checkpoint.

## Phase 1 Design

- [data-model.md](data-model.md) define snapshot, nodos, filas, selección, expansión, campos y estados
  del editor secreto, además de sus transiciones e invariantes.
- [contracts/tui.md](contracts/tui.md) define presentación, navegación, acciones contextuales, recarga,
  pegado normal/secreto, errores seguros y capacidades de terminal.
- [quickstart.md](quickstart.md) define validación automatizada y escenarios manuales para árbol,
  payloads, secretos y restauración multiplataforma.

## Complexity Tracking

No hay violaciones constitucionales. El lector Ultraviolet directo y su goroutine cancelable son la
menor solución que distingue de forma fiable un pegado bracketed de una tecla en Unix y Windows. La
alternativa `term.ReadPassword` incumple selección y seguridad de envío; un parser propio duplicaría
protocolos VT y consola Windows; introducir secretos en Bubble Tea ampliaría innecesariamente su
superficie de trazas.
