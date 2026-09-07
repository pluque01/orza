# Research: Árbol contextual y pegado en TUI

## Decisión 1: Proyección completa del catálogo

**Decision**: Al abrir o recargar el navegador, obtener la raíz y recorrer carpetas por niveles mediante
`FolderService.Get` y `FolderService.List`. Construir un snapshot inmutable por respuesta con índices
`nodeByID`, `childrenByParent` y `parentByID`; derivar las filas visibles en preorder según expansión.
Ordenar con el contrato que ya entrega `ListChildren`: carpetas antes que conexiones, nombre mediante
comparación binaria sensible a mayúsculas y después ID. La raíz empieza expandida/seleccionada y las
demás carpetas contraídas.

**Rationale**: La TUI ya dispone de esos límites y la escala máxima es 100 carpetas y 1.000 conexiones.
Mantener la proyección en presentación evita modificar el modelo persistente o duplicar reglas de
catálogo. El estado visible se recalcula en memoria sin I/O al expandir o contraer.

**Alternatives considered**:

- Navegación por carpeta actual: descartada porque no representa simultáneamente padres e hijos.
- CTE recursiva nueva expuesta por aplicación: reservada como optimización si el benchmark de recarga
  supera un segundo; no es necesaria para la semántica.
- Árbol de punteros mutable: descartado por complicar recargas, identidad y pruebas.

## Decisión 2: Identidad, expansión y recuperación de selección

**Decision**: Guardar selección y expansión por `app.NodeID`, no por índice ni ruta. La raíz usa una
identidad de sesión explícita aunque el catálogo la represente con ID vacío. Antes de recargar se
captura la cadena de ancestros del seleccionado. Tras una respuesta, se conserva el mismo ID si existe;
si no, se elige el primer ancestro todavía existente y finalmente la raíz. Después de crear se expanden
todos los ancestros del ID devuelto; renombrar y mover preservan el ID; eliminar usa la cadena capturada.
Los IDs de expansión inexistentes se podan en cada snapshot.

**Rationale**: Las rutas cambian al mover/renombrar y los índices cambian al expandir u ordenar. `NodeID`
es la identidad estable que ya usa la aplicación y permite reconciliación determinista.

**Alternatives considered**:

- Índice de fila: descartado porque puede apuntar a otro nodo tras cualquier cambio.
- Ruta lógica: descartada porque cambia al renombrar o mover.
- Seleccionar siempre la primera fila: descartada porque rompe continuidad y requisitos FR-009/010/011.

## Decisión 3: Acciones contextuales y concurrencia

**Decision**: Resolver cada acción desde el nodo seleccionado del snapshot. `n` usa el ID/ruta de la
carpeta seleccionada o el padre de una conexión; `f` usa la carpeta seleccionada o el padre de la
conexión. Renombrar, mover y eliminar usan ID y revisión mostrados; conectar solo acepta conexiones y
exige confirmación con ruta/endpoint y `ConnectRequest.Expected` antes de abrir red. El servicio vuelve
a comparar la revisión tras resolver el ID y antes del runner. `f` crea dentro de raíz/carpeta o como hermano
de una conexión. Antes de confirmar una operación destructiva se muestra la ruta completa capturada. Los conflictos o
nodos ausentes producen un error recuperable y una recarga, nunca una operación sobre otra fila.

Crear transporta revisión y ruta capturadas del padre; mover transporta revisión/ruta de origen y destino.
El repositorio compara todo dentro de `BEGIN IMMEDIATE`, por lo que renombrar/mover un ancestro invalida
la intención sin convertir cambios no relacionados en conflictos globales.

**Rationale**: Reutiliza el control optimista existente y vincula la intención al nodo que el usuario vio,
incluso si otra instancia cambia el catálogo.

**Alternatives considered**:

- Resolver por la posición actual al recibir la respuesta: descartado por riesgo de actuar sobre otro nodo.
- Bloquear el catálogo durante formularios: descartado; el producto admite varias instancias.

## Decisión 4: Pegado en campos normales

