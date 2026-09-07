# Feature Specification: Gestión de conexiones SSH

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-07-29

**Status**: Draft

**Input**: User description: "Quiero una aplicación para gestionar mis conexiones SSH a través del
terminal. Sobre las conexiones se pueden hacer todas las operaciones CRUD. Las conexiones se pueden
organizar en conjuntos, como si fuesen carpetas. Puede haber carpetas de conexiones dentro de otras
carpetas. La aplicación se debe poder usar de manera interactiva (TUI) o con comandos (CLI) para
operar sobre una conexión o iniciarla. Al iniciar una conexión, el programa puede terminar
correctamente."

## Clarifications

### Session 2026-07-29

- Q: ¿Cuál debe ser la fuente principal de las conexiones gestionadas por la aplicación? → A:
  Catálogo propio; no modifica `~/.ssh/config`.
- Q: ¿Qué métodos de autenticación debe admitir una conexión guardada en la primera versión? → A:
  Agente SSH, referencia a clave y contraseña; las contraseñas solo se guardan con consentimiento
  explícito en el almacén seguro del sistema, nunca en el catálogo.
- Q: ¿Qué debe ocurrir si dos instancias modifican simultáneamente el mismo elemento del catálogo? → A:
  Rechazar la escritura obsoleta y pedir al usuario que recargue.
- Q: ¿Qué debe ocurrir con una contraseña guardada cuando se elimina la conexión o se cambia su método
  de autenticación? → A: Eliminar automáticamente la contraseña asociada.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gestionar e iniciar conexiones por comandos (Priority: P1)

Como usuario del terminal, quiero crear, consultar, modificar y eliminar conexiones guardadas, e
iniciar cualquiera de ellas mediante comandos, para administrar y usar mis destinos SSH sin recordar
todos sus parámetros.

**Why this priority**: Es el flujo mínimo que aporta valor: guardar destinos reutilizables y abrir una
sesión desde cualquier terminal, incluso sin una interfaz interactiva disponible.

**Independent Test**: Se puede probar creando una conexión mediante un comando, consultándola,
modificándola, iniciando una sesión con ella y eliminándola, comprobando en cada paso el resultado y
el estado de salida comunicado al usuario.

**Acceptance Scenarios**:

1. **Given** que no existe una conexión llamada `produccion` en la carpeta actual, **When** el usuario
   la crea con un host, usuario y puerto válidos, **Then** la conexión queda guardada y sus datos no
   secretos se pueden consultar mediante comandos.
2. **Given** una conexión guardada, **When** el usuario modifica uno o varios de sus datos, **Then**
   una consulta posterior muestra los nuevos valores y conserva los no modificados.
3. **Given** una conexión guardada, **When** el usuario solicita eliminarla y confirma la acción,
   **Then** deja de aparecer en consultas y no se puede iniciar por su identificador anterior.
4. **Given** una conexión válida y un destino accesible, **When** el usuario solicita iniciarla,
   **Then** obtiene una sesión SSH interactiva asociada a ese destino y el gestor puede dejar de
   permanecer activo sin interrumpir la sesión.
5. **Given** una conexión inexistente o inválida, **When** el usuario intenta consultarla, modificarla,
   eliminarla o iniciarla, **Then** recibe un error accionable, ningún otro registro cambia y se
   comunica un resultado fallido.
6. **Given** una conexión con autenticación por contraseña, **When** el usuario decide recordarla,
   **Then** se solicita consentimiento explícito, la contraseña se guarda únicamente en el almacén
   seguro del sistema y nunca se muestra en los detalles de la conexión.
7. **Given** que otra instancia confirmó un cambio sobre el mismo elemento, **When** el usuario intenta
   guardar una edición basada en la versión anterior, **Then** su escritura se rechaza, se conserva el
   cambio ya confirmado y se le ofrece recargar antes de reintentar.
8. **Given** una conexión con una contraseña guardada, **When** el usuario elimina la conexión o cambia
   a otro método de autenticación, **Then** la contraseña asociada se elimina automáticamente del
   almacén seguro y no queda como credencial huérfana.

---

### User Story 2 - Gestionar conexiones de forma interactiva (Priority: P2)

Como usuario, quiero navegar y realizar las mismas operaciones desde una interfaz interactiva de
terminal para trabajar con mis conexiones sin memorizar comandos.

**Why this priority**: La interfaz interactiva mejora la descubribilidad y la rapidez de uso, pero la
gestión básica ya es posible con la historia P1.

