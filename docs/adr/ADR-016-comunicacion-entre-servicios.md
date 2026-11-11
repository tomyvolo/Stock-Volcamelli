# ADR-016 — Comunicación entre servicios (D5)

## Estado
Aceptado. Reemplaza a [ADR-005](ADR-005-comunicacion-inicial.md).

## Por qué revisamos ADR-005
Con los servicios de [ADR-014](ADR-014-limites-de-servicios.md), las comunicaciones son otras: ahora la transferencia pasa por Transferencias, Sucursales, Inventario y Rumbo. Además, ADR-005 dejaba sin definir el broker, los timeouts y qué hacer con los mensajes que fallan, y eso lo necesitamos para empezar a implementar.

## Decisión

### Llamadas sincrónicas (HTTP)
Las usamos cuando el servicio necesita la respuesta para seguir.

| Quién llama | A quién | Para qué | Si falla |
|---|---|---|---|
| Transferencias | Sucursales | Traer nombre y dirección de origen y destino | No se crea la transferencia y el usuario ve el error |
| Transferencias | Inventario | Reservar las unidades en el origen | La transferencia queda `FALLIDA` y se libera la reserva (ver abajo) |
| Inventario | Sucursales | Validar que la sucursal existe en un ajuste manual | No se hace el ajuste |

Las llamadas a Inventario pasan por un NGINX interno que reparte los pedidos entre sus dos instancias ([ADR-012](ADR-012-balanceo-de-carga.md)). Las demás van directo de un servicio al otro, sin pasar por el gateway.

Las llamadas entre servicios tienen un timeout de 2 segundos. Las que esperan a otras llamadas tienen más: el gateway espera 5 segundos al crear o cancelar una transferencia y 30 al asistente, y la conexión de tiempo real queda abierta. La tabla completa está en [ADR-010](ADR-010-resiliencia.md). No hay reintentos automáticos: si una llamada falla, el usuario ve el error y puede volver a intentar. Los únicos que reintentan solos son los procesos de fondo, porque ahí no hay nadie esperando.

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

`TrasladoCancelado` lleva el motivo: que Rumbo no tenía lugar o rechazó el pedido, que el usuario lo canceló, que el viaje no se realizó o que pasaron 24 horas sin poder mandarle el pedido a Rumbo.

- **Publicación.** El servicio guarda el cambio en su base y, apenas lo guardó, publica el evento en RabbitMQ. Si RabbitMQ no responde, reintenta con backoff: espera 1 segundo, después 2 y después 4. Si después del cuarto intento sigue sin poder, depende de quién publica:
  - **Transferencias** no puede perder un evento, porque `TrasladoCompletado` y `TrasladoCancelado` son los que mueven el stock. Por eso guarda la marca `eventos_por_publicar` en la misma escritura que el cambio de estado, y la saca recién cuando los eventos salieron. Si quedó marcada, el proceso de fondo que corre cada 10 segundos vuelve a armar los eventos desde la transferencia y los publica, hasta que RabbitMQ los acepta. Como MongoDB escribe el documento entero de una vez, no puede quedar un cambio de estado guardado sin su marca. Los eventos republicados salen con un id nuevo, así que lo que evita mover el stock dos veces no es el id sino el estado de la reserva: Inventario no vuelve a liberar una reserva `LIBERADA` ni a confirmar una `CONFIRMADA`.
  - **Inventario** escribe un log de error con el TrackID y el evento completo. Sus eventos no mueven stock: un producto que no llegó a Solr se arregla reconstruyendo el índice ([ADR-006](ADR-006-busqueda.md)), y un `StockActualizado` perdido solo deja una pantalla sin actualizar hasta que se recarga.
- **Consumidor idempotente.** Un evento puede llegar más de una vez. Cada evento lleva un id, e Inventario guarda el id de cada evento que procesa, en la misma transacción en la que confirma o libera. Si llega uno repetido, lo ignora. Además, una reserva ya confirmada no se puede liberar ni al revés.
- **ACK manual.** Inventario consume con QoS=1 (un mensaje por vez) y confirma cada mensaje con ACK recién cuando terminó de procesarlo. Si se cae en el medio, RabbitMQ se lo vuelve a entregar.
- **Mensajes que no se pueden procesar.** La cola de cada consumidor de Inventario tiene configurada una cola de errores (DLQ) con `x-dead-letter-exchange` y `x-dead-letter-routing-key`. Si un mensaje no se puede procesar (por ejemplo, no se puede leer o la reserva no existe), Inventario lo rechaza sin volver a encolarlo (`Nack(false, false)`) y RabbitMQ lo mueve a la DLQ para revisarlo a mano, sin trabar al resto. Si en cambio falla por algo pasajero, como que MySQL no responde, espera 5 segundos y lo devuelve a la cola (`Nack(false, true)`) para intentar de nuevo.
- **Exchange fanout.** Cada evento se publica en su propio exchange de tipo fanout, y cada consumidor tiene su cola enlazada a ese exchange. Hoy cada evento tiene un solo consumidor, pero si mañana se suma otro, solo crea su cola y recibe su copia, sin tocar a quien publica.

