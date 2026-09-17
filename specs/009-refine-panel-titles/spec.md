# Feature Specification: Refinar títulos de panel

**Feature Branch**: `N/A`

**Created**: 2026-09-17

**Status**: Draft

**Input**: User description: "No me termina de convencer el estilo que se ha aplicado en la ultima feature. Creo que es mejor que los títulos de las ventanas o paneles no tengan el background con color. Por ejemplo [Tree] o [Details]. Sí que se puede quedar en otros, como por ejemplo para [Connection] o [Folder]. Eso sí me gustaría que estos que tengan background con color no estuviesen dentro de corchetes []. El color tampoco me gusta demasiado. Creo que el contraste entre el azul claro y el gris del texto no es el suficiente para una buena lectura."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Leer títulos de panel con claridad (Priority: P1)

Como usuario que navega por la interfaz de terminal, quiero que los títulos estructurales de los paneles, como `[Tree]` y `[Details]`, se muestren sin fondo de color para distinguirlos con comodidad del contenido y las etiquetas de contexto.

**Why this priority**: Los títulos estructurales orientan la navegación; un fondo llamativo compite innecesariamente con la información y la acción activa.

**Independent Test**: Se puede abrir Tree y Details, con y sin una conexión seleccionada, y comprobar que sus títulos conservan los corchetes pero no muestran fondo de color.

**Acceptance Scenarios**:

1. **Given** la vista principal está visible, **When** se presenta el panel Tree, **Then** su título aparece como `[Tree]` sin fondo de color.
2. **Given** se muestra el panel Details, **When** cambia la selección, **Then** su título aparece como `[Details]` sin fondo de color y sigue identificando el panel.
3. **Given** cualquier panel o ventana con un título estructural, **When** se presenta, **Then** el título conserva la convención de corchetes y no usa el tratamiento visual reservado a las etiquetas de contexto.

---

### User Story 2 - Distinguir el contexto destacado (Priority: P2)

Como usuario que consulta un objetivo seleccionado, quiero que etiquetas de contexto como `Connection` y `Folder` mantengan un fondo destacado, pero sin corchetes, para reconocer el tipo de elemento sin confundirlo con el título del panel.

**Why this priority**: El contexto seleccionado debe destacar, pero su semántica debe ser distinta de la estructura persistente de la pantalla.

**Independent Test**: Se pueden seleccionar una conexión y una carpeta, y verificar que cada tipo se presenta como una etiqueta coloreada sin `[` ni `]`.

**Acceptance Scenarios**:

1. **Given** una conexión está seleccionada, **When** Details presenta su tipo, **Then** se muestra `Connection` con fondo destacado y sin corchetes.
2. **Given** una carpeta está seleccionada, **When** Details presenta su tipo, **Then** se muestra `Folder` con fondo destacado y sin corchetes.
3. **Given** no hay un contexto que requiera una etiqueta de tipo, **When** se presenta el panel, **Then** no se añade una etiqueta destacada vacía ni duplicada.

---

### User Story 3 - Leer etiquetas coloreadas sin esfuerzo (Priority: P3)

Como usuario que trabaja en distintos terminales, quiero que el nuevo color de las etiquetas destacadas tenga contraste suficiente con su texto para leerlo con rapidez y sin depender exclusivamente del color.

**Why this priority**: La mejora estética debe aumentar la legibilidad y mantener la accesibilidad terminal-first.

**Independent Test**: Se pueden mostrar etiquetas `Connection` y `Folder` en terminales con color y sin color, comprobando que el texto se lee claramente y que el tipo se reconoce por el texto cuando el color no está disponible.

**Acceptance Scenarios**:

1. **Given** el color está disponible, **When** se muestra una etiqueta de contexto destacada, **Then** su texto y fondo alcanzan una relación de contraste de al menos 4.5:1.
2. **Given** el color está desactivado o no está disponible, **When** se muestra una etiqueta de contexto, **Then** el texto `Connection` o `Folder` sigue identificando el tipo sin requerir color ni fondo.
3. **Given** se muestra una etiqueta destacada junto a etiquetas descriptivas secundarias, **When** el usuario lee el panel, **Then** ambas siguen siendo distinguibles sin reducir la prioridad de foco, error o acción primaria.

### Edge Cases

