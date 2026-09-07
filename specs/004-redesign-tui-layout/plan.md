# Implementation Plan: Rediseño multipanel de la TUI

> **Superseded overflow design**: Feature 005 replaces directional marker rows with proportional right-edge scrollbars. Use `specs/005-add-scrollbar-indicator/` for current overflow planning.

**Branch**: `004-redesign-tui-layout` (sin rama Git) | **Date**: 2026-08-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/004-redesign-tui-layout/spec.md`

## Summary

Recomponer la TUI existente en Tree, Details y Actions con tres modos deterministas: dos columnas desde 80
columnas, composición apilada entre 40x12 y 79 columnas, y shell de tamaño insuficiente por debajo de 40x12.
Se conserva un único modelo Bubble Tea y los formularios seguros actuales; se añaden foco explícito,
viewports con `↑ more`/`↓ more`/`…`, matriz normativa de acciones, un panel centrado de inventario cerrado,
conflicto por target capturado y una única operación asíncrona con ownership/cancelación. Todo texto controlado
es inglés seguro. Trust y secret prompts conservan ownership de seguridad por encima de los cuatro propietarios
de foco de la aplicación; Help modal es inline y los errores de save permanecen en el form. Se preservan host
verification, CAS, secretos y cleanup sin dependencias nuevas.

## Technical Context

**Language/Version**: Go 1.26.5 (`go 1.26.0`, `toolchain go1.26.5`)

**Primary Dependencies**: Bubble Tea v2.0.8 para Model-Update-View, Bubbles v2.1.1 para bindings y entradas,
Lip Gloss v2.0.5 para estilos/composición y `charmbracelet/x/ansi` v0.11.7 para ancho visible, recorte,
wrapping y escaping terminal-safe; sin Huh ni dependencias nuevas

**Storage**: SQLite y almacenes de credenciales existentes, sin esquema ni persistencia nuevos; layout,
viewports, foco, conflicto, operación, formulario y panel viven solo durante la ejecución

**Testing**: `go test` unitario/integración, race detector, fuzz de estado/resize/control bytes, matrices
deterministas de 20 ejecuciones, benchmarks monotónicos, host-key regressions, `go vet`, `staticcheck`,
`govulncheck`, Nix, cross-builds, terminales nativas, stream de 128 MiB, timeout/network interruption y estudio
de usabilidad fijado

**Target Platform**: Linux, Windows y macOS en `amd64`/`arm64`, terminal interactiva VT según soporte ya
documentado; equivalencia con `NO_COLOR`/`--no-color`; 40x12 es el mínimo reducido garantizado

**Project Type**: Aplicación única de terminal, con TUI y CLI sobre los casos de uso existentes

**Performance Goals**: En runner Linux amd64 con 2 CPU lógicas, 4 GiB libres, catálogo temporal de 1.000
conexiones/100 carpetas/profundidad 10, sin carga y tras warm-up: 19/20 frames de selección, foco, scroll y
resize en ≤100 ms; 19/20 recargas con snapshot visible en ≤1 s, medidas con reloj monotónico

**Constraints**: `wide` desde 80 columnas, `stacked` desde 40x12 hasta 79 columnas y `undersized` si ancho
<40 o alto <12; `reduced` es ortogonal cuando width <80 o height <24, incluido wide-short; Actions/Help/dispatch comparten inventario exacto; un foco, un modal y una operación owner;
cancel/quit esperan cleanup; resultados stale no actualizan; conflicto nunca retargetea por fila; indicadores
de overflow textuales; UI controlada en inglés; valores del usuario se preservan y se escapan solo al presentar;
host keys desconocidas/cambiadas siempre exigen decisión explícita; gates obligatorios no admiten waiver

Validation errors son field-local; save/persistence errors son form-level inline. Domain actions,
recovery actions y navigation controls son inventarios distintos. Tests nuevos prueban red-first; pruebas de
seguridad preservada son characterization regressions y deben pasar en baseline.

UX geometry, closed keyboard/content/modal matrices, no-color cues, terminal fallback and non-retained render
history follow the normative TUI contract; no plan-only UX behavior is permitted.

SSH stdout/stderr se conecta directamente a writers terminales y nunca entra en Model/viewport; la TUI retiene
como máximo 1 MiB en el test de 128 MiB y cada technical diagnostic está acotado a 256 caracteres visibles.
Timeout/network interruption usa el context owner existente, cierra session/transport/auth/resize exactamente
una vez y restaura terminal antes de stable failure/exit; late completions siguen owner-ID rejection.

**Scale/Scope**: Tres regiones, tres display modes, cuatro propietarios de foco, diez variantes concretas del
inventario modal, cuatro operaciones (`initial_load`, `reload`, `save`, `ssh_start`), dos conflictos, siete
recovery/operation keys y matrices de 20 ejecuciones en 40x12, 60x16, 79x23, 80x12, 80x24, 100x20,
100x30 y 160x40

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Gate | Pre-Research | Post-Design | Evidence |
|------|--------------|-------------|----------|
| Secure SSH Operations | PASS | PASS | Trust verifier/prompt existentes quedan fuera del modal genérico; unknown/changed keys requieren target, fingerprint y decisión explícita; secretos nunca llegan a proyección. |
| Terminal-First Usability | PASS | PASS | Teclado completo, `r`/`b`/`Esc`, Actions exactas, 80x24, mínimo 40x12, fallback undersized, no-color e indicadores textuales están contratados. |
| Predictable State and Failure Handling | PASS | PASS | Foco/modal/operation owner únicos, commit points, cancellation/timeout/network cleanup, stale result rejection y conflicto por ID/revisión tienen transiciones explícitas. |
| Behavior-Focused Testing | PASS | PASS | Quickstart fija 20-run matrices, corpus de control bytes, stream 128 MiB, timeout/network interruption, host trust, SC-010–SC-017, rendimiento y fronteras manuales. |
| Simplicity and Maintainability | PASS | PASS | Un root model y componentes existentes; offsets enteros; sin Huh, submodelos, persistencia, goroutines o servicios nuevos. |
| Security and Operational Constraints | PASS | PASS | Threat assumptions documentadas; remote stdout/stderr se transmite sin retención proporcional, diagnostics ≤256 caracteres visibles y user/backend data se escapa al presentar; sin shell, esquema, protocolo o configuración nuevos. |
| Development Quality Gates | PASS | PASS | El diseño declara formato, tests completos, race, vet, staticcheck, vuln, Nix, cross-build y fronteras nativas como merge-blocking; evidencia de ejecución sigue pendiente hasta implementación. |

No se requieren excepciones constitucionales. `PASS` valida el diseño, no afirma que los comandos de
implementación ya se hayan ejecutado. Un gate fallido, no ejecutado o no disponible mantiene la feature no
mergeable; solo hardware nativo no disponible queda pendiente, nunca aprobado ni waived.

## Threat Assumptions and Manual Boundaries

- Nombres, rutas, hosts, usuarios, catálogo/DB, environment, backend diagnostics, Unicode y control bytes son
  no confiables. Sus bytes se preservan en modelo/persistencia, pero la proyección escapa controles y acota ancho.
- SSH servers, fingerprints y host keys pueden ser maliciosos, desconocidos o cambiar. La feature reutiliza
  verifier y trust prompt existentes y no introduce auto-accept en retry, reload, resize, cancel o stale result.
- Otra instancia puede eliminar o revisar targets; resultados asíncronos pueden retrasarse, duplicarse o
  llegar tras cancel/reemplazo. Solo ID/revisión/operation owner capturados autorizan transición.
- Passwords, passphrases, private keys, credential material y raw causes quedan fuera de UI, logs, fixtures,
  usability evidence y cualquier string controlado.
- Terminal, font, key translation y host-trust frontera real no son completamente reproducibles de forma
  portable. Tests automatizados son obligatorios; verificación manual nativa complementaria también lo es.
- Remote stdout/stderr puede ser ilimitado y malicioso; se transmite a los writers terminales existentes sin
  entrar en UI state, fixtures o diagnostics. Timeout e interrupción pueden ocurrir en cualquier lifecycle stage.
- Compromiso del host/cuenta/emulador local queda fuera del threat model. No se añaden shell interpolation,
  crypto/protocolo, persistencia UI, telemetría ni relajación de seguridad.

## Project Structure

### Documentation (this feature)

```text
specs/004-redesign-tui-layout/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── usability-study.md       # Evidencia SC-008 durante implementación
├── contracts/
│   └── tui.md
└── tasks.md                 # Se regenera después de este plan
```

### Source Code (repository root)

```text
internal/tui/
├── model.go                       # Foco, conflict, operation owner, cancel/cleanup y undersized routing
├── layout.go                      # wide/stacked/undersized y rectángulos/overlay geometry-only
├── viewport.go                    # Budget, active-line visibility, markers y ellipsis
├── safe_text.go                   # Proyección terminal-safe acotada de valores no confiables
├── detail.go                      # Proyección segura, viewport e indicadores
├── actions.go                     # Inventarios exactos, inglés, conflict/loading controls
├── browser.go                     # Árbol, fallback ancestor, viewport e indicadores
├── keys.go                        # Bindings normales y `r`/`b`/`Esc` recovery
├── styles.go                      # Jerarquía visual y fallback no-color
├── connection_form.go             # Formulario en Details, viewport y conflicto
├── text_field.go                  # Entrada existente, ancho dinámico y proyección terminal-safe
├── folder_form.go                 # Payload modal cerrado para folder
├── move_picker.go                 # Picker modal, viewport e indicadores
├── modal.go                       # Diez payloads, inline help/error/conflict y viewport rendering
├── errors.go                      # Proyecciones inglesas seguras, nunca raw errors
├── session.go                     # Operation owner SSH, commit point, host trust y cleanup
├── trust_prompt.go                # Superficie segura existente para unknown/changed host keys
├── secret_prompt.go               # Password/passphrase owner, masking y secret lifecycle
├── run.go                         # Prioridad terminal/security owner y restauración
├── *_test.go                      # Matrices, corpus, benchmark, host trust y regresiones
└── testdata/controlled_english.txt # Manifest revisado y exacto de controlled copy

