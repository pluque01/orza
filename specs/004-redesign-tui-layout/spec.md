# Feature Specification: Rediseño multipanel de la TUI

> **Superseded overflow contract**: Feature 005 replaces every vertical `↑ more`/`↓ more` rule in this document with proportional right-edge scrollbars and replaces the Actions omission cue with `Hidden actions — ? Help`. Other requirements remain historical and normative where feature 005 does not override them.

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-08-07

**Status**: Approved

**Approved**: 2026-08-12, by explicit user request to resolve implementation blockers

**Input**: User description: "Quiero una interfaz más clara y atractiva. A la izquierda debe estar el
árbol de conexiones y carpetas; a la derecha, información del elemento bajo el cursor. Si el elemento
activo es una carpeta, se muestran sus conexiones directas, sin incluir carpetas contenidas. En la
parte inferior deben aparecer las opciones disponibles y sus teclas. Las acciones que editan campos de
una conexión deben trasladar el foco a esos campos del panel derecho. Las demás acciones, como crear
una carpeta, confirmar una eliminación o mostrar ayuda, deben abrir un panel centrado. En terminales
pequeñas, el panel derecho debe situarse debajo del árbol. Puede usarse nexus-tui-builder para ayudar a
definir la interfaz."

## Clarifications

### Session 2026-08-07

- Q: Durante la navegación normal, ¿cómo debe acceder el usuario al panel de detalle cuando su contenido
  necesita desplazamiento? → A: `Tab` y `Shift+Tab` alternan el foco entre árbol y detalle; el detalle es
  de solo lectura y permite desplazamiento.
- Q: ¿Qué debe ocurrir si el usuario intenta salir mientras hay cambios de conexión sin guardar? → A: Un
  panel centrado ofrece Guardar, Descartar y Cancelar; solo Guardar con éxito o Descartar permiten salir.
- Q: ¿Cómo debe mostrarse un error recuperable cuando ya hay un panel centrado abierto? → A: El error se
  muestra dentro del panel actual; no se apilan ni sustituyen paneles centrados.
- Q: ¿Cuál debe ser el tamaño mínimo garantizado para el modo reducido y qué debe ocurrir por debajo de él?
  → A: El mínimo reducido es 40x12; por debajo se conserva todo el estado y solo se muestra aviso de resize,
  ayuda y salida segura.
- Q: ¿Qué límite de tiempo debe cumplir la interfaz para navegación local y para recargar el catálogo? → A:
  Navegación, foco, scroll y resize completan en ≤100 ms; recargar el catálogo completa en ≤1 s; ambos en
  19/20 mediciones controladas.
- Q: ¿Cómo debe acotarse la evidencia “20/20” y “100 %” para acciones, formularios, teclado y
  redimensionamiento? → A: Cada flujo principal se repite 20 veces; resize usa siempre la secuencia
  40→60→79→80→100→160→80→79→40 en cada estado aplicable.
- Q: ¿Cómo debe estandarizarse el estudio de claridad visual con 10 usuarios? → A: Se usa catálogo fijo,
  terminal 100x30, cronómetro desde la primera vista, cuatro identificaciones y una pregunta exacta de 1–5.
- Q: ¿Qué inventario de acciones debe ser normativo para raíz, carpeta y conexión en navegación normal? → A:
  Raíz usa `n/f/r/?/q`; carpeta `n/f/e/m/d/r/?/q`; conexión `c/n/f/e/m/d/r/?/q`.
- Q: ¿Cómo debe indicar cada región que existe contenido oculto por scroll o truncamiento? → A: Usa
  `↑ more` para contenido anterior, `↓ more` para posterior y `…` dentro de toda línea truncada.
- Q: ¿Qué debe ocurrir cuando el nodo seleccionado desaparece o cambia desde otra instancia durante
  navegación, formulario o panel centrado? → A: Navegación selecciona el padre existente más cercano;
  formulario/panel conserva datos y objetivo capturado y ofrece Recargar, Volver y Cancelar.
- Q: ¿Cómo debe presentarse una operación asíncrona en curso, como carga inicial, recarga, guardado o inicio
  SSH? → A: Conserva el último contenido, Actions muestra `Loading: <action> — <target>` y bloquea mutaciones
  salvo Cancelar, Ayuda y Salir.
- Q: ¿Qué interacciones forman el inventario cerrado de paneles centrados? → A: Crear/editar carpeta, mover,
  eliminar, conectar, cambios sin guardar, ayuda, error operativo y fallo SSH.
- Q: ¿Qué idioma debe usar la nueva interfaz para títulos, acciones, ayuda y errores? → A: Inglés únicamente;
  la localización queda fuera de alcance.

## Definitions and Closed Inventories

- **Panel de árbol**: región principal que presenta la jerarquía existente de raíz, carpetas y
  conexiones y conserva una única fila seleccionada.
- **Panel de detalle**: región contextual que presenta información de la selección o los campos
  editables de una conexión. En disposición amplia aparece a la derecha del árbol y en disposición
  estrecha aparece debajo.
- **Panel de acciones**: región inferior que enumera las acciones disponibles para el contexto actual
  junto con sus teclas.
- **Panel centrado**: superficie temporal superpuesta al contenido normal que posee el foco hasta que
  se confirma o cancela. Su inventario cerrado es: crear carpeta, editar carpeta, seleccionar destino para
  mover, confirmar eliminación de conexión/carpeta, confirmar conexión, decidir cambios sin guardar, ayuda,
  error operativo recuperable y fallo SSH. Crear/editar conexión nunca pertenece a este inventario.
- **Disposición amplia**: terminal con 80 columnas o más; árbol y detalle se muestran lado a lado y el
  panel de acciones ocupa la parte inferior.
- **Disposición estrecha**: terminal con menos de 80 columnas; árbol y detalle se apilan en ese orden y
  el panel de acciones permanece al final de la pantalla.
