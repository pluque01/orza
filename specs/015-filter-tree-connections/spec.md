# Feature Specification: Filter Tree Connections

**Feature Branch**: `015-filter-tree-connections`

**Created**: 2026-10-01

**Status**: Draft

**Input**: User description: "Quiero una forma de poder filtrar elementos del arbol de conexiones. Cuando tenga el panel de Tree activo, si pulso la tecla \"/\", se abra un input de texto en algún sitio visible y relevante que me permita filtrar los resultados. Este filtro se aplica sólo a las conexiones, las carpetas no son un elemento que se pueda buscar. Sin embargo, cuando se esté haciendo la búsqueda, las carpetas que sean padres de cada conexión que haga match seguirán apareciendo, manteniendo la estructura de arbol de siempre. Mientras se estén filtrando elementos, el usuario puede moverse por la lista de arbol filtrada usando las flechas arriba y abajo o con ctrl+p y ctrl+n. La busqueda debe permitir el uso de expresiones regulares, y por defecto busca en cualquier parte del nombre de la conexion. Si el usuario cancela la búsqueda se debe restablecer el arbol a su estado natural. Para aceptar la busqueda en el elemento activo el usuario pulsa enter. al pulsar enter el arbol se restablece, pero el foco pasa a la conexion seleccionada directamente, aunque no inicia ninguna otra accion como la conexión. La parte de la cadena del nombre que haga match con la búsqueda debe ser resaltada con un color diferente."

## Clarifications

### Session 2026-10-01

- Q: ¿Cómo deben comportarse las carpetas cerradas que contienen una conexión coincidente durante la búsqueda? → A: Expandir temporalmente; cancelar restaura el estado previo y aceptar abre la ruta seleccionada.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Find a Connection in the Tree (Priority: P1)

An operator with the connection tree focused starts a search, types a name or regular expression, and sees only matching connections with their ancestral folders so they can quickly identify the desired connection in its normal hierarchy.

**Why this priority**: Finding a connection quickly is the core value of the feature.

**Independent Test**: Can be fully tested by searching a tree containing connections in several folders and confirming that only matching connections and their parent paths remain visible.

**Acceptance Scenarios**:

1. **Given** the tree panel is focused, **When** the operator presses `/`, **Then** a visible search field is opened and receives text input.
2. **Given** a search query matching a connection name, **When** the operator enters the query, **Then** the tree shows the matching connection, all of its parent folders, and no unrelated connections or folders.
3. **Given** a query that is a valid regular expression, **When** it matches part of a connection name, **Then** the connection is included even when the match is not at the beginning or end of the name.
4. **Given** a matching connection is displayed, **When** its name contains text matched by the query, **Then** every matched portion is visually distinguished from the rest of the name without relying only on color to convey selection.

---

### User Story 2 - Navigate Filtered Matches (Priority: P2)

An operator moves among the matching connections while the search remains active, using either arrow keys or the established Ctrl+P and Ctrl+N shortcuts.

**Why this priority**: Search only saves time if the operator can select a result without leaving the keyboard workflow.

**Independent Test**: Can be fully tested with three matching connections by moving to adjacent matches with each supported shortcut and verifying the active connection changes accordingly.

**Acceptance Scenarios**:

1. **Given** a search with multiple matching connections, **When** the operator presses the up arrow or Ctrl+P, **Then** the active result moves to the preceding matching connection when one exists.
2. **Given** a search with multiple matching connections, **When** the operator presses the down arrow or Ctrl+N, **Then** the active result moves to the following matching connection when one exists.
3. **Given** the first or last matching connection is active, **When** the operator attempts to move beyond that end, **Then** the active result remains unchanged.

---

### User Story 3 - Exit Search Safely (Priority: P3)

An operator cancels a search to return to the unfiltered tree, or confirms the active result to restore the tree with focus placed on that connection without opening a connection.

**Why this priority**: Operators need a predictable way to leave a temporary search state without unintended connection activity.

**Independent Test**: Can be fully tested by starting a search, cancelling it, then repeating the search and confirming a result while observing the displayed tree, focus, and connection state.