tests/integration/
├── tui_tree_test.go               # Detail/action/fallback/overflow
├── tui_actions_test.go            # Inventarios y dispatch contextual
├── tui_connection_test.go         # Form/modal/operation/host-trust integration
├── folder_hierarchy_test.go       # Direct children y conflictos
└── terminal_paste_test.go         # PTY, control bytes y cleanup
tests/integration/ssh_failure_parity_test.go # SSH diagnostics/cleanup parity

internal/cli/root.go               # NO_COLOR y vocabulario CLI canónico
README.md                          # Layout, keys, language, terminal/platform/security assumptions
```

**Structure Decision**: Se conservan capas, servicios, root model y secure prompts. `layout.go`, `detail.go` y
`actions.go` aíslan geometría, proyección y dispatch. Conflict/operation son estados del modelo, no servicios ni
workers nuevos. El trust prompt no entra en el inventario modal porque es una frontera SSH segura preexistente.
`layout.go` calcula geometría; `viewport.go` y cada renderer aplican contenido/overflow. Foundation implementa
solo el slot modal tipado y sus invariantes; los diez payloads concretos pertenecen a la historia de paneles.
La geometría responsive puede probarse con payloads sintéticos, pero conformance final espera las historias
concretas de Tree, Details, Actions, form y modal.

## Phase 0 Outcome

Todas las decisiones están resueltas en [research.md](research.md): tres display modes, overflow universal,
modal enum cerrado, ayuda inline cuando el modal ya posee foco, target conflict, operation owner/commit points,
keys de recovery, inglés seguro, host trust preservado, threats y protocolo de aceptación. No quedan
`NEEDS CLARIFICATION`.

## Phase 1 Design

- [data-model.md](data-model.md) define layout/viewports, foco, target/conflict, edición, modal y operación.
- [contracts/tui.md](contracts/tui.md) fija composición, keys, loading, recovery, overflow, idioma, host trust y
  conformance matrices.
- [quickstart.md](quickstart.md) fija comandos reproducibles, SC-010–SC-017, rendimiento, amenazas, gates y
  fronteras nativas; además cierra principal-flow, form-order, language-corpus y characterization matrices.

## Complexity Tracking

No hay violaciones constitucionales. Los estados `ConflictState` y `OperationState` son necesarios para
impedir retargeting y resultados stale; se mantienen dentro del root model. Se rechazan una modal stack,
background workers propios, Huh y persistencia porque añaden ownership sin necesidad demostrada.
Los diez modal kinds comparten un renderer y estado; delete connection/folder son variantes distintas porque
capturan tipos, scope copy y confirmaciones diferentes, no porque creen dos mecanismos de overlay.