Transferencias marca la transferencia en su estado final (`COMPLETADA`, `RECHAZADA`, `CANCELADA` o `NO_REALIZADA`) y después publica el evento. No espera que Inventario le avise que lo procesó: si el evento no se puede procesar, nos enteramos por la DLQ.

`TransferenciaActualizada` se publica en cada cambio de estado, de la misma forma, y la usa solo el servicio de tiempo real para avisarle al frontend ([ADR-019](ADR-019-tiempo-real.md)). Ese consumidor es distinto de los de Inventario: su cola se borra cuando se desconecta y no tiene cola de errores, porque un aviso viejo a la pantalla ya no sirve.

Inventario publica `ProductoActualizado` después de cada alta o cambio de un producto, para mantener actualizado el buscador ([ADR-006](ADR-006-busqueda.md)), y `StockActualizado` después de cada transacción que cambia el stock (reservar, confirmar, liberar o ajustar), para avisarle al frontend.

### Rumbo
A Rumbo le habla solamente Transferencias, por HTTP. La reserva del traslado y la consulta de cambios cada 60 segundos corren en segundo plano, así el usuario nunca espera a Rumbo. Cuando el usuario cancela una transferencia programada, Transferencias guarda la cancelación y se la manda a Rumbo, también en segundo plano.

Cuando el traslado se completa publicamos `TrasladoCompletado`. Si se rechaza, se cancela o no se realiza, publicamos `TrasladoCancelado`. Qué operaciones de Rumbo usamos, cómo traducimos sus estados y qué hacemos con cada error está en [ADR-009](ADR-009-consumo-de-rumbo.md). Los reintentos y el circuit breaker están en [ADR-010](ADR-010-resiliencia.md).

## Validación
Validamos la comunicación HTTP del gateway con Usuarios e Inventario, con el sistema levantado con Docker Compose:

- **El TrackID llega a todos.** El gateway lo crea, lo manda en `X-Track-ID` y cada servicio lo guarda en sus logs. Con un mismo TrackID vemos en Grafana el paso por el gateway y por Inventario.
- **Timeouts.** El gateway espera 2 segundos a cada servicio (5 al crear o cancelar una transferencia) y, si no responde, devuelve `503`. NGINX espera 1 segundo para conectar con una instancia de Inventario y, si no puede, prueba la otra.
- **Lo probamos con fallas.** Con una instancia de Inventario apagada, las consultas siguen respondiendo desde la otra; con las dos apagadas, el gateway responde `503`.

## Alternativas que descartamos
- **Una tabla de eventos pendientes en cada servicio (outbox completo).** Cada evento entero se guarda en una tabla, en la misma transacción que el cambio, y lo publica un proceso aparte. En Transferencias alcanza con la marca en el mismo documento, porque los eventos se pueden volver a armar desde la transferencia y ya había un proceso de fondo que la recorre. En Inventario no lo sumamos, porque sus eventos no mueven stock.
- **Publicar apenas se guarda y, si falla, dejarlo en el log para publicarlo a mano.** Así arrancamos también en Transferencias. Una revisión externa lo probó: con RabbitMQ caído, cancelar una transferencia la dejaba `CANCELADA`, pero cuando RabbitMQ volvía nadie publicaba el `TrasladoCancelado` y la reserva seguía activa ([validacion.md](../validacion.md), sección 15).
- **Que Inventario avise de vuelta cuando confirma o libera.** Eran dos eventos y dos colas más para un dato que Transferencias ya tiene cuando publica.

## Consecuencias
- Transferencias puede mostrar una transferencia como completada unos segundos antes de que Inventario mueva el stock.
- Si un evento termina en la DLQ, la transferencia figura en su estado final pero el stock no cambió hasta que lo resolvamos a mano.
- Con RabbitMQ caído, el stock de los traslados que terminan mientras tanto se mueve recién cuando vuelve, en la vuelta siguiente del proceso de fondo.
- Un evento de Transferencias puede salir más de una vez, por ejemplo si Transferencias se cae después de publicar y antes de sacar la marca. Inventario no lo aplica de nuevo porque la reserva ya está liberada o confirmada, y Tiempo Real manda un aviso de más a la pantalla.
- Que publicar no devuelva error no asegura que el evento llegó a una cola: publicamos sin confirmación de RabbitMQ y sin `mandatory`. Si RabbitMQ está andando pero la cola de Inventario no existe o no está enlazada al exchange, por ejemplo si RabbitMQ se recreó sin datos y Transferencias publica antes de que Inventario se reconecte, RabbitMQ descarta el mensaje y la marca se saca igual. La reserva queda activa y hay que liberarla a mano. Lo encontró una revisión externa desenlazando la cola. Para resolverlo habría que publicar con confirmaciones y `mandatory`, y dejar la marca si RabbitMQ no confirma o devuelve el mensaje.
- Si Inventario se cae justo después de guardar un producto y antes de publicar, ese producto no se actualiza en Solr hasta la próxima reconstrucción del índice.
- Si hay un timeout al reservar, el usuario ve la transferencia como fallida aunque Inventario haya llegado a reservar. En ese caso la reserva se libera y el usuario la vuelve a crear.
- Hay que levantar y mantener RabbitMQ y el NGINX interno, y revisar la cola de errores cuando tenga mensajes.
- Si Sucursales está caído, no se pueden crear transferencias.
