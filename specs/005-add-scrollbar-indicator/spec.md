# Feature Specification: Indicador de barra de desplazamiento

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-08-21

**Status**: Approved

**Approved**: 2026-08-21, by explicit user request to resolve all blockers before implementation

**Input**: User description: "Mejorar la interfaz para mostrar el contenido que queda fuera de la vista. Sustituir los indicadores `more` con flechas por una barra de desplazamiento rectangular y de tamaño variable, situada junto al borde derecho del contenedor. Su tamaño debe reflejar la relación entre el contenido visible, el contenido total y el tamaño de la ventana. No se requiere control por ratón."

## Definitions

- **Contenedor desplazable**: Región de la interfaz cuyo contenido vertical puede superar las filas disponibles y cuya posición visible puede cambiar mediante los controles de teclado existentes.
- **Pista**: Columna vertical reservada para representar toda la extensión desplazable del contenedor.
- **Indicador**: Rectángulo visible dentro de la pista que representa tanto la proporción de contenido visible como la posición actual dentro del contenido total.
- **Área visible**: Número de filas de contenido disponibles en el contenedor después de títulos, bordes y otros elementos fijos.
- **Posición lógica**: Posición retenida por el contenedor para poder restaurarla después de un redimensionamiento; puede quedar temporalmente fuera del rango visible válido.
- **Primera fila visible**: Posición efectiva, ajustada al rango actual y a la visibilidad del elemento activo, que la barra DEBE representar sin modificar la posición lógica retenida.
- **Inventario cerrado de superficies desplazables**: Tree, Details, formulario de conexión, cuerpo de Help, cuerpo del selector de destino, cuerpo de confirmaciones y cuerpo de errores recuperables. Actions, avisos fijos, controles fijos y la vista de tamaño insuficiente no pertenecen al inventario.
- **Precedencia**: Esta especificación sustituye todas las reglas de `↑ more` y `↓ more` de feature 004; sus reglas de truncado horizontal, foco, teclado, prioridad y geometría exterior continúan vigentes salvo contradicción explícita aquí.

## Clarifications

### Session 2026-08-21

- Q: En paneles con contenido desplazable y controles fijos, ¿qué filas debe abarcar la barra? → A: Solo el cuerpo desplazable.
- Q: Cuando el panel no desplazable de acciones oculte opciones por falta de espacio, ¿cómo debe sustituirse su texto actual `↓ more — ? Help`? → A: Mostrar `Hidden actions — ? Help`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reconocer contenido fuera de la vista (Priority: P1)

Como usuario, quiero ver una barra en el lateral derecho de un contenedor con contenido oculto para saber inmediatamente que puedo desplazarme y cuánto contenido queda por explorar.

**Why this priority**: Sustituye el indicador actual y resuelve la necesidad principal de descubrir contenido que no cabe sin consumir filas con mensajes adicionales.

**Independent Test**: Se puede mostrar un contenedor con más filas de contenido que espacio disponible y verificar que aparece una barra vertical junto a su borde derecho, sin ningún marcador `more`.

**Acceptance Scenarios**:

1. **Given** un contenedor cuyo contenido completo no cabe en el área visible, **When** se presenta la interfaz, **Then** aparece una pista vertical con un indicador rectangular junto al borde derecho del contenedor.
2. **Given** un contenedor cuyo contenido completo cabe en el área visible, **When** se presenta la interfaz, **Then** no aparece la barra de desplazamiento ni se reserva espacio vacío para ella.
3. **Given** un contenedor con contenido oculto, **When** se presenta cualquiera de sus posiciones de desplazamiento, **Then** no aparecen los textos `↑ more` ni `↓ more`.
4. **Given** que el panel Actions omite opciones por falta de espacio, **When** se presenta su aviso de desbordamiento, **Then** muestra exactamente `Hidden actions — ? Help`, Help permite consultar el inventario completo y Actions no presenta una barra.

---

### User Story 2 - Entender posición y proporción (Priority: P2)

Como usuario, quiero que el tamaño y la posición del indicador cambien con el contenido y el área visible para estimar qué parte estoy viendo y dónde me encuentro.

**Why this priority**: Una barra que no refleje proporción y posición solo indicaría desbordamiento, pero no ofrecería la información visual solicitada sobre el contenido pendiente.