**Independent Test**: Se puede probar exclusivamente con el teclado abriendo la interfaz, creando una
conexión, consultando sus detalles, editándola, iniciándola, volviendo a la interfaz tras un fallo
recuperable y eliminándola.

**Acceptance Scenarios**:

1. **Given** que el usuario abre la interfaz interactiva, **When** navega con el teclado, **Then** puede
   descubrir las acciones de crear, consultar, editar, eliminar e iniciar una conexión.
2. **Given** una conexión seleccionada, **When** el usuario inicia la sesión, **Then** el terminal queda
   dedicado a la sesión SSH y, cuando esta termina, el terminal queda en un estado utilizable.
3. **Given** una operación destructiva, **When** el usuario la selecciona, **Then** la interfaz muestra
   el destino afectado y permite confirmar o cancelar sin alterar datos al cancelar.
4. **Given** un fallo recuperable al guardar o iniciar, **When** se muestra el error, **Then** el mensaje
   no revela secretos y el usuario puede corregir, reintentar, volver o salir de forma segura.

---

### User Story 3 - Organizar conexiones en carpetas anidadas (Priority: P3)

Como usuario con múltiples entornos y proyectos, quiero crear una jerarquía de carpetas, mover
conexiones entre ellas y administrarla tanto por comandos como de forma interactiva para localizar
rápidamente cada destino.

**Why this priority**: La organización jerárquica resulta valiosa al crecer el inventario, pero no es
necesaria para guardar e iniciar las primeras conexiones.

**Independent Test**: Se puede probar creando dos niveles de carpetas, guardando y moviendo conexiones
y carpetas, renombrando elementos y verificando que las rutas actualizadas funcionan en CLI y TUI.

**Acceptance Scenarios**:

1. **Given** la carpeta raíz, **When** el usuario crea `clientes/acme/produccion`, **Then** puede navegar
   por cada nivel y guardar conexiones en cualquiera de ellos.
2. **Given** una conexión o carpeta existente, **When** el usuario la mueve a otra carpeta válida,
   **Then** aparece únicamente en el destino y se accede a ella mediante su nueva ruta.
3. **Given** una carpeta que contiene elementos, **When** el usuario intenta eliminarla sin solicitar
   explícitamente un borrado recursivo, **Then** la operación se rechaza e informa del contenido que
   impide eliminarla.
4. **Given** una carpeta anidada, **When** el usuario intenta moverla dentro de sí misma o de una de sus
   descendientes, **Then** la operación se rechaza y la jerarquía permanece sin cambios.

### Edge Cases

- Dos conexiones, dos carpetas o una conexión y una carpeta intentan usar el mismo nombre dentro de
  la misma carpeta.
- Un nombre incluye separadores de ruta, está vacío o solo contiene espacios.
- El usuario intenta mover o eliminar la carpeta raíz.
- Una ruta referencia una carpeta o conexión que fue renombrada, movida o eliminada.
- El host está vacío, el puerto queda fuera de su rango válido o faltan datos obligatorios.
- El host es desconocido o presenta una identidad diferente de la conocida durante el inicio de
  sesión.
- El almacén seguro del sistema no está disponible cuando el usuario solicita recordar una contraseña.
- El almacén seguro rechaza la eliminación de una contraseña asociada a una conexión.
- La autenticación falla, la red no responde, el usuario cancela o el destino cierra la sesión de
  forma inesperada.
- El proceso recibe una solicitud de terminación durante una edición o durante una sesión activa.
- Dos instancias intentan modificar o eliminar simultáneamente el mismo elemento o elementos distintos.
- El inventario contiene al menos 1.000 conexiones, 100 carpetas o 10 niveles de anidamiento.
- La terminal es demasiado pequeña o no admite color.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir crear una conexión indicando un nombre, host y los datos de
  acceso no secretos necesarios; el puerto DEBE aceptar un valor válido y usar 22 cuando se omite.
- **FR-002**: El sistema DEBE permitir listar conexiones y consultar los detalles no secretos de una
  conexión concreta mediante su ruta o identificador inequívoco.
- **FR-003**: El sistema DEBE permitir modificar campos concretos de una conexión sin alterar los
  campos que el usuario no haya solicitado cambiar.
- **FR-004**: El sistema DEBE permitir eliminar una conexión solo después de identificar claramente
  el destino y obtener confirmación, salvo que el usuario use deliberadamente una opción documentada
  de no interacción.