**Decision**: Añadir un adaptador `textField` alrededor de `textinput.Model`. El adaptador conserva
ancla de selección y viewport, delega la edición ordinaria cuando no hay selección, intercepta
Shift+Left/Right/Home/End y procesa exclusivamente `tea.PasteMsg` como pegado. Su keymap deshabilita la
acción Paste de Bubbles para que `Ctrl+V` nunca invoque `clipboard.ReadAll`. `model.Update` enruta el
mensaje solo al campo editable con foco antes de evaluar atajos globales.

**Rationale**: Bubble Tea v2 activa bracketed paste y emite `PasteStartMsg`, un `PasteMsg` atómico y
`PasteEndMsg`; el contenido no se convierte en `KeyPressMsg`. Bubbles ya resuelve escritura y cursor,
pero no ofrece selección y su binding `Ctrl+V` accede al portapapeles del SO. Un adaptador pequeño
preserva lo útil y hace explícitas las garantías nuevas.

**Alternatives considered**:

- Mantener `textinput.Model` sin adaptador: descartado porque carece de selección y lee el portapapeles.
- Editor de texto completo propio para metadatos: descartado porque duplicaría edición Unicode y cursor.
- Leer el portapapeles por API del SO: excluido expresamente por FR-012.

## Decisión 5: Normalización y atomicidad del payload

**Decision**: Inspeccionar el payload completo antes de cambiar valor, cursor o selección. Eliminar CR,
LF, NEL (`U+0085`), `U+2028` y `U+2029`; si queda cualquier rune para la que `unicode.IsControl` sea true,
rechazar el payload completo con un mensaje genérico. Rechazar candidatos de más de 4.096 runes normales
o 4.096 bytes secretos y aplicar después reglas inmediatas; nunca truncar parcialmente. Un payload vacío normalizado no cambia estado. Si se acepta, reemplaza
la selección o se inserta en el cursor como una sola transición y borra el error anterior del campo.

**Rationale**: Evita que tab, escape, Ctrl+C o secuencias parecidas a atajos produzcan efectos laterales
y garantiza que entrada manual y pegada llegan a la misma validación de formulario.

**Alternatives considered**:

- Procesar runes como teclas: descartado porque permitiría enviar/cancelar/navegar.
- Sustituir controles por espacios: descartado porque oculta contenido no admitido.
- Insertar el prefijo válido: descartado porque viola el rechazo atómico de FR-015.

## Decisión 6: Editor dedicado para secretos

**Decision**: Mantener `Terminal.ReadSecret` y sustituir `term.ReadPassword` por un editor no-echo en
`internal/terminal`. Usar la versión ya fijada de `github.com/charmbracelet/ultraviolet`:

```go
con := uv.NewConsole(input, output, os.Environ())
_, err := con.MakeRaw()
cancelReader, err := uv.NewCancelReader(con.Reader())
reader := uv.NewTerminalReader(cancelReader, os.Getenv("TERM"))
reader.SetLogger(nil)
```

Activar y desactivar bracketed paste con `uv.EncodeBracketedPaste`. Aceptar `uv.PasteEvent` con la misma
política atómica, y `uv.KeyPressEvent` para cursor, selección, edición, Enter real y cancelación real.
Renderizar solo caracteres de máscara y selección visual de máscaras. Ignorar release, mouse,
clipboard y eventos desconocidos. El prompt conserva propiedad hasta completar cleanup y entonces
transfiere bytes temporalmente a autenticación; solo `Remember password` autoriza una copia en el almacén
seguro. El buffer propio se limpia al cancelar/fallar y los propietarios posteriores lo limpian tras uso;
ninguna estructura de diagnóstico, error o log incluye el valor.

**Rationale**: `term.ReadPassword` no distingue pegado, Enter pegado, cursor ni selección. Usar
`textinput.EchoPassword` introduciría el secreto en Bubble Tea, cuyo tracing puede registrar entrada.
Ultraviolet ya está transitivamente fijada, analiza VT y consola Windows y permite logger nil.

**Alternatives considered**:

- `term.ReadPassword`: descartado por incumplir FR-013/014/015.
- Password input de Bubbles: descartado por ampliar la exposición del secreto y mantener `Ctrl+V` de SO.
- Parser ANSI/Windows propio: descartado por complejidad y riesgo de restauración/protocolo.

## Decisión 7: Ciclo de vida y capacidades de terminal

**Decision**: `ReadSecret` captura estado, entra en raw, crea lector cancelable, activa bracketed paste y
lanza `StreamEvents` con logger nil. En submit, cancelación, contexto, SIGINT/SIGTERM o error: cancelar el reader, esperar
su goroutine, cerrarlo, desactivar bracketed paste, limpiar estilo/mostrar cursor, emitir newline y
restaurar el snapshot usando un contexto sin cancelar. Los errores se unen sin omitir cleanup; si falla
la restauración después de submit, limpiar el resultado y devolver error.

En Windows, el lector nativo cancelable solo se usa cuando la entrada es `os.Stdin`; una consola
interactiva con otro handle falla antes de raw mode y no ofrece degradación insegura. Como
`uv.NewCancelReader` puede devolver un fallback silencioso, el adaptador certifica el tipo nativo
`conInputReader` de la versión fijada y rechaza cualquier otro antes de capturar/modificar modos. En terminales que no respetan bracketed paste no se
puede distinguir protocolariamente pegado de escritura: el formulario queda usable, pero la garantía
de que un Enter pegado no envía requiere soporte bracketed paste y se documentará.

**Rationale**: `TerminalReader.StreamEvents` no cierra su canal y necesita cancelación explícita para
desbloquear la lectura. La restauración determinista es un gate constitucional.

El contexto raíz creado en `cmd/orza/main.go` posee las señales. La cancelación llega al prompt, que
restaura antes de devolver; después el proceso termina con 130 para `os.Interrupt`/SIGINT y 143 para
SIGTERM Unix. Windows valida `os.Interrupt` y cancelación de contexto, no una señal SIGTERM inexistente.

**Alternatives considered**:

- Confiar solo en cancelación de contexto: descartado porque puede dejar bloqueada la lectura.
- Continuar con handles Windows no cancelables: descartado porque puede competir con la restauración.
- Terminal Ultraviolet completo: descartado; activa render/resize/screen no necesarios para una línea.

## Decisión 8: Pruebas y rendimiento

**Decision**: Probar índices y reconciliación como lógica pura; modelo y formularios con mensajes Bubble
Tea; política de paste con ASCII, Unicode, ancho variable, >1.000 caracteres y controles; secreto con
fuentes/sumideros inyectados y canarios; PTY para secuencias/modos/restauración Unix; compilación y tests
nativos Windows para consola. Añadir benchmark con 100 carpetas, 1.000 conexiones y profundidad 10 en el
runner de referencia mediante `BenchmarkTreeRefresh`, `BenchmarkTreeExpand` y `BenchmarkTreeCollapse`.
Cada benchmark ejecuta un warm-up interno, reinicia su temporizador y mide exactamente 20 iteraciones;
19 deben quedar bajo un segundo en la evidencia de aceptación.

**Rationale**: Separa semántica determinista de capacidades reales del terminal y cubre el principal
fallo de cada frontera. El benchmark decide objetivamente si los servicios existentes necesitan una
consulta agregada.

**Alternatives considered**:

- Solo pruebas manuales: descartado por los gates de input y cleanup.
- Solo golden snapshots: descartado porque no prueban identidad, acciones ni invariantes de seguridad.

## Riesgos de seguridad aceptados

- `uv.PasteEvent.Content` es un `string` de Go que no puede borrarse de forma fiable; nunca se registra
  ni formatea y se limita el valor aceptado, pero pueden existir copias temporales administradas por GC.
- El acumulador de paste del parser es previo al límite de aplicación. El terminal local forma parte del
  límite de confianza operativo; una defensa estricta contra payload ilimitado requeriría parser propio.
- El enmascarado filtra longitud aproximada y timing, como otros prompts interactivos locales.
- Ningún proceso puede restaurar el terminal tras `SIGKILL` o terminación abrupta de la máquina.

No quedan incógnitas técnicas para la implementación.