- **Vista de tamaño insuficiente**: fallback para terminales menores de 40x12 que conserva todo el estado,
  solicita redimensionar y solo admite ayuda y salida segura.
- **Conexión directa**: conexión cuyo padre inmediato es la carpeta seleccionada. No incluye conexiones
  de carpetas descendientes.
- **Estados de interacción**: navegación del árbol, navegación de detalle, edición de conexión y panel
  centrado. Exactamente uno posee el foco de teclado en cada momento; la navegación de detalle es de
  solo lectura. Los prompts especializados de trust/password/passphrase preemptan temporalmente la entrada
  sin sustituir ese propietario de foco y restauran el mismo estado al cerrarse.
- **Inventario normativo de acciones en navegación normal**: raíz ofrece `n` nueva conexión, `f` nueva
  carpeta, `r` recargar, `?` ayuda y `q`/`Ctrl+C` salir; carpeta añade `e` editar, `m` mover y `d` eliminar;
  conexión añade `c` conectar y ofrece además `n`, `f`, `e`, `m`, `d`, `r`, `?` y `q`/`Ctrl+C`. `n` y `f`
  sobre conexión usan su carpeta padre conforme a la semántica existente. Movimiento, expansión, foco y
  scroll son controles de navegación y se muestran junto a las acciones cuando corresponda.
- **Indicadores de contenido adicional**: `↑ more` ocupa la primera fila de contenido cuando existe
  contenido anterior, `↓ more` ocupa la última cuando existe contenido posterior y `…` aparece dentro de
  toda línea truncada. Los indicadores son textuales, no reciben foco y no sustituyen selección, campo,
  error ni control activo.
- **Estado de operación**: propiedad única de una carga, recarga, guardado o inicio SSH en curso. Conserva
  el último contenido válido y presenta `Loading: <action> — <target>` en Actions; en la carga inicial usa
  el shell vacío y `catalog` como target.
- **Idioma de interfaz**: todo texto controlado por la aplicación en títulos, etiquetas, acciones, ayuda,
  estado, validación y errores usa inglés. Nombres/rutas del usuario no se traducen.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Explorar con contexto visible (Priority: P1)

Como usuario, quiero navegar por el árbol y ver inmediatamente información útil del elemento
seleccionado para comprender su ubicación y contenido sin abrir pantallas adicionales.

**Why this priority**: La navegación y la comprensión del objetivo activo son la base de todas las
demás operaciones y reducen acciones sobre elementos equivocados.

**Independent Test**: Se puede probar con un catálogo que contenga carpetas anidadas y conexiones,
moviendo el cursor por cada tipo de nodo y verificando que el panel de detalle cambia al mismo tiempo y
solo presenta información del objetivo seleccionado.

**Acceptance Scenarios**:

1. **Given** una conexión seleccionada en el árbol, **When** se muestra la vista normal, **Then** el
   panel de detalle identifica la conexión y presenta su ruta, destino y configuración no secreta.
2. **Given** una carpeta con conexiones y subcarpetas, **When** el cursor selecciona la carpeta, **Then**
   el panel de detalle muestra sus datos y únicamente sus conexiones hijas directas.
3. **Given** una carpeta sin conexiones directas, **When** queda seleccionada, **Then** el panel de
   detalle muestra un estado vacío explícito en lugar de contenido anterior o espacio ambiguo.
4. **Given** cualquier nodo seleccionado, **When** el usuario mueve el cursor a otro nodo, **Then** el
   encabezado, la ruta y el contenido contextual pasan a identificar el nuevo nodo sin modificarlo.
5. **Given** un nodo seleccionado que desaparece al recargar, **When** se aplica el nuevo snapshot, **Then**
   el árbol selecciona su padre existente más cercano o la raíz y detalle y acciones identifican ese fallback.

---

### User Story 2 - Descubrir y ejecutar acciones contextuales (Priority: P2)

Como usuario, quiero ver en la parte inferior solo las acciones aplicables a mi contexto y las teclas
que las ejecutan para operar la aplicación sin memorizar atajos ni abrir ayuda constantemente.

**Why this priority**: Los controles visibles hacen descubribles las funciones existentes y previenen
que un atajo parezca actuar sobre un objetivo distinto del seleccionado.

**Independent Test**: Se puede probar seleccionando la raíz, una carpeta y una conexión, y entrando en
cada estado interactivo para comprobar que las acciones y teclas visibles coinciden exactamente con
las que se pueden ejecutar.

**Acceptance Scenarios**:

1. **Given** la raíz, una carpeta o una conexión seleccionada, **When** cambia la selección, **Then** el
   panel inferior actualiza sus acciones y teclas para ese tipo de objetivo.
2. **Given** una acción no disponible para el contexto actual, **When** se muestra el panel inferior,
   **Then** la acción no se presenta como ejecutable y su tecla no altera el catálogo.
3. **Given** un formulario de conexión o un panel centrado activo, **When** se consulta el panel de
   acciones, **Then** muestra los controles de ese estado, incluidos guardar o confirmar cuando
   corresponda, cancelar y la navegación disponible.
4. **Given** una acción destructiva o que inicia una conexión, **When** el usuario la invoca, **Then** la
   confirmación identifica inequívocamente la ruta y el destino capturados antes de continuar.
5. **Given** una operación asíncrona en curso, **When** se muestra Actions, **Then** identifica acción y
   objetivo y solo ofrece Cancelar, Ayuda y Salir hasta que termine limpiamente; durante `ssh_start`, un prompt
   especializado de trust/password/passphrase preempta temporalmente ese inventario con sus controles seguros.

---

### User Story 3 - Editar conexiones en contexto (Priority: P3)

Como usuario, quiero crear o editar una conexión directamente en el panel de detalle para conservar la
referencia visual del árbol y completar sus campos con un recorrido de foco predecible.

**Why this priority**: Mantener el formulario junto a la jerarquía reduce cambios de contexto y deja
claro qué conexión o carpeta destino se está modificando.