- **FR-005**: Todas las operaciones CRUD sobre conexiones DEBEN estar disponibles tanto mediante
  comandos como mediante la interfaz interactiva y producir el mismo estado persistido.
- **FR-006**: El sistema DEBE proporcionar una carpeta raíz y permitir crear, consultar, renombrar,
  mover y eliminar carpetas anidadas mediante ambos modos de uso.
- **FR-007**: El sistema DEBE permitir crear y mover conexiones a cualquier carpeta válida, además de
  mover carpetas completas conservando su contenido.
- **FR-008**: Cada elemento DEBE tener un nombre no vacío y único dentro de su carpeta; los nombres no
  DEBEN contener el separador utilizado para expresar rutas.
- **FR-009**: El sistema DEBE impedir que una carpeta se mueva dentro de sí misma o de una descendiente,
  y DEBE impedir mover o eliminar la carpeta raíz.
- **FR-010**: El sistema DEBE rechazar por defecto la eliminación de una carpeta no vacía. El borrado
  recursivo DEBE requerir una solicitud explícita que identifique el alcance antes de confirmarse.
- **FR-011**: El sistema DEBE resolver las rutas de forma determinista y comunicar si una ruta no
  existe, identifica el tipo de elemento equivocado o entra en conflicto con otro elemento.
- **FR-012**: El sistema DEBE permitir iniciar una sesión SSH desde una conexión guardada mediante
  ambos modos de uso y DEBE mostrar el destino seleccionado antes de cederle el terminal.
- **FR-013**: Una vez iniciada la sesión, el gestor DEBE poder dejar de permanecer activo sin cerrar la
  sesión SSH. Al finalizar o cancelar la sesión, el terminal DEBE quedar restaurado y el resultado de
  la sesión DEBE comunicarse correctamente al entorno invocador.
- **FR-014**: Un fallo recuperable al iniciar una sesión desde la interfaz interactiva DEBE devolver al
  usuario a una pantalla estable desde la que pueda reintentar, volver o salir.
- **FR-015**: El sistema DEBE verificar la identidad del host por defecto. Un host desconocido o cuya
  identidad haya cambiado DEBE requerir una decisión explícita y mostrar una advertencia clara.
- **FR-016**: El sistema NO DEBE registrar, mostrar después de su entrada ni persistir claves privadas,
  frases de paso, contraseñas o secretos de sesión sin consentimiento explícito del usuario.
- **FR-017**: Los errores DEBEN indicar la operación y el elemento afectados, ofrecer una acción
  posible cuando sea recuperable y excluir cualquier secreto.
- **FR-018**: La interfaz interactiva DEBE ser completamente operable con teclado, permitir cancelar,
  volver y salir, e indicar estados sin depender únicamente del color.
- **FR-019**: Una operación de creación, modificación, movimiento o eliminación que falle DEBE dejar
  intactos todos los elementos implicados.
- **FR-020**: Las operaciones por comandos DEBEN comunicar de forma inequívoca éxito o fallo para que
  puedan utilizarse de manera automatizada sin abrir la interfaz interactiva.
- **FR-021**: Todas las operaciones CRUD DEBEN usar un catálogo propio como fuente de las conexiones
  gestionadas. El sistema NO DEBE modificar `~/.ssh/config` como resultado de esas operaciones.
- **FR-022**: Una conexión DEBE poder usar un agente SSH, una referencia a una clave privada o una
  contraseña solicitada al conectar. Una contraseña solo PUEDE recordarse con consentimiento
  explícito mediante el almacén seguro del sistema y DEBE permanecer separada del catálogo. Si ese
  almacén no está disponible, el sistema DEBE solicitar la contraseña sin persistirla.
- **FR-023**: Antes de confirmar una modificación o eliminación, el sistema DEBE detectar si el
  elemento cambió desde que la instancia lo leyó. Una escritura obsoleta DEBE rechazarse sin
  sobrescribir el cambio confirmado y DEBE indicar al usuario que recargue antes de reintentar.
- **FR-024**: Al eliminar una conexión o cambiar desde autenticación por contraseña, el sistema DEBE
  eliminar automáticamente la contraseña asociada del almacén seguro. La operación NO DEBE dejar una
  credencial huérfana; si no puede eliminarla, DEBE informar del fallo y conservar el estado anterior.

### Key Entities

