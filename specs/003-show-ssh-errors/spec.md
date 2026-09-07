# Feature Specification: Motivos de error al iniciar SSH

**Feature Branch**: Sin rama (hook de Git no configurado)

**Created**: 2026-07-31

**Status**: Approved for implementation

**Input**: User description: "Al iniciar una sesión SSH fallida, se le debe mostrar al usuario el motivo del error, ya sea timeout, permission denied u otro."

## Clarifications

### Session 2026-07-31

- Q: ¿Dónde debe mostrarse el motivo seguro de un fallo al iniciar SSH? → A: En TUI y CLI, incluida salida estructurada.
- Q: ¿Cuánto detalle debe mostrar el diagnóstico visible además de la categoría del fallo? → A: Categoría, etapa, recomendación segura y mensaje técnico sanitizado.
- Q: ¿Cómo debe funcionar la opción de reintentar después de mostrar un fallo? → A: Reintento explícito con nueva validación y confirmación del objetivo.
- Q: ¿Cómo debe presentarse el detalle técnico sanitizado en la TUI? → A: Oculto inicialmente y desplegable mediante teclado.
- Q: Cuando un error contiene varias causas encadenadas, ¿qué regla debe decidir la categoría mostrada? → A: Usar la etapa que bloqueó el inicio y una causa reconocida; la cancelación explícita tiene prioridad.

### Session 2026-08-06

- Q: ¿Cómo se mide el límite del detalle técnico? → A: En puntos de código Unicode (runes de Go), con un máximo total de 256; al truncar se conservan 255 y se añade `…`.
- Q: ¿Qué distingue cancelación de terminación por señal? → A: La cancelación manejada dentro de la aplicación que devuelve control es `canceled`; una señal que termina el proceso conserva 130/143 y no requiere diagnóstico.
- Q: ¿Qué texto puede alimentar el detalle técnico? → A: Solo plantillas y campos públicos enumerados; sanitizar un `error.Error()` arbitrario no autoriza mostrarlo.
- Q: ¿Qué significa resolución en la etapa del diagnóstico? → A: Resolución de nombre de red después de confirmar un objetivo de catálogo; lookup/conflicto de catálogo conserva su error de gestión existente.
- Q: ¿Cómo conviven prompts y salida CLI estructurada? → A: Los prompts/stream permanecen en stdout; un fallo pre-activo produce exactamente un objeto JSON en stderr y no se emite envelope de éxito.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Comprender por qué falló la conexión (Priority: P1)

Como usuario que intenta abrir una conexión guardada, quiero ver el motivo concreto cuando SSH no llega
a iniciar una sesión para saber si debo corregir credenciales, red, confianza del host o configuración.

**Why this priority**: Un mensaje genérico impide distinguir problemas que requieren soluciones
completamente diferentes y obliga al usuario a investigar fuera de la aplicación.

**Independent Test**: Se puede provocar cada clase de fallo previa a sesión y comprobar que la salida
identifica el destino, muestra exactamente una categoría segura y diferencia timeout de autenticación
denegada.

**Acceptance Scenarios**:

1. **Given** una conexión cuyo servidor no responde dentro del plazo, **When** el inicio falla, **Then**
   el usuario ve que la conexión agotó el tiempo y no un error genérico de autenticación.
2. **Given** un servidor alcanzable que rechaza las credenciales, **When** el inicio falla, **Then** el
   usuario ve `Permission denied` o una descripción equivalente de autenticación denegada.
3. **Given** una conexión rechazada, host inexistente, red inalcanzable o identidad de host no aceptada,
   **When** el inicio falla, **Then** el usuario ve la categoría correspondiente y el destino afectado.
4. **Given** un fallo no reconocido, **When** no puede asignarse una causa específica con seguridad,
   **Then** el usuario ve un motivo desconocido seguro y una acción recomendada, no una pantalla vacía.

---

### User Story 2 - Recuperarse sin perder el contexto (Priority: P2)

Como usuario de la interfaz interactiva, quiero que un fallo previo a sesión me devuelva a una pantalla
estable desde la que pueda revisar la conexión, reintentar o volver atrás.

