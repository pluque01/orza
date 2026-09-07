# Feature Specification: Publicar el proyecto en GitHub

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-09-04

**Status**: Draft

**Input**: User description: "Quiero que trabajes en lo necesario para publicar el proyecto en GitHub. Esto tendrá que incluir pipelines que validen el código, generación de binarios para las diferentes plataformas y un readme con instrucciones de instalación y uso. Incluye también una herramienta que asegure que las dependencias están actualizadas."

## Clarifications

### Session 2026-09-04

- Q: ¿Deben publicarse versiones únicamente cuando la etiqueta apunte a un commit ya integrado en la rama principal? → A: Solo commits de la rama principal.
- Q: ¿Debe exigirse la aprobación de otra persona para integrar cualquier cambio en la rama principal? → A: Solo cuando haya varios mantenedores; mientras exista un único mantenedor, las validaciones obligatorias bastan.
- Q: ¿Qué nivel de ejecución real debe comprobarse para los seis binarios antes de publicar una versión? → A: Solo comprobar que los seis destinos compilan correctamente; no se exige ejecutar los binarios como puerta de publicación.
- Q: ¿Cómo debe actuar el pipeline cuando detecte una vulnerabilidad alcanzable cuya corrección aún no esté disponible en el entorno reproducible del proyecto? → A: Puede continuar mediante una excepción temporal documentada y revisable.
- Q: ¿Qué formato exacto deben tener las etiquetas que publican una versión estable? → A: `vMAJOR.MINOR.PATCH`.

## Definitions

- **Repositorio publicable**: Código fuente, historial, documentación y configuración que han sido revisados para no contener secretos, credenciales, claves privadas, datos SSH reales ni archivos locales, y que incluyen licencia y metadatos de proyecto.
- **Entrega repetible**: Una ejecución posterior para la misma revisión y matriz produce el mismo inventario de destinos, nombres y versiones informadas; cualquier diferencia de contenido queda reflejada por sumas distintas y bloquea la sustitución silenciosa de artefactos ya publicados.
- **Diagnóstico accionable**: Resultado que identifica comprobación, etapa o destino afectado, causa observada y ubicación de los registros o pasos de repetición.
- **Vulnerabilidad bloqueante**: Vulnerabilidad conocida que el analizador vigente puede alcanzar desde el código del proyecto; su presencia impide aceptar o publicar el cambio salvo que exista una excepción temporal vigente, mientras los hallazgos no alcanzables quedan visibles para revisión.
- **Excepción temporal de vulnerabilidad**: Decisión explícita que identifica vulnerabilidad, alcance, propietario, motivo por el que la corrección aún no está disponible, mitigación y fecha de expiración o revisión. Al vencer, vuelve a bloquear automáticamente hasta renovarse o corregirse.
- **Actualización compatible**: Actualización que no cambia la versión mayor según el esquema de versiones de la dependencia. Una dependencia sin esquema equivalente se trata de forma separada.
- **Referencia externa revisable**: Versión o revisión inmutable, visible en el repositorio, de toda automatización de terceros utilizada para validar o publicar.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Publicar el código fuente con seguridad (Priority: P1)

Como mantenedor, quiero preparar y hacer público el repositorio con una licencia, descripción y controles de integración claros, sin exponer secretos ni datos operativos.

**Why this priority**: Ninguna automatización o entrega debe publicarse antes de comprobar que el propio repositorio puede exponerse y reutilizarse de forma segura y legal.

**Independent Test**: Se puede ejecutar la revisión previa, hacer público un repositorio de prueba y comprobar que es visible y clonable, contiene los metadatos obligatorios y rechaza cambios que no hayan superado las validaciones requeridas.

**Acceptance Scenarios**:

1. **Given** el repositorio todavía privado, **When** se revisan archivos e historial, **Then** no aparecen secretos, claves privadas, credenciales, datos SSH reales ni archivos locales destinados a permanecer fuera del control de versiones.
2. **Given** una revisión previa favorable y autorización del propietario, **When** se publica el repositorio, **Then** cualquier visitante puede verlo y clonarlo, conocer su propósito y consultar una licencia explícita.
3. **Given** la rama principal pública, **When** se propone integrar un cambio, **Then** las validaciones obligatorias impiden una integración no validada; la aprobación de otra persona se exige cuando el proyecto tenga más de un mantenedor.
4. **Given** un visitante que detecta un problema de seguridad, **When** consulta el repositorio, **Then** encuentra un canal privado y documentado para comunicarlo sin publicar detalles sensibles.

---

### User Story 2 - Validar cada cambio automáticamente (Priority: P2)

Como mantenedor, quiero que cada propuesta de cambio y cada actualización de la rama principal se valide automáticamente para evitar publicar código que no compile, no cumpla las reglas de calidad o introduzca vulnerabilidades conocidas.

**Why this priority**: La publicación pública requiere una señal de calidad repetible antes de aceptar cambios o generar entregas.

**Independent Test**: Se puede proponer un cambio correcto y otro con un fallo controlado, y comprobar que el primero recibe todas las validaciones favorables mientras el segundo queda bloqueado con un diagnóstico identificable.

**Acceptance Scenarios**:

1. **Given** una propuesta de cambio, **When** se crea o actualiza, **Then** se ejecutan automáticamente formato, pruebas completas, detección de condiciones de carrera, análisis estático, análisis de vulnerabilidades y compilación.
2. **Given** una actualización de la rama principal, **When** se recibe el cambio, **Then** se ejecuta el mismo conjunto de validaciones sin depender de una acción manual.
3. **Given** una validación fallida, **When** el mantenedor consulta el resultado, **Then** puede identificar la validación, plataforma y causa que impiden continuar.
4. **Given** una validación que requiere acceso al repositorio, **When** se ejecuta, **Then** utiliza los permisos mínimos y no expone credenciales, datos de conexión ni secretos en registros o artefactos.

---

### User Story 3 - Descargar una versión para la plataforma propia (Priority: P3)

Como usuario, quiero descargar una versión identificable del programa para mi sistema operativo y arquitectura, verificar su integridad e instalarla sin tener que compilar el proyecto.

**Why this priority**: Los binarios listos para usar convierten el repositorio validado en una entrega accesible para usuarios finales.

**Independent Test**: Se puede crear una versión de prueba y verificar que ofrece archivos identificados para cada combinación soportada, junto con información de versión, integridad e instrucciones de instalación.

**Acceptance Scenarios**:

1. **Given** una etiqueta de versión válida, **When** se publica, **Then** se genera una única entrega asociada con binarios para Linux, macOS y Windows en amd64 y arm64.
2. **Given** un artefacto publicado, **When** el usuario revisa su nombre, **Then** puede determinar sin ambigüedad la versión, el sistema operativo y la arquitectura.
3. **Given** un binario descargado, **When** el usuario consulta su versión, **Then** muestra la misma versión que la entrega de la que procede.
4. **Given** una entrega, **When** el usuario verifica un artefacto, **Then** dispone de una suma de comprobación publicada para detectar corrupción o sustitución accidental.
5. **Given** que cualquier validación o compilación de una plataforma falla, **When** finaliza el proceso de entrega, **Then** no se publica una versión parcial como entrega válida.
6. **Given** que los seis destinos compilan correctamente, **When** se completa la entrega, **Then** la publicación no depende de ejecutar los binarios en entornos nativos de cada plataforma o arquitectura.

---

### User Story 4 - Instalar y empezar a usar el proyecto (Priority: P4)

Como usuario nuevo, quiero encontrar en la portada del repositorio instrucciones claras para instalar, configurar y usar las operaciones principales de forma segura.

**Why this priority**: Una entrega descargable no resulta útil si el usuario no puede instalarla ni comprender sus requisitos y flujos básicos.

