# Feature Specification: Refine Action Legend

**Feature Branch**: `011-refine-action-legend`

**Created**: 2026-09-24

**Status**: Draft

**Input**: User description: "Me gustaría mejorar la apariencia y funcionalidad del panel de Actions. En primer lugar creo que debería de dejar de ser un panel, ya que el usuario nunca interaciona directamente con él. Por ello podría ser simplemente una leyenda. En segundo lugar, tiene demasiada información. Solo deberían aparecer funciones principales, para el resto está el menú de Help. Acciones principales podrían ser: Quit, New connection, New folder y Move Up/Down. Connect también puede aparecer cuando sea necesario. Para el resto podrían estar simplemente en el menú de ayuda. Ponlos en un orden coherente todos, ya sea los de la layenda o el menú de ayuda. En tercer lugar, la presentación de la leyenda debe ser clara: los teclas pueden estar resaltadas con algun color indicativo y la acción que realiza con un color menos llamativo, como el que hemos puesto para las etiquetas del panel de detalles. La separación entre cada pareja de tecla-acción debe ser más clara, ya que ahora mismo solo las separa un espacio y no es suficiente. En cuarto lugar, el menú de ayuda podría tener también una organización de tecla-acción similar a la que le dimos al panel de detalles con su etiqueta-valor. Esto es la alineación en columnas para ambos campos. Las parejas del menú de ayuda pueden tener el mismo color que les des a las parejas de la leyenda."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consultar atajos principales (Priority: P1)

Como usuario del explorador de conexiones, quiero ver una leyenda breve de los atajos más frecuentes sin un panel independiente, para poder reconocer rápidamente las acciones disponibles y conservar espacio para el contenido principal.

**Why this priority**: La leyenda es visible durante la navegación habitual y determina la claridad inmediata de la pantalla principal.

**Independent Test**: Se puede verificar abriendo la pantalla principal y comprobando que se muestra una leyenda compacta, con los atajos principales separados y legibles, sin un contenedor de Actions.

**Acceptance Scenarios**:

1. **Given** la pantalla principal del explorador, **When** el usuario la visualiza, **Then** ve una leyenda, no un panel de Actions, con Quit, New connection, New folder, Move Up y Move Down.
2. **Given** una conexión seleccionada que se puede abrir, **When** el usuario visualiza la leyenda, **Then** también ve Connect.
3. **Given** una carpeta, una lista vacía o una selección que no se puede abrir, **When** el usuario visualiza la leyenda, **Then** Connect no aparece.
4. **Given** la leyenda visible, **When** el usuario identifica sus elementos, **Then** cada tecla se distingue visualmente de su acción y cada pareja tecla-acción queda claramente separada de la siguiente.

---

### User Story 2 - Consultar todos los atajos (Priority: P2)

Como usuario que necesita una acción menos frecuente, quiero consultar un menú de ayuda ordenado y escaneable, para encontrar su atajo sin recargar la pantalla principal.

**Why this priority**: Mantiene las funciones menos frecuentes disponibles sin competir con las operaciones diarias.

**Independent Test**: Se puede verificar abriendo la ayuda y localizando cada acción disponible en una fila con columnas alineadas para tecla y descripción.

**Acceptance Scenarios**:

1. **Given** el menú de ayuda abierto, **When** el usuario revisa los atajos, **Then** cada acción disponible aparece con su tecla y descripción en columnas alineadas.
2. **Given** el menú de ayuda abierto, **When** el usuario compara sus filas con la leyenda principal, **Then** las teclas y descripciones emplean el mismo tratamiento visual que sus equivalentes de la leyenda.
3. **Given** el menú de ayuda abierto, **When** el usuario busca una acción secundaria, **Then** la encuentra fuera de la leyenda principal y agrupada en un orden coherente con las acciones relacionadas.

---

### Edge Cases

- Cuando no hay conexiones ni carpetas, la leyenda mantiene Quit, New connection, New folder y Move Up/Move Down sin mostrar Connect.
- Cuando la selección cambia entre elementos conectables y no conectables, la presencia de Connect se actualiza para reflejar únicamente la acción disponible.
- Cuando el terminal no muestra color, las columnas, los textos y la separación siguen permitiendo distinguir cada tecla, acción y pareja.
- Cuando el ancho disponible obliga a compactar la presentación, ninguna pareja de tecla-acción se confunde con otra y la ayuda conserva sus columnas legibles.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La pantalla principal DEBE sustituir el panel de Actions por una leyenda de atajos que no se presente como un panel independiente.
- **FR-002**: La leyenda DEBE mostrar únicamente Quit, New connection, New folder, Move Up y Move Down como acciones principales.
- **FR-003**: La leyenda DEBE mostrar Connect únicamente cuando la selección actual admita iniciar una conexión.
- **FR-004**: La leyenda NO DEBE mostrar acciones secundarias; todas las acciones disponibles que no formen parte de la leyenda DEBEN permanecer consultables desde Help.
- **FR-005**: La leyenda DEBE ordenar las acciones por navegación, apertura de conexión cuando aplique, creación y salida.
- **FR-006**: Cada entrada de la leyenda DEBE mostrar una tecla o combinación de teclas resaltada y una descripción con un tratamiento visual más discreto, coherente con las etiquetas del panel de detalles.
- **FR-007**: La leyenda DEBE separar claramente cada pareja tecla-acción mediante espaciado o delimitación visual suficiente para que no se lea como una única secuencia continua.
- **FR-008**: Help DEBE incluir todas las acciones disponibles, incluidas las que se muestran en la leyenda, ordenadas en grupos coherentes de navegación, conexión, gestión y aplicación.
- **FR-009**: Help DEBE presentar cada atajo como una pareja de columnas alineadas: tecla o combinación de teclas y descripción de la acción.
- **FR-010**: Help DEBE aplicar a las teclas y descripciones el mismo tratamiento visual utilizado por las parejas correspondientes de la leyenda.
- **FR-011**: La información de teclas, acciones, orden y separación DEBE seguir siendo comprensible cuando el color no esté disponible.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En la pantalla principal, el 100% de las acciones mostradas en la leyenda pertenece al conjunto de cinco acciones principales, más Connect solo cuando está disponible.
- **SC-002**: En una comprobación visual de la pantalla principal, un evaluador puede identificar los cinco atajos principales y sus acciones correctas en 10 segundos o menos.
- **SC-003**: El 100% de las acciones disponibles puede localizarse en Help, y cada una presenta una tecla y una descripción en columnas alineadas.
- **SC-004**: En una revisión con color y sin color, el 100% de las parejas tecla-acción de la leyenda y Help se distinguen de las parejas adyacentes sin ambigüedad.
- **SC-005**: Al alternar la selección entre un elemento conectable y uno no conectable, Connect aparece o desaparece de la leyenda en el siguiente estado visible de la pantalla.

## Assumptions

- Los atajos existentes y el comportamiento de las acciones no cambian; esta característica solo reorganiza y presenta su ayuda visible.
- Move Up y Move Down se consideran acciones principales de navegación incluso cuando no producen desplazamiento por estar en un límite de la lista.
- Help ya es el punto de consulta accesible para acciones secundarias y seguirá incluyendo las acciones principales para ofrecer una referencia completa.
- La agrupación de Help seguirá el orden navegación, conexión, gestión y aplicación, manteniendo el orden interno estable de cada grupo.
- La presentación se adaptará al espacio disponible sin depender exclusivamente del color para transmitir significado.
