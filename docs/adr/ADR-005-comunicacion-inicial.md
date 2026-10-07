# ADR-005 — Comunicación inicial entre servicios

## Estado
Propuesto para la Entrega 1; los mecanismos concretos de transporte se confirmarán durante la implementación.

## Contexto
Algunas operaciones necesitan datos de otro servicio antes de responder. Otras producen efectos secundarios que pueden procesarse sin bloquear la operación original.

## Decisión inicial
- Se usará HTTP síncrono para solicitudes que requieren respuesta inmediata. El caso inicial es Servicio de Inventario consultando Servicio de Sucursales para validar sucursales.
- Se usarán eventos asíncronos para cambios relevantes de stock. Servicio de Inventario publicará el evento y Servicio de Notificaciones lo consumirá para crear notificaciones LOW_STOCK o OUT_OF_STOCK.
- El API Gateway será el punto de entrada de los clientes; no reemplazará la comunicación de dominio entre servicios.
- Cada llamada síncrona deberá tener timeout explícito. Los reintentos se limitarán a fallos transitorios y operaciones seguras para reintentar; las escrituras llevarán una clave de idempotencia cuando su contrato lo requiera.
- El consumidor de eventos deberá tolerar entregas duplicadas y registrar fallos de procesamiento. La elección del broker y la política de reintentos quedan pendientes de implementación.

## Justificación
HTTP permite validar una sucursal cuando la decisión necesita información actual. Los eventos desacoplan la creación de notificaciones del registro del movimiento y permiten procesarlas aunque Servicio de Notificaciones tenga una interrupción temporal.

## Alternativas consideradas
- Todo síncrono: acoplaría la disponibilidad del inventario a Servicio de Notificaciones.
- Todo asíncrono: complicaría respuestas que necesitan validar datos antes de confirmar una operación.
- Acceso compartido a bases de datos: descartado porque rompería la propiedad establecida en ADR-001.

## Consecuencias y temas pendientes
- Los fallos de red forman parte del comportamiento normal y deben representarse en las respuestas.
- La notificación puede aparecer después de que la operación de inventario finalice.
- Quedan por definir el broker, los nombres y esquemas de eventos, los valores de timeout, el backoff y la retención de mensajes fallidos.