**Independent Test**: Una persona que no haya contribuido al proyecto puede seguir únicamente la documentación principal para instalarlo en una plataforma soportada, consultar la ayuda y completar una operación local sin conectarse a un host real.

**Acceptance Scenarios**:

1. **Given** un visitante nuevo, **When** abre la documentación principal, **Then** encuentra propósito, estado del proyecto, plataformas soportadas, requisitos y advertencias de seguridad relevantes.
2. **Given** un usuario de Linux, macOS o Windows, **When** sigue la instalación documentada para su plataforma, **Then** puede verificar la versión instalada y abrir la ayuda del programa.
3. **Given** un usuario que quiere empezar, **When** consulta la guía de uso, **Then** encuentra ejemplos de creación y gestión local de carpetas y conexiones, navegación por teclado y lanzamiento de la interfaz.
4. **Given** una operación SSH, **When** el usuario consulta la documentación, **Then** recibe una explicación visible sobre verificación de identidad del host, tratamiento de credenciales y cómo cancelar o salir.
5. **Given** un colaborador, **When** consulta la documentación, **Then** encuentra cómo preparar el entorno y ejecutar localmente las mismas validaciones requeridas para aceptar cambios.

---

### User Story 5 - Mantener dependencias al día (Priority: P5)

Como mantenedor, quiero recibir propuestas automáticas, revisables y validadas cuando existan actualizaciones de dependencias o de automatizaciones, para reducir exposición a vulnerabilidades y evitar actualizaciones manuales olvidadas.

**Why this priority**: La actualización continua preserva la seguridad de la publicación una vez que los pipelines y artefactos existen.

**Independent Test**: Se puede simular o detectar una versión posterior de una dependencia y comprobar que aparece una propuesta limitada, con contexto suficiente y todas las validaciones normales asociadas.

**Acceptance Scenarios**:

1. **Given** una dependencia directa, indirecta o de automatización desactualizada, **When** se ejecuta la revisión programada, **Then** se crea una propuesta de actualización revisable con versión anterior, versión propuesta y archivos afectados.
2. **Given** varias actualizaciones compatibles, **When** se detectan en el mismo periodo, **Then** pueden agruparse para evitar ruido sin mezclar actualizaciones mayores incompatibles.
3. **Given** una propuesta automática, **When** se abre o actualiza, **Then** ejecuta las mismas validaciones obligatorias que cualquier otra propuesta y no se integra automáticamente sin una decisión explícita del mantenedor.
4. **Given** una actualización que no puede aplicarse o validar correctamente, **When** finaliza la revisión, **Then** el mantenedor recibe un resultado accionable sin modificar la rama principal.

### Edge Cases