- **Conexión SSH**: Destino reutilizable identificado por un nombre dentro de una carpeta. Incluye
  host, puerto, usuario opcional, método de autenticación y, cuando corresponda, una referencia a una
  clave o credencial segura. Pertenece a exactamente una carpeta y no contiene secretos en texto
  visible. La credencial segura asociada comparte su ciclo de vida con la conexión mientras esta use
  autenticación por contraseña.
- **Carpeta**: Contenedor identificado por un nombre dentro de su carpeta padre. Puede contener
  conexiones y otras carpetas. Toda carpeta salvo la raíz tiene exactamente una carpeta padre.
- **Ruta de elemento**: Secuencia de nombres desde la raíz que identifica de forma inequívoca una
  conexión o carpeta y cambia cuando el elemento o cualquiera de sus ancestros se renombra o mueve.
- **Sesión SSH**: Uso temporal de una conexión guardada. Tiene un inicio, un resultado final y puede
  finalizar por cierre normal, cancelación, error de autenticación o interrupción de red.

### Scope Boundaries

**Included**:

- Inventario local de conexiones y carpetas para un único usuario.
- Catálogo propio e independiente de `~/.ssh/config` para las conexiones gestionadas.
- Paridad funcional entre los modos CLI y TUI para CRUD, navegación e inicio de conexiones.
- Inicio de sesiones SSH interactivas y restauración segura del terminal.

**Excluded**:

- Sincronización del inventario entre equipos o usuarios.
- Gestión centralizada de permisos, equipos u organizaciones.
- Administración de servidores remotos distinta de abrir una sesión SSH.
- Almacenamiento propio de contraseñas, frases de paso o claves privadas.
- Lectura o modificación directa de `~/.ssh/config` como catálogo gestionado.
- Transferencia de archivos, túneles o ejecución remota no interactiva como flujos dedicados.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Al menos el 90 % de usuarios nuevos puede crear una conexión e iniciar una sesión por
  comandos en menos de 2 minutos usando únicamente la ayuda integrada.
- **SC-002**: Al menos el 90 % de usuarios nuevos puede crear, editar e iniciar una conexión desde la
  interfaz interactiva en menos de 3 minutos sin asistencia externa.
- **SC-003**: El 100 % de las operaciones CRUD de conexiones y carpetas definidas en esta especificación
  se puede completar tanto por CLI como por TUI con resultados persistidos equivalentes.
- **SC-004**: En un inventario de 1.000 conexiones distribuidas en 100 carpetas y hasta 10 niveles, el
  usuario puede abrir cualquier carpeta o consultar una ruta conocida en menos de 2 segundos en al
  menos el 95 % de los intentos.
- **SC-005**: El 100 % de los casos de fallo y cancelación definidos deja el inventario sin cambios
  parciales, el terminal utilizable y los mensajes visibles libres de secretos.
- **SC-006**: Al menos el 95 % de los participantes en una prueba de uso completa las tareas de crear,
  organizar, localizar e iniciar una conexión sin seleccionar un destino equivocado.
- **SC-007**: El 100 % de las pruebas con modificaciones simultáneas sobre el mismo elemento conserva
  el primer cambio confirmado, rechaza la escritura obsoleta y deja el catálogo consistente.
- **SC-008**: El 100 % de las pruebas de eliminación de conexiones o cambio de autenticación termina
  sin contraseñas huérfanas y conserva el estado anterior si el almacén seguro rechaza la eliminación.

## Assumptions

- La aplicación se usa localmente por una sola persona; no se requieren cuentas ni permisos entre
  usuarios dentro de la aplicación.
- El usuario dispone de acceso de red, credenciales válidas y un destino compatible con SSH para
  iniciar sesiones reales.
- Los mecanismos existentes del entorno para claves, agentes y verificación de identidad del host
  siguen siendo la fuente de confianza; la aplicación no reemplaza su gestión criptográfica.
- Recordar una contraseña depende de que el entorno disponga de un almacén seguro de credenciales;
  sin él, la contraseña se solicita para cada conexión.
- El inventario gestionado se conserva en un catálogo propio y no modifica `~/.ssh/config`.
- Una conexión puede omitir el usuario para utilizar el valor que determine el entorno del usuario.
- Las conexiones y carpetas comparten el mismo espacio de nombres dentro de cada carpeta para que una
  ruta siempre sea inequívoca.
- Iniciar una conexión cede el uso interactivo del terminal a la sesión. El gestor no necesita seguir
  visible ni activo mientras la sesión está abierta.
- Las opciones y la semántica exacta de los comandos se definirán durante la planificación sin cambiar
  las capacidades ni los resultados exigidos por esta especificación.