**Independent Test**: Se puede probar iniciando creación y edición de conexión, recorriendo todos los
campos solo con teclado, cancelando y guardando, y verificando el foco, el objetivo y el estado final.

**Acceptance Scenarios**:

1. **Given** una conexión seleccionada, **When** el usuario elige editarla, **Then** el panel de detalle se
   convierte en formulario, el primer campo editable recibe el foco y el árbol sigue mostrando el
   objetivo de la edición.
2. **Given** una carpeta seleccionada, **When** el usuario elige crear una conexión, **Then** el panel de
   detalle muestra el formulario con esa carpeta como destino y traslada el foco al primer campo.
3. **Given** un formulario de conexión activo, **When** el usuario recorre sus controles, **Then** el
   foco avanza y retrocede en un orden estable y cada campo enfocado se distingue sin depender del color.
4. **Given** cambios sin guardar, **When** el usuario cancela, **Then** vuelve a la navegación con el
   mismo nodo seleccionado y no persiste ningún cambio.
5. **Given** valores válidos, **When** el usuario guarda, **Then** el árbol y el panel de detalle muestran
   la conexión resultante seleccionada; con valores inválidos, el formulario conserva los datos y
   señala los campos que deben corregirse.
6. **Given** cambios de conexión sin guardar, **When** el usuario intenta salir, **Then** un panel centrado
   permite guardar, descartar o cancelar; un error al guardar conserva el formulario y sus valores.

---

### User Story 4 - Resolver acciones en paneles centrados (Priority: P4)

Como usuario, quiero que las operaciones breves o decisivas aparezcan en un panel centrado para
concentrarme en la decisión actual sin perder el contexto visible detrás.

**Why this priority**: Una superficie temporal separa decisiones puntuales de la edición extensa de
conexiones y establece una prioridad de foco inequívoca.

**Independent Test**: Se puede probar abriendo creación y edición de carpeta, selección de destino,
confirmaciones y ayuda, verificando aislamiento del foco, cancelación y restauración de la selección.

**Acceptance Scenarios**:

1. **Given** cualquier contexto válido, **When** el usuario abre la ayuda, **Then** aparece un panel
   centrado con las teclas aplicables y el contenido subyacente no responde hasta cerrarlo.
2. **Given** una carpeta destino válida, **When** el usuario crea una carpeta, **Then** introduce el
   nombre en un panel centrado que identifica el destino y puede guardar o cancelar.
3. **Given** un elemento eliminable, **When** el usuario solicita eliminarlo, **Then** un panel centrado
   identifica su ruta, describe el efecto y exige confirmación explícita.
4. **Given** un panel centrado abierto, **When** el usuario cancela, **Then** se cierra sin aplicar la
   acción y el foco vuelve al estado y elemento que lo abrió.
5. **Given** un panel centrado abierto, **When** su acción produce un error recuperable, **Then** el mismo
   panel conserva foco, datos y objetivo y muestra el error con opciones para corregir, reintentar o
   cancelar.
6. **Given** una carpeta seleccionada, **When** el usuario edita su nombre y guarda, **Then** el panel identifica
   la carpeta capturada, aplica el cambio y selecciona la carpeta resultante; cancelar restaura el opener.
7. **Given** una carpeta o conexión movible, **When** el usuario selecciona un destino válido y confirma,
   **Then** el panel aplica el movimiento al ID capturado y selecciona el resultado; cancelar no muta catálogo.
8. **Given** una conexión capturada, **When** el usuario confirma conectar, **Then** se inicia SSH solo para ese
   ID/revisión; cancelar inicia cero red y restaura el opener.
9. **Given** cambios de conexión sin guardar, **When** aparece la decisión de salida, **Then** Save persiste y
   sale tras cleanup, Discard sale sin persistir y Cancel restaura exactamente el formulario.
10. **Given** un error operativo recuperable, **When** el usuario elige Retry, Reload o Back según los controles
    visibles, **Then** se conserva el target capturado y solo se ejecuta la transición elegida; cerrar restaura
    el opener sin efectos adicionales.
11. **Given** un fallo SSH, **When** el usuario muestra Detail, reintenta, edita o vuelve mediante un control
    disponible, **Then** el único panel conserva el intento capturado y aplica solo esa transición; cerrar no
    inicia red.

---

### User Story 5 - Usar la interfaz en distintos tamaños (Priority: P5)

Como usuario, quiero que la interfaz reorganice sus paneles al redimensionar la terminal para mantener
visibles la selección, su información y las acciones esenciales sin solapamientos.

**Why this priority**: La aplicación se usa en terminales y sesiones remotas de tamaños variables, y
una composición rígida puede ocultar controles necesarios para recuperar o cancelar una operación.

**Independent Test**: Se puede probar redimensionando por encima y por debajo de 80 columnas durante
navegación, edición y un panel centrado, y comprobando orden, foco, scroll y controles esenciales.

**Acceptance Scenarios**:

1. **Given** una terminal de 80 columnas o más, **When** se muestra la vista normal, **Then** el árbol
   aparece a la izquierda, el detalle a la derecha y las acciones debajo de ambos.
2. **Given** una terminal de menos de 80 columnas, **When** se muestra la vista normal, **Then** el árbol
   aparece primero, el detalle inmediatamente debajo y las acciones en la parte inferior.
3. **Given** una selección o formulario activo, **When** la terminal cruza el umbral de disposición,
   **Then** conserva el mismo objetivo, datos introducidos, posición de foco y acción activa.
4. **Given** contenido mayor que el espacio disponible, **When** el usuario navega, **Then** puede
   desplazar la región activa, ve `↑ more` o `↓ more` según la dirección con contenido oculto y siempre ve
   la selección o campo enfocado y los controles para cancelar.
5. **Given** una terminal menor de 40x12, **When** se muestra la TUI, **Then** conserva el estado actual y
   presenta únicamente el tamaño mínimo requerido, ayuda y salida segura hasta que vuelva a caber.

