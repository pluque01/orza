# Research: Rediseño multipanel de la TUI

> **Superseded decision**: Feature 005 supersedes this document's vertical directional-marker decision with proportional right-edge scrollbars.

## Decisión 1: Conservar un único modelo Bubble Tea

**Decision**: Mantener `internal/tui.Model` como único owner de estado, comandos, foco, modal y operación.

**Rationale**: Servicios, catálogo, sesión y terminal ya convergen en el root model; ownership único permite
filtrar resultados por operation ID sin duplicar selección.

**Alternatives considered**:

- Modelo por panel: rechazado por sincronización y targets duplicados.
- Reescritura completa: rechazada por riesgo sobre SSH, terminal y seguridad aprobada.

## Decisión 2: Tres display modes calculados antes de presentar

**Decision**: `wide` cuando ancho >=80 y tamaño >=40x12; `stacked` cuando ancho <80 y tamaño >=40x12;
`undersized` cuando ancho <40 o alto <12. `undersized` presenta aviso/minimum/Help/Quit y no clampa ni muta
selección, expansión, offset, form, focus, modal, conflict u operation. `reduced` es una prioridad ortogonal
para cualquier wide/stacked con width <80 o height <24; por tanto 80x12 y 100x20 son wide reduced.

**Rationale**: Un fallback explícito elimina geometría imposible y restaura exactamente la interacción al
recuperar 40x12.

**Alternatives considered**:

- Best effort en cualquier dimensión: rechazado por controles imposibles.
- Tratar toda altura <24 como stacked: rechazado porque orientación depende solo del ancho.

## Decisión 3: Composición Charmbracelet sin dependencia nueva

**Decision**: Usar Lip Gloss v2 para estilos/composición y `x/ansi` para display width, cortes, wrapping y
escaping terminal-safe. Viewports usan offsets enteros; no se añade Huh ni Bubbles viewport.

**Rationale**: Los forms actuales ya garantizan paste atómico, Unicode, límites y secretos; una segunda
abstracción añadiría riesgo sin necesidad.

**Alternatives considered**:

- Huh: rechazado por duplicar validation/focus.
- ANSI manual global: rechazado; solo el compositor overlay requiere reemplazo width-aware.

## Decisión 4: Inventario cerrado y un solo panel centrado

**Decision**: El enum cerrado contiene folder create/edit, move destination, delete connection/folder,
connect confirmation, unsaved changes, Help, operation error y SSH failure. Connection create/edit permanece
en Details. No hay stack ni replacement; errores, conflictos y Help se embeben en el panel owner.

Son diez variantes concretas: delete connection y delete folder son kinds distintos por target/scope copy y
confirmación, pero comparten renderer. El slot genérico y sus invariantes se implementan antes que payloads.
Con modal activo, `?` muestra/oculta una sección Help inline; `Esc` desde esa sección vuelve al control previo,
su viewport es acotado y kind/target/payload/focus subyacente no cambian.

**Rationale**: Un slot hace estructural el foco único. Si una operación modal permite Help, el mismo payload
alterna una sección inline y mantiene kind/target/operation; no abre un segundo `help` modal.

**Alternatives considered**:

- Stack de modales: rechazado por foco ambiguo.
- Incluir trust prompt: rechazado; host trust conserva su superficie segura especializada.

## Decisión 5: Proyección de detalle desde snapshot

**Decision**: Connection proyecta name/path/endpoint/user/method/identity reference; folder/root filtra
`snapshot.children[id]` a conexiones directas en orden contractual. Nunca hace I/O en View.

**Rationale**: El snapshot ya posee hijos directos consistentes y evita escanear 1.100 nodos.

**Alternatives considered**:

- Query por selección: rechazada por I/O y race.
- Recorrido global: rechazado por coste y descendientes accidentales.

## Decisión 6: Viewport e indicadores universales

**Decision**: Toda región desplazable reserva la primera fila para `↑ more` cuando hay contenido anterior y
la última para `↓ more` cuando hay posterior; el budget de items se reduce en consecuencia. Toda línea que no
cabe incluye `…` dentro del ancho. Con una o dos filas, active/error/recovery gana prioridad y los markers se
muestran solo si queda fila sin ocultarlo.

**Rationale**: Marcadores textuales funcionan con no-color y forman un contrato común para Tree, Details,
forms, Actions, Help, picker y confirmations.

**Alternatives considered**:

- Scrollbar: rechazada por ancho y dependencia visual.
- Ellipsis único: rechazado porque no indica dirección.

## Decisión 7: Matriz única de acciones, ayuda y dispatch

**Decision**: Root `n/f/r/?/q`; folder `n/f/e/m/d/r/?/q`; connection `c/n/f/e/m/d/r/?/q`; `Ctrl+C` alias de
`q`. `n/f` sobre connection usa parent. Movement/focus/scroll son controles separados. Mientras operation está
activa solo `Esc` Cancel, `?` Help y `q`/`Ctrl+C` Quit son dispatchables.
Si Help está visible, sus controles locales de scroll/navigation son la única excepción no mutante; no cuentan
como acciones y no pueden iniciar persistence/network/otra operation.

