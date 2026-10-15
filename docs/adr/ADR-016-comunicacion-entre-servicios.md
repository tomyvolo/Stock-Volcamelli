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
| Transferencias | Inventario | Reservar las unidades en el origen | La transferencia queda `FALLIDA` y se libera la reserva (ver abajo) |
| Inventario | Sucursales | Validar que la sucursal existe en un ajuste manual | No se hace el ajuste |

Las llamadas a Inventario pasan por un NGINX interno que reparte los pedidos entre sus dos instancias (ADR de D12). Las demás van directo de un servicio al otro, sin pasar por el gateway.

Todas las llamadas entre servicios tienen un timeout de 2 segundos. No hay reintentos automáticos: si una llamada falla, el operador ve el error y puede volver a intentar. Los únicos que reintentan solos son los procesos de fondo, porque ahí no hay nadie esperando.

El caso que más cuidamos es la reserva. Si Inventario no responde a tiempo, no sabemos si llegó a reservar o no. No la reintentamos:

1. La transferencia queda `FALLIDA` y el operador ve que no se pudo hacer.
2. En segundo plano, Transferencias le pide a Inventario que libere la reserva de esa transferencia, y lo repite hasta que Inventario responda.
3. Si la reserva existía, Inventario la libera. Si no existía, la anota como liberada, y si el pedido de reserva le llega tarde, lo rechaza.

Así las unidades nunca quedan reservadas para una transferencia que el operador ve como fallida.

### Eventos (RabbitMQ)
Los usamos cuando algo tiene que pasar en otro servicio, pero quien lo genera no necesita esperar la respuesta.

| Evento | Lo publica | Lo consume | Qué hace |
|---|---|---|---|
| `TrasladoAceptado` | Transferencias | Inventario | Confirma la reserva: resta en el origen y suma en el destino |
| `TrasladoRechazado` | Transferencias | Inventario | Libera la reserva en el origen |

`TrasladoRechazado` lleva el motivo: que Rumbo lo rechazó, que respondió con un error de datos o que pasaron 24 horas sin poder mandarle el pedido.

- **Publicación con outbox.** El evento se guarda en el mismo documento de la transferencia, junto con el cambio de estado. Un proceso de fondo revisa cada pocos segundos los eventos pendientes, los publica en RabbitMQ y los marca como enviados. Si el servicio se cae en el medio, el evento sigue guardado y se publica cuando vuelve.
- **Consumidor idempotente.** Un evento puede llegar más de una vez. Inventario guarda el id de cada evento que procesa, en la misma transacción en la que confirma o libera. Si llega uno repetido, lo ignora. Además, una reserva ya confirmada no se puede liberar ni al revés.
- **Mensajes que fallan.** Si Inventario no puede procesar un mensaje después de 3 intentos, el mensaje va a una cola de errores (DLQ) para revisarlo a mano, y no traba al resto.

Transferencias marca la transferencia como `CONFIRMADA` o `RECHAZADA` en el momento en que guarda el evento. No espera que Inventario le avise que lo procesó: el outbox garantiza que el evento sale y la DLQ, que si falla nos enteramos.

Inventario también publica sus cambios de productos y stock para mantener actualizado el buscador. Funciona igual, con su propio outbox; los detalles van en el ADR de D6.

### Rumbo
A Rumbo le habla solamente Transferencias, por HTTP y en segundo plano, así el operador nunca espera a Rumbo. De su contrato usamos cuatro operaciones:

- pedir el token;
- pedir el traslado, sin horario y con el id de la transferencia como `Idempotency-Key`;
- consultar cada 60 segundos qué traslados cambiaron;
- aceptar el horario que proponga, porque a nosotros el horario no nos importa.

Cuando el pedido queda aceptado publicamos `TrasladoAceptado`. Si lo rechazan, publicamos `TrasladoRechazado`. No seguimos el viaje del camión. Cómo manejamos los errores y la caída de Rumbo lo definimos en los ADRs de D9 y D10.

## Alternativas que descartamos
- **Publicar el evento directamente después de guardar el cambio, sin outbox.** Es más simple, pero si el servicio se cae entre guardar y publicar, el evento se pierde y la reserva queda colgada.
- **Que Inventario avise de vuelta cuando confirma o libera.** Eran dos eventos y dos colas más para un dato que Transferencias ya tiene cuando publica.

## Consecuencias
- Transferencias puede mostrar una transferencia como confirmada unos segundos antes de que Inventario mueva el stock.
- Si un evento termina en la DLQ, la transferencia figura confirmada o rechazada pero el stock no cambió hasta que lo resolvamos a mano.
- Si hay un timeout al reservar, el operador ve la transferencia como fallida aunque Inventario haya llegado a reservar. En ese caso la reserva se libera y el operador la vuelve a crear.
- Hay que levantar y mantener RabbitMQ y el NGINX interno, y revisar la cola de errores cuando tenga mensajes.
- Si Sucursales está caído, no se pueden crear transferencias.