### Edge Cases

- El catálogo está vacío y solo existe la raíz.
- Una carpeta contiene subcarpetas pero ninguna conexión directa.
- Una carpeta contiene tantas conexiones directas que el panel de detalle necesita desplazamiento.
- Nombres, rutas, usuarios o destinos superan el ancho del panel o contienen Unicode de ancho variable.
- El árbol tiene más profundidad que el ancho disponible y la fila seleccionada no cabe completa.
- La terminal cambia repetidamente entre 79 y 80 columnas durante una edición o confirmación.
- La terminal está entre 40x12 y 79x23 y requiere priorización, o cae por debajo de 40x12 y activa la vista
  de tamaño insuficiente sin perder selección, formulario, foco, panel ni operación pendiente.
- La terminal no admite color o usa una combinación de colores de bajo contraste.
- Un terminal soportado no distingue Shift+Tab u otro modificador requerido; la UI ofrece y muestra una
  alternativa `F2 Previous` sin modificador, o rechaza el terminal al inicio con un mensaje accionable si no
  existe alternativa.
- El elemento seleccionado desaparece durante navegación: se selecciona su padre existente más cercano o
  la raíz y no se conserva detalle ni acción ejecutable del nodo ausente.
- El objetivo desaparece o cambia de revisión durante formulario o panel: la interacción conserva target,
  valores y foco, bloquea persistencia/red y muestra Recargar, Volver y Cancelar dentro de la misma superficie.
- Un error de validación, persistencia o conexión aparece con un panel centrado ya abierto; el error se
  integra en ese mismo panel sin ocultar sus datos, objetivo ni controles de recuperación.
- Una carga, recarga, guardado o inicio SSH tarda, se cancela o devuelve un resultado después de que otra
  operación lo haya sustituido; solo su propietario puede actualizar la superficie activa.
- El usuario intenta salir con cambios de conexión sin guardar.
- La ayuda o una confirmación contiene más líneas que la altura disponible.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La vista normal DEBE contener tres regiones tituladas y visualmente diferenciables: árbol,
  detalle contextual y acciones. La composición NO DEBE depender del color para comunicar selección,
  foco, tipo de nodo o límites entre regiones. El contenido requerido y los controles de Tree, Details,
  connection form, Actions y cada tipo de panel DEBEN ser exactamente los enumerados en las matrices del
  contrato TUI; contenido secundario adicional NO DEBE alterar target, prioridad o inventario de controles.
- **FR-002**: Después de cargar un snapshot, la TUI DEBE mantener exactamente un nodo seleccionado y exactamente
  un propietario de foco de aplicación. Durante carga inicial, un root sintético seleccionado y Tree enfocado
  satisfacen el estado hasta aceptar el snapshot. Toda transición DEBE conservar o transferir foco explícita y
  visiblemente. Trust/password/passphrase pueden preemptar entrada como security owner sin sustituir el
  propietario de aplicación, que DEBE reaparecer intacto al cerrar el prompt.
- **FR-003**: Con 80 columnas o más, el árbol DEBE aparecer a la izquierda y el detalle a la derecha, con
  alturas alineadas y el panel de acciones debajo de ambos. Cada región DEBE ajustar su contenido sin
  dibujar fuera de sus límites ni solaparse con otra. Tras reservar un gutter de una celda, Tree DEBE recibir
  40 % del ancho base redondeado hacia abajo y Details el resto. Actions DEBE medir exactamente 5 filas
  exteriores en todo modo no-undersized; Tree/Details usan las filas restantes.
- **FR-004**: Con menos de 80 columnas, el árbol DEBE aparecer encima del detalle y el panel de acciones
  debajo de ambos. La disposición DEBE recalcularse en cada redimensionamiento sin perder selección,
  expansión, datos de formulario, foco ni panel centrado activo. Tras reservar Actions según FR-003, Tree y
  Details DEBEN dividir por igual las filas restantes; si sobra una fila, pertenece a la región enfocada.
- **FR-005**: En terminales de 80x24 o mayores, la vista normal DEBE presentar simultáneamente la fila
  seleccionada, la identificación del detalle y todas las acciones contextuales. Por debajo de 80x24,
  pero desde 40x12 inclusive, DEBE priorizar esos elementos, los errores activos y cancelar/volver/salir
  sobre información secundaria. Por debajo de 40x12 DEBE sustituir las regiones por una vista de tamaño
  insuficiente que muestre el mínimo requerido y solo permita abrir ayuda, solicitar resize o salir; DEBE
  conservar selección, expansión, formulario, valores, foco, panel y operación pendiente. La salida desde
  esa vista DEBE conservar las protecciones de confirmación y cambios sin guardar. En cualquier modo reducido,
  cada región DEBE priorizar en este orden: selección/campo/error activo; identidad del target; recovery,
  Cancel, Back y Quit; acción primaria; datos/acciones requeridos restantes; navegación; contenido secundario.
  Solo categorías posteriores pueden omitirse, siempre con indicador de overflow cuando aplique.
  En Actions, todos los controles recovery/safety visibles DEBEN caber antes de mostrar un indicador; el marker
  solo puede reemplazar una fila de categorías posteriores.
- **FR-006**: Cuando una conexión esté seleccionada, el detalle DEBE mostrar como mínimo nombre, ruta
  completa, `host:port`, usuario, método de autenticación y referencia de identidad cuando corresponda.
  NO DEBE mostrar contraseñas, frases de paso, claves privadas ni otros secretos.
- **FR-007**: Cuando una carpeta o la raíz esté seleccionada, el detalle DEBE mostrar su nombre, ruta
  completa, cantidad de conexiones directas y una lista de esas conexiones con nombre y destino. NO
  DEBE listar carpetas hijas ni conexiones pertenecientes a carpetas descendientes. La lista DEBE usar orden
  canónico por nombre y, en empate, ID, igual que el catálogo/árbol.
