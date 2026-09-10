# Feature Specification: Selección directa del método de autenticación

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-09-10

**Status**: Draft

**Input**: User description: "Al crear una conexión, mostrar los métodos predefinidos y permitir seleccionarlos directamente en lugar de escribir manualmente `agent`, `key` o `password`."

## Definitions

- **Método de autenticación**: Una de las tres alternativas admitidas para una conexión: `Agent`, `Key` o `Password`.
- **Selector de método**: Control del formulario que presenta simultáneamente los métodos disponibles y conserva exactamente uno como seleccionado.
- **Campo dependiente**: Control que solo resulta aplicable a un método concreto: `Identity file` para `Key` y `Remember password` para `Password`.
- **Selección directa**: Elección entre opciones cerradas mediante controles de navegación; no permite introducir ni pegar texto libre como método.

## Clarifications

### Session 2026-09-10

- Q: ¿Debe el selector predefinido reemplazar la escritura manual tanto al crear como al editar conexiones? → A: Aplicarlo al crear y editar conexiones.
- Q: ¿Debe la navegación pasar de `Password` a `Agent` y viceversa al sobrepasar los extremos? → A: Navegación cíclica entre extremos.
- Q: ¿Deben conservarse los valores de `Identity file` y `Remember password` al cambiar temporalmente a otro método y volver? → A: Conservarlos durante el formulario.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Elegir entre métodos visibles (Priority: P1)

Como usuario que crea o edita una conexión, quiero ver los métodos de autenticación disponibles como opciones predefinidas para elegir uno sin recordar ni escribir su nombre exacto.

**Why this priority**: Es el propósito principal de la feature y elimina errores de escritura en un dato que solo admite un conjunto cerrado de valores.

**Independent Test**: Se pueden abrir los formularios de creación y edición, llegar al selector y comprobar que muestran las tres alternativas, que una está seleccionada y que cada alternativa puede elegirse sin introducir texto.

**Acceptance Scenarios**:

1. **Given** un formulario nuevo de conexión, **When** se presenta el campo de método, **Then** aparecen simultáneamente las opciones `Agent`, `Key` y `Password`, con `Agent` seleccionado inicialmente.
2. **Given** una conexión existente con cualquier método admitido, **When** se abre para editarla, **Then** el selector muestra ese método como la única opción seleccionada.
3. **Given** que el selector de método tiene el foco, **When** el usuario navega hacia la opción anterior o siguiente, **Then** la selección cambia a esa opción y continúa habiendo exactamente un método seleccionado.
4. **Given** que el selector de método tiene el foco, **When** el usuario escribe o pega texto, **Then** el texto no modifica el método ni crea una opción nueva.
5. **Given** cualquier método seleccionado y valores válidos en los demás campos aplicables, **When** el usuario guarda la conexión nueva o editada, **Then** la conexión conserva exactamente el método mostrado como seleccionado.

---

### User Story 2 - Completar solo la configuración aplicable (Priority: P2)

Como usuario, quiero que el formulario muestre los controles adicionales que corresponden al método elegido para completar una configuración válida sin campos irrelevantes.

**Why this priority**: La selección solo es útil si el resto del formulario reacciona de forma inequívoca y evita mezclar configuraciones incompatibles.

**Independent Test**: Se puede seleccionar cada método sucesivamente y comprobar la visibilidad, el foco y la validación de `Identity file` y `Remember password`.

**Acceptance Scenarios**:

1. **Given** `Agent` seleccionado, **When** se presenta el formulario, **Then** no aparecen `Identity file` ni `Remember password`.
2. **Given** cualquier otro método seleccionado, **When** el usuario elige `Key`, **Then** aparece `Identity file` y pasa a formar parte del recorrido normal del formulario.
3. **Given** cualquier otro método seleccionado, **When** el usuario elige `Password`, **Then** aparece `Remember password` sin marcar y pasa a formar parte del recorrido normal del formulario.
4. **Given** el foco en un campo dependiente visible, **When** el usuario cambia a un método para el que ese campo no aplica, **Then** el campo desaparece y el foco vuelve al selector de método.
5. **Given** que el usuario vuelve a un método elegido anteriormente durante el mismo formulario, **When** reaparece su campo dependiente, **Then** se conserva el valor no secreto que había introducido o la elección de recordar contraseña hasta que guarde o cancele.

---

### User Story 3 - Reconocer y operar el selector en cualquier terminal soportada (Priority: P3)

