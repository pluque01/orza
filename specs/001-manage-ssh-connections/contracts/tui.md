# TUI Contract: Gestión interactiva

**Date**: 2026-07-29

## Entry and Exit

- `orza` sin subcomando abre la TUI solo si stdin y stdout son terminales.
- `Ctrl-C` o la acción `quit` salen desde una pantalla estable sin modificar datos pendientes.
- Una sesión SSH establecida toma el terminal después de que Bubble Tea restaure su estado. Al terminar
  la sesión se restaura de nuevo el terminal y el proceso devuelve su resultado.
- Un fallo anterior al establecimiento de la sesión vuelve a la pantalla estable desde la que se inició.

## Minimum Terminal

El tamaño mínimo soportado es 80 columnas por 24 filas. Por debajo de ese tamaño, la TUI reemplaza el
contenido por un mensaje que indica el mínimo y mantiene disponibles `help`, `back` y `quit`. No se
truncan confirmaciones destructivas ni huellas de host para forzarlas a caber.

## Global Keys

| Action | Keys | Behavior |
|--------|------|----------|
| Move selection | Arrow keys or `j`/`k` | Cambia el elemento activo sin ejecutar acciones. |
| Enter/open | `Enter` | Abre carpeta, detalles o confirma la acción primaria no destructiva. |
| Back/cancel | `Esc` | Cancela formulario/modal o sube un nivel. |
| Help | `?` | Muestra acciones disponibles para la pantalla actual. |
| Quit | `q`, `Ctrl-C` | Sale si no hay modal o edición; con cambios pendientes pide descartarlos. |
| Focus | `Tab`, `Shift-Tab` | Recorre todos los controles editables. |

No existe una acción disponible solo por color. Se respeta `NO_COLOR`; selección, error y estado usan
texto o símbolos ASCII además de estilo.

## Screens

### Catalog Browser

- Muestra breadcrumb de ruta, carpetas antes que conexiones y el elemento seleccionado.
- El panel de detalle muestra ID, revisión, host, puerto, usuario y método, nunca secretos.
- Acciones: crear carpeta, crear conexión, abrir, editar, mover, eliminar, conectar, recargar y salir.
- El estado vacío explica cómo crear el primer elemento.
- Una revisión global distinta muestra “catalog changed” y permite recargar sin perder un formulario;
  guardar con revisión obsoleta devuelve conflicto.

### Connection Form

- Campos: nombre, carpeta, host, puerto, usuario, método y referencia de clave cuando aplique.
- Cambiar método actualiza los campos visibles sin borrar la configuración anterior hasta confirmar.
- Para contraseña, la opción de recordar empieza desmarcada y nombra el almacén nativo del sistema.
- Validación de campo aparece junto al campo y el foco puede volver a corregirlo.
- `Esc` descarta solo tras confirmar si existe algún cambio.

### Folder Form and Move Picker

- Crear o renombrar valida el espacio de nombres compartido.
- El selector de destino oculta o deshabilita el propio subárbol para movimientos de carpetas.
- La ruta completa de origen y destino permanece visible antes de confirmar.

### Destructive Confirmation

- Muestra acción, ruta y objetivo no ambiguos.
- Eliminar conexión con credencial indica que también se borrará la contraseña guardada.
- Eliminar carpeta recursivamente muestra número de carpetas, conexiones y credenciales afectadas.
- La opción inicial es cancelar; confirmar requiere una tecla explícita documentada.
- Si cambian membresía o revisiones, se cancela la confirmación y se solicita recargar.

### Host Trust Prompt

- Aparece antes de solicitar contraseña o frase de paso.
- Muestra host, puerto, dirección remota, algoritmo y huella SHA-256.
- Para clave cambiada muestra también la huella conocida y una advertencia de posible ataque.
- Opciones: rechazar, confiar una vez y confiar y persistir; rechazar está seleccionado por defecto.
- Una clave revocada solo permite volver o salir.

### Secret Prompt

- La entrada no tiene eco ni queda en historial, modelo de vista, logs o mensajes de error.
- Una frase de paso nunca se persiste.
- Guardar contraseña requiere una segunda decisión de consentimiento desmarcada por defecto.
- Si el almacén seguro está bloqueado o ausente, se permite continuar solo para esta sesión.

### Recoverable Error

- Incluye operación, objetivo, causa segura y una acción concreta.
- Ofrece reintentar, recargar, volver o salir según corresponda.
- Un conflicto nunca fusiona ni sobrescribe automáticamente.
- Un fallo de saga bloquea nuevas mutaciones sobre esa conexión hasta recuperar o compensar.

## Session Transition

```text
TUI stable screen
  -> restore Bubble Tea terminal
  -> verify host and authenticate
  -> enter raw session mode
  -> stream remote session
  -> restore terminal
  -> exit with remote/local outcome
```

Si verificación o autenticación falla antes de `active`, se reinicia la TUI con el error recuperable.
Después de `active`, el gestor no necesita permanecer visible: al cerrar la sesión termina con el estado
remoto o local correspondiente. `Ctrl-C` durante raw mode se envía al host remoto; no mata localmente la
sesión. La cancelación local usa una acción dedicada documentada en ayuda.

## Resize and Rendering

- Todo resize recalcula layout sin perder selección, formulario ni modal.
- Durante SSH se propaga el nuevo tamaño al PTY remoto.
- Las vistas limitan el contenido a las dimensiones recibidas y no generan crecimiento de memoria.
- Texto largo usa viewport desplazable; huellas y rutas completas se pueden copiar o inspeccionar sin
  depender de truncamiento silencioso.

## State Ownership

- El modelo TUI posee selección, formulario y navegación, pero no reglas de dominio.
- Cada operación larga se representa con un único ID y resultado; mensajes tardíos de una operación
  cancelada se ignoran.
- Solo el componente de sesión puede poseer una conexión SSH activa.
- Todas las mutaciones pasan por los mismos casos de uso y revisiones que los comandos CLI.

## TUI Acceptance Matrix

| Flow | Required result |
|------|-----------------|
| Empty catalog | Help visible and first connection can be created by keyboard. |
| Narrow terminal | Minimum-size message, back/help/quit remain usable. |
| CRUD success | Browser reflects committed path and revision. |
| Validation failure | Form remains editable; catalog unchanged. |
| Stale edit | Conflict shown; confirmed external change preserved. |
| Unknown host | Explicit fingerprint decision before any credential. |
| Credential store unavailable | Session-only password remains possible; no persistence claimed. |
| SSH setup failure | Stable TUI restored with actionable error. |
| SSH session end | Terminal restored and outcome returned. |
| No color | Every action and state remains distinguishable. |