- El contexto seleccionado cambia entre raíz, carpeta y conexión mientras el panel Details permanece visible.
- Una terminal no admite el color elegido, desactiva el color o lo representa con contraste insuficiente.
- El panel tiene ancho reducido y debe conservar tanto su título estructural como una etiqueta de contexto sin solaparse.
- Un panel de ayuda, confirmación o error muestra un título estructural, pero no un contexto seleccionable.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE mostrar los títulos estructurales de ventanas y paneles, incluidos `[Tree]` y `[Details]`, sin fondo de color.
- **FR-002**: Los títulos estructurales DEBEN conservar la convención textual entre corchetes y permanecer diferenciables de etiquetas, valores, foco, errores y acciones primarias sin depender solo del color.
- **FR-003**: El sistema DEBE reservar el fondo destacado para etiquetas que identifican el contexto o tipo del elemento mostrado, como `Connection` y `Folder`.
- **FR-004**: Toda etiqueta de contexto con fondo destacado DEBE mostrarse sin los caracteres `[` y `]`.
- **FR-005**: El texto de cada etiqueta de contexto destacada DEBE alcanzar una relación de contraste de al menos 4.5:1 con su fondo cuando el color esté disponible.
- **FR-006**: Cuando el color no esté disponible, el sistema DEBE conservar el texto completo de la etiqueta de contexto y su significado no DEBE depender del color ni del fondo.
- **FR-007**: El cambio de presentación NO DEBE modificar acciones, atajos, orden de foco, contenido de conexión, mensajes de error, reglas de confirmación ni datos persistidos.
- **FR-008**: En los tamaños de terminal ya soportados, título estructural y etiqueta de contexto DEBEN permanecer dentro de los límites del panel, sin solaparse ni ocultar contenido prioritario.

### Key Entities

- **Título estructural**: Identificador persistente de una ventana o panel, presentado entre corchetes y sin fondo de color.
- **Etiqueta de contexto**: Identificador del tipo de elemento actualmente mostrado, como `Connection` o `Folder`, presentado sin corchetes y con fondo destacado cuando el color está disponible.
- **Paleta de etiquetas**: Combinación visual de texto y fondo utilizada para las etiquetas de contexto destacadas.

### Scope Boundaries

**Included**:

- Separación visual entre títulos estructurales y etiquetas de contexto en las superficies TUI existentes.
- Conservación de los corchetes en títulos estructurales y eliminación de corchetes en etiquetas de contexto destacadas.
- Revisión de la paleta de etiquetas destacadas para cumplir el contraste exigido.
- Comportamiento equivalente cuando el color no está disponible.

**Excluded**:

- Nuevos paneles, tipos de contexto, atajos, acciones o flujos de navegación.
- Cambios en conexiones SSH, credenciales, datos mostrados, persistencia o políticas de seguridad.
- Personalización de tema o color por usuario.
- Cambios de geometría, tamaño mínimo de terminal, orden de paneles o reglas de scrollbar existentes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En una revisión de todas las ventanas y paneles existentes, el 100 % de los títulos estructurales se presenta entre corchetes y sin fondo de color.
- **SC-002**: En 30 presentaciones combinadas de raíz, carpeta y conexión, el 100 % de las etiquetas de contexto destacadas se muestra sin corchetes y conserva su texto de tipo correcto.
- **SC-003**: El 100 % de las combinaciones de texto y fondo de etiquetas de contexto alcanza una relación de contraste de al menos 4.5:1.
- **SC-004**: Al menos 19 de 20 participantes identifican correctamente en menos de 5 segundos el título del panel y el tipo de contexto, cuando existe, tanto con color como sin color.
- **SC-005**: En 20 pruebas de cambio de selección y redimensionamiento dentro de los tamaños soportados, el 100 % mantiene títulos y etiquetas dentro de los límites del panel sin ocultar foco, errores ni acciones prioritarias.

## Assumptions

- `Tree` y `Details` representan títulos estructurales; la misma regla se aplicará a los demás títulos de ventanas y paneles existentes.
- `Connection` y `Folder` representan etiquetas de contexto y conservarán un fondo destacado tras eliminar sus corchetes.
- La revisión se limita a la paleta de etiquetas de contexto, no a un rediseño integral del tema de terminal.
- Las reglas existentes de prioridad visual, funcionamiento por teclado, modo sin color y tamaño mínimo siguen siendo normativas.

## Dependencies

- Son normativas las reglas de accesibilidad terminal-first y de no depender solo del color de la Constitución de Orza.
- La presentación de paneles, insignias y modo sin color definida en `specs/008-unify-panel-style/spec.md` se ajustará por esta especificación donde exista conflicto.