- Una etiqueta no sigue exactamente el formato `vMAJOR.MINOR.PATCH`, ya existe, no apunta a la rama principal o apunta a código que no supera las validaciones.
- Una plataforma o arquitectura falla mientras las demás producen binarios correctamente.
- La generación de sumas o la carga de artefactos se interrumpe y deja resultados temporales.
- Dos procesos intentan publicar la misma versión o actualizar el mismo grupo de dependencias simultáneamente.
- Una propuesta procede de una bifurcación sin acceso a secretos del repositorio.
- El servicio de automatización o su fuente de dependencias no está disponible temporalmente.
- Una dependencia requiere una versión nueva del compilador o un cambio incompatible.
- Los nombres de archivos, notas o registros contienen datos inesperados que podrían interpretarse como comandos o revelar información sensible.
- El usuario instala un binario de una arquitectura incorrecta o en un sistema no soportado.
- La revisión previa encuentra un secreto en el historial aunque ya no aparezca en la revisión actual.
- El repositorio carece de autorización del propietario o de una licencia aprobada para distribución pública.
- Una excepción temporal de vulnerabilidad vence mientras todavía no existe una corrección compatible con el entorno reproducible.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El repositorio DEBE ejecutar automáticamente el conjunto obligatorio de validaciones en cada propuesta de cambio y actualización de la rama principal.
- **FR-002**: El conjunto obligatorio DEBE incluir comprobación de formato, pruebas completas, detección de condiciones de carrera, análisis estático, análisis de vulnerabilidades y compilación.
- **FR-003**: Cada validación DEBE producir un resultado independiente y un diagnóstico accionable, y cualquier fallo obligatorio DEBE impedir que el cambio se considere apto para publicación salvo una vulnerabilidad cubierta por una excepción temporal vigente.
- **FR-004**: Las automatizaciones DEBEN utilizar permisos mínimos, usar referencias externas revisables y evitar registrar o publicar secretos, credenciales o datos SSH reales.
- **FR-005**: La validación DEBE usar datos sintéticos y no DEBE requerir acceso a servidores SSH reales, almacenes de credenciales de usuario ni recursos privados de producción.
- **FR-006**: Una etiqueta estable con formato exacto `vMAJOR.MINOR.PATCH` DEBE iniciar una entrega repetible solo si apunta a un commit ya integrado en la rama principal y ese commit ha superado las validaciones obligatorias.
- **FR-007**: Cada entrega DEBE producir binarios para Linux, macOS y Windows, en amd64 y arm64, y la puerta de publicación DEBE comprobar que los seis destinos compilan correctamente sin exigir su ejecución previa en cada sistema o arquitectura.
- **FR-008**: Cada artefacto DEBE tener un nombre no ambiguo que incluya proyecto, versión, sistema operativo y arquitectura, y el ejecutable de Windows DEBE conservar su extensión de plataforma.
- **FR-009**: Todo binario publicado DEBE informar la versión exacta de su entrega mediante el comando de versión existente.
- **FR-010**: Cada entrega DEBE incluir sumas de comprobación para todos sus artefactos descargables y notas que identifiquen la versión y los cambios incluidos.
- **FR-011**: La entrega DEBE ser atómica desde la perspectiva del usuario: si falla una validación, plataforma, suma o carga, no DEBE presentarse una versión parcial como estable.
- **FR-012**: Las automatizaciones DEBEN evitar ejecuciones de publicación duplicadas para la misma versión y DEBEN conservar diagnósticos suficientes para investigar un fallo sin incluir secretos.
- **FR-013**: La documentación principal DEBE explicar el propósito del proyecto, estado, plataformas y arquitecturas soportadas, tamaño mínimo y capacidades de terminal, compatibilidad SSH asumida, requisitos, métodos de instalación, verificación de integridad y desinstalación o actualización.
- **FR-014**: La documentación principal DEBE incluir instrucciones verificables para instalar desde una entrega, compilar desde el código fuente y utilizar el entorno reproducible existente.
- **FR-015**: La documentación principal DEBE incluir ejemplos de ayuda, versión, gestión local de carpetas y conexiones, apertura y navegación por teclado de la interfaz y salida segura.
- **FR-016**: La documentación principal DEBE explicar las garantías y responsabilidades de seguridad relacionadas con identidad del host, credenciales, permisos de archivos, legibilidad sin color y ausencia de hosts reales en ejemplos. También DEBE advertir que las sumas de comprobación detectan diferencias o corrupción, pero no autentican por sí solas un artefacto cuando la firma queda fuera del alcance.
- **FR-017**: La documentación para colaboradores DEBE indicar cómo ejecutar localmente el mismo conjunto de validaciones obligatorias y cómo se crea una entrega.
- **FR-018**: El repositorio DEBE revisar automáticamente, al menos una vez por semana, actualizaciones de dependencias de la aplicación y de las automatizaciones de validación y publicación.
- **FR-019**: Las propuestas automáticas de dependencias DEBEN ejecutar todas las validaciones obligatorias, agrupar solo actualizaciones compatibles y separar cada actualización mayor o dependencia sin esquema equivalente; toda integración DEBE requerir aprobación explícita.
- **FR-020**: La herramienta de actualización DEBE ejecutar como máximo un ciclo semanal, mantener como máximo cinco propuestas abiertas, indicar versiones anterior y propuesta, archivos afectados y diagnóstico accionable cuando una actualización falle.
- **FR-021**: Las versiones de dependencias y las referencias externas revisables utilizadas para validar o publicar DEBEN quedar determinadas de forma inmutable y visible en el repositorio.
- **FR-022**: La solución NO DEBE cambiar los controles, datos, comportamiento SSH ni formatos persistentes de la aplicación, salvo la versión informada por los binarios publicados.
- **FR-023**: Antes de hacer público el repositorio, un mantenedor DEBE revisar los archivos y el historial completo con detección automatizada y revisión manual para confirmar que no contienen secretos, credenciales, claves privadas, datos SSH reales ni archivos locales no publicables; todo hallazgo DEBE remediarse antes de continuar.
- **FR-024**: El repositorio público DEBE incluir una descripción breve, temas de descubrimiento, documentación principal, canal privado para vulnerabilidades y la licencia MIT para el código propio del proyecto.
- **FR-025**: La rama principal DEBE exigir todas las validaciones obligatorias antes de integrar propuestas. Mientras exista un único mantenedor, este PUEDE integrar sus propios cambios validados; cuando el proyecto tenga más de un mantenedor, DEBE exigirse al menos una aprobación de otra persona. Las automatizaciones de dependencias NO DEBEN eludir estas reglas ni integrarse por sí solas.
- **FR-026**: Etiquetas, nombres de archivo, rutas, notas de entrega y valores de entorno DEBEN tratarse como entradas no confiables y NO DEBEN interpolarse de forma que puedan ejecutar instrucciones no previstas.
- **FR-027**: La visibilidad pública solo DEBE activarse tras autorización explícita del propietario del repositorio y confirmación de que el código propio y sus artefactos pueden distribuirse bajo la licencia elegida.
- **FR-028**: Una vulnerabilidad alcanzable cuya corrección no esté disponible en el entorno reproducible solo PUEDE exceptuarse mediante un registro visible que indique identificador, alcance, propietario, justificación, mitigación y fecha de expiración o revisión. La excepción DEBE dejar de aplicarse automáticamente al vencer o cuando exista una corrección compatible.

