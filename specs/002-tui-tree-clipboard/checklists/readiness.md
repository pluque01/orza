# Checklist de preparación de requisitos: Árbol contextual y pegado en TUI

**Purpose**: Evaluar si los requisitos del árbol, las acciones contextuales, el pegado y la entrada secreta son completos, claros, consistentes y medibles antes de generar tareas o implementar.
**Created**: 2026-07-30
**Feature**: [spec.md](../spec.md)

**Note**: Este checklist trata los requisitos como código escrito en lenguaje natural. No valida la implementación ni sustituye un plan de pruebas.

## Completitud de requisitos

- [x] CHK001 ¿Se enumeran de forma cerrada todos los controles editables cubiertos por “todo control”, incluidos los campos condicionales y cada tipo de prompt secreto? [Completeness, Spec §Definitions and Closed Inventories, Spec §FR-012, Spec §SC-002]
- [x] CHK002 ¿Se define el orden de hermanos para carpetas y conexiones, incluido el desempate entre nombres que solo difieren por mayúsculas? [Resolved, Spec §FR-001]
- [x] CHK003 ¿Se especifican el estado de expansión inicial y el nodo inicialmente seleccionado tanto para un catálogo vacío como para uno poblado? [Resolved, Spec §FR-003–FR-004]
- [x] CHK004 ¿Están definidos los requisitos contextuales de creación de carpetas desde raíz, carpeta y conexión con el mismo detalle que la creación de conexiones? [Resolved, Spec §FR-021]
- [x] CHK005 ¿Se documentan todas las acciones válidas e inválidas por tipo de nodo, incluida la raíz, y el resultado requerido de una acción no aplicable? [Resolved, Spec §FR-006–FR-010, Spec §FR-021–FR-022, Contract §Tree Keymap]

## Claridad de requisitos

- [x] CHK006 ¿Se cuantifica la “separación consistente” de la indentación o se define una regla determinista por profundidad y terminal estrecho? [Resolved, Spec §FR-002, Spec §FR-019]
- [x] CHK007 ¿Se definen sin ambigüedad los símbolos o propiedades textuales que distinguen raíz, carpeta, conexión, expansión y selección sin color? [Resolved, Spec §FR-005]
- [x] CHK008 ¿Se aclara qué significa que un nodo “cambió desde que se mostró” y qué cambios invalidan cada acción contextual? [Resolved, Spec §FR-006]
- [x] CHK009 ¿Se define qué posiciones de cursor constituyen una selección activa y cómo se crea, extiende o colapsa mediante teclado? [Resolved, Spec §FR-013]
- [x] CHK010 ¿Se define “salto de línea” para incluir o excluir CR, LF, CRLF y separadores Unicode de línea/párrafo? [Resolved, Spec §FR-015]
- [x] CHK011 ¿Se identifica el conjunto exacto de caracteres de control no admitidos y se distingue de texto Unicode válido de ancho cero o variable? [Resolved, Spec §FR-015]
- [x] CHK012 ¿Se especifica qué información puede contener un “error seguro” y dónde debe mostrarse sin incluir el payload rechazado? [Resolved, Spec §FR-015, Spec §Definitions and Closed Inventories]

## Consistencia de requisitos

- [x] CHK013 ¿Son coherentes la garantía de pegado para el 100 % de controles y la limitación declarada de que el aislamiento completo requiere terminal con bracketed paste? [Resolved, Spec §FR-012, Spec §SC-003, Contract §Terminal Capability Contract]
- [x] CHK014 ¿Es coherente considerar la raíz como selección en todos los catálogos con la redacción que limita ese papel al catálogo vacío? [Resolved, Spec §FR-004, Contract §Catalog Tree]
- [x] CHK015 ¿Coinciden la especificación y el contrato sobre las teclas que expanden, contraen, conectan, vuelven al padre y salen, sin asignaciones incompatibles según pantalla? [Resolved, Spec §FR-003, Spec §FR-019, Contract §Tree Keymap]
- [x] CHK016 ¿Son compatibles el requisito de mantener formularios abiertos intactos y la necesidad de invalidar destinos o revisiones que cambian durante esos formularios? [Resolved, Spec §FR-006, Spec §FR-020]

## Calidad de criterios de aceptación

- [x] CHK017 ¿Define SC-001 una ejecución y denominador deterministas en lugar de porcentajes de usuarios sin protocolo? [Resolved, Spec §SC-001]
- [x] CHK018 ¿Define SC-002 el inventario de controles, corpus ASCII/Unicode y criterio de equivalencia entre entrada manual y pegada? [Resolved, Spec §Definitions and Closed Inventories, Spec §SC-002]
- [x] CHK019 ¿Define SC-003 un conjunto representativo y acotado de atajos, modificadores y payloads para que el “100 %” sea objetivamente interpretable? [Resolved, Spec §Definitions and Closed Inventories, Spec §SC-003]
- [x] CHK020 ¿Define SC-004 entorno de referencia, operación medida, inicio/fin, número de intentos y distribución exacta de los 1.100 elementos? [Resolved, Spec §SC-004, Plan §Performance Goals]
- [x] CHK021 ¿Define SC-006 una secuencia, configuración sin color y criterio observable deterministas? [Resolved, Spec §SC-006]
- [x] CHK022 ¿Delimita SC-007 todas las superficies consideradas vistas, errores, diagnósticos, historial y logs, incluido tracing de dependencias y fallos de terminal? [Resolved, Spec §Definitions and Closed Inventories, Spec §SC-007]

