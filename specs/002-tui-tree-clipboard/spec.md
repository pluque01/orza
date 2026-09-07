# Feature Specification: Árbol contextual y pegado en TUI

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-07-30

**Status**: Approved

**Input**: User description: "El usuario debe poder pegar texto del portapapeles en cualquier entrada
de texto de la interfaz. La vista de las carpetas debe ser la de un árbol de izquierda a derecha. Al
tener el selector en una carpeta, las operaciones se realizarán sobre la carpeta en cuestión. Por
ejemplo, si tengo el selector sobre la carpeta X y pulso n para crear una nueva conexión, esta se
creará en la carpeta X."

## Clarifications

### Session 2026-07-30

- Q: ¿Cómo debe mostrarse exactamente el árbol “de izquierda a derecha”? → A: Como una lista vertical,
  con los hijos debajo de su padre e indentados hacia la derecha.
- Q: ¿Cómo debe obtener la aplicación el texto que el usuario quiere pegar? → A: Mediante el mecanismo
  de pegado del terminal, como `Ctrl+Shift+V` o su equivalente.
- Q: ¿Qué debe ocurrir al pulsar `n` cuando la selección está sobre una conexión? → A: Crear una nueva
  conexión en la carpeta padre de la conexión seleccionada.

### Session 2026-07-31

- Q: ¿Cuál es el estado inicial y el orden del árbol? → A: La raíz aparece primero, expandida y
  seleccionada; las demás carpetas empiezan contraídas. Cada conjunto de hermanos muestra carpetas
  antes que conexiones y ordena cada tipo por nombre con comparación binaria sensible a mayúsculas y,
  en empate, por ID.
- Q: ¿En qué terminales se garantiza el aislamiento del pegado? → A: En terminales interactivos que
  respetan bracketed paste y eventos VT; fuera de esa capacidad la aplicación no accede al portapapeles
  y permite escribir/cancelar, pero no puede distinguir bytes pegados de teclas físicas.
- Q: ¿Qué límites tienen las entradas? → A: Cada campo normal admite como máximo 4.096 runes Unicode y
  cada secreto 4.096 bytes; entrada manual y pegada comparten el límite y un pegado que lo excede se
  rechaza por completo.
- Q: ¿Conectar requiere confirmación? → A: Sí. Antes de abrir una sesión, la TUI muestra la ruta completa
  y el endpoint de la conexión seleccionada y solicita confirmación explícita.

## Definitions and Closed Inventories

- **Controles normales editables**: `Name`, `Folder`, `Host`, `Port`, `User`, `Method` e `Identity file`
  del formulario de conexión, y `Name` del formulario de carpeta. `Remember password` y botones no son
  controles de texto.
- **Prompts secretos de la TUI**: contraseña que el usuario decide recordar al crear/editar, contraseña
  SSH solicitada al conectar y frase de paso de clave privada solicitada al conectar.
- **Corpus mínimo de aislamiento**: payloads que contienen `q`, `n`, `f`, `e`, `m`, `d`, `c`, `r`, `?`,
  Enter, Escape, Tab, Shift-Tab, Ctrl+C, Ctrl+S, Ctrl+R, flechas, Home y End, solos y rodeados por texto.
- **Corpus ASCII/Unicode de equivalencia**: `plain`, `user@example.com`, `22`, `/tmp/id_ed25519`,
  `café`, `e\u0301` (combinante), `用户`, `пароль`, `界🙂`, `👩‍💻` (ZWJ), 1.001 letras `a` y
  4.096 letras `a`. Cada payload se compara con su inserción manual equivalente en inicio, medio y sobre
  selección; valores inválidos para un campo deben producir el mismo error en ambas vías.
- **Superficies de confidencialidad**: vistas TUI, máscaras, confirmaciones, historial de entrada,
  mensajes de estado/error, errores envueltos, diagnósticos, logs propios y de dependencias, tracing,
  snapshots/golden tests y salida stdout/stderr capturada.