### Key Entities

- **Entrega versionada**: Publicación identificada por una versión única, estado, notas y conjunto completo de artefactos.
- **Artefacto de plataforma**: Binario descargable asociado a una entrega, sistema operativo, arquitectura, nombre y suma de comprobación.
- **Resultado de validación**: Resultado favorable o fallido de una comprobación obligatoria, con ámbito y diagnóstico accionable.
- **Propuesta de actualización**: Cambio automático y revisable de una o más dependencias compatibles, con versiones anterior/propuesta y resultados de validación.
- **Repositorio público**: Fuente visible y clonable, con propietario, licencia, metadatos, rama principal protegida y canal privado de seguridad.

### Scope Boundaries

**Included**:

- Revisión previa, publicación pública del código fuente, metadatos, licencia, canal de seguridad y protección de la rama principal.
- Preparación del repositorio para colaboración mediante validaciones automáticas.
- Generación y publicación de binarios versionados para las plataformas soportadas.
- Documentación principal de instalación, integridad, uso, seguridad y contribución.
- Revisión y propuesta automatizada de actualizaciones de dependencias y automatizaciones.

**Excluded**:

- Publicación en tiendas de aplicaciones, gestores de paquetes de terceros o registros ajenos a las entregas del repositorio.
- Firma con certificados de plataforma, notarización de macOS o firma de código de Windows en esta primera versión.
- Despliegue de servicios, telemetría, actualizaciones automáticas dentro de la aplicación o alojamiento de servidores SSH.
- Integración automática sin revisión de propuestas de dependencias.
- Cambios funcionales en la gestión de conexiones o en la interfaz terminal.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de las propuestas de cambio y actualizaciones de la rama principal ejecuta las seis categorías de validación obligatorias, y un fallo controlado en cada categoría impide que el cambio se considere apto salvo una vulnerabilidad cubierta por una excepción temporal vigente y completa.
- **SC-002**: Una etiqueta de versión válida produce exactamente seis artefactos de plataforma, uno por cada combinación soportada, más sus sumas de comprobación, sin artefactos ausentes ni nombres ambiguos.
- **SC-003**: El 100 % de las entregas completa correctamente las seis compilaciones y produce seis archivos no vacíos con la plataforma, arquitectura y versión esperadas; ningún destino requiere ejecución nativa como condición de publicación.
- **SC-004**: Una interrupción simulada en cada etapa de publicación deja cero entregas parciales presentadas como estables y muestra comprobación o etapa afectada, causa y ubicación de registros o pasos de repetición.
- **SC-005**: Al menos 9 de 10 usuarios nuevos completan instalación, verificación de versión y apertura de ayuda en menos de 10 minutos usando únicamente la documentación principal.
- **SC-006**: El 100 % de los ejemplos documentados utiliza datos sintéticos, evita hosts reales y describe una forma visible de cancelar o salir de los flujos interactivos.
- **SC-007**: Una dependencia desactualizada detectada en una prueba genera una propuesta revisable dentro del siguiente ciclo semanal, con validaciones completas y sin integración automática.
- **SC-008**: Durante cuatro ciclos semanales consecutivos, la automatización mantiene como máximo cinco propuestas de dependencias abiertas y ninguna actualización mayor se mezcla con actualizaciones compatibles.
- **SC-009**: Un colaborador nuevo puede localizar y ejecutar el conjunto local de validaciones e identificar correctamente, en menos de 15 minutos y usando solo la documentación, qué evento crea una entrega, qué seis destinos produce y qué fallo la bloquea.
- **SC-010**: Tras la publicación, una comprobación sin autenticación puede ver y clonar el repositorio, localizar licencia, documentación y canal privado de seguridad, y confirmar que la rama principal exige las seis categorías de validación y aplica la política de aprobación correspondiente al número de mantenedores.
- **SC-011**: La revisión previa automatizada y manual encuentra cero secretos, claves privadas, credenciales, datos SSH reales o archivos locales no publicables en archivos e historial antes del cambio de visibilidad.
- **SC-012**: En pruebas con una excepción vigente, una vencida y una vulnerabilidad ya corregible, solo la excepción vigente permite continuar; las otras dos bloquean y muestran todos los campos exigidos por FR-028.

