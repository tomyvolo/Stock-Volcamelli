# ADR-005 — Comunicación entre servicios (D5)

## Estado
Reemplazado por [ADR-016](ADR-016-comunicacion-entre-servicios.md). Lo dejamos como registro de lo que decidimos en la Entrega 1.

## Contexto
Algunas operaciones necesitan un dato de otro servicio antes de responder. Otras generan algo que puede hacerse después, sin que el usuario espere.

## Decisión
- **HTTP sincrónico cuando hace falta la respuesta en el momento.** El único caso era Inventario consultando a Sucursales para validar que una sucursal existe.
- **Eventos para los cambios de stock.** Inventario publica el evento y Notificaciones lo consume para generar los avisos `LOW_STOCK` y `OUT_OF_STOCK`.
- **El API Gateway es la entrada del frontend.** Los servicios se llaman entre ellos directamente, sin pasar por el gateway.
- **Pendientes para la implementación:** todas las llamadas iban a tener timeout, los reintentos iban a ser solo para errores transitorios y el consumidor de eventos tenía que tolerar mensajes duplicados. Faltaba definir el broker, los valores de timeout y qué hacer con los mensajes que fallan.

## Alternativas que descartamos
- **Todo sincrónico.** Si Notificaciones se caía, no se podía registrar un movimiento de stock.
- **Todo con eventos.** Para validar una sucursal necesitamos la respuesta antes de seguir, y con eventos eso se complica.

## Consecuencias
- Una notificación puede aparecer un rato después del movimiento que la generó.
- Las fallas de red entre servicios hay que manejarlas en cada llamada.