- **Terminal soportado para pegado aislado**: TTY/PTY Unix o consola estándar de proceso Windows que
  soporte secuencias VT, bracketed paste y cancelación de lectura. La selección con Shift requiere que
  el terminal reporte modificadores; si no lo hace, el campo sigue admitiendo cursor, escritura,
  pegado y cancelación, pero no selección extendida por Shift. Un prompt secreto Windows conectado a un
  handle de consola distinto de `os.Stdin` no es degradable porque su lectura no puede cancelarse con
  seguridad: DEBE fallar antes de entrar en raw mode con un error accionable y sin contenido secreto.
  En Windows, un reader fallback distinto del reader nativo cancelable de la versión fijada de
  Ultraviolet también DEBE rechazarse antes de capturar o modificar modos.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Navegar y actuar sobre el árbol (Priority: P1)

Como usuario de la interfaz interactiva, quiero ver carpetas y conexiones en un árbol jerárquico y
realizar acciones sobre el elemento seleccionado para comprender siempre dónde se aplicará cada
operación.

**Why this priority**: La selección contextual evita crear, mover, editar o eliminar elementos en una
carpeta equivocada y convierte la jerarquía existente en el flujo principal de navegación.

**Independent Test**: Se puede probar con tres niveles de carpetas y varias conexiones, navegando,
expandiendo y contrayendo nodos, y verificando que cada acción modifica únicamente el nodo seleccionado
o la carpeta que ese nodo determina.

**Acceptance Scenarios**:

1. **Given** un catálogo con carpetas y conexiones anidadas, **When** el usuario abre la TUI, **Then**
   ve una lista vertical donde cada hijo aparece debajo de su padre y cada nivel descendiente está más
   indentado hacia la derecha.
2. **Given** una carpeta contraída, **When** el usuario la expande con el teclado, **Then** sus hijos
   aparecen inmediatamente debajo y a la derecha, sin perder la selección actual.
3. **Given** la carpeta `X` seleccionada, **When** el usuario pulsa `n`, completa y guarda el formulario,
   **Then** la nueva conexión se crea dentro de `X` y queda seleccionada en el árbol.
4. **Given** una conexión seleccionada, **When** el usuario pulsa `n`, **Then** el formulario propone
   como destino la carpeta padre de esa conexión.
5. **Given** una carpeta seleccionada, **When** el usuario elige renombrar, mover o eliminar, **Then**
   la acción identifica y afecta a esa carpeta; una operación destructiva muestra su ruta completa
   antes de confirmar.
6. **Given** una conexión seleccionada, **When** el usuario elige editar, mover, eliminar o conectar,
   **Then** la acción identifica y afecta únicamente a esa conexión.
7. **Given** una mutación confirmada o una recarga del catálogo, **When** el árbol se actualiza, **Then**
   conserva la selección del mismo elemento si aún existe o selecciona su carpeta padre más cercana.
8. **Given** una carpeta o conexión seleccionada, **When** el usuario pulsa `f`, **Then** el formulario
   propone respectivamente esa carpeta o la carpeta padre de la conexión como destino.
9. **Given** una conexión seleccionada, **When** el usuario pulsa `c`, **Then** la TUI muestra su ruta y
   endpoint y solo inicia la sesión tras confirmación explícita.

---

### User Story 2 - Pegar texto en cualquier entrada (Priority: P2)

Como usuario, quiero pegar texto del portapapeles en cualquier campo editable de la TUI para introducir
nombres, hosts, usuarios, rutas y secretos sin volver a escribirlos manualmente.

**Why this priority**: El pegado reduce errores de transcripción y acelera la edición, pero depende de
que los formularios y su contexto de navegación ya sean inequívocos.

**Independent Test**: Se puede probar recorriendo todos los controles editables, pegando texto ASCII,
Unicode y valores largos en distintas posiciones, y comprobando que el contenido se inserta sin
activar acciones ni exponer secretos.

