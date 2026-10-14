# ADR-016 — Comunicación entre servicios (D5)

## Estado
Aceptado. Reemplaza a [ADR-005](ADR-005-comunicacion-inicial.md).

## Por qué revisamos ADR-005
Con los servicios de [ADR-014](ADR-014-limites-de-servicios.md), las comunicaciones son otras: ahora el flujo principal es la transferencia, que pasa por Transferencias, Sucursales, Inventario y Rumbo. Además, ADR-005 dejaba sin definir el broker, los timeouts y qué hacer con los mensajes que fallan, y eso lo necesitamos para empezar a implementar.

## Decisión

### Llamadas sincrónicas (HTTP)
Las usamos cuando el servicio necesita la respuesta para seguir.

| Quién llama | A quién | Para qué | Si falla |
|---|---|---|---|
| Transferencias | Sucursales | Traer nombre y dirección de origen y destino | No se crea la transferencia y el operador ve el error |
| Transferencias | Inventario | Mover el stock de origen a destino | La transferencia queda `FALLIDA` y se anula (ver abajo) |
| Inventario | Sucursales | Validar que la sucursal existe en un ajuste manual | No se hace el ajuste |

Todas las llamadas entre servicios tienen un timeout de 2 segundos. No hay reintentos automáticos: si una llamada falla, el operador ve el error y puede volver a intentar. Los únicos que reintentan solos son los procesos de fondo, porque ahí no hay nadie esperando.

El caso que más cuidamos es Transferencias → Inventario. En Inventario la transferencia nunca queda a medias, porque restar en origen y sumar en destino es una sola transacción. El problema es que, si se corta por timeout, Transferencias no sabe si Inventario llegó a hacerla. Queremos que en ese caso la transferencia no se haga, así que:

1. La transferencia queda `FALLIDA` y el operador ve que no se pudo hacer.
2. En segundo plano, Transferencias le pide a Inventario que anule esa transferencia y lo repite hasta que Inventario responda.
3. Si Inventario la había hecho, devuelve el stock al origen. Si no, la anota como anulada, y si el pedido original le llega tarde, lo rechaza.

Para eso Inventario guarda el id de cada transferencia que hizo o anuló.

El frontend entra siempre por el API Gateway. Los servicios se llaman entre ellos directamente, sin pasar por el gateway.

### Eventos (RabbitMQ)
Los usamos cuando algo tiene que pasar en otro servicio, pero quien lo genera no necesita esperar la respuesta.

El evento principal es `TransferenciaRevertida`: Transferencias lo publica cuando Rumbo rechaza un traslado, e Inventario lo consume para devolver el stock al origen.

- **Publicación con outbox.** El evento se guarda en el mismo documento de la transferencia, junto con el cambio de estado. Un proceso de fondo revisa cada pocos segundos los eventos pendientes, los publica en RabbitMQ y los marca como enviados. Si el servicio se cae en el medio, el evento sigue guardado y se publica cuando vuelve.
- **Consumidor idempotente.** Un evento puede llegar más de una vez. Inventario guarda el id de cada evento que procesa, en la misma transacción en la que devuelve el stock. Si llega uno repetido, lo ignora.
- **Mensajes que fallan.** Si Inventario no puede procesar un mensaje después de 3 intentos, el mensaje va a una cola de errores (DLQ) para revisarlo a mano, y no traba al resto.

Inventario también publica sus cambios de productos y stock para mantener actualizado el buscador. Funciona igual, con su propio outbox; los detalles van en el ADR de D6.

### Rumbo
A Rumbo le habla solamente Transferencias, por HTTP y en segundo plano, así el operador nunca espera a Rumbo. Cómo pedimos el token, cada cuánto consultamos el estado y qué hacemos si Rumbo no responde lo definimos en los ADRs de D9 y D10.

## Alternativas que descartamos
- **Publicar el evento directamente después de guardar el cambio, sin outbox.** Es más simple, pero si el servicio se cae entre guardar y publicar, el evento se pierde y el stock nunca vuelve al origen.

## Consecuencias
- Cuando Rumbo rechaza un traslado, el stock no vuelve al instante: pasa un rato entre que se publica el evento e Inventario lo procesa.
- Si hay un timeout, el operador ve que la transferencia no se hizo aunque Inventario la haya llegado a hacer. En ese caso se deshace, y el operador la vuelve a crear.
- Inventario tiene que guardar los ids de las transferencias que hizo o anuló y los de los eventos que procesó, y esas tablas crecen con el tiempo.
- Hay que levantar y mantener RabbitMQ, y revisar la cola de errores cuando tenga mensajes.
- Si Sucursales está caído, no se pueden crear transferencias.