**Rationale**: Un origen evita divergencia entre footer, Help y handler.

**Alternatives considered**:

- Switches separados: rechazados por drift.
- Acciones deshabilitadas visibles: rechazadas por spec.

## Decisión 8: Formularios existentes en nuevas superficies

**Decision**: Conservar `connectionForm`, `folderForm`, `movePicker` y `textField`; connection form ocupa
Details y ajusta width/offset; folder/picker son payloads modales cerrados.

El orden canónico es Name → Folder → Host → Port → User → Method → Identity file solo para key → Remember
password solo para password → Save, con recorrido inverso exacto en Shift+Tab. Un control condicional oculto
no participa; si Method lo oculta mientras posee foco, foco vuelve a Method. Validation es field-local;
save/persistence failure es form-level inline sobre Save y nunca abre/reemplaza modal.

**Rationale**: Reutiliza validation, dirty state, CAS, destinations, paste y secrets existentes.

**Alternatives considered**:

- Forms nuevos: rechazados por duplicación y regresión.
- Tree interactivo durante form: rechazado por target ambiguo.

## Decisión 9: Conflicto por target capturado

**Decision**: Navigation fallback selecciona nearest existing ancestor/root. Form/modal/operation conserva
ID/revision/path/values/focus y entra en conflict `missing|revision_changed`, bloqueando persistence/network.
`r` rechecks same ID, `b` Back abandona y selecciona fallback, `Esc` oculta detail pero deja banner compacto,
blocked state y recovery controls visibles para reabrir/reintentar.

Una operación running/cancel_requested/cleaning nunca comparte dispatch con conflict. Si su owner result es
conflict, primero alcanza completed y termina cleanup; después se elimina OperationState y se activa
ConflictState con el pending intent capturado. Desde ese frame gana el inventario `r/b/Esc/?/q`.

**Rationale**: Ninguna transición depende de row position ni pierde input temporal.

**Alternatives considered**:

- Retarget a fila actual: rechazado por seguridad.
- Cerrar interacción automáticamente: rechazado por pérdida de datos.

## Decisión 10: Una operación asíncrona owner

**Decision**: `OperationState` posee ID, kind, target, surface, phase y quit intent. Último frame permanece;
initial load usa shell vacío. Actions muestra `Loading: <action> — <target>`. Extra mutations son inert.
`Esc` solicita cancel, `?` muestra Help inline si hay modal owner o modal Help si no, y `q` solicita cancel+quit.
Cuando Help está visible, sus controles locales de scroll son la única excepción no mutante al filtro de
operation; no aparecen como domain actions ni pueden iniciar I/O.

Commit points:

- `initial_load`/`reload`: snapshot validado aceptado por el model.
- `save`: servicio retorna éxito después del commit SQLite con ID/revision resultantes.
- `ssh_start`: resultado marca sesión activa (`StartedAt` no cero) después de trust/auth/session setup.

Antes del commit, cancel restaura; después, el resultado confirmado se aplica antes de restaurar/salir. Solo
matching operation ID actualiza. Cancel/quit espera phase `cleaning_up -> completed`.

Un result conflictivo también es completion: cleanup termina, operation owner se libera y solo entonces se
publica ConflictState. Nunca existen simultáneamente una operación activa y un conflicto despachable.

Antes de dispatch se revalida same ID/revision y conflicto bloquea I/O. Durante running, operation inventory
gana. Un conflicto detectado antes del commit termina sin commit; después del commit, confirmed result se
reconcilia antes de cleanup y no se presenta como conflicto pre-commit. Cancel during cleanup no cambia este
orden; stale/non-owner completion siempre se ignora.

**Rationale**: Explicita races de cancel, commit, stale completion y cleanup.

**Alternatives considered**:

- Boolean loading: rechazado por falta de target/owner/phase.
- Aceptar resultados tardíos: rechazado por state corruption.

## Decisión 11: Salida con cambios

**Decision**: Quit dirty ofrece Save/Discard/Cancel. Save marca quit intent y solo sale tras commit y cleanup;
Discard sale sin persistir; Cancel vuelve al form. Save failure/conflict conserva todo.

**Rationale**: Protege entrada y cumple salida explícita.

**Alternatives considered**:

- Autosave o autodiscard: rechazados por consentimiento/pérdida.

## Decisión 12: Inglés controlado y presentación segura

**Decision**: Un vocabulario central en Actions/errors usa términos CLI: Tree, Details, Actions, New
connection, New folder, Edit, Move, Delete, Connect, Reload, Save, Discard, Cancel, Back, Help y Quit.
Strings controlados son inglés. Valores del usuario conservan bytes en modelo/persistencia; al presentar,
control/escape/bidi bytes se hacen inertes y contenido imprimible se conserva salvo `…` visual.

**Rationale**: Concilia SC-014 con input no confiable y evita escape injection o copy drift.

**Alternatives considered**:

- Mostrar bytes de control literalmente: rechazado por seguridad terminal.
- Localización nueva: fuera de scope.

## Decisión 13: Host trust permanece en frontera segura