**Acceptance Scenarios**:

1. **Given** cualquier campo de texto con foco, **When** el usuario usa el mecanismo de pegado de su
   terminal, como `Ctrl+Shift+V` o su equivalente, **Then** el texto entregado se inserta en la posición
   del cursor y reemplaza la selección de texto existente, si la hubiera.
2. **Given** texto pegado que contiene letras asociadas a atajos, **When** se procesa el pegado, **Then**
   todo el contenido se trata como texto y no crea, elimina, guarda, cancela ni cambia de pantalla.
3. **Given** una entrada de una sola línea, **When** el usuario pega contenido con saltos de línea,
   **Then** los saltos se eliminan sin enviar el formulario y el resto del contenido permanece en el
   campo para su validación normal.
4. **Given** un campo de contraseña o frase de paso, **When** el usuario pega un secreto, **Then** puede
   usarlo como entrada, pero el valor permanece oculto y no aparece en historial, mensajes ni logs.
5. **Given** un terminal que no entrega contenido de portapapeles, **When** el usuario intenta pegar,
   **Then** el formulario permanece estable y permite seguir escribiendo o cancelar.
6. **Given** un payload cuyo valor candidato excede el límite del campo, **When** se procesa el pegado,
   **Then** se rechaza completo y conserva valor, cursor y selección.

### Edge Cases

- El catálogo está vacío y solo puede seleccionarse la raíz.
- La carpeta seleccionada está contraída, se elimina desde otra instancia o cambia mientras hay un
  formulario abierto.
- Al eliminar el nodo seleccionado ya no existe su carpeta padre inmediata.
- El árbol contiene 1.000 conexiones, 100 carpetas y 10 niveles de profundidad.
- Dos elementos hermanos tienen nombres que solo se diferencian por mayúsculas.
- El terminal es menor de 80x24, no admite color, no reporta Shift o no soporta bracketed paste.
- El texto pegado está vacío, contiene Unicode de ancho variable, supera 1.000 caracteres o excede el
  límite de 4.096 runes/bytes.
- El contenido pegado incluye saltos de línea, tabuladores, caracteres de control o secuencias que
  coinciden con atajos de teclado.
- El usuario pega sobre una selección parcial o en medio de un valor existente.
- El pegado ocurre en una entrada secreta mientras el almacén seguro no está disponible.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La TUI DEBE representar la raíz, las carpetas y las conexiones como un único árbol en
  preorder vertical, con cada hijo debajo de su padre. Entre hermanos DEBE mostrar carpetas antes que
  conexiones y ordenar cada tipo por nombre mediante comparación binaria sensible a mayúsculas y,
  cuando nombre y tipo coincidan, por ID ascendente.
- **FR-002**: Cada nivel descendiente DEBE indentarse exactamente dos celdas de terminal a la derecha de
  su carpeta padre; la jerarquía NO DEBE depender del color.
- **FR-003**: La raíz DEBE aparecer siempre expandida y no puede contraerse; las demás carpetas DEBEN
  empezar contraídas. El usuario DEBE poder expandir y contraer carpetas mediante teclado; una carpeta
  contraída DEBE ocultar todos sus descendientes sin modificar el catálogo.
- **FR-004**: La raíz DEBE ser la selección inicial tanto en catálogos vacíos como poblados. El árbol
  DEBE mantener exactamente un nodo visible seleccionado; si se contrae un ancestro de la selección,
  la selección DEBE pasar a la carpeta contraída antes de ocultar sus descendientes.
- **FR-005**: La raíz, las carpetas contraídas/expandidas, las conexiones y la selección DEBEN usar
  marcadores ASCII distintos (`[/]`, `[+]`/`[-]`, `[ssh]` y `>` respectivamente) además del color. Los
  detalles y toda confirmación destructiva DEBEN mostrar la ruta completa capturada del objetivo.
