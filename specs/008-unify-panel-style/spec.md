# Feature Specification: Estilo visual unificado de paneles

**Feature Branch**: `008-unify-panel-style`

**Created**: 2026-09-11

**Status**: Approved

**Input**: User description: "Quiero mejorar el aspecto visual del panel de detalles. El campo de kind: Connection podría sustituirse dejando solo Connection con un color de background llamativo, que puede ser uno característico de la aplicacion. Luego los campos que identifican la conexión deberían estar alineados horizontalmente. Las etiquetas de descripción pueden aparecer sin los ':' y con un color menos llamativo. Este estilo visual debería de ser igual para el resto de pestañas, como la de confirmacion de conexión, panel de acciones, panel de ayuda..."

## Clarifications

### Session 2026-09-11

- Q: ¿Cómo deben distribuirse los campos que identifican una conexión en el panel Details cuando hay espacio suficiente? → A: Una fila por campo, con las etiquetas en una columna y todos los valores alineados en otra.
- Q: ¿Debe la jerarquía común de etiquetas y valores aplicarse también a los formularios editables y paneles de error, además de Details, confirmaciones, Actions y Help? → A: Todas las superficies TUI existentes con contenido estructurado.
- Q: ¿En qué superficies debe mostrarse la insignia destacada `Connection`? → A: Cada panel debe tener una insignia destacada propia según su tipo.
- Q: ¿Debe la insignia sustituir el título actual de cada panel o añadirse dentro de su contenido? → A: El título se estiliza como insignia si ya identifica el tipo; solo se añade otra cuando identifica contenido distinto.
- Q: ¿Cómo deben mostrarse las filas de identidad cuando el panel es demasiado estrecho para mantener alineadas las dos columnas? → A: Apilar etiqueta y valor, sin ocultar campos.

## Definitions

- **Panel**: Región o superficie visible con título y contenido propio: Details, Actions, formularios de conexión y paneles centrados como confirmación y Help.
- **Insignia de tipo**: Etiqueta breve y destacada que identifica cada panel o su contenido, como `Connection`, `Actions`, `Help`, `Confirmation` o `Error`, sin prefijos técnicos como `Kind:`. Reutiliza el título cuando este ya identifica el tipo y se añade al contenido solo cuando ambos identificadores son distintos.
- **Etiqueta descriptiva**: Texto que nombra un valor mostrado, como `Host` o `Path`.
- **Fila de identidad**: Fila que presenta una etiqueta descriptiva y su valor asociado para identificar una conexión u otro objetivo.
- **Jerarquía visual**: Diferencia perceptible entre tipo, etiquetas, valores, foco, selección, error y acción primaria, que sigue siendo comprensible sin color.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reconocer una conexión de un vistazo (Priority: P1)

Como usuario que explora conexiones, quiero que Details identifique claramente una conexión y alinee sus datos principales para reconocer el objetivo y sus atributos sin leer una lista técnica desordenada.

**Why this priority**: Details es la referencia contextual principal durante la navegación y una jerarquía clara reduce el riesgo de actuar sobre una conexión equivocada.

**Independent Test**: Se puede seleccionar una conexión con nombre, ruta, destino, usuario y método, y comprobar que Details muestra una única insignia `Connection`, filas de identidad alineadas y etiquetas sin dos puntos.

**Acceptance Scenarios**:

1. **Given** una conexión seleccionada, **When** se muestra Details, **Then** aparece `Connection` como insignia de tipo y no aparece el texto `Kind: Connection`.
2. **Given** una conexión seleccionada, **When** se presentan sus datos identificativos, **Then** aparece una fila por campo, con las etiquetas en una columna y todos los valores iniciando en otra columna alineada.
3. **Given** una conexión seleccionada, **When** se presentan sus etiquetas descriptivas, **Then** ninguna etiqueta controlada termina en `:` y los valores se distinguen visualmente de ellas.
4. **Given** una terminal sin color, **When** se muestra Details, **Then** tipo, etiquetas y valores siguen siendo distinguibles mediante texto, espaciado y marcadores existentes.

---

### User Story 2 - Mantener una apariencia consistente en operaciones (Priority: P2)

Como usuario que confirma, busca ayuda o consulta acciones, quiero encontrar la misma jerarquía de títulos, tipos, etiquetas y valores en cada panel para interpretar rápidamente la información importante.

**Why this priority**: La coherencia entre navegación y operaciones disminuye la carga cognitiva en decisiones de conexión, confirmación y recuperación.

**Independent Test**: Se pueden abrir Actions, Help y una confirmación de conexión, comparando que aplican la misma presentación de etiquetas y valores sin alterar sus controles de teclado o mensajes de seguridad.

**Acceptance Scenarios**:

1. **Given** cualquier superficie TUI existente con contenido estructurado, incluidos formularios y errores, **When** muestra etiquetas y valores controlados, **Then** aplica la misma regla de etiquetas sin `:` y contraste secundario que Details.
2. **Given** cualquier panel visible, **When** se presenta, **Then** muestra una insignia destacada propia que identifica textualmente su tipo mediante la misma convención visual que `Connection` en Details, sin duplicar un título que ya cumple esa función.
3. **Given** un panel con acciones, confirmación o ayuda, **When** se aplica el nuevo estilo, **Then** sus teclas, acción primaria, cancelación, advertencias y errores conservan la prioridad visual existente.
4. **Given** cualquier panel incluido, **When** se desactiva el color, **Then** el panel conserva significado equivalente y no depende solo del color de fondo o primer plano.

---

### User Story 3 - Conservar legibilidad en terminales reducidas (Priority: P3)

Como usuario de terminal, quiero que el estilo común se adapte a paneles estrechos, contenido largo y desplazamiento para que la mejora visual no oculte identidad, errores ni controles de salida.

**Why this priority**: El estilo debe mejorar la lectura sin degradar la operación terminal-first en el tamaño mínimo soportado.

**Independent Test**: Se puede redimensionar Details, formularios, Help y confirmaciones a tamaños soportados, con etiquetas y valores largos, verificando que se conservan contenido prioritario, foco, scroll y controles seguros.

**Acceptance Scenarios**:

1. **Given** un panel cuyo ancho no permite dos columnas completas, **When** presenta filas de identidad, **Then** apila la etiqueta y el valor de cada campo en líneas consecutivas sin ocultar campos ni superponer texto.
2. **Given** contenido largo o desplazable, **When** se aplica el estilo común, **Then** el foco, la selección, el error activo, la acción primaria y los controles de cancelar, volver o salir siguen visibles según las reglas existentes.
3. **Given** una terminal por debajo de 40x12, **When** se muestra la vista de tamaño insuficiente, **Then** conserva su comportamiento actual y no intenta presentar un panel estilizado incompleto.
4. **Given** contenido aportado por el usuario con caracteres Unicode de ancho variable, **When** se muestra junto a etiquetas estilizadas, **Then** permanece dentro de los límites del panel y conserva las reglas existentes de representación segura y truncado.

### Edge Cases

- Una conexión tiene un nombre, ruta, host, usuario o archivo de identidad más largo que el ancho disponible.
- Details identifica raíz, carpeta, conexión, formulario o una operación de seguridad en curso.
- Una confirmación contiene ruta y endpoint completos que deben envolver sin truncarse.
- Actions omite opciones de baja prioridad por falta de espacio.
- Help, errores o confirmaciones requieren desplazamiento vertical.
- Color está desactivado, tiene bajo contraste o la terminal no admite el color preferido.
- La terminal cruza repetidamente los umbrales de disposición ancha, estrecha y tamaño insuficiente.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Details DEBE reemplazar la fila técnica `Kind: Connection` por una única insignia visible `Connection` cuando una conexión esté seleccionada.
- **FR-002**: La insignia DEBE tener un tratamiento visual destacado con color de fondo característico de la aplicación cuando el color esté disponible, y una representación textual equivalente cuando no lo esté.
- **FR-003**: Details DEBE presentar una fila por cada campo identificativo de la conexión, con las etiquetas en una columna y todos los valores iniciando en otra columna alineada mientras el ancho lo permita.
- **FR-004**: Las etiquetas descriptivas controladas en todas las superficies TUI existentes con contenido estructurado, incluidos formularios y errores, NO DEBEN terminar en `:`. Deben seguir siendo distinguibles de los valores mediante contraste visual secundario y semántica textual existente.
- **FR-005**: El estilo de etiquetas, valores, insignias de tipo y espaciado DEBE ser coherente en todas las superficies TUI existentes con contenido estructurado, incluidos Details, Actions, formularios, Help, confirmaciones y errores, salvo que un control de seguridad o recuperación tenga una prioridad superior definida previamente.
- **FR-006**: El cambio visual NO DEBE alterar las acciones disponibles, teclas, orden de foco, propietarios de foco, requisitos de confirmación, mensajes de error, datos mostrados ni reglas de persistencia o red.
- **FR-007**: Ningún contenido secreto, contraseña, frase de paso, clave privada o referencia de credencial DEBE aparecer debido a la nueva presentación.
- **FR-008**: En modo sin color, el tipo, las etiquetas, los valores, el foco, la selección, los errores y la acción primaria DEBEN conservar señales textuales distinguibles y no depender exclusivamente del color.
- **FR-009**: En un ancho insuficiente para mantener las dos columnas alineadas, los grupos de identidad no interactivos DEBEN apilar la etiqueta y el valor de cada campo en líneas consecutivas sin ocultar campos; los valores largos DEBEN conservar las reglas existentes de envoltura o truncado seguro, sin solapamiento ni desbordamiento fuera del panel. Los formularios interactivos DEBEN conservar el bloque compacto de campo y error necesario para seguir siendo operables a 40x12.
- **FR-010**: La adaptación visual DEBE conservar las prioridades responsive existentes: selección/campo/error activo; identidad del objetivo; recuperación/cancelar/volver/salir; acción primaria; contenido restante.
- **FR-011**: Help y Actions DEBEN describir y presentar únicamente los controles existentes; el estilo nuevo NO DEBE anunciar nuevos atajos, interacción de ratón ni activación por clic.
- **FR-012**: La vista de tamaño insuficiente por debajo de 40x12 DEBE permanecer sin cambios funcionales y restaurar el estado de los paneles al volver a un tamaño soportado.
- **FR-013**: La actualización visual de cualquier panel DEBE completarse en la misma actualización visible que su cambio de selección, foco, contenido, error o tamaño.
- **FR-014**: Cada panel DEBE mostrar una insignia destacada propia que identifique textualmente su tipo, como `Connection`, `Actions`, `Help`, `Confirmation` o `Error`, sin usar un prefijo técnico `Kind:`. Cuando el título actual ya identifica ese tipo DEBE adoptar el estilo de insignia en lugar de duplicarse; cuando el título y el contenido identifican tipos distintos, como `Details` y `Connection`, DEBEN mostrarse ambos.