- **FR-008**: Si una carpeta no tiene conexiones directas, el detalle DEBE mostrar un estado vacío que
  identifique la carpeta y explique que no contiene conexiones directas.
- **FR-009**: Un cambio de selección DEBE actualizar el detalle y las acciones en la misma transición
  visible. La TUI NO DEBE presentar datos o acciones ejecutables de una selección anterior.
  El primer frame posterior al evento DEBE contener fila, Details y Actions del mismo node ID; no puede existir
  un frame intermedio vacío o que combine IDs distintos.
- **FR-010**: El panel inferior DEBE asociar cada acción disponible con su tecla y adaptarse al tipo de
  nodo y estado interactivo actual. Las acciones no válidas NO DEBEN ejecutarse mediante atajos ocultos;
  si una tecla compartida no aplica, el estado debe permanecer intacto.
- **FR-011**: En navegación normal, raíz DEBE ofrecer exactamente `n/f/r/?/q`; carpeta DEBE ofrecer
  exactamente `n/f/e/m/d/r/?/q`; conexión DEBE ofrecer exactamente `c/n/f/e/m/d/r/?/q`. `Ctrl+C` DEBE
  equivaler a `q`. Footer, ayuda y dispatch DEBEN derivar de este mismo inventario; una acción no listada
  para el tipo de nodo NO DEBE mostrarse ni producir efecto. Los controles de movimiento, expansión,
  foco y scroll se añaden según región enfocada sin alterar el inventario de acciones del nodo.
- **FR-012**: Crear o editar una conexión DEBE convertir el panel de detalle en un formulario y trasladar
  el foco al primer campo editable. El árbol DEBE permanecer visible e identificar la conexión editada o
  la carpeta destino de una conexión nueva.
- **FR-013**: El formulario de conexión DEBE permitir recorrer todos sus campos y controles hacia delante
  y atrás solo con teclado. Campo enfocado, valor inválido y control de guardar DEBEN distinguirse sin
  depender exclusivamente del color. El mapa cerrado de foco/activación DEBE ser el del contrato TUI; cualquier
  tecla ausente de la fila ganadora según prioridad de input DEBE ser inerte.
- **FR-014**: Cancelar un formulario de conexión DEBE descartar sus cambios no guardados y restaurar foco,
  selección y expansión previos. Guardar con error DEBE conservar valores y foco, mostrar un mensaje
  accionable y no modificar silenciosamente otro elemento. Si el usuario intenta salir de la aplicación
  con cambios sin guardar, un panel centrado DEBE ofrecer Guardar, Descartar y Cancelar: Guardar solo
  permite salir tras persistir con éxito, Descartar abandona los cambios y sale, y Cancelar vuelve al
  formulario sin alterar sus valores.
- **FR-015**: La TUI DEBE usar un panel centrado exactamente para crear carpeta, editar carpeta, seleccionar
  destino para mover, confirmar eliminación de conexión/carpeta, confirmar conexión, decidir cambios sin
  guardar, mostrar ayuda, atender error operativo recuperable y mostrar fallo SSH. Crear o editar conexión
  DEBE permanecer en el panel de detalle. Ninguna otra interacción DEBE usar panel centrado. Help usa su panel
  standalone cuando ningún panel está activo; con otro panel owner, `?` abre/cierra Help inline sin crear otro.
- **FR-016**: Un panel centrado DEBE identificar acción y objetivo, poseer todo el foco mientras esté
  abierto, impedir que el contenido subyacente procese teclas y ofrecer una tecla visible para cancelar
  o cerrar. Cancelar o cerrar DEBE devolver foco al opener intacto. Un fallo recuperable DEBE conservar foco
  en el panel actual; solo un fallo terminal lo cierra y restaura opener. Una finalización exitosa DEBE
  transferirlo explícitamente al resultado seleccionado/formulario siguiente, sesión SSH o salida; no restaura
  un opener eliminado. La TUI NO DEBE apilar ni sustituir paneles centrados: cualquier error recuperable de la
  acción DEBE integrarse en el panel actual.
- **FR-017**: Las confirmaciones destructivas y de conexión DEBEN mostrar la ruta completa y el destino
  capturados y requerir una aceptación explícita. Cancelar NO DEBE mutar el catálogo ni iniciar red.
- **FR-018**: Árbol, detalle, formularios, paneles centrados, acciones, desplazamiento y salida DEBEN ser
  completamente operables mediante teclado. Ayuda y acciones contextuales DEBEN hacer descubribles las
  teclas necesarias en cada estado. Cada focus owner DEBE usar el mapa cerrado del contrato TUI: movimiento y
  scroll con `Up/k`, `Down/j`, `Home/g`, `End/G`; forms con `Tab`/`Shift+Tab`; choices por tecla visible;
  `Esc` para cancel/back; y recovery `r/b/Esc`. Cuando Shift+Tab no sea distinguible, Help DEBE mostrar una
  alternativa `F2` que produzca Previous. Mientras un campo editable o secret prompt tenga foco, caracteres
  printable, incluidos `p`, `q` y `?`, DEBEN introducir texto y no despachar comandos; Help/Quit usan controles
  no-printable visibles. Teclas no listadas DEBEN ser inertes.