- **FR-006**: Las acciones de renombrar, mover y eliminar DEBEN aplicarse al ID y revisión capturados del
  nodo seleccionado. Un objetivo se considera cambiado si desaparece o su revisión ya no coincide; la
  operación DEBE rechazarse, conservar el último árbol válido y ofrecer recarga sin usar otra fila.
  Crear DEBE comparar atómicamente revisión y ruta completa capturadas del padre; mover DEBE comparar
  revisión y ruta capturadas de origen y destino dentro de la misma transacción, detectando cambios de
  cualquier ancestro sin rechazar mutaciones no relacionadas.
- **FR-007**: Al pulsar `n` con una carpeta seleccionada, el destino inicial de la nueva conexión DEBE
  ser esa carpeta.
- **FR-008**: Al pulsar `n` con una conexión seleccionada, el destino inicial de la nueva conexión DEBE
  ser la carpeta padre de la conexión.
- **FR-009**: Después de crear un elemento, el árbol DEBE expandir sus ancestros y seleccionar el nuevo
  elemento. Después de mover o renombrar, DEBE conservar su identidad y selección.
- **FR-010**: Después de eliminar el elemento seleccionado, la TUI DEBE seleccionar su carpeta padre
  existente más cercana; si ninguna queda disponible, DEBE seleccionar la raíz.
- **FR-011**: Una recarga DEBE preservar los estados de expansión de las carpetas que todavía existan y
  DEBE eliminar estados asociados a nodos inexistentes. La expansión y selección también DEBEN
  conservarse al volver de formularios o confirmaciones dentro de la misma ejecución; una sesión SSH
  activa finaliza la TUI y, por tanto, no tiene estado de retorno.
- **FR-012**: Los ocho controles normales y los tres tipos de prompt secreto enumerados DEBEN aceptar
  contenido entregado por bracketed paste en terminales soportados; la aplicación NO DEBE leer ni
  administrar directamente el portapapeles del sistema. Sin esa capacidad, DEBE permitir escritura y
  cancelación; la ayuda y documentación DEBEN advertir que no puede prometer aislamiento de bytes
  pegados, ya que el soporte ignorado por el emulador no es detectable de forma fiable. La única
  excepción es el handle Windows alternativo definido en el inventario, que DEBE fallar cerrado.
- **FR-013**: El contenido pegado DEBE insertarse en la posición del cursor y DEBE reemplazar cualquier
  selección activa. Shift+Left/Right/Home/End DEBE crear o extender la selección cuando el terminal
  reporte modificadores; movimiento sin Shift DEBE colapsarla al borde correspondiente.
- **FR-014**: Durante un evento delimitado de bracketed paste, ningún carácter del contenido DEBE
  interpretarse como atajo, navegación, confirmación, envío, cancelación ni salida.
- **FR-015**: Los separadores CR, LF, NEL (`U+0085`), Line Separator (`U+2028`) y Paragraph Separator
  (`U+2029`) pegados DEBEN eliminarse sin enviar el formulario. Si después queda cualquier rune para la
  que `unicode.IsControl` sea verdadera, el payload completo DEBE rechazarse con un mensaje genérico que
  no incluya su contenido y conservar exactamente valor, cursor y selección anteriores.
- **FR-016**: Entrada manual y pegada DEBEN compartir validación y límites: máximo 4.096 runes por control
  normal y 4.096 bytes por secreto. Un payload cuyo valor candidato exceda el límite DEBE rechazarse por
  completo, sin truncamiento; más de 1.000 caracteres siguen siendo válidos mientras no excedan el máximo.
- **FR-017**: Cada secreto DEBE mostrarse como un carácter de máscara por rune, aceptando la filtración
  explícita de longitud y timing propia de un prompt local. El valor NO DEBE aparecer en ninguna
  superficie de confidencialidad enumerada y no DEBE existir historial de secretos.
- **FR-018**: Un intento de pegado vacío, cancelado o no soportado DEBE dejar el campo y la pantalla en
  un estado estable, sin ejecutar acciones laterales.