### Key Entities

- **Insignia de tipo**: Identificador visual controlado de un tipo de objetivo o panel.
- **Fila de identidad**: Asociación visible de una etiqueta secundaria y un valor de identidad o configuración no secreta.
- **Tema de panel**: Conjunto común de jerarquía visual para tipos, etiquetas, valores, foco, estado, error y acción primaria.

### Scope Boundaries

**Included**:

- Presentación de tipo, etiquetas, valores y alineación en Details para conexiones.
- Aplicación de la misma jerarquía visual a todas las superficies TUI existentes con contenido estructurado, incluidos Actions, formularios, Help, confirmaciones y errores.
- Presentación equivalente con y sin color.
- Adaptación a contenido largo, desplazamiento y tamaños de terminal ya soportados.

**Excluded**:

- Nuevas acciones, atajos, flujos, paneles, métodos de autenticación o capacidad de ratón.
- Cambios en el contenido de conexión, persistencia, SSH, credenciales o políticas de host trust.
- Localización o cambio del idioma inglés controlado de la interfaz.
- Cambio de tamaño mínimo, geometría exterior, orden de paneles o reglas de scrollbar existentes.
- Personalización por usuario de colores, tipografía o tema.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En 30 presentaciones de Details con conexiones de distintos datos y anchos, el 100 % muestra exactamente una insignia `Connection`, cero apariciones de `Kind: Connection` y filas identificativas sin etiquetas controladas terminadas en `:`.
- **SC-002**: En una matriz de Details, Actions, formulario de conexión, Help, confirmación de conexión, confirmación destructiva y error recuperable, el 100 % aplica la convención común de etiquetas y valores sin cambiar el inventario de controles aplicables.
- **SC-003**: En color y sin color, al menos 19 de 20 participantes identifican correctamente tipo, valor objetivo, campo o acción enfocada y error/acción primaria, cuando aplique, en menos de 5 segundos para cada una de las superficies incluidas a 80x24.
- **SC-004**: En las 20 repeticiones de la secuencia de anchos `40→60→79→80→100→160→80→79→40` a alturas 12 y 24, más `40x12→39x11→40x12`, el 100 % mantiene estado, foco, selección, prioridad de controles y contenido activo visible conforme a las reglas existentes.
- **SC-005**: En 100 casos de etiquetas y valores largos, Unicode de ancho variable y paneles desplazables, el 100 % mantiene la salida dentro de los límites del panel y no muestra secretos ni texto superpuesto.
- **SC-006**: En el catálogo sintético existente de 1.000 conexiones, 100 carpetas y profundidad 10, al menos 19 de 20 actualizaciones de selección, foco, Help, confirmación y resize terminan en 100 ms o menos.

## Assumptions

- El color de fondo destacado usa el tema visual característico ya establecido por Orza y cuenta con una señal textual equivalente cuando el color no está disponible.
- "Resto de pestañas" se interpreta como todas las superficies TUI existentes con contenido estructurado, incluidos Details, Actions, formularios, Help, confirmaciones y errores.
- Las dos columnas de las filas identificativas se mantienen mientras el ancho disponible conserva los valores y controles prioritarios; cuando no caben, cada etiqueta aparece seguida por su valor en la línea siguiente y no se oculta ningún campo.
- `Connection` se mantiene como término controlado en inglés, coherente con el idioma actual de la interfaz.

## Dependencies

- Son normativas las reglas de paneles, prioridades de contenido, foco, teclado, no-color, tamaños 40x12 y 80x24, errores y seguridad definidas por feature 004.
- Son normativas las reglas de scrollbar, contenido activo y composición de paneles de feature 005.
- La feature depende del sistema visual y del inventario de paneles existente; no añade datos persistentes ni servicios externos.