- **FR-019**: En navegación normal, `Tab` y `Shift+Tab` DEBEN alternar el foco entre árbol y detalle sin
  cambiar el nodo seleccionado. El detalle DEBE ser de solo lectura y sus teclas de navegación DEBEN
  desplazar únicamente su contenido. Cada región desplazable DEBE mantener visible su selección o
  posición activa. Si existe contenido anterior oculto, la primera fila de contenido DEBE mostrar `↑ more`;
  si existe contenido posterior oculto, la última DEBE mostrar `↓ more`; ambos DEBEN aparecer cuando apliquen.
  Toda línea truncada DEBE incluir `…` dentro de su ancho asignado. Los indicadores NO DEBEN depender del
  color, recibir foco ni desplazar selección, campo, error, acción primaria o controles de cancelación/salida.
  Nombres Tree, paths, endpoints, labels de campo y cada descriptor `<key> <label>` DEBEN permanecer en una
  línea y truncarse por ancho visible; Actions solo puede envolver entre descriptors completos. Help,
  confirmaciones y error prose DEBEN envolver en límites de grapheme y usar markers al clip vertical. El offset
  lógico se conserva si sigue siendo válido; el render offset puede clamparse para bounds/active visibility y
  al crecer DEBE restaurar el lógico si vuelve a ser válido. Actions NO recibe foco ni scroll manual: empaqueta
  descriptors según el contrato y, si omite alguno, muestra `↓ more — ? Help`; Help contiene y desplaza el
  inventario completo.
  Como excepción a la regla single-line, full path, endpoint e ID/revisión capturados en confirmaciones
  destructivas/de conexión DEBEN envolver de forma segura y completa, nunca truncarse; si exceden la altura,
  su viewport y Help DEBEN permitir revelar todos los graphemes con markers direccionales.
- **FR-020**: La jerarquía visual DEBE usar títulos, separación, espaciado y marcadores consistentes para
  distinguir región activa, fila seleccionada, información secundaria y acciones primarias. Debe existir
  una representación textual equivalente cuando el color esté desactivado. Los títulos exactos son `Tree`,
  `Details` y `Actions`; cada región tiene border textual visible, una celda de padding interior y un gutter/
  separador de al menos una celda. Sin color: title activo usa `[*]`, inactivo `[ ]`, fila/campo/control enfocado
  usa `>`, inválido usa `!`, primary usa `*`, y mensajes comienzan `Warning:` o `Error:`. Node-kind y overflow
  conservan sus markers textuales. Los roles de color normativos son title, selected/focus, muted, status,
  warning y failure según el contrato, y nunca sustituyen estos cues.
- **FR-021**: Si una recarga en navegación descubre que el nodo seleccionado desapareció, la TUI DEBE
  seleccionar su carpeta padre existente más cercana o la raíz, actualizar detalle y acciones en la misma
  transición y mostrar que el target desapareció. Si el mismo ID seleccionado cambia de revisión en navegación,
  DEBE mantenerse seleccionado, actualizar Details/Actions al nuevo snapshot y mostrar que cambió, sin entrar
  en recovery conflict. Si el ID o revisión capturados por un formulario, panel
  u operación pendiente desaparecen o cambian, la TUI DEBE bloquear persistencia y red, conservar objetivo
  capturado, valores, foco y acción pendiente, e integrar el conflicto en la superficie activa con Recargar,
  Volver y Cancelar. Recargar DEBE actualizar el catálogo y volver a comprobar el mismo ID sin retargeting ni
  perder valores; Volver DEBE abandonar la interacción y seleccionar el padre existente más cercano o raíz;
  Cancelar DEBE descartar solo el aviso y volver a la interacción intacta, que seguirá sin poder aplicarse
  mientras el conflicto exista. Tras Cancelar DEBE permanecer visible un banner compacto con `r` Reload,
  `b` Back y `Esc` Cancel; ninguna ruta DEBE usar otro nodo por posición de fila.
- **FR-022**: Los errores de validación u operación DEBEN aparecer junto al campo afectado o en un panel
  centrado recuperable, identificar la acción y el objetivo sin secretos, y conservar acceso a los
  controles necesarios para corregir, reintentar, cancelar o volver. Si ya existe un panel centrado,
  el error DEBE mostrarse dentro de él y conservar su foco, valores, objetivo y acción pendiente. Validación de
  connection form es field-local y save/persistence failure es form-level inline sobre Save; `operation_error`
  se usa solo cuando el owner no dispone ya de una superficie form/modal para el error.
- **FR-023**: Durante carga inicial, recarga, guardado o inicio SSH, la TUI DEBE conservar el último frame
  válido; la carga inicial DEBE usar el shell vacío. Actions DEBE mostrar textualmente
  `Loading: <action> — <target>`, usando ruta/endpoint capturados o `catalog`, sin depender de color. DEBE
  existir una sola operación propietaria; mientras esté activa, toda mutación adicional DEBE ignorarse y
  solo Cancelar, Ayuda y Salir DEBEN estar disponibles. Cancelar o Salir DEBEN solicitar cancelación,
  esperar cleanup y aplicar cualquier resultado ya confirmado antes de restaurar o terminar. Solo un
  resultado cuyo operation ID coincida con el propietario actual PUEDE actualizar estado, foco o contenido.
  Durante `ssh_start`, trust/password/passphrase es la única excepción: su security input preempta el filtro y
  permite exclusivamente los controles locales requeridos para verificar host o autenticar.
- **FR-024**: Todo texto controlado por la TUI, incluidos títulos de región/panel, acciones, ayuda, estados,
  validación, confirmaciones y errores, DEBE estar en inglés y usar términos consistentes con la CLI
  existente. Nombres, rutas, hosts y usuarios aportados por el usuario DEBEN conservarse sin traducción; sus
  bytes subyacentes DEBEN permanecer intactos aunque controles, secuencias terminales, bidi o bytes inválidos
  se representen de forma visual inerte y el contenido visible se recorte.
- **FR-025**: La salida stdout/stderr de una sesión SSH activa DEBE transmitirse al terminal sin acumular una
  copia completa en el estado de la TUI ni crecer en memoria proporcionalmente al volumen remoto. Los
  diagnósticos retenidos o presentados por la aplicación DEBEN excluir secretos, causas crudas y contenido
  remoto arbitrario, y cada detalle técnico controlado DEBE limitarse a 256 caracteres visibles.
