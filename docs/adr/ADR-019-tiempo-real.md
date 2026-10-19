# ADR-019 — Actualizaciones en tiempo real

## Estado
Propuesto.

## Contexto
Una transferencia cambia de estado mientras el usuario la está mirando: Rumbo propone un horario, la acepta, el camión sale, llega. Lo mismo el stock: otra persona reserva o ajusta y el disponible de la pantalla queda viejo. Sin nada más, el usuario tiene que recargar para enterarse.

Esos cambios ya pasan por RabbitMQ como eventos ([ADR-016](ADR-016-comunicacion-entre-servicios.md)), así que lo que falta es llevarlos hasta el navegador.

## Decisión

### Piezas
Un servicio nuevo, **Tiempo Real** (Go), que:

1. Consume de RabbitMQ dos eventos, con su propia cola:
   - `TransferenciaActualizada`: id, estado nuevo, origen, destino y, si hay, horario o demora. Transferencias la publica en cada cambio de estado.
   - `StockActualizado`: producto, sucursal y el nuevo disponible. Lo publica Inventario ([ADR-016](ADR-016-comunicacion-entre-servicios.md)).
2. Mantiene abiertas las conexiones de los navegadores y les manda cada evento apenas llega.

### Cómo llega al navegador
Usamos **Server-Sent Events (SSE)**: el frontend abre `GET /api/events` y la conexión queda abierta; el servidor escribe un mensaje cada vez que hay un evento. Es HTTP común, así que pasa por el gateway como cualquier pedido.

El navegador trae `EventSource` para SSE, pero no permite mandar el header `Authorization`. Por eso el frontend abre la conexión con `fetch` y lee la respuesta a medida que llega, mandando el token igual que en el resto de los pedidos. El gateway valida el token como siempre.

Para esta ruta el gateway no corta la conexión por timeout y reenvía cada mensaje apenas lo recibe. Tiempo Real manda un comentario vacío cada 25 segundos para que la conexión no se cierre por inactividad.

### Qué hace el frontend
- Cada mensaje dice qué cambió. Si el usuario tiene abierta esa transferencia o el stock de esa sucursal, la pantalla se actualiza sola; si no, lo ignora.
- Si se corta la conexión, la vuelve a abrir esperando cada vez más (1, 2, 4 segundos, hasta 30). Al reconectar recarga los datos de la pantalla, porque los eventos que pasaron mientras estaba desconectado no se vuelven a mandar.
- Cuando vence el token (1 hora), Tiempo Real cierra la conexión y el usuario vuelve a iniciar sesión, como en el resto del sistema.

### Quién recibe qué
Todos los usuarios conectados, sea cual sea su rol, reciben todos los eventos, y el frontend filtra lo que muestra. `INTEGRACION` no puede usar `/api/events`.

## Consecuencias
- Es un servicio más y una ruta más en el gateway, que además tiene que soportar conexiones largas.
- Tiempo Real corre en una sola instancia, porque las conexiones abiertas están en su memoria. Si se cae, las pantallas dejan de actualizarse solas, pero todo sigue andando: el usuario recarga como antes.
- Transferencias publica un evento más en cada cambio de estado.
- Los eventos que se pierden durante una desconexión no se recuperan, pero la recarga al reconectar deja la pantalla al día.