**Independent Test**: Se puede recorrer un contenido largo desde el inicio hasta el final y redimensionar la ventana, comprobando que el indicador cambia de posición y tamaño de forma proporcional.

**Acceptance Scenarios**:

1. **Given** un contenedor situado al inicio de su contenido, **When** aparece la barra, **Then** el indicador toca el extremo superior de la pista.
2. **Given** un contenedor situado al final de su contenido, **When** aparece la barra, **Then** el indicador toca el extremo inferior de la pista.
3. **Given** un contenedor situado entre ambos extremos, **When** el usuario se desplaza hacia abajo o arriba, **Then** el indicador avanza o retrocede en la misma dirección sin saltar al extremo contrario.
4. **Given** el mismo contenido y posición, **When** aumenta el número de filas visibles sin llegar a mostrar todo el contenido, **Then** el indicador aumenta de tamaño y mantiene una posición relativa representativa.
5. **Given** el mismo área visible, **When** aumenta la cantidad de contenido total, **Then** el indicador disminuye de tamaño hasta el mínimo visible.

---

### User Story 3 - Mantener navegación y legibilidad (Priority: P3)

Como usuario de teclado, quiero que la nueva barra sea únicamente informativa para conservar los controles de desplazamiento, foco y selección que ya conozco.

**Why this priority**: La mejora visual no debe alterar la operación terminal-first ni introducir una interacción de ratón fuera del alcance solicitado.

**Independent Test**: Se puede navegar y redimensionar todos los contenedores desplazables usando solo teclado, verificando que el elemento activo permanece visible y que cada barra refleja exclusivamente su contenedor.

**Acceptance Scenarios**:

1. **Given** un contenedor desplazable con foco, **When** el usuario utiliza cualquiera de sus controles de teclado existentes, **Then** el contenido se desplaza como antes y la barra se actualiza sin recibir foco.
2. **Given** varios contenedores desplazables visibles, **When** cambia la posición de uno de ellos, **Then** solo cambia el indicador asociado a ese contenedor.
3. **Given** contenido desbordado, **When** la ventana cambia de tamaño, **Then** selección, foco y posición lógica se conservan y la barra se recalcula para las nuevas dimensiones.
4. **Given** una interfaz sin color, **When** aparece la barra, **Then** pista e indicador siguen siendo distinguibles mediante caracteres y contraste textual.
5. **Given** un panel con cuerpo desplazable y controles fijos, **When** su cuerpo tiene contenido oculto, **Then** la pista abarca solo las filas del cuerpo y los controles quedan fuera de ella y permanecen visibles.

### Edge Cases

