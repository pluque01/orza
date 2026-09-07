# Data Model: Árbol contextual y pegado en TUI

Este feature no añade tablas ni modifica entidades persistentes. Los siguientes modelos son proyecciones
y estado efímero de presentación construidos desde tipos `internal/app` existentes.

## CatalogSnapshot

Vista coherente del catálogo usada por una renderización del árbol.

| Field | Type | Rules |
|-------|------|-------|
| `revision` | `app.CatalogRevision` | Revisión observada por la carga; una divergencia durante el recorrido invalida el intento. |
| `root` | `TreeNode` | Siempre existe y representa `/`. |
| `nodeByID` | `map[app.NodeID]TreeNode` | Una entrada por carpeta/conexión; la raíz usa una clave interna reservada. |
| `childrenByParent` | `map[app.NodeID][]app.NodeID` | Carpetas antes que conexiones; nombre binario sensible a mayúsculas y luego ID. |
| `parentByID` | `map[app.NodeID]app.NodeID` | Todo nodo salvo raíz tiene un padre existente en el snapshot. |

### Validation

- Ningún ID aparece dos veces.
- No hay ciclos y todo nodo es alcanzable desde raíz.
- Profundidad derivada máxima admitida: 10 según escala del producto; datos corruptos fallan con error
  recuperable en lugar de recursión ilimitada.
- Todas las respuestas usadas en una carga deben tener la misma `CatalogRevision`; si cambia, se repite
  una vez y después se muestra conflicto/recarga disponible.

## TreeNode

Proyección de una carpeta, conexión o raíz.

| Field | Type | Rules |
|-------|------|-------|
| `id` | `app.NodeID` | Identidad estable; raíz se normaliza a una identidad interna. |
| `kind` | root/folder/connection | Determina icono textual y acciones válidas. |
| `name` | `string` | Valor validado por dominio; raíz se muestra `/`. |
| `path` | `string` | Ruta completa mostrada en detalles y confirmaciones. |
| `revision` | `app.Revision` existente | Se captura al iniciar mutaciones optimistas. |
| `folder` | `*app.Folder` | Solo para folder; datos necesarios para formularios/acciones. |
| `connection` | `*app.Connection` | Solo para connection; datos necesarios para detalles/conectar. |

## TreeViewState

| Field | Type | Rules |
|-------|------|-------|
| `snapshot` | `CatalogSnapshot` | Última carga válida; no se muta durante una carga nueva. |
| `expanded` | `map[app.NodeID]struct{}` | Solo IDs presentes; raíz siempre expandida y demás carpetas contraídas inicialmente. |
| `selectedID` | `app.NodeID` | Exactamente un nodo visible; raíz en toda carga inicial. |
| `visibleRows` | `[]TreeRow` | Derivada en preorder; nunca fuente de identidad. |
| `selectedAncestors` | `[]app.NodeID` | Seleccionado a raíz, capturado antes de mutar/recargar para fallback. |
| `pendingSelection` | `*app.NodeID` | ID creado/movido que debe revelarse tras recarga. |
| `pendingConnect` | ID/revision/path/endpoint opcional | Snapshot inmutable mostrado y enviado como revisión esperada al confirmar conexión. |
| `loading` | `bool` | Una carga en curso no borra snapshot/formulario anterior. |
| `error` | `string` | Mensaje recuperable sin datos secretos. |

### Invariants

- `selectedID` corresponde a una fila visible.
- Contraer una carpeta que contiene la selección selecciona primero esa carpeta.
- Expandir/contraer no modifica `CatalogSnapshot` ni catálogo persistente.
- `visibleRows` se reemplaza como conjunto después de cada cambio, nunca parcialmente.

## TreeRow

| Field | Type | Rules |
|-------|------|-------|
| `nodeID` | `app.NodeID` | Referencia a `nodeByID`. |
| `depth` | `int` | Raíz 0; hijo = padre + 1. |
| `expanded` | `bool` | Solo significativo para carpeta/raíz. |
| `hasChildren` | `bool` | Controla indicador expandible sin depender de color. |