Como usuario de teclado, quiero identificar el método seleccionado y cambiarlo en terminales amplias, reducidas o sin color para completar el formulario de manera predecible.

**Why this priority**: El selector no debe degradar la operación terminal-first ni ocultar información de selección en los tamaños ya soportados.

**Independent Test**: Se puede recorrer el selector con teclado, sin color y en las disposiciones soportadas, verificando que foco, opción seleccionada y controles disponibles permanecen distinguibles.

**Acceptance Scenarios**:

1. **Given** una terminal sin color, **When** se enfoca o cambia el selector, **Then** el foco y el método seleccionado se distinguen mediante marcadores textuales.
2. **Given** una terminal soportada con espacio reducido, **When** el selector aparece en el área visible del formulario, **Then** se pueden consultar las tres opciones sin truncar sus nombres ni ocultar la opción seleccionada.
3. **Given** que cualquier otro campo tiene el foco, **When** el usuario recorre el formulario hacia delante o atrás, **Then** el selector ocupa una única posición estable en el orden de foco.
4. **Given** que el selector tiene el foco, **When** se muestra la ayuda contextual, **Then** indica las teclas disponibles para cambiar de método sin sugerir escritura manual.

### Edge Cases

- La terminal se redimensiona mientras el selector o un campo dependiente tiene el foco.
- El formulario necesita desplazamiento vertical para mostrar el selector y todos sus campos dependientes.
- El usuario navega más allá de la primera o última opción del selector.
- El usuario pega un nombre válido, uno inválido o contenido con caracteres de control mientras el selector tiene el foco.
- El usuario cambia repetidamente entre métodos después de introducir una ruta de identidad o activar el recuerdo de contraseña.
- El formulario se abre o vuelve a mostrarse después de un error de validación o persistencia.
- La terminal cae por debajo del tamaño mínimo soportado y posteriormente recupera un tamaño válido.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Los formularios de creación y edición de conexión DEBEN sustituir la entrada de texto libre del método de autenticación por un selector de opciones predefinidas.
- **FR-002**: El selector DEBE presentar simultáneamente y en este orden `Agent`, `Key` y `Password`; NO DEBE aceptar valores distintos ni permitir editar los nombres de las opciones.
- **FR-003**: Un formulario nuevo DEBE iniciar con `Agent` seleccionado; un formulario de edición DEBE iniciar con el método guardado de la conexión seleccionado. El selector DEBE conservar exactamente una selección en todo momento.
- **FR-004**: El usuario DEBE poder alcanzar el selector y cambiar la selección hacia la opción anterior o siguiente usando solo teclado. El recorrido DEBE ser cíclico: una acción hacia atrás desde `Agent` selecciona `Password`, y una acción hacia delante desde `Password` selecciona `Agent`.
- **FR-005**: La opción seleccionada y el foco del selector DEBEN ser distinguibles entre sí y no depender exclusivamente del color. Cada nombre de opción DEBE permanecer completo en todos los tamaños de terminal soportados.
- **FR-006**: El selector DEBE ocupar una sola posición en el recorrido de foco del formulario; avanzar desde `User` DEBE enfocarlo y avanzar desde él DEBE enfocar el siguiente control aplicable. El recorrido inverso DEBE aplicar el orden opuesto.
- **FR-007**: Con `Agent` seleccionado, el formulario NO DEBE mostrar ni validar `Identity file` ni `Remember password`.
- **FR-008**: Con `Key` seleccionado, el formulario DEBE mostrar `Identity file`, incluirlo inmediatamente después del selector en el recorrido de foco y exigir una ruta no vacía antes de guardar. NO DEBE mostrar ni validar `Remember password`.
- **FR-009**: Con `Password` seleccionado, el formulario DEBE mostrar `Remember password` sin marcar inicialmente e incluirlo inmediatamente después del selector en el recorrido de foco. NO DEBE mostrar ni validar `Identity file`.
- **FR-010**: Cambiar el método DEBE actualizar los campos dependientes en la misma transición visible. Si el cambio oculta el control enfocado, el foco DEBE volver al selector.
- **FR-011**: Los valores de campos dependientes ocultos DEBEN conservarse durante la vida del formulario para que reaparezcan al volver a su método, pero solo los datos aplicables al método seleccionado PUEDEN validarse o guardarse. Cancelar el formulario DEBE descartarlos junto con los demás cambios no guardados.
- **FR-012**: Escribir, borrar o pegar mientras el selector tiene el foco NO DEBE cambiar la selección, alterar los nombres disponibles ni insertar contenido en otro campo.
- **FR-013**: Guardar DEBE persistir exactamente el método seleccionado y únicamente su configuración aplicable. Un fallo de validación o persistencia DEBE conservar selección, valores, foco y mensaje accionable sin sustituir el método por otro.
- **FR-014**: La ayuda del formulario DEBE enumerar las teclas para recorrer el selector y NO DEBE indicar que el método puede escribirse o pegarse.
- **FR-015**: El selector y sus opciones DEBEN respetar el desplazamiento, redimensionamiento, tamaño mínimo, cancelación, salida segura y reglas de prioridad visual ya vigentes para el formulario de conexión.
- **FR-016**: El selector NO DEBE cambiar los tres métodos de autenticación admitidos, su significado, la protección de secretos ni el comportamiento de conexión posterior al guardado.