**Why this priority**: Conocer el motivo solo es útil si la aplicación conserva el objetivo y permite
actuar sin reiniciar ni reconstruir la navegación.

**Independent Test**: Se inicia una conexión que falla antes de activar la sesión y se comprueba que el
mensaje conserva el objetivo, ofrece controles de recuperación y que cada control mantiene el catálogo
sin cambios no confirmados.

**Acceptance Scenarios**:

1. **Given** un fallo recuperable en la TUI, **When** se muestra el motivo, **Then** el usuario puede
   volver al nodo, editar la conexión o reintentar exclusivamente mediante teclado.
2. **Given** que el catálogo o la conexión cambia mientras se informa el fallo, **When** el usuario
   vuelve o reintenta, **Then** la aplicación no conecta silenciosamente a un objetivo distinto y aplica
   las protecciones de identidad/revisión existentes.
3. **Given** una cancelación iniciada por el usuario, **When** termina el intento, **Then** se presenta
   como cancelación y no como timeout, permiso denegado ni fallo inesperado.
4. **Given** que el usuario elige reintentar, **When** la conexión sigue existiendo, **Then** la TUI vuelve
   a validar y mostrar la confirmación actualizada antes de iniciar otro intento.
5. **Given** que la conexión fue eliminada o entra en conflicto antes de volver, editar o reintentar,
   **When** se resuelve el ID capturado, **Then** la TUI conserva el objetivo fallido como contexto, no
   inicia red ni persiste cambios y permite volver o recargar el catálogo.
6. **Given** que el usuario cancela la confirmación fresca de un reintento, **When** vuelve al diagnóstico,
   **Then** no se inicia red y permanecen disponibles las mismas acciones de recuperación.

---

### User Story 3 - Obtener el mismo diagnóstico en cada interfaz (Priority: P3)

Como usuario que alterna entre TUI y CLI, quiero que la misma causa produzca la misma categoría de error
para no interpretar resultados contradictorios.

**Why this priority**: La paridad de significado facilita soporte, automatización y reproducción del
problema, aunque la presentación concreta de cada interfaz sea distinta.

**Independent Test**: Se ejecuta la misma matriz controlada de fallos desde ambas interfaces y se compara
la categoría, el objetivo y la recomendación comunicados.

**Acceptance Scenarios**:

1. **Given** el mismo timeout, rechazo de autenticación o fallo de red, **When** se inicia desde TUI y
   CLI, **Then** ambas interfaces comunican la misma categoría y objetivo.
2. **Given** salida CLI legible o estructurada, **When** el inicio falla, **Then** ambas variantes incluyen
   el motivo seguro y conservan el código de salida no exitoso aplicable.

### Edge Cases

- El error llega envuelto por varias capas y contiene más de una descripción textual.
- El sistema operativo expresa el mismo fallo con textos diferentes o localizados.
- El timeout ocurre durante resolución, conexión, negociación, verificación o autenticación.
- El servidor ofrece una clave de host desconocida, cambiada o revocada y el usuario la rechaza.
- El agente, almacén seguro, archivo de identidad o frase de paso no están disponibles.
- La causa subyacente contiene una contraseña, frase de paso, material de clave u otro valor sensible.
- El intento se cancela al mismo tiempo que vence el plazo o se interrumpe la red.
- El terminal es estrecho, no admite color o el mensaje es más largo que el ancho disponible.
- La sesión llegó a estar activa y después terminó con estado remoto no cero.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Todo intento SSH que falle antes de que la sesión quede activa DEBE mostrar al usuario un
  motivo no vacío además de indicar que el inicio falló. Una sesión se considera activa cuando su
  `StartedAt` deja de ser cero o existe un estado remoto; desde ese punto se aplica FR-016.
- **FR-002**: El motivo DEBE pertenecer exactamente a una de estas categorías: tiempo agotado,
  autenticación denegada, conexión rechazada, host no encontrado, red inalcanzable, confianza de host,
  credencial/agente/clave no disponible, negociación SSH, cancelación o fallo inesperado.
- **FR-003**: Tiempo agotado y autenticación denegada DEBEN distinguirse siempre que la causa observada
  permita hacerlo; autenticación denegada DEBE usar `Permission denied` o una descripción equivalente.
