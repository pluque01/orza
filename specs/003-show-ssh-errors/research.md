# Research: Motivos de error al iniciar SSH

## Decisión 1: Modelo tipado en la capa de aplicación

**Decision**: Definir `SSHStartError`, `SSHFailureReason`, `SSHFailureStage` y una proyección cause-free en
`internal/app`. El adaptador SSH construye el wrapper mediante un constructor seguro y conserva debajo su
`sshclient.Error` y causa original para `errors.Is/As`. `Error()` devuelve solo texto controlado.

**Rationale**: `sshclient` ya depende de `app`, mientras que TUI/CLI también consumen aplicación. Un único
modelo evita ciclos y divergencia sin crear otro paquete.

**Alternatives considered**:

- Clasificar en TUI/CLI: descartado por duplicación y pérdida de etapa.
- Añadir diez valores a `ErrorKind`: descartado porque mezclaría CRUD con fallos SSH.
- Nuevo `internal/sshdiag`: válido, pero innecesario con la dependencia existente.

## Decisión 2: Precedencia por etapa y causa tipada

**Decision**: Aplicar clasificación solo cuando `StartedAt` es cero y no hay remote status. Precedencia:
cancelación explícita; diagnóstico ya normalizado; etapa bloqueante con causa reconocida; `unexpected`.
Deadline es `timeout`, no cancelación. Wrappers exteriores y cleanup joined no sustituyen la causa primaria.

**Rationale**: La implementación actual conserva la mayoría de causas con `Unwrap`, pero hoy confunde
deadline/cancel y puede perder etapa. La regla cumple la aclaración y mantiene resultados post-activos.

**Alternatives considered**:

- Causa más interna: descartada porque puede ser cleanup secundario.
- Error exterior: descartado porque suele ser un wrapper genérico.
- Parsear textos localizados: descartado por fragilidad y riesgo de secretos.

## Decisión 3: Conversión de causas conocidas

**Decision**: Mapear `net.DNSError`, timeout tipado y errno refusal/unreachable en el adaptador de red;
errores de host trust en el gate donde aún se conoce estado; sentinels de agente/clave/prompt como
`credential_unavailable`; negociación/PTY/shell pre-activos como `ssh_negotiation`. Crear un sentinel
privado de autenticación denegada cuando al menos un método real fue ofrecido/rechazado y el callback queda
agotado, dando precedencia a fallos locales de callback y sin parsear textos de servidor o sistema.

**Rationale**: DNS, errno, algorithm negotiation, host trust y credenciales ya conservan información
tipada. x/crypto no exporta un tipo de rechazo de cliente y el código actual lo reemplaza por “methods
exhausted”; el sentinel por agotamiento posterior a un método ofrecido recupera el significado sin parsear
ni exponer ese texto.

**Alternatives considered**:

- Fork de x/crypto: descartado por tamaño/riesgo.
- Considerar todo handshake como permission denied: descartado por falsos diagnósticos.
- Separar resolución DNS de dial en producción: descartado porque cambiaría semántica fuera del scope.

## Decisión 4: Detalle técnico por allowlist

**Decision**: Generar detalle solo desde templates y campos públicos explícitamente aprobados. Sanitizar
UTF-8, ANSI, saltos/controles/bidi, whitespace y patrones sensibles; máximo 256 puntos de código Unicode
(255 más `…`). Las únicas entradas son plantillas y valores enumerados en `spec.md`; causas desconocidas no
tienen detalle. Nunca pasar `err.Error()` arbitrario al sanitizador como permiso de salida.

**Rationale**: Regex/truncamiento no puede saber si un texto contiene contraseña, clave, respuesta o
mensaje server-controlled. La allowlist satisface el detalle solicitado sin debilitar redacción actual.

**Alternatives considered**:

- Truncar causa cruda: descartado por fuga.
- No mostrar detalle: contradice la aclaración C.
- Persistir detalle para soporte: fuera de scope y contrario a FR-017.

## Decisión 5: Recuperación TUI por target capturado

**Decision**: La modal conserva ID/revisión/ruta/endpoint del intento y un diagnóstico seguro. `d` alterna
detalle, inicialmente oculto; `r` resuelve por ID y abre confirmación fresca antes de red; `e` resuelve por
ID y abre el formulario actual; `Esc` vuelve al nodo si existe; `q` sale. Retry muestra anterior/actual si
cambió y usa revisión actual solo tras `y`.

**Rationale**: Hoy retry usa la selección corriente y llama directamente a sesión. El flujo elegido evita
conectar a otro target y reutiliza CAS/confirmación de feature 002.

**Alternatives considered**:

- Retry inmediato: rechazado por aclaración y seguridad.
- Guardar snapshot editable viejo: rechazado por sobrescritura obsoleta.
- Solo back/edit: rechazado por aclaración B.

## Decisión 6: Contrato CLI estructurado compatible

**Decision**: Permitir `--json connect`. Ante fallo pre-activo, stderr contiene un único objeto JSON con
campos existentes `ok/error.code/message/target` más `endpoint`, `category`, `stage`, `recommendation` y
`technicalDetail` opcional. stdout mantiene prompts/stream; no hay envelope JSON de éxito. Códigos/exit
existentes se conservan y remote exit sigue propagándose.

**Rationale**: Cumple salida estructurada sin reinterpretar `error.code` ni mezclar JSON con la sesión.
Los campos nuevos son aditivos; esta feature sustituye expresamente la antigua prohibición de JSON connect.

**Alternatives considered**:

- Nuevo `--error-format=json`: descartado por duplicar mecanismos.
- Reusar `error.code` para diez categorías: descartado por incompatibilidad.
- JSON en stdout: descartado porque corrompería el stream interactivo.

## Decisión 7: Estrategia de pruebas

**Decision**: Matriz table-driven de diez categorías; tests de precedence/wrapping, causas tipadas por
plataforma, rechazo real en servidor SSH controlado, fuzz del sanitizer, paridad TUI/human/JSON, retry
CAS, detail toggle/narrow views, canarios y regresión remote exit. Inyección reemplaza timeouts/red pública.

**Rationale**: Cubre clasificación y fallo principal sin depender de textos del SO, reloj real o Internet.

**Alternatives considered**:

- Solo integración real: descartada por no determinista.
- Solo unit tests: descartados porque no prueban el rechazo concreto de x/crypto ni presentación final.
- Snapshots completos de UI: complementarios, pero insuficientes para causes/precedence.

## Riesgos y supuestos de seguridad

- Preservar una causa con `Unwrap` no autoriza mostrarla ni loguearla.
- Mensajes del servidor, resolver, secure store y parser de claves se consideran no confiables.
- `technicalDetail` es opcional y vacío es preferible a una clasificación insegura.
- Señales que terminan el proceso conservan salida 130/143 y no prometen un diagnóstico cancelado.
- Actualizar x/crypto exige revisar la estrategia de sentinel para authentication denied.
- `target_resolution` significa DNS/resolución de nombre; lookup/conflicto de catálogo queda fuera del
  diagnóstico SSH y conserva su error de gestión.

No quedan incógnitas técnicas para la implementación.