### Key Entities

- **Opción de autenticación**: Alternativa cerrada con nombre visible y valor de conexión asociado; pertenece al inventario `Agent`, `Key`, `Password`.
- **Selección de autenticación**: Opción única actualmente elegida en el formulario.
- **Estado dependiente del método**: Ruta de identidad o preferencia de recordar contraseña conservada durante la edición del formulario y aplicable solo a su método correspondiente.

### Scope Boundaries

**Included**:

- Sustitución de la escritura manual del método en la creación y edición de conexiones.
- Presentación simultánea y selección por teclado de los tres métodos existentes.
- Actualización de campos dependientes, foco, validación, ayuda y presentación responsive del formulario.
- Conservación de los datos no guardados al alternar temporalmente entre métodos.

**Excluded**:

- Incorporación, eliminación o cambio de significado de métodos de autenticación.
- Selección de múltiples métodos o fallback automático entre ellos.
- Cambios en comandos no interactivos, argumentos de línea de comandos o formato del catálogo.
- Introducción de navegación o selección mediante ratón.
- Cambios en prompts de host trust, contraseña o frase de paso y en el almacenamiento de credenciales.
- Rediseño de otros campos del formulario o de la vista de detalle de conexiones existentes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En el 100 % de 30 aperturas de un formulario nuevo, y de 30 aperturas de edición repartidas por igual entre los tres métodos, las tres opciones aparecen sin interacción previa, `Agent` es la única seleccionada al crear, el método guardado es el único seleccionado al editar y no existe un campo editable para escribir el método.
- **SC-002**: En 20 recorridos completos en ambas direcciones, cada uno de los tres métodos puede seleccionarse solo con teclado, ambos extremos continúan por la opción del extremo opuesto y nunca existen cero o más de una opciones seleccionadas.
- **SC-003**: En las nueve transiciones posibles entre métodos, el 100 % muestra únicamente los campos dependientes aplicables en la misma actualización y mantiene el foco en un control visible.
- **SC-004**: En una matriz de `Agent`, `Key` y `Password` con guardado válido e inválido, el 100 % de los guardados válidos conserva el método seleccionado y cero datos de campos no aplicables se validan o persisten.
- **SC-005**: Con color activado y desactivado, y en terminales de 40x12, 79x24 y 80x24, el 100 % de los casos permite identificar foco y selección, leer completos los tres nombres y completar o cancelar el formulario solo con teclado.
- **SC-006**: En pruebas de escritura y pegado de los tres nombres admitidos, tres nombres no admitidos y contenido con caracteres de control, el 100 % deja sin cambios la selección y el resto del formulario.

## Assumptions

- Los métodos admitidos continúan siendo exactamente `Agent`, `Key` y `Password`, con los comportamientos ya documentados.
- `Agent` continúa siendo el valor inicial de una conexión nueva.
- Las teclas de dirección izquierda y derecha se usan para recorrer opciones porque ya forman parte del contrato visible del formulario; no se requiere una tecla de confirmación separada para aplicar la opción enfocada.
- La creación y la edición siguen usando el formulario de conexión existente y conservan sus reglas de guardado, cancelación, cambios sin guardar y errores recuperables.

## Dependencies

- Son normativas las reglas de foco, formulario de conexión, operación exclusiva, errores, idioma inglés y uso exclusivo de teclado de feature 004, salvo la sustitución explícita de la entrada manual del método.
- Son normativas las reglas de desplazamiento y presentación en tamaños reducidos de feature 005.
- La feature depende del inventario y semántica existentes de los métodos `Agent`, `Key` y `Password`; no modifica autenticación SSH ni almacenamiento de secretos.