La vista usa siempre los prefijos ASCII normativos `>`, `[/]`, `[+]`, `[-]` y `[ssh]`, con dos celdas de
indentación por `depth`. El viewport vertical conserva visible la fila seleccionada.

## TextFieldState

Adaptador efímero para cada campo normal.

| Field | Type | Rules |
|-------|------|-------|
| `input` | `textinput.Model` | Valor, foco y cursor base; binding de paste de SO deshabilitado. |
| `anchor` | cursor index opcional | Junto al cursor define selección; nil significa sin selección. |
| `width` | terminal cells | Limita render; el valor está limitado a 4.096 runes. |
| `error` | `string` | Validación o rechazo de pegado seguro. |

### Selection rules

- Shift+movimiento crea/extiende selección desde la posición inicial.
- Movimiento sin Shift colapsa selección hacia el borde correspondiente.
- Escritura, Backspace y Delete reemplazan/eliminan la selección antes de delegar edición ordinaria.
- Pérdida de foco conserva valor pero elimina selección visual.

## PasteTransaction

No se persiste ni se pone en cola.

| Field | Type | Rules |
|-------|------|-------|
| `raw` | `string` | Solo vive durante el handler; nunca se registra. |
| `normalized` | `string` | `raw` sin CR, LF, NEL, `U+2028` ni `U+2029`. |
| `candidate` | `string` | Valor resultante tras reemplazar selección/inserción. |
| `rejected` | `bool` | True si hay otro control o falla un límite inmediato. |

### Transition

`received -> inspect -> normalize-line-separators -> reject-controls | validate-4096-limit -> reject | commit`

`reject` conserva exactamente valor, cursor y selección. `commit` actualiza los tres como una operación.
Los mensajes `PasteStart`/`PasteEnd` no cambian el campo.

## SecretEditorState

Estado privado de `internal/terminal`; nunca cruza a Bubble Tea.

| Field | Type | Rules |
|-------|------|-------|
| `buffer` | byte/rune buffer borrable | Secreto de máximo 4.096 bytes; se limpia en cancelación/fallo y tras transferencia. |
| `cursor` | boundary index | Siempre en un límite Unicode válido. |
| `anchor` | boundary index opcional | Define selección; solo se renderizan máscaras. |
| `error` | enum/mensaje genérico | Nunca incluye payload ni secreto. |
| `submitted` | `bool` | Solo un `KeyPressEvent` Enter real puede activarlo. |

### State transitions

- `idle -> editing`: terminal capturado, raw y bracketed paste activos.
- `editing -> editing`: tecla de edición, movimiento, selección o paste aceptado.
- `editing -> rejected`: paste inválido; buffer/cursor/selección no cambian.
- `rejected -> editing`: siguiente tecla/paste borra mensaje genérico.
- `editing -> submitted`: Enter real; se inicia cleanup antes de devolver bytes.
- `editing -> canceled`: Escape, Ctrl+C, señal o contexto; se limpia buffer y se inicia cleanup. Tras la
  restauración, el contexto raíz termina con 130 para `os.Interrupt`/SIGINT o 143 para SIGTERM Unix.
- `* -> failed`: I/O/parser/cleanup; se limpia cualquier resultado y se intenta toda restauración.
- `submitted -> returned`: solo después de cleanup completo.

## Reconciliation Transitions

| Event | Resulting selection | Expansion |
|-------|---------------------|-----------|
| Initial load | Root | Root expanded; all other folders collapsed. |
| Expand folder | Same folder | Add folder ID. |
| Collapse folder | Folder itself if selection was a descendant; otherwise unchanged | Remove folder ID. |
| Create node | Created ID | Add all ancestor folder IDs. |
| Rename node | Same ID | Preserve valid IDs. |
| Move node | Same ID | Expand new ancestors; prune none still valid. |
| Delete selected | Nearest surviving captured parent, else root | Prune missing IDs. |
| External reload | Same ID if present, else nearest surviving captured parent, else root | Preserve only existing folder IDs. |