**Decision**: Reutilizar verifier, `trustPrompt` y secret prompts existentes bajo un `SecurityInputOwner`.
Known key procede; unknown/changed muestra target, fingerprint/status y exige decisión explícita. Cancel no
conecta ni persiste trust. Password/passphrase mantiene masking y lifecycle existente. Ningún security prompt
usa el slot modal genérico ni puede auto-aceptarse por reload/retry/resize/cancel/stale completion.

Tree/Details/form/modal conservan su FocusOwner, pero `SecurityInputOwner` es prioritario incluso sobre
operation filtering y bloquea todo dispatch de aplicación. Trust permite decision/cancel y prompt-local
navigation; secret permite entrada/masking/cancel según lifecycle existente. En undersized no se muestra ni
acepta trust/secret input: se preserva pending y el shell común solo ofrece Help/safe Quit hasta resize. Cancel
restaura FocusOwner; quit/cancel/terminal failure espera session cleanup.

**Rationale**: La constitución prevalece y la frontera ya implementa terminal/secret lifecycle especializado.

**Alternatives considered**:

- Convertirlo en connect confirmation: rechazado porque ocurre tras resolución de host key.
- Auto-accept: prohibido constitucionalmente.

## Decisión 14: Estilo adaptativo y semántica textual

**Decision**: Ampliar ANSI-16 existente con panel active/inactive, title, primary, muted, warning y failure;
mantener `>`, `[/]`, `[+]`, `[-]`, `[ssh]`, `!`, keys y markers overflow. `NO_COLOR` elimina ANSI, no semántica.

**Rationale**: Preserva identidad existente y accesibilidad.

**Alternatives considered**:

- Paleta Nexus impuesta: rechazada por romper sistema actual.
- Color-only focus: prohibido.

## Decisión 15: Protocolo de verificación no dispensable

**Decision**: Pruebas estructurales/table-driven cubren SC-010–SC-017, exact dimensions, 20-run matrices,
operation Cartesian cases, conflict cells, overflow corpus, language corpus y host trust. Performance usa el
runner/clock de SC-011. Usability reproduce SC-008. Formatting/test/race/vet/staticcheck/vuln/Nix/cross-build
y fronteras manuales aplicables son gates; unavailable/unrun nunca equivale a pass.

Cada SC usa una matriz cerrada en quickstart. Todos los controlled literals, descriptors y error projections
extraídos de `internal/tui` deben coincidir exactamente con el manifest humano revisado
`internal/tui/testdata/controlled_english.txt`; el manifest fija English copy y término canónico por concepto,
con allowlist explícita para user/backend values. Tests de comportamiento nuevo deben fallar antes de
implementar; decisiones trust, paste, secret lifecycle, signal y cleanup ya existentes son characterization
regressions. Nuevo ownership/reflow/undersized security input es red-first.

**Rationale**: Convierte porcentajes y MUST constitucionales en evidencia reproducible.

**Alternatives considered**:

- Solo snapshots o manual: rechazados por no probar ownership/target.
- Waiver por entorno: rechazado; la evidencia queda pendiente.

## Threat Assumptions and Risks

- Catalog, DB, environment, server/backend strings, terminal dimensions, Unicode, control/bidi bytes y paste
  son no confiables; presentation escapa/limita sin mutar datos persistidos.
- Secrets y raw causes nunca llegan a UI, logs, tests, usability evidence o vocabulario controlado.
- Other instances y async scheduling pueden producir missing/revision conflict, duplicates y stale results.
- SSH peer/host key puede ser malicioso o cambiar; verifier/trust decision no se relaja.
- ANSI cutting, one/two-row marker budgets, 39x11 transitions, cancel-after-commit y quit-during-cleanup son
  fronteras de alto riesgo con tests explícitos.
- Host/account/emulator comprometidos quedan fuera. No se añade shell, crypto/protocol, telemetry o persistence.

## Decisión 16: Streaming y fallos de transporte acotados

**Decision**: Conservar `io.Reader`/`io.Writer` directos de la sesión existente: stdout/stderr remoto nunca se
copian a Model, viewport o diagnostic buffer. La aceptación transmite 64 MiB por stream y permite como máximo
1 MiB retenido por la TUI; los technical details permanecen limitados a 256 caracteres visibles. Timeout y
network interruption en startup o active session cancelan el mismo context owner, cierran session, transport,
auth y resize forwarding exactamente una vez, restauran terminal y descartan completion tardío por operation ID.

**Rationale**: El streaming directo ya preserva interactividad y evita crecimiento proporcional. Extender el
owner/cleanup existente cubre fallos constitucionales sin añadir buffering, workers o protocolo.

**Alternatives considered**:

- Capturar output para mostrarlo en Details: rechazado por memoria, secretos y semántica de terminal interactiva.
- Añadir un buffer circular: rechazado porque no hay requisito de replay y aún retendría contenido remoto.
- Tratar timeout/interrupción como salida abrupta: rechazado porque puede dejar terminal y recursos activos.

No quedan incógnitas técnicas ni `NEEDS CLARIFICATION` para implementación.