- **FR-004**: El mensaje DEBE identificar la conexión mediante su ruta y, cuando se haya resuelto la
  conexión de catálogo, su endpoint `host:port` capturado, sin sustituirla por la selección actual si esta
  cambió. El endpoint NO incluye username, dirección remota resuelta, referencia de credencial ni ruta de
  identidad. Antes de resolver catálogo se conserva el selector en el error de gestión existente y no se
  crea un diagnóstico de inicio SSH.
- **FR-005**: Cada categoría específica DEBE ofrecer una recomendación breve y aplicable, como revisar
  red/host, credenciales, confianza, agente, archivo de clave o volver a intentar. Cuando exista, DEBE
  incluir además un detalle técnico sanitizado de hasta 256 puntos de código Unicode, en una sola línea y
  sin secretos. Si se trunca, DEBE conservar 255 puntos y añadir `…` como punto 256.
  En la TUI ese detalle DEBE empezar oculto y poder desplegarse/ocultarse mediante teclado; la categoría,
  etapa y recomendación permanecen siempre visibles. La CLI legible lo muestra y la salida estructurada
  lo incluye en un campo separado.
- **FR-006**: Los mensajes visibles, salidas estructuradas, errores, diagnósticos y logs NO DEBEN incluir
  contraseñas, frases de paso, material de clave privada, respuestas de autenticación ni secretos de sesión.
  Tampoco DEBEN incluir referencias de credencial, rutas de identidad ni texto arbitrario del servidor,
  sistema operativo, resolver, secure store, parser de claves o cadena de errores.
- **FR-007**: Una causa desconocida o no clasificable DEBE producir un fallback seguro que indique la
  etapa general si se conoce, incluya detalle técnico sanitizado cuando sea seguro y recomiende revisar
  configuración o reintentar, sin mostrar el error crudo completo.
- **FR-008**: Una cancelación explícita manejada dentro de la aplicación y que devuelve control al proceso
  DEBE identificarse como cancelación y NO DEBE clasificarse como timeout, autenticación denegada o fallo
  inesperado. SIGINT/SIGTERM u otra señal que termine el proceso conserva el código 130/143 aplicable y no
  requiere crear ni mostrar un diagnóstico `canceled`.
- **FR-009**: TUI, salida CLI legible y salida CLI estructurada DEBEN comunicar la misma categoría para
  una misma causa, aunque adapten el formato visual a cada interfaz.
- **FR-010**: Después de un fallo previo a sesión, la TUI DEBE permanecer estable y permitir mediante
  teclado volver al nodo, editar la conexión o reintentar sin aplicar cambios persistentes implícitos.
  Reintentar DEBE ser una acción explícita, volver a resolver/validar el objetivo y mostrar una nueva
  confirmación con ruta y endpoint actuales antes de cualquier operación de red.
  Si el ID ya no existe o la revisión entra en conflicto, la TUI DEBE conservar el snapshot fallido como
  contexto, no iniciar red ni mutar catálogo y ofrecer volver o recargar. Cancelar la confirmación fresca
  DEBE volver al diagnóstico sin iniciar red; cancelar una edición DEBE volver al diagnóstico o navegador
  estable sin persistir cambios.
- **FR-011**: La CLI DEBE conservar los códigos de salida existentes y comunicar el motivo por stderr. En
  `connect --json`, un fallo pre-activo DEBE emitir exactamente un objeto JSON en stderr con `ok:false` y
  `error.code`, `message`, `target`, `category`, `stage`, `recommendation`, más `endpoint` y
  `technicalDetail` solo cuando estén disponibles. Los prompts de confianza y el stream interactivo
  permanecen en stdout; no se emite envelope JSON de éxito. Los campos opcionales se omiten, nunca son null.
  `canceled` conserva `error.code=canceled`/exit 5; `host_trust`, `authentication_denied` y
  `credential_unavailable` conservan `security_failure`/exit 6; las demás categorías conservan
  `transport_failure`/exit 10. Not-found/conflict de catálogo y señales conservan sus estados existentes.