## Cobertura de escenarios

- [x] CHK023 ¿Se define el flujo alternativo cuando contraer una carpeta ocultaría el nodo seleccionado, incluida la nueva selección requerida? [Resolved, Spec §FR-004]
- [x] CHK024 ¿Se especifica la recuperación cuando una mutación o recarga falla, incluida la conservación del último árbol válido, formulario y selección? [Resolved, Spec §FR-006, Spec §FR-020, Spec §FR-025]
- [x] CHK025 ¿Se define el resultado cuando el nodo seleccionado y varios ancestros desaparecen durante una operación, no solo después de una eliminación local confirmada? [Resolved, Spec §FR-010, Spec §FR-025]
- [x] CHK026 ¿Se cubren por requisitos separados el pegado aceptado, vacío, cancelado, no soportado, normalizado y rechazado, con estado posterior inequívoco para cada clase? [Resolved, Spec §FR-012–FR-018]
- [x] CHK027 ¿Se especifican submit, cancelación, error de lectura, error de salida, cancelación de contexto y fallo de restauración para prompts secretos? [Resolved, Spec §FR-023, Spec §SC-008]

## Cobertura de casos límite

- [x] CHK028 ¿Se define el resultado requerido para payloads mayores de 1.000 caracteres, en vez de limitarse a mencionarlos como caso límite? [Resolved, Spec §FR-016]
- [x] CHK029 ¿Se exige que el rechazo preserve también cursor y selección, además del “valor anterior” indicado expresamente? [Resolved, Spec §FR-015]
- [x] CHK030 ¿Se define el comportamiento para terminales menores de 80x24, sin color o sin reporte de modificadores Shift, incluida una alternativa accesible para selección de texto? [Resolved, Spec §Definitions and Closed Inventories, Spec §FR-013, Spec §FR-019]

## Requisitos no funcionales

- [x] CHK031 ¿Se especifica un límite de memoria o payload para evitar crecimiento no acotado durante un pegado antes y después de su validación? [Resolved, Spec §FR-016 y Spec §Scope Boundaries documentan el límite de aplicación y la exclusión previa; Research §Riesgos de seguridad aceptados documenta el riesgo]
- [x] CHK032 ¿Se documentan requisitos explícitos de restauración de raw mode, bracketed paste, cursor y estilos para todas las salidas del editor secreto? [Resolved, Spec §FR-023, Spec §SC-008]
- [x] CHK033 ¿Se especifican los niveles mínimos de terminal/VT/bracketed-paste soportados por plataforma y el comportamiento requerido fuera de esa matriz? [Resolved, Spec §Definitions and Closed Inventories, Spec §FR-012]
- [x] CHK034 ¿Son objetivamente evaluables la operabilidad exclusiva por teclado, la legibilidad sin color y la usabilidad en el tamaño mínimo documentado? [Resolved, Spec §FR-019, Spec §SC-006]

## Dependencias y supuestos

- [x] CHK035 ¿Se valida o convierte en requisito el supuesto de que los servicios compartidos pueden producir una vista coherente de todo el catálogo dentro del objetivo temporal? [Resolved, Spec §FR-025, Spec §SC-004, Plan §Phase 0 Outcome]
- [x] CHK036 ¿Se documenta como dependencia normativa que el terminal, no la aplicación, obtiene el portapapeles y delimita el payload mediante bracketed paste? [Resolved, Spec §Definitions and Closed Inventories, Spec §FR-012]
- [x] CHK037 ¿Se aclara si la falta de selección con modificadores, bracketed paste o handle estándar de consola reduce soporte, degrada funciones o debe producir un error? [Resolved, Spec §Definitions and Closed Inventories, Spec §FR-012–FR-013, Contract §Terminal Capability Contract]

## Ambigüedades y conflictos pendientes

- [x] CHK038 ¿Se resuelve si “enmascarado” exige un símbolo por carácter, longitud oculta o únicamente ausencia de texto claro? [Resolved, Spec §FR-017]
- [x] CHK039 ¿Se aclara si los estados de expansión deben preservarse solo durante recargas de la pantalla o también al entrar y volver de formularios, confirmaciones y sesiones SSH? [Resolved, Spec §FR-011]
- [x] CHK040 ¿Se establece una trazabilidad explícita entre cada FR/SC y sus escenarios, entidades y excepciones para detectar requisitos sin criterio de aceptación? [Resolved, Tasks T001–T039 include FR/SC references]

## Notes

- Marcar los ítems resueltos con `[x]`.
- Añadir la resolución o el enlace al requisito actualizado junto al ítem.
- Un ítem marcado no afirma que la implementación funcione; afirma que el requisito quedó suficientemente definido para revisión.
- Los ítems están numerados de forma global para facilitar referencias en PR.