- **FR-026**: Un timeout o una interrupción de red durante resolución, conexión, verificación de host,
  autenticación, apertura o sesión SSH activa DEBE detener la operación propietaria, cerrar sesión, transporte,
  autenticación, lectores y resize forwarding, y restaurar el terminal antes de devolver control o salir. Un
  timeout previo a sesión activa DEBE producir un fallo SSH recuperable y accionable; una interrupción posterior
  al inicio DEBE conservar el resultado/estado remoto confirmado disponible y terminar con resultado de
  transporte, sin dejar una operación o recurso activo. Resultados tardíos posteriores NO DEBEN modificar UI.
- **FR-027**: Los entornos soportados son terminales VT de Linux, macOS y Windows que reporten printable keys,
  arrows, Tab, Escape, F1/F2 y Ctrl+C. La falta de color DEBE activar semántica textual equivalente. Si un modificador
  requerido no es distinguible, DEBE existir una alternativa visible sin modificador; si falta una capacidad VT
  sin alternativa segura, la aplicación DEBE detenerse con un error de startup accionable y restaurar terminal.
- **FR-028**: Details, Help, errores y resize/render DEBEN conservar solo snapshot actual e interacción activa;
  NO DEBEN acumular frames anteriores, output renderizado ni historial de resize. La memoria retenida NO DEBE
  crecer con el número de renders/resizes, y cada frame DEBE quedar acotado al viewport terminal actual.

### Key Entities

- **Región de interfaz**: Área titulada con propósito, dimensiones, contenido visible y estado de foco;
  puede representar árbol, detalle o acciones.
- **Contexto de selección**: Identidad, tipo y ruta del nodo bajo el cursor que determina detalle y
  acciones disponibles.
- **Detalle contextual**: Proyección no secreta de una conexión o carpeta; para carpetas incluye solo
  conexiones hijas directas.
- **Contexto de edición**: Copia temporal de los campos de una conexión, su objetivo original o carpeta
  destino, validación y control enfocado.
- **Panel centrado**: Interacción temporal que conserva el contexto que la abrió y bloquea entradas a la
  vista subyacente hasta completar o cancelar.

### Scope Boundaries

**Included**:

- Reorganización visual de la TUI en árbol, detalle y acciones contextuales.
- Presentación y edición contextual de conexiones.
- Paneles centrados para interacciones que no editan campos de conexión.
- Adaptación amplia/estrecha, desplazamiento, foco, teclado y modo sin color.
- Conservación de las operaciones y garantías de seguridad ya especificadas.

**Excluded**:

- Cambios en el modelo persistente, formato de configuración o semántica de las conexiones y carpetas.
- Nuevas operaciones de catálogo, sesiones SSH o comandos de CLI.
- Navegación y activación mediante ratón; esta feature no especifica mouse y toda conformidad usa teclado.
- Localización o selección de idioma; esta feature mantiene la TUI únicamente en inglés.
- Persistencia del tamaño, disposición, foco o posición de desplazamiento entre ejecuciones.
- Presentar conexiones descendientes de forma recursiva en el detalle de una carpeta.
- Mostrar, revelar o editar secretos directamente en el resumen de detalle.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En 100 % de una matriz con raíz, carpeta vacía, carpeta con conexiones directas, carpeta con
  subcarpetas y conexión, cada cambio de cursor presenta antes de la siguiente entrada el detalle y las
  acciones del mismo nodo, sin datos ejecutables de la selección anterior.
- **SC-002**: En 20 ejecuciones completas de cada contexto raíz, carpeta y conexión, todas las acciones
  mostradas son ejecutables en su contexto y ninguna acción no mostrada modifica catálogo, selección,
  foco ni operación mediante su atajo.
- **SC-003**: En 20 ejecuciones completas de crear conexión y 20 de editar conexión, el foco comienza en
  el primer campo, recorre hacia delante y atrás todos los controles visibles en orden estable y cancelar
  restaura selección y catálogo sin cambios. En 20 ejecuciones de cada elección Guardar, Descartar y
  Cancelar al salir con cambios, solo ocurre su efecto definido; las 20 ejecuciones con fallo de guardado
  conservan todos los valores y no salen.
- **SC-004**: En el producto cartesiano de anchos `{40,60,79,80,100,160}` y alturas `{12,24}`, 20/20 frames
  cumplen el display mode, fórmulas de rectángulo, orden, bounds, no-solapamiento y prioridades reduced de
  FR-003–FR-005; 80/100/160 son wide y 40/60/79 stacked en ambas alturas.
- **SC-005**: En 20 repeticiones de la secuencia
  40→60→79→80→100→160→80→79→40 durante navegación del árbol, navegación de detalle, formulario de
  conexión dirty, cada uno de los diez tipos de panel centrado y cada operación pendiente, cada transición
  conserva objetivo, selección, expansión, offset lógico, texto introducido, foco, panel y acción pendiente.
- **SC-006**: A 80x24 y con color desactivado, 20 ejecuciones completas de cada flujo principal de las
  historias 1–4, incluidas sus rutas de completar y cancelar, permiten identificar región enfocada,
  selección, detalle, objetivo, errores y acciones usando únicamente texto y teclado.
- **SC-007**: En catálogos con 1.000 conexiones, 100 carpetas y profundidad 10, todo nodo o campo enfocado
  permanece visible durante navegación y ningún contenido se dibuja fuera de su región en los doce
  tamaños de SC-004; en árbol, detalle, formulario, acciones, ayuda, selector y confirmación, 100 % de los
  viewports con contenido oculto muestran los indicadores direccionales aplicables y 100 % de las líneas
  truncadas incluyen `…` dentro de su región.