- **FR-012**: Cuando varias causas estén encadenadas, la categoría DEBE corresponder al fallo que impidió
  alcanzar la sesión activa. La precedencia DEBE ser: cancelación explícita manejada en la aplicación;
  diagnóstico ya normalizado; después la etapa registrada que falló junto con una causa tipada reconocida;
  finalmente fallback inesperado. Las etapas estables son `target_resolution`, `network_connection`,
  `host_trust`, `ssh_negotiation`, `credential`, `authentication`, `session_setup`, `local_terminal` y
  `unknown`. `target_resolution` significa resolución de nombre de red, no lookup del catálogo. Los wrappers
  exteriores y errores secundarios de cleanup NO DEBEN cambiar una clasificación reconocida ni combinar
  categorías incompatibles.
- **FR-013**: El motivo DEBE quedar visible o completamente escrito en stderr en menos de un segundo desde
  que `Connect` retorna después de cerrar recursos y restaurar el terminal, sin esperar una segunda
  operación de red. La medición controlada DEBE usar un reloj monotónico o inyectado entre ese retorno y la
  actualización del modelo/escritura completa; no incluye el tiempo de red anterior al retorno.
- **FR-014**: El mensaje y los controles de recuperación DEBEN seguir siendo legibles y operables a
  80x24, en modo reducido y sin depender del color. El control textual para desplegar/ocultar detalle
  DEBE estar disponible mediante teclado sin desplazar fuera de vista la categoría ni la recuperación. La
  modal DEBE poseer el foco de entrada mientras esté visible; el modo reducido prioriza categoría, target,
  etapa/recomendación y controles antes del detalle opcional.
- **FR-015**: Las decisiones existentes de confianza de host DEBEN conservar su contexto específico;
  una clave desconocida, rechazada, cambiada o revocada NO DEBE presentarse como `Permission denied`.
- **FR-016**: Un fallo posterior a que la sesión estuvo activa, incluido un estado remoto no cero, DEBE
  conservar el tratamiento de resultado de sesión existente y queda fuera de la clasificación de fallo
  de inicio.
- **FR-017**: Mostrar el motivo NO DEBE persistir nuevos datos de diagnóstico ni habilitar logging de
  causas sensibles; la información visible se limita al intento actual.

### Diagnostic Vocabulary

Cada categoría tiene un propósito y una recomendación controlados. El texto humano puede adaptarse al
idioma actual sin cambiar el identificador ni introducir texto de la causa.

| Categoría estable | Etapas permitidas | Resumen requerido | Recomendación requerida | Detalle técnico permitido |
|---|---|---|---|---|
| `timeout` | cualquier etapa conocida | La operación agotó el tiempo | Revisar conectividad/plazo y reintentar | `operation timed out` |
| `authentication_denied` | `authentication` | `Permission denied` o equivalente localizado | Revisar usuario y credenciales | `server rejected available authentication methods` |
| `connection_refused` | `network_connection` | El endpoint rechazó la conexión | Revisar host, puerto y servicio SSH | `remote endpoint refused connection` |
| `host_not_found` | `target_resolution` | No se resolvió el host | Revisar el nombre del host y DNS | `host name could not be resolved` |
| `network_unreachable` | `network_connection` | No existe ruta de red | Revisar red, VPN y routing | `network is unreachable` |
| `host_trust` | `host_trust` | No se aceptó la identidad del host | Revisar fingerprint y política de confianza | Solo `unknown`, `changed`, `revoked` o `rejected` |
| `credential_unavailable` | `credential`, `authentication` | No está disponible la credencial local | Revisar agente, almacén, clave o prompt | Solo `agent`, `secure store`, `identity file` o `secret prompt` |
| `ssh_negotiation` | `ssh_negotiation`, `session_setup` | Falló la negociación/configuración de sesión | Revisar compatibilidad SSH/PTY/shell | Solo `handshake`, `session`, `pty` o `shell` |
| `canceled` | cualquier etapa conocida | El intento fue cancelado | Reintentar cuando corresponda | `operation canceled` |
| `unexpected` | cualquier etapa o `unknown` | El inicio falló de forma no clasificada | Revisar configuración o reintentar | Ninguno |

