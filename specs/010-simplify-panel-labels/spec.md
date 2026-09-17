# Feature Specification: Simplificar etiquetas de panel

**Feature Branch**: `N/A`

**Created**: 2026-09-17

**Status**: Draft

**Input**: User description: "Tienes razón, es mejor no usar background color para el contexto (Connection, Folder. Quita el background color de esos textos, pero ponlos con negrita para que destaquen. Para los títulos, déjalos como están pero elimina los corchetes del texto, no los del [*]"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Leer la estructura sin ruido visual (Priority: P1)

Como usuario de la interfaz de terminal, quiero que los títulos de panel se muestren sin corchetes para reconocer la estructura de la pantalla sin adornos redundantes, manteniendo visibles los indicadores de foco.

**Why this priority**: Los títulos de panel son visibles en toda interacción y deben ser claros sin competir con el contenido.

**Independent Test**: Se puede mostrar Tree, Details, Actions y un modal, verificando que cada título se presenta como texto simple y que `[*]` o `[ ]` sigue identificando el foco.

**Acceptance Scenarios**:

1. **Given** la vista principal está visible, **When** Tree tiene el foco, **Then** su encabezado contiene `[*] Tree` y no contiene `[Tree]`.
2. **Given** Details no tiene el foco, **When** se presenta junto a Tree, **Then** su encabezado contiene `[ ] Details` y no contiene `[Details]`.
3. **Given** un modal está abierto, **When** se muestra su título, **Then** el título no está entre corchetes y conserva el indicador de foco aplicable.

---

### User Story 2 - Identificar contexto en cualquier tema (Priority: P2)

Como usuario con cualquier tema de terminal, quiero que las etiquetas de contexto como `Connection` y `Folder` destaquen mediante negrita sin color de fondo para leerlas sin depender de la paleta del terminal.

**Why this priority**: El fondo coloreado puede perder contraste al variar los colores ANSI entre temas.

**Independent Test**: Se pueden seleccionar raíz, carpeta y conexión en modos con color y sin color, comprobando que la etiqueta de tipo se presenta una vez, sin corchetes ni fondo y con negrita cuando el terminal admite estilo.

**Acceptance Scenarios**:

1. **Given** una conexión está seleccionada, **When** se presenta Details, **Then** `Connection` aparece sin corchetes, sin fondo de color y en negrita.
2. **Given** una carpeta está seleccionada, **When** se presenta Details, **Then** `Folder` aparece sin corchetes, sin fondo de color y en negrita.
3. **Given** el color está desactivado, **When** se presenta una etiqueta de contexto, **Then** conserva su texto completo y sigue siendo distinguible sin color.

### Edge Cases

- El foco cambia entre Tree, Details y Actions mientras el tipo de contexto permanece visible.
- La terminal elimina códigos de estilo o usa una paleta ANSI personalizada.
- El ancho reducido exige truncar contenido sin ocultar título, foco o etiqueta de contexto.
- Formularios, prompts de confianza y de secretos muestran etiquetas de contexto ya existentes.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE mostrar todos los títulos estructurales de regiones y modales sin los caracteres `[` y `]` alrededor del texto del título.
- **FR-002**: El sistema DEBE conservar sin cambios los marcadores textuales `[*]` y `[ ]` que indican foco activo e inactivo.
- **FR-003**: El sistema DEBE mostrar las etiquetas de contexto existentes, como `Connection`, `Folder` y `Root`, sin corchetes ni fondo de color.
- **FR-004**: Cuando el terminal admite estilo, las etiquetas de contexto DEBEN usar negrita para destacar del contenido descriptivo.
- **FR-005**: Cuando el estilo no está disponible, títulos, marcadores de foco y etiquetas de contexto DEBEN conservar significado completo mediante su texto y posición.
- **FR-006**: El cambio visual NO DEBE alterar controles de teclado, orden de foco, geometría, contenido, confirmaciones, errores, secretos, conexiones ni datos persistidos.
- **FR-007**: En todos los tamaños de terminal ya soportados, títulos, marcadores de foco y etiquetas de contexto DEBEN permanecer dentro de los límites de su panel.

### Key Entities

- **Título estructural**: Nombre textual de una región o modal, sin corchetes propios.
- **Marcador de foco**: Señal textual existente `[*]` o `[ ]` que precede al título estructural.
- **Etiqueta de contexto**: Tipo del contenido presentado, sin corchetes ni fondo y destacado con negrita cuando esté disponible.

### Scope Boundaries

**Included**:

- Presentación de títulos, marcadores de foco y etiquetas de contexto en superficies TUI existentes.
- Equivalencia semántica en terminales con estilo y sin estilo.

**Excluded**:

- Nuevos paneles, etiquetas, acciones, atajos, flujos o temas configurables.
- Cambios en SSH, credenciales, datos, persistencia, tamaño mínimo o geometría de paneles.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En todas las regiones y modales existentes, el 100 % de los títulos se muestra sin corchetes propios y el 100 % de los marcadores de foco conserva `[*]` o `[ ]`.
- **SC-002**: En 30 presentaciones de raíz, carpeta y conexión, el 100 % de las etiquetas de contexto aparece una sola vez sin corchetes ni fondo de color.
- **SC-003**: En color y sin color, al menos 19 de 20 participantes identifican el panel activo y el tipo de contexto en menos de 5 segundos.
- **SC-004**: En 20 cambios de selección, foco y tamaño soportado, el 100 % conserva títulos y contexto dentro de los límites del panel sin ocultar controles prioritarios.

## Assumptions

- La negrita es un énfasis opcional; el texto de contexto conserva significado si la terminal no la representa.
- La regla de títulos sin corchetes se aplica a todas las regiones y modales existentes.
- La regla sustituye los fondos y colores de la feature 009 para etiquetas de contexto.

## Dependencies

- Las reglas de accesibilidad terminal-first y de no depender solo del color de la Constitución de Orza son normativas.
- Esta especificación ajusta la presentación definida en `specs/009-refine-panel-titles/spec.md` cuando exista conflicto.
