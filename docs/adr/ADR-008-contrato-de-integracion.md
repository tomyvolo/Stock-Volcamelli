# ADR-008 — Contrato de integración del Servicio de Inventario

## Estado
Aceptado para la Entrega 1.

## Contexto
Otro grupo necesita integrar su flujo con el dominio de inventario de Stock Volcamelli. La capacidad publicada debe representar el servicio Inventory completo dentro de su responsabilidad de negocio: productos, stock por sucursal, movimientos y transferencias.

## Decisión
Se publicará la API de Servicio de Inventario bajo la ruta versionada /api/v1. El contrato canónico está en ../contracts/openapi.yaml (OpenAPI 3.1); el mock se levanta con Prism en Docker a partir de esa misma especificación (ver ../contracts/README.md).

La superficie pública de v1 incluye:

| Área | Operaciones |
|---|---|
| Productos | Listar, consultar por identificador, crear y modificar nombre o descripción |
| Stock | Consultar stock de una sucursal o de un producto en una sucursal |
| Movimientos | Registrar entradas y salidas y consultar los movimientos de un producto en una sucursal |
| Transferencias | Registrar una transferencia entre sucursales y consultar su resultado |

Las transferencias se completan dentro de Servicio de Inventario: descuentan stock en origen, acreditan stock en destino y generan sus movimientos. No se permite stock negativo. Los IDs de sucursal son referencias administradas por Servicio de Sucursales; Servicio de Inventario no expone la administración de sucursales.

Las operaciones POST para crear productos, registrar movimientos o transferir stock requieren la cabecera Idempotency-Key para que un reintento no duplique el efecto. La modificación de campos de producto usa PATCH con asignación de valores y puede repetirse de forma segura. Las operaciones de lectura no requieren idempotencia. El mock académico es público y usa datos ficticios; antes de exponer una implementación real se deberá acordar autenticación, autorización y límites de uso con el grupo consumidor.

## Versionado y compatibilidad
La versión mayor figura en la ruta (/api/v1) y la versión de la especificación en info.version. Agregar operaciones o campos opcionales sin cambiar el significado existente puede ser compatible. Quitar campos obligatorios, cambiar tipos o alterar reglas requiere una versión mayor nueva y una ruta nueva.

## Justificación
Servicio de Inventario es la capacidad de negocio que el equipo decidió ofrecer. Publicar su superficie funcional completa evita que el consumidor dependa de una única consulta si necesita administrar productos, registrar movimientos o transferir stock.

## Alternativas consideradas
- Publicar solo la consulta de stock: descartada porque deja fuera las operaciones centrales de Servicio de Inventario.
- Publicar también la gestión de sucursales: descartada porque esos datos pertenecen a Servicio de Sucursales.
- Publicar interfaces directas a persistencia: descartada porque expondría detalles internos y rompería la propiedad de los datos.

## Consecuencias
- El grupo consumidor puede integrar el ciclo de inventario completo mediante un contrato estable.
- Las transferencias deben ser atómicas dentro de Servicio de Inventario y validar disponibilidad antes de confirmar.
- Las operaciones POST idempotentes deben conservar el resultado asociado a cada clave dentro del período de retención que defina la implementación.
- Las notificaciones de stock continúan siendo responsabilidad de Servicio de Notificaciones y pueden generarse de manera asíncrona.
- El contrato no promete autenticación para el mock de aula. Antes de producción deben acordarse seguridad, límites de uso, disponibilidad y política de compatibilidad.
- La eliminación de productos no forma parte de esta versión porque hay que definir su relación con el historial de movimientos.