## Assumptions

- GitHub será el repositorio público y su mecanismo de entregas será el canal inicial de distribución binaria.
- El propietario ha elegido la licencia MIT para el código propio del proyecto; los componentes de terceros conservan sus licencias correspondientes.
- El propietario autorizará explícitamente el cambio a visibilidad pública y dispone de permisos administrativos para configurar metadatos, canal de seguridad y protección de rama.
- La rama principal y las reglas de protección se configurarán para exigir las validaciones obligatorias antes de integrar cambios.
- La primera matriz soportada incluye Linux, macOS y Windows en amd64 y arm64; las combinaciones se generan con las capacidades de compilación ya disponibles en el proyecto.
- Las entregas estables se iniciarán únicamente mediante etiquetas `vMAJOR.MINOR.PATCH` y no mediante cada actualización de la rama principal.
- El repositorio dispondrá de permisos para crear entregas y cargar artefactos, pero las validaciones ordinarias funcionarán sin secretos adicionales.
- Una herramienta mantenida para el ecosistema de GitHub revisará dependencias semanalmente; las decisiones concretas de herramienta y configuración corresponden a la planificación.
- La firma criptográfica y notarización de plataforma aportan valor, pero quedan fuera de esta primera publicación porque requieren identidades y secretos de firma no proporcionados.
- Los artefactos y registros seguirán las políticas de retención predeterminadas del servicio de alojamiento salvo decisión posterior documentada.
