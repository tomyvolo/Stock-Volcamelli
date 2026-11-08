# ADR-019 — Actualizaciones en tiempo real

## Estado
Aceptado.

## Contexto
Una transferencia cambia de estado mientras el usuario la está mirando: Rumbo propone un horario, la acepta, el camión sale, llega. Lo mismo el stock: otra persona reserva o ajusta y el disponible de la pantalla queda viejo. Sin nada más, el usuario tiene que recargar para enterarse.

Esos cambios ya pasan por RabbitMQ como eventos ([ADR-016](ADR-016-comunicacion-entre-servicios.md)), así que lo que falta es llevarlos hasta el navegador.

## Decisión

### Piezas
Un servicio nuevo, **Tiempo Real** (Go), que:

1. Consume de RabbitMQ dos eventos:
   - `TransferenciaActualizada`: id de la transferencia, estado nuevo, sucursal de origen y de destino. Transferencias la publica en cada cambio de estado.
   - `StockActualizado`: sucursal, producto y el nuevo disponible. Inventario la publica después de cada transacción que cambia el disponible: un ajuste, una reserva, una liberación o una confirmación. La confirmación cambia dos sucursales, así que publica uno por cada una.
2. Mantiene abiertas las conexiones de los navegadores y les manda cada evento apenas llega.

El evento de la transferencia no trae el horario ni la demora. Cuando llega, el frontend vuelve a pedir esa transferencia y trae todos sus datos, así el evento no tiene que copiar todo lo que tiene la transferencia.

### La cola de Tiempo Real
Tiempo Real enlaza su propia cola a cada exchange, como cualquier consumidor, pero la cola se borra cuando el servicio se desconecta. Si Tiempo Real estuvo caído, los eventos de ese rato no le sirven a nadie: los navegadores que estaban conectados ya recargaron al reconectar. Con una cola que queda, esos eventos se acumularían para mandarse tarde.

Por lo mismo no lleva cola de errores: un evento que no se puede leer se descarta y queda en el log.

### Cómo llega al navegador
Usamos **Server-Sent Events (SSE)**: el frontend abre `GET /api/events` y la conexión queda abierta; el servidor escribe un mensaje cada vez que hay un evento. Es HTTP común, así que pasa por el gateway como cualquier pedido.

El navegador trae `EventSource` para SSE, pero no permite mandar el header `Authorization`. Por eso el frontend abre la conexión con `fetch` y lee la respuesta a medida que llega, mandando el token igual que en el resto de los pedidos. El gateway valida el token como siempre.

Para esta ruta el gateway espera como mucho 2 segundos a que Tiempo Real responda, pero después no corta la conexión y reenvía cada mensaje apenas lo recibe. Esta ruta no entra en las métricas de latencia del gateway: una conexión que dura una hora arruinaría el p95 de todo lo demás. Tiempo Real manda un comentario vacío cada 25 segundos para que la conexión no se cierre por inactividad.

### Qué hace el frontend
- Cada mensaje dice qué cambió. Si el usuario tiene abierta esa transferencia o el stock de esa sucursal, la pantalla se actualiza sola; si no, lo ignora.
- Si se corta la conexión, la vuelve a abrir esperando cada vez más (1, 2, 4 segundos, hasta 30). Al reconectar recarga los datos de la pantalla, porque los eventos que pasaron mientras estaba desconectado no se vuelven a mandar.
- Cuando vence el token (1 hora), el gateway cierra la conexión y el usuario vuelve a iniciar sesión, como en el resto del sistema. Lo cierra el gateway porque es el que ya validó el token; así Tiempo Real no necesita la clave del JWT.

### Quién recibe qué
Todos los usuarios conectados, sea cual sea su rol, reciben todos los eventos, y el frontend filtra lo que muestra.

Si un navegador no lee lo que le mandamos y se le juntan más de 20 mensajes sin enviar, Tiempo Real lo desconecta. Él reconecta y recarga, y así uno lento no frena a los demás.

## Alternativas que descartamos
- **Polling: que el frontend vuelva a pedir la pantalla cada algunos segundos.** Es lo más simple, porque no necesita un servicio nuevo ni eventos, y con los pocos usuarios que tenemos la carga no sería un problema. Pero la pantalla se entera tarde: preguntando cada 5 segundos, un cambio aparece entre 0 y 5 segundos después, y para bajar ese tiempo hay que preguntar más seguido aunque casi nunca haya novedades. Con SSE el cambio llega en menos de un segundo y solo se manda algo cuando pasa algo.

## Consecuencias
- Es un servicio más y una ruta más en el gateway, que además tiene que soportar conexiones largas.
- Tiempo Real corre en una sola instancia, porque las conexiones abiertas están en su memoria. Si se cae, las pantallas dejan de actualizarse solas, pero todo sigue andando: el usuario recarga como antes.
- Transferencias publica un evento más en cada cambio de estado, e Inventario uno más en cada cambio de stock.
- Los eventos que se pierden durante una desconexión no se recuperan, pero la recarga al reconectar deja la pantalla al día.
- Inventario publicado en la nube no tiene RabbitMQ, así que sus cambios de stock no generan eventos. No hace falta: ahí no hay frontend nuestro mirando.