- **FR-019**: El árbol, los formularios y el pegado DEBEN ser operables exclusivamente mediante teclado
  y sin depender del color. A 80x24 DEBE mostrarse toda la vista normal; por debajo de ese tamaño el modo
  reducido DEBE mantener visibles y operables la fila seleccionada, ruta completa, acción/confirmación
  activa, error y teclas de cancelar/volver/salir, aunque oculte detalles secundarios.
- **FR-020**: Actualizar, expandir o contraer el árbol NO DEBE alterar el contenido de formularios
  abiertos ni aplicar cambios persistentes antes de una confirmación explícita. Si el destino o
  revisión del formulario queda obsoleto, guardar DEBE fallar sin perder el contenido y ofrecer recarga.
- **FR-021**: Al pulsar `f` con raíz o carpeta seleccionada, el destino inicial de la nueva carpeta DEBE
  ser ese nodo; con una conexión seleccionada DEBE ser su carpeta padre. Renombrar, mover o eliminar la
  raíz DEBE dejar catálogo y selección intactos y mostrar un error accionable.
- **FR-022**: Antes de iniciar una conexión SSH, la TUI DEBE capturar ID, revisión, ruta y endpoint,
  mostrar la ruta completa y `host:port` y solicitar confirmación explícita. La solicitud aceptada DEBE
  incluir la revisión esperada y el servicio DEBE volver a compararla antes de cualquier red; un cambio
  concurrente DEBE rechazar la conexión y recargar. Cancelar DEBE volver al mismo nodo sin abrir red ni
  modificar estado persistente.
- **FR-023**: El prompt secreto DEBE cancelar y unir su lector, desactivar bracketed paste, restaurar
  cursor/estilo y modo de terminal y emitir el newline de plataforma antes de devolver en submit,
  cancelación, error de lectura/escritura, cancelación de contexto y señales capturables como SIGINT o
  SIGTERM. El contexto raíz DEBE poseer las señales: tras cleanup, SIGINT/os.Interrupt termina con código
  130 y SIGTERM Unix con 143; ninguna se consume para volver a una TUI activa. `SIGKILL` y pérdida abrupta
  de la máquina quedan fuera de la restauración posible. El prompt NO DEBE restaurar mientras sobreviva
  un goroutine lector; si no puede certificar cancelación antes de raw mode, DEBE fallar cerrado.
- **FR-024**: Los secretos introducidos DEBEN permanecer bajo propiedad del prompt hasta completar su
  cleanup; después pueden transferirse temporalmente al flujo de autenticación. Solo una elección
  explícita de `Remember password` puede enviarlos al almacén seguro del SO; todos los propietarios
  controlados DEBEN limpiar sus buffers tras uso, cancelación o error.
- **FR-025**: Una carga completa del árbol DEBE usar una única revisión de catálogo. Si la revisión cambia
  durante el recorrido, DEBE reintentarse una vez; un segundo cambio DEBE conservar el último snapshot
  válido y mostrar un conflicto recuperable mediante `r`.

### Key Entities

- **Nodo del árbol**: Proyección visible de la raíz, una carpeta o una conexión. Conserva la identidad,
  ruta, profundidad, tipo y estado expandido cuando corresponda.
- **Selección contextual**: Referencia al único nodo activo que determina el objetivo de navegación y
  operaciones; no crea una copia independiente del elemento persistido.
- **Entrada de texto**: Control editable con valor, cursor, selección opcional, visibilidad normal o
  secreta y reglas de validación propias del campo.
- **Evento de pegado**: Entrada de texto tratada como una sola acción, separada de los atajos y comandos
  de navegación.

### Scope Boundaries

**Included**:

- Navegación jerárquica, expansión, selección y acciones contextuales dentro de la TUI.
- Pegado en todos los campos editables, incluidos campos secretos.
- Conservación segura de selección y expansión después de mutaciones y recargas.

**Excluded**:

- Cambios en la semántica o comandos de la CLI.
- Historial, sincronización o administración del portapapeles del sistema.
- Arrastrar y soltar, selección múltiple o acciones masivas sobre varios nodos.
- Navegación obligatoria mediante ratón.
- Modificaciones al modelo persistente de conexiones y carpetas.
- Defensa contra un emulador/TTY local malicioso que envíe un payload ilimitado antes de que Ultraviolet
  delimite el evento; el terminal local es parte del límite de confianza y el límite de FR-016 se aplica
  inmediatamente después de recibir el evento.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En 20 ejecuciones automatizadas del escenario de tres niveles, `n` propone la carpeta
  seleccionada o la carpeta padre de la conexión en 20/20 casos y guardar no requiere corregir destino.
- **SC-002**: Los ocho controles normales y tres tipos de prompt secreto producen en 100 % del corpus
  ASCII/Unicode el mismo valor candidato y validación que una entrada manual equivalente dentro de límite.
- **SC-003**: En un terminal soportado, 100 % del corpus mínimo de aislamiento se procesa como una única
  operación de texto sin ejecutar navegación, creación, edición, movimiento, borrado, conexión,
  confirmación, guardado, cancelación ni salida.
- **SC-004**: En un runner Linux amd64 con al menos 2 CPU lógicas y 4 GiB libres, SQLite local temporal,
  terminal 80x24, sin carga concurrente y tras una ejecución de calentamiento, al menos 19 de 20
  mediciones de cada operación quedan bajo 1 segundo con 1.000 conexiones, 100 carpetas y profundidad
  10. La recarga se mide desde el comando hasta snapshot visible e incluye SQLite; expandir/contraer se
  mide desde el evento de tecla hasta producir la vista.
- **SC-005**: En 100 % de crear carpeta/conexión, renombrar, mover, eliminar y conectar, toda pantalla de
  confirmación o resultado aplicable identifica el ID/ruta capturados del objetivo original. En creación,
  el formulario identifica la carpeta destino y el resultado identifica tanto esa carpeta como el nuevo nodo.
- **SC-006**: En 20/20 ejecuciones a 80x24 con `NO_COLOR`, la secuencia solo-teclado raíz → hijo → nieto →
  raíz mantiene exactamente una fila marcada con `>` y distingue tipos/expansión mediante marcadores ASCII.
- **SC-007**: Para los tres tipos de prompt secreto y en submit, cancelación y fallos inyectados, el
  canario no aparece en ninguna superficie de confidencialidad enumerada en 100 % de los casos.
- **SC-008**: En Linux/macOS amd64/arm64 y Windows amd64/arm64, en 100 % de submit, Escape, Ctrl+C,
  os.Interrupt, SIGINT/SIGTERM donde existan, cancelación de contexto y fallos
  inyectados de lectura/escritura/cleanup, los recursos cancelables terminan y el terminal recupera
  bracketed paste, cursor/estilo y modo capturado antes de la salida 130/143 aplicable; `SIGKILL` queda excluida.

## Assumptions

- La TUI existente ya puede consultar y mutar el catálogo jerárquico mediante servicios compartidos.
- El terminal o emulador es responsable de obtener el contenido del portapapeles y entregarlo mediante
  bracketed paste; la aplicación no lee ni administra directamente el portapapeles del sistema.
- Los controles actuales son de una sola línea; los separadores definidos en FR-015 se eliminan y nunca
  equivalen a pulsar `Enter`.
- Si se pulsa `n` sobre una conexión, crear una conexión hermana en la misma carpeta es el comportamiento
  más coherente con la selección contextual.
- Los estados de expansión y selección son estado de sesión de la TUI y no necesitan persistirse entre
  ejecuciones de la aplicación.
- La raíz permanece siempre seleccionable y puede recibir nuevas conexiones y carpetas.
- La comparación binaria y el desempate por ID coinciden con el orden contractual del catálogo local.