- El área visible tiene una sola fila y existe más de una fila de contenido.
- El contenido excede el área visible por exactamente una fila.
- La cantidad de contenido es mucho mayor que la altura de la ventana y la proporción calculada sería menor que una celda.
- Un redimensionamiento hace que todo el contenido pase a caber o deje de caber.
- El desplazamiento conservado deja de ser válido después de reducirse el contenido.
- Dos o más contenedores con distintas cantidades de contenido aparecen simultáneamente.
- Un panel centrado con contenido largo aparece sobre otros contenedores desplazables.
- El contenido contiene caracteres Unicode de ancho variable o líneas truncadas horizontalmente.
- La terminal cae por debajo del tamaño mínimo soportado y muestra la vista de tamaño insuficiente.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Cada superficie del inventario cerrado DEBE mostrar una barra de desplazamiento cuando su contenido total supere su área visible.
- **FR-002**: La barra DEBE permanecer oculta cuando todo el contenido del contenedor quepa en su área visible, y la celda que habría ocupado DEBE recuperar el padding derecho normal del contenedor sin reservar una pista vacía.
- **FR-003**: La barra DEBE ocupar una única columna situada en la última celda interior, inmediatamente a la izquierda del borde derecho exterior, sin dibujar encima del borde ni fuera del contenedor. Mientras exista overflow, esa columna DEBE sustituir el padding derecho existente y el texto DEBE conservar su ancho de contenido; cuando el contenido quepa, la celda DEBE volver a ser padding.
- **FR-004**: La pista DEBE abarcar todas las filas desplazables del área visible y distinguirse visualmente del indicador tanto con color como sin él. Los avisos y controles fijos situados antes o después del cuerpo desplazable DEBEN quedar fuera de la pista y permanecer visibles en toda posición.
- **FR-005**: La longitud del indicador DEBE ser el resultado de `filas de pista × filas visibles ÷ filas totales` redondeado al entero más cercano, con los empates exactos redondeados hacia el entero superior, y limitada a un mínimo de una fila y a un máximo igual a la longitud de la pista.
- **FR-006**: La posición superior del indicador DEBE ser `(filas de pista - longitud del indicador) × primera fila visible ÷ (filas totales - filas visibles)` redondeada al entero más cercano, con los empates exactos redondeados hacia el entero superior. Los cálculos DEBEN aceptar todas las dimensiones válidas sin desbordamiento aritmético.
- **FR-007**: En la primera posición efectiva, el indicador DEBE tocar el extremo superior de la pista; en la última, DEBE tocar el extremo inferior. Toda posición intermedia DEBE producir una posición acotada y no decreciente; por cuantización puede compartir celda con un extremo cuando el recorrido disponible sea insuficiente. Una pista de una fila DEBE mostrar un único indicador que comunica overflow, sin pretender distinguir posición o proporción.
- **FR-008**: El indicador DEBE actualizarse en la misma actualización visible que cualquier desplazamiento, cambio de contenido o redimensionamiento que altere su tamaño o posición.
- **FR-009**: La barra DEBE ser informativa: no debe recibir foco, selección ni entrada, y los controles de teclado existentes DEBEN seguir siendo la única forma de desplazar el contenido.
- **FR-010**: Cuando haya varios contenedores desplazables, cada barra DEBE calcularse exclusivamente con el contenido, área visible y posición de su propio contenedor.
- **FR-011**: La fila seleccionada, el campo enfocado, los mensajes prioritarios y los controles de cancelación o salida DEBEN continuar visibles conforme a las reglas existentes; la barra NO DEBE sustituirlos ni cubrirlos.
- **FR-012**: Los indicadores textuales verticales `↑ more` y `↓ more` NO DEBEN aparecer en ninguna parte de la interfaz, incluidos contenedores, Actions y el recorte final del frame. El indicador de truncado horizontal de líneas DEBE conservarse sin cambios.
- **FR-013**: Al aparecer la barra, el contenido DEBE adaptarse al ancho restante sin cruzar la pista; cualquier truncado horizontal resultante DEBE usar el indicador de truncado existente dentro del ancho disponible.
- **FR-014**: Si el contenido o la ventana cambian y la posición previa deja de ser válida, la vista DEBE ajustarse a la posición válida más cercana, mantener visible el elemento activo y reflejar esa posición en la barra.
- **FR-015**: La vista de tamaño insuficiente que sustituye los contenedores normales NO DEBE mostrar barras de desplazamiento; al recuperar un tamaño soportado, las barras aplicables DEBEN reaparecer con el estado conservado.
- **FR-016**: La ayuda visible para cada superficie del inventario DEBE describir únicamente los controles de teclado existentes y NO DEBE presentar interacción mediante ratón.
- **FR-017**: Actions DEBE permanecer no desplazable y sin barra. Cuando omita opciones por falta de espacio, DEBE mostrar exactamente `Hidden actions — ? Help`, y Help DEBE conservar el inventario completo de acciones y controles aplicables conforme a FR-012.
- **FR-018**: Si una superficie dispone de cero filas desplazables o de un ancho interior insuficiente para conservar al menos una celda de contenido además de la barra, NO DEBE crear una barra; DEBE aplicar las prioridades reducidas y la vista de tamaño insuficiente existentes sin ocultar el elemento activo ni los controles de seguridad.

### Key Entities

- **Estado de desplazamiento**: Cantidad total de filas, cantidad de filas visibles y primera fila visible de un contenedor en un momento determinado.
- **Barra de desplazamiento**: Pista e indicador asociados a un único estado de desplazamiento y delimitados por el área interior del contenedor.
- **Contenedor desplazable**: Región que conserva su propio foco o contexto, contenido, área visible y estado de desplazamiento.

### Scope Boundaries

**Included**:

- Sustitución de los indicadores verticales `more` por barras de desplazamiento proporcionales.
- Barras independientes para todos los contenedores que ya admiten desplazamiento vertical.
- Actualización de la barra al navegar, cambiar contenido o redimensionar la ventana.
- Presentación legible con y sin color.
- Conservación de la navegación exclusivamente mediante teclado.

**Excluded**:

- Control de desplazamiento mediante ratón, rueda, clic o arrastre.
- Incorporación de nuevos atajos o cambios en los controles de teclado existentes.
- Desplazamiento horizontal o sustitución del indicador de truncado horizontal.
- Persistencia de la posición de desplazamiento entre ejecuciones.
- Cambios en el contenido, orden, acciones o datos mostrados por cada contenedor.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En una matriz de todos los contenedores desplazables y posiciones de inicio, centro y final, el 100 % de los casos con contenido oculto muestra una barra junto al borde derecho y la interfaz completa contiene cero apariciones de `↑ more` o `↓ more`; todo Actions con opciones omitidas muestra exactamente `Hidden actions — ? Help`.
- **SC-002**: En el 100 % de los casos donde todo el contenido cabe, no aparece barra ni pista vacía y la última celda interior recupera el padding derecho normal.
- **SC-003**: En las 36 combinaciones del producto `filas totales ∈ {5,10,101,1000}`, `filas visibles y altura de pista ∈ {1,2,4}` y primera fila visible `∈ {0, redondeo superior de (filas totales-filas visibles)/2, filas totales-filas visibles}`, tamaño y posición coinciden exactamente con FR-005–FR-007 después del redondeo definido; los casos de pista de una fila cumplen su limitación explícita.
- **SC-004**: En 20 recorridos completos de inicio a fin y de fin a inicio por cada tipo de contenedor, el indicador se mueve siempre en la dirección del contenido y alcanza los extremos correctos sin salir de la pista.
- **SC-005**: En 20 repeticiones de la secuencia de anchos `40→60→79→80→100→160→80→79→40` a alturas 12 y 24, más la transición `40x12→39x11→40x12`, el 100 % conserva selección, foco, posición lógica y contenido activo visible, mantiene cero barras en la vista insuficiente y actualiza, oculta o restaura cada barra antes de la siguiente entrada.
- **SC-006**: Antes de aprobar el merge, un mantenedor ejecuta y registra una revisión con 10 participantes que no hayan contribuido a la feature. En terminal 80x24, con el mismo cuerpo de Help sintético de 30 líneas y color activado/desactivado, cada participante recibe aleatoriamente una vista en inicio, mitad y final y responde exactamente “¿Hay contenido oculto y estás al inicio, en medio o al final?”. El cronómetro comienza al presentar cada frame y termina con la respuesta; al menos 9 de 10 identifican correctamente ambas respuestas en menos de 5 segundos en los seis casos. Solo se registran tiempos, aciertos y resultado agregado, sin datos personales.
- **SC-007**: El 100 % de los flujos de desplazamiento existentes puede completarse solo con teclado después del cambio, sin pasos adicionales y sin que la barra reciba foco.
- **SC-008**: Con el catálogo sintético existente de 1.000 conexiones, 100 carpetas y profundidad 10, al menos 19 de 20 actualizaciones completas de navegación, foco, desplazamiento y resize terminan en 100 ms o menos en el entorno controlado definido por feature 004.

## Assumptions

- La palabra "scrollwheel" de la descripción se interpreta como un indicador visual de barra de desplazamiento con pista e indicador rectangular, no como soporte para la rueda física del ratón.
- La mejora se aplica a todos los contenedores que actualmente admiten desplazamiento vertical y utilizan indicadores `more`, incluidos paneles temporales cuando su contenido excede el área visible.
- Los controles, reglas de foco y posiciones lógicas de desplazamiento existentes se conservan.
- La barra consume una columna interior solo mientras existe contenido oculto; no aumenta el tamaño exterior del contenedor.
- El truncado horizontal continúa comunicándose mediante el indicador existente y queda fuera de este reemplazo.
- La vista de tamaño insuficiente definida por la interfaz actual conserva su comportamiento y no presenta contenedores normales.

## Dependencies

- Son normativos los controles de teclado, propietarios de foco, prioridades de contenido, tamaño mínimo 40x12, matriz responsive, truncado horizontal con `…` y restauración de posición lógica definidos por feature 004, salvo sustitución explícita en esta especificación.
- `contracts/tui-scrollbar.md` fija la representación textual y la geometría observable; `plan.md` y `research.md` describen decisiones de implementación y no pueden ampliar el comportamiento normativo de esta especificación.
- No se requiere red, servidor SSH, credenciales, persistencia nueva ni capacidad de ratón para implementar o aceptar esta feature.