El detalle se construye exclusivamente con las plantillas/valores de la tabla y campos públicos ya
permitidos (`host:port` y estado de confianza enumerado). DEBE normalizar UTF-8 y espacios y eliminar ANSI,
saltos, controles y caracteres bidireccionales. Nunca se pasa un `error.Error()` arbitrario, mensaje del
servidor, ruta, referencia de credencial o secreto al sanitizador como autorización para mostrarlo.

### Key Entities

- **Motivo de fallo de inicio**: Diagnóstico seguro con categoría estable, resumen visible, etapa general,
  recomendación y detalle técnico sanitizado opcional de una línea; nunca contiene secretos.
- **Objetivo del intento**: Identidad capturada de la conexión, compuesta por ruta y endpoint seguro,
  usada para que el mensaje no dependa de una selección posterior.
- **Acción de recuperación**: Opción disponible después del fallo, como volver, editar o reintentar, que
  no modifica el catálogo por sí misma.

### Scope Boundaries

**Included**:

- Fallos desde que el usuario confirma la conexión hasta antes de que la sesión SSH quede activa.
- Presentación segura y consistente en TUI, CLI legible y CLI estructurada.
- Recuperación de la TUI y conservación del objetivo original.

**Excluded**:

- Cambios en autenticación, verificación de host, timeouts o política de reintentos.
  La nueva acción manual de reintento está incluida; siguen excluidos los reintentos automáticos, backoff y
  cambios en cantidad o duración de intentos.
- Mostrar errores crudos completos del sistema operativo, librería SSH o servidor; solo se admite el
  detalle sanitizado y acotado definido en FR-005.
- Nuevos archivos de log, telemetría, historial de diagnósticos o persistencia de errores.
- Interpretar como fallo de inicio una sesión que estuvo activa y terminó después.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de una matriz controlada que cubra las diez categorías produce exactamente la
  categoría esperada, un mensaje no vacío y el objetivo correcto.
- **SC-002**: Timeout, `Permission denied`, conexión rechazada, host no encontrado, red inalcanzable,
  confianza de host y cancelación son distinguibles entre sí en el 100 % de sus escenarios controlados.
- **SC-003**: El 100 % de pruebas con canarios en contraseñas, frases de paso, claves y causas envueltas
  termina sin que el canario aparezca en vistas, salidas, detalles técnicos sanitizados, errores,
  diagnósticos ni logs; todos los detalles visibles respetan una línea y 256 puntos de código Unicode.
- **SC-004**: En el 100 % de fallos controlados, el intervalo monotónico/inyectado entre el retorno de
  `Connect` con recursos cerrados y la actualización visible o escritura completa en stderr es menor a un
  segundo, sin segunda operación de red.
- **SC-005**: En el 100 % de fallos previos a sesión probados en TUI, el usuario puede volver, editar o
  reintentar con teclado; cada reintento vuelve a validar/confirmar el objetivo y nunca usa una revisión
  obsoleta ni inicia red antes de la confirmación.
- **SC-006**: Para una misma causa controlada, TUI, CLI legible y CLI estructurada comunican la misma
  categoría en el 100 % de los casos.
- **SC-007**: El 100 % de causas desconocidas produce el fallback seguro definido, sin mensaje vacío,
  panic, terminación abrupta ni exposición del error crudo.

## Assumptions

- El flujo existente puede determinar si una sesión llegó a estar activa y conserva el objetivo
  original del intento.
- Las causas conocidas pueden reconocerse por su significado o etapa; cuando no sea seguro clasificarlas
  se usa el fallback inesperado en lugar de adivinar.
- La presentación sigue el idioma actual de la aplicación; esta feature no introduce localización.
- Los códigos de salida CLI existentes se mantienen sin cambios en esta feature; las diez categorías son
  campos adicionales y no redefinen `error.code` ni el estado de proceso.
- Las protecciones existentes de host key, secretos, terminal y revisión del catálogo permanecen vigentes.
- Las plataformas soportadas para esta feature son Windows, Linux y macOS en amd64/arm64; la presentación
  requiere un terminal de al menos 80x24 para el layout completo y conserva un modo reducido por debajo.