**Acceptance Scenarios**:

1. **Given** an active search, **When** the operator cancels it, **Then** the search field closes and the complete tree is restored without changing the focused item.
2. **Given** an active search with a matching connection selected, **When** the operator presses Enter, **Then** the search field closes, the complete tree is restored, and focus is placed on that connection.
3. **Given** an operator confirms a search result, **When** the focus returns to the selected connection, **Then** no connection attempt or other connection action is initiated.

---

### Edge Cases

- A query with no matching connection shows no connection results and clearly indicates that no matches were found; folders alone are not shown as results.
- An invalid regular expression does not alter the underlying tree, does not select a connection, and provides an understandable validation message while allowing the operator to correct or cancel the query.
- An empty query preserves the natural tree structure and does not apply match highlighting.
- A matching connection located inside multiple nested folders displays every folder in its path, even if none of those folder names match the query.
- Folder names are never evaluated as search candidates and cannot produce a result by themselves.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST open a visible, keyboard-focused search field when `/` is pressed while the tree panel has focus.
- **FR-002**: The system MUST limit this search workflow to the tree panel and MUST NOT open it when another panel has focus.
- **FR-003**: The system MUST evaluate the current query only against connection names; folder names MUST NOT be searchable.
- **FR-004**: The system MUST treat a valid query as a regular expression and include a connection when any portion of its name matches the expression unless the expression itself constrains the position.
- **FR-005**: While a non-empty valid query is active, the system MUST display every matching connection and only the folders required to preserve each displayed connection's original parent path; it MUST temporarily expand any collapsed folders in those paths.
- **FR-006**: While search is active, the system MUST make matching connections, rather than structural folders, the navigable search results.
- **FR-007**: While search is active, the system MUST move the active matching connection using Up Arrow and Ctrl+P for the previous result and Down Arrow and Ctrl+N for the next result.
- **FR-008**: The system MUST keep the active result unchanged when navigation is requested beyond the first or last matching connection.
- **FR-009**: The system MUST visually distinguish all portions of each displayed connection name that match the current valid query, while retaining a non-color cue for the active result.
- **FR-010**: The system MUST allow the operator to cancel an active search and restore the complete tree, the folder expansion state, and the focus state that existed before search began.
- **FR-011**: The system MUST accept an active matching connection with Enter, restore the complete tree, expand every folder in the accepted connection's path, and place tree focus on that connection without initiating a connection or any other action.
- **FR-012**: The system MUST preserve the unfiltered tree and provide an understandable correction-or-cancel path when the query is not a valid regular expression.
- **FR-013**: The system MUST display an understandable no-results state when a valid non-empty query matches no connection names.

### Key Entities *(include if feature involves data)*

- **Connection**: A named, actionable entry in the connection tree and the only entity eligible to match a search query.
- **Folder**: A structural tree entry that groups connections; it remains visible only when it is an ancestor of a displayed matching connection and is never a search result.
- **Search Query**: The temporary text entered by the operator, interpreted as a regular expression for matching connection names.
- **Active Result**: The matching connection currently selected during search and eligible for acceptance with Enter.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a tree with at least 100 connections across nested folders, operators see the correctly filtered hierarchy within 1 second of changing a valid query.
- **SC-002**: In usability verification, at least 95% of operators can locate and focus a named connection using search and keyboard navigation on their first attempt without initiating it.
- **SC-003**: For a test set of at least 20 valid regular-expression queries, 100% of displayed connections match the query and 100% of each displayed folder is an ancestor of at least one displayed connection.
- **SC-004**: In cancellation and acceptance tests, 100% of attempts restore the complete tree, and 100% of accepted results leave the selected connection focused without starting a connection.

## Assumptions

- The existing tree has a visible focused state and can place focus on an individual connection without activating it.
- Escape is the standard cancellation key for the search field; any existing general cancellation key retains the same behavior while search is active.
- Searches are case-sensitive unless the entered regular expression explicitly requests otherwise.
- Searches are temporary and are not persisted after the search is cancelled or accepted.
- This feature does not change folder expansion preferences or add search across connection properties other than the displayed connection name.
