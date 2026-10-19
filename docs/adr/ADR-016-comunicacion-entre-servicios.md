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
| Transferencias | Sucursales | Traer nombre y dirección de origen y destino | No se crea la transferencia y el usuario ve el error |
| Transferencias | Inventario | Reservar las unidades en el origen | La transferencia queda `FALLIDA` y se libera la reserva (ver abajo) |
| Inventario | Sucursales | Validar que la sucursal existe en un ajuste manual | No se hace el ajuste |

Las llamadas a Inventario pasan por un NGINX interno que reparte los pedidos entre sus dos instancias ([ADR-012](ADR-012-balanceo-de-carga.md)). Las demás van directo de un servicio al otro, sin pasar por el gateway.

Todas las llamadas entre servicios tienen un timeout de 2 segundos. No hay reintentos automáticos: si una llamada falla, el usuario ve el error y puede volver a intentar. Los únicos que reintentan solos son los procesos de fondo, porque ahí no hay nadie esperando.

El caso que más cuidamos es la reserva. Si Inventario no responde a tiempo, no sabemos si llegó a reservar o no. No la reintentamos:

1. La transferencia queda `FALLIDA` y el usuario ve que no se pudo hacer.
2. En segundo plano, Transferencias le pide a Inventario que libere la reserva de esa transferencia, y lo repite hasta que Inventario responda.
3. Si la reserva existía, Inventario la libera. Si no existía, la anota como liberada, y si el pedido de reserva le llega tarde, lo rechaza.

Así las unidades nunca quedan reservadas para una transferencia que el usuario ve como fallida.

### Eventos (RabbitMQ)
Los usamos cuando algo tiene que pasar en otro servicio, pero quien lo genera no necesita esperar la respuesta.

| Evento | Lo publica | Lo consume | Qué hace |
|---|---|---|---|
| `TrasladoCompletado` | Transferencias | Inventario | Confirma la reserva: resta en el origen y suma en el destino |
| `TrasladoCancelado` | Transferencias | Inventario | Libera la reserva en el origen |
| `TransferenciaActualizada` | Transferencias | Tiempo Real | Avisa al frontend que una transferencia cambió de estado |
| `ProductoActualizado` | Inventario | Indexador de Inventario | Actualiza el producto en Solr |
| `StockActualizado` | Inventario | Tiempo Real | Avisa al frontend que cambió el disponible de un producto en una sucursal |

`TrasladoCancelado` lleva el motivo: que Rumbo lo rechazó, que el usuario lo canceló o rechazó el horario propuesto, que el viaje no se realizó o que pasaron 24 horas sin poder mandarle el pedido a Rumbo.

- **Publicación.** El servicio guarda el cambio en su base y, apenas lo guardó, publica el evento en RabbitMQ. Si RabbitMQ no responde, reintenta con backoff: espera 1 segundo, después 2 y después 4. Si después del cuarto intento sigue sin poder, escribe un log de error con el TrackID y el evento completo, para publicarlo a mano.
- **Consumidor idempotente.** Un evento puede llegar más de una vez. Cada evento lleva un id, e Inventario guarda el id de cada evento que procesa, en la misma transacción en la que confirma o libera. Si llega uno repetido, lo ignora. Además, una reserva ya confirmada no se puede liberar ni al revés.
- **ACK manual.** Inventario consume con QoS=1 (un mensaje por vez) y confirma cada mensaje con ACK recién cuando terminó de procesarlo. Si se cae en el medio, RabbitMQ se lo vuelve a entregar.
- **Mensajes que no se pueden procesar.** La cola de cada consumidor tiene configurada una cola de errores (DLQ) con `x-dead-letter-exchange` y `x-dead-letter-routing-key`. Si un mensaje no se puede procesar (por ejemplo, no se puede leer o la reserva no existe), Inventario lo rechaza sin volver a encolarlo (`Nack(false, false)`) y RabbitMQ lo mueve a la DLQ para revisarlo a mano, sin trabar al resto.
- **Exchange fanout.** Cada evento se publica en su propio exchange de tipo fanout, y cada consumidor tiene su cola enlazada a ese exchange. Hoy cada evento tiene un solo consumidor, pero si mañana se suma otro, solo crea su cola y recibe su copia, sin tocar a quien publica.

Transferencias marca la transferencia en su estado final (`COMPLETADA`, `RECHAZADA`, `CANCELADA` o `NO_REALIZADA`) y después publica el evento. No espera que Inventario le avise que lo procesó: si el evento falla, nos enteramos por la DLQ o por el log de error.

`TransferenciaActualizada` se publica en cada cambio de estado, de la misma forma, y la usa solo el servicio de tiempo real para avisarle al frontend ([ADR-019](ADR-019-tiempo-real.md)).

Inventario publica `ProductoActualizado` después de cada alta o cambio de un producto, para mantener actualizado el buscador ([ADR-006](ADR-006-busqueda.md)), y `StockActualizado` después de cada transacción que cambia el stock (reservar, confirmar, liberar o ajustar), para avisarle al frontend.

### Rumbo
A Rumbo le habla solamente Transferencias, por HTTP. Los pedidos de traslado y la consulta de cambios cada 60 segundos corren en segundo plano, así el usuario nunca espera a Rumbo. Cuando el usuario responde un horario propuesto o cancela una transferencia, Transferencias guarda la acción y se la manda a Rumbo, también en segundo plano.

Cuando el traslado se completa publicamos `TrasladoCompletado`. Si se rechaza, se cancela o no se realiza, publicamos `TrasladoCancelado`. Qué operaciones de Rumbo usamos, cómo traducimos sus estados y qué hacemos con cada error está en [ADR-009](ADR-009-consumo-de-rumbo.md). Los reintentos y el circuit breaker están en [ADR-010](ADR-010-resiliencia.md).

## Alternativas que descartamos
- **Guardar el evento junto con el cambio y publicarlo con un proceso aparte (outbox).** Asegura que el evento no se pierda si el servicio se cae entre guardar y publicar, pero suma una tabla y un proceso de fondo en cada servicio que publica. Preferimos publicar apenas se guarda y reintentar, que es más simple. Si en las pruebas aparecen eventos perdidos, es lo primero que sumaríamos.
- **Que Inventario avise de vuelta cuando confirma o libera.** Eran dos eventos y dos colas más para un dato que Transferencias ya tiene cuando publica.

## Consecuencias
- Transferencias puede mostrar una transferencia como completada unos segundos antes de que Inventario mueva el stock.
- Si un evento termina en la DLQ, la transferencia figura en su estado final pero el stock no cambió hasta que lo resolvamos a mano.
- Si el servicio se cae justo después de guardar y antes de publicar, el evento se pierde: la transferencia queda en su estado final y la reserva sigue activa. Se detecta porque hay reservas activas de transferencias terminadas, y se resuelve a mano.
- Si hay un timeout al reservar, el usuario ve la transferencia como fallida aunque Inventario haya llegado a reservar. En ese caso la reserva se libera y el usuario la vuelve a crear.
- Hay que levantar y mantener RabbitMQ y el NGINX interno, y revisar la cola de errores cuando tenga mensajes.
- Si Sucursales está caído, no se pueden crear transferencias.