- **SC-008**: En un estudio con 10 participantes que no hayan contribuido a la feature ni visto la nueva
  disposición, se usa una terminal 100x30 y el mismo catálogo semilla: raíz, carpetas `Empty` y `Team`,
  conexiones directas `Prod` y `Stage` dentro de `Team`, subcarpeta `Nested` con conexión `Deep`, y `Prod`
  seleccionada con foco en el árbol. El cronómetro comienza al aparecer la primera vista completa y termina
  cuando el participante identifica correctamente elemento seleccionado, información de detalle, región
  enfocada y dos acciones disponibles, sin ayuda del moderador. Después responde exactamente “La
  organización visual permite entender rápidamente el contexto y las acciones disponibles” en escala 1
  (totalmente en desacuerdo) a 5 (totalmente de acuerdo). Al menos 8 de 10 participantes DEBEN completar
  las cuatro identificaciones en ≤10 segundos y puntuar 4 o 5; protocolo, tiempos, respuestas y resultado
  agregado se registran sin datos personales.
  El moderador DEBE decir exactamente: “Sin abrir Help y sin asistencia, identifica el elemento seleccionado,
  describe la información mostrada en Details, identifica la región enfocada y nombra dos acciones disponibles.”
- **SC-009**: En 20 ejecuciones de cada tipo del inventario cerrado, la interacción aparece centrada, aísla
  el foco y permite cancelar volviendo al mismo contexto sin efectos laterales; errores operativos y fallos
  SSH permanecen en el único panel activo y conservan todos sus datos y controles. En 20 ejecuciones de crear
  y 20 de editar conexión, el formulario permanece en detalle y no abre panel centrado. Para cada kind, 20
  ejecuciones de cada terminal path habilitado en su matriz DEBEN verificar efecto de catálogo/red, target,
  valores retenidos/limpiados, selección y foco; no se permite sustituir paths por casos representativos.
- **SC-010**: En 20/20 transiciones 40x12 → 39x11 → 40x12 durante navegación, edición, confirmación y
  operación pendiente, la vista insuficiente limita sus controles a resize, ayuda y salida segura y al
  recuperar tamaño restaura exactamente selección, expansión, valores, foco, panel y operación.
- **SC-011**: En un runner Linux amd64 con al menos 2 CPU lógicas y 4 GiB libres, catálogo SQLite temporal
  de 1.000 conexiones, 100 carpetas y profundidad 10, sin carga concurrente y tras una ejecución de
  calentamiento, al menos 19 de 20 mediciones de selección, cambio de foco, scroll y resize producen la
  vista completa en ≤100 ms, y al menos 19 de 20 recargas completas producen el nuevo snapshot visible en
  ≤1 s. Cada medición usa reloj monotónico desde el evento hasta producir la vista e incluye SQLite solo
  para recarga.
- **SC-012**: En 20 ejecuciones de desaparición y 20 de cambio de revisión para navegación, formulario,
  panel y operación pendiente, navegación selecciona siempre el fallback definido y cada interacción activa
  conserva exactamente objetivo, valores y foco y no ejecuta persistencia/red sobre otro nodo. Navegación missing
  usa fallback y navigation revision-change mantiene el mismo ID con snapshot nuevo; formulario/panel/operación
  producen los efectos definidos de Recargar, Volver y Cancelar.
- **SC-013**: En 20 ejecuciones de carga inicial, recarga, guardado e inicio SSH con finalización, cancelación
  y resultado obsoleto inyectados, Actions identifica siempre acción/objetivo, ninguna tecla fuera de
  Cancelar/Ayuda/Salir inicia otra mutación, cleanup termina antes de restaurar/salir y ningún resultado sin
  ownership modifica frame, foco, selección, formulario o panel.
- **SC-014**: En 100 % del corpus de títulos, acciones, ayuda, estados, validaciones, confirmaciones y errores
  controlados por la aplicación, el texto está en inglés y usa el mismo término para cada concepto; 100 % de
  nombres, rutas, hosts y usuarios del catálogo se conservan byte por byte en el modelo; su proyección solo puede
  aplicar representación inerte de bytes inseguros y el recorte visual indicado.
- **SC-015**: Al transmitir 64 MiB por stdout y 64 MiB por stderr desde una sesión controlada, 20/20 ejecuciones
  entregan ambos streams sin que la TUI retenga más de 1 MiB de contenido remoto, y 100 % de los diagnósticos
  conservan detalles técnicos de como máximo 256 caracteres visibles sin secretos ni causas crudas.
- **SC-016**: En 20 ejecuciones de timeout y 20 de interrupción de red antes y después de activar la sesión,
  20/20 cierran exactamente una vez sesión, transporte, autenticación y resize forwarding aplicables, restauran
  el terminal antes de devolver control/salir, dejan cero operación propietaria y rechazan todo resultado tardío.
- **SC-017**: Tras 10.000 ciclos de resize entre los doce tamaños SC-004 y 10.000 renders de Details, Help y
  errores largos, la memoria retenida atribuible a historial de frame/resize crece menos de 1 MiB, no existe
  colección de frames/eventos previos y cada output individual permanece dentro del viewport actual.

## Assumptions

- La semántica y permisos de las acciones del inventario normativo se conservan; esta feature cambia su
  presentación, descubribilidad y ubicación, no su efecto de dominio.
- La raíz se presenta como una carpeta para el detalle de conexiones directas, pero mantiene las
  restricciones existentes sobre renombrado, movimiento y eliminación.
- El ancho de 80 columnas conserva el mínimo completo ya documentado por el proyecto; anchos menores
  usan la disposición apilada y pueden reducir información secundaria.
- El árbol posee el foco inicial y mover el cursor actualiza el detalle sin transferirle el foco.
- Crear una conexión usa el panel de detalle porque edita campos de conexión; crear o editar una carpeta
  usa un panel centrado aunque contenga su campo de nombre.
- El estilo visual seguirá la identidad existente del proyecto y priorizará contraste, jerarquía y
  legibilidad antes que ornamentación. Sus roles normativos son title, selected/focus, muted, status, warning y
  failure, junto con borders y markers textuales; el contrato fija los tokens actuales y no-color los omite sin
  perder semántica.
- La matriz de contenido, keys, layout, wrapping, cues y modal paths de `contracts/tui.md` es normativa para
  aceptación y es la única extensión cerrada de los requisitos UX de esta especificación.
