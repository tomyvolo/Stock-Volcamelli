# ADR-004 — Consistencia y concurrencia (D4)

## Estado
Aceptado.

## Contexto
La operación que no puede duplicar ni perder información es mover stock por una transferencia. Si se duplica, una sucursal recibe unidades que no existen; si se pierde, desaparecen unidades del origen o quedan reservadas para siempre.

Esa operación cruza tres piezas que no comparten base: Transferencias (MongoDB), Inventario (MySQL, con dos instancias) y Rumbo, que es de otro grupo. No hay una transacción que cubra las tres. Además pasan cosas al mismo tiempo: dos vendedores transfieren el mismo producto desde la misma sucursal, un admin ajusta el stock mientras se reserva, las dos instancias de Inventario leen la misma cola y RabbitMQ puede entregar un mensaje dos veces.

## Decisión

### Cómo se mueve el stock
La transferencia se hace en pasos, y cada paso es una transacción local en el MySQL de Inventario:

1. **Al crear la transferencia**, Inventario reserva las unidades en el origen: suben las reservadas y baja el disponible, pero el total no cambia.
2. **Cuando el traslado llega**, Inventario resta del origen, suma al destino y anota un movimiento en cada sucursal, todo en la misma transacción.
3. **Si el traslado se rechaza, se cancela o no se hace**, Inventario libera la reserva y el origen vuelve a tener esas unidades disponibles.

Los tres pasos se identifican con el id de la transferencia. Entre un paso y otro el stock siempre está en un estado válido: las unidades están en el origen, reservadas o ya en el destino, nunca en dos lugares ni en ninguno.

### Concurrencia sobre el mismo stock
- **Bloqueo de la fila.** Reservar, liberar, confirmar y ajustar leen la fila de stock del producto en la sucursal con `SELECT ... FOR UPDATE`, dentro de una transacción. Si dos pedidos tocan la misma fila, el segundo espera a que el primero termine y recién ahí ve el disponible. Por eso, con 5 disponibles y dos transferencias de 4 al mismo tiempo, una queda `RESERVADA` y la otra `SIN_STOCK` (CA-13).
- **La base también controla.** Las columnas de total y reservado no admiten negativos, y un `CHECK` impide que lo reservado supere al total. Si el código tuviera un error, la base rechaza el cambio en lugar de guardar un stock imposible.
- **Siempre en el mismo orden.** Confirmar toca dos filas, la del origen y la del destino. Las bloqueamos siempre primero la de la sucursal con id menor, así dos transferencias entre las mismas sucursales en sentidos opuestos no se quedan esperando una a la otra.
- **Las consultas no bloquean.** Ver el stock no espera a nadie. Puede mostrar el disponible de un instante antes, pero la que decide es la reserva, que vuelve a leerlo con el bloqueo.
- **Transferencias, en una sola instancia.** Cada cambio de una transferencia (lo que pide el usuario y lo que hacen los procesos de fondo) la vuelve a leer y la guarda con el lock en memoria de esa transferencia. Así una cancelación del usuario y una respuesta de Rumbo no se pisan. Es un lock por transferencia y no uno para todas: el proceso de fondo lo tiene tomado mientras espera a Rumbo, que puede tardar varios segundos, y con uno solo la cancelación de cualquier otra transferencia esperaba hasta que el gateway cortaba con `503`. Sigue pasando si el usuario cancela justo la transferencia que se está mandando a Rumbo: espera esa respuesta, puede ver un `503` y la cancelación se aplica igual en cuanto Rumbo contesta. Alcanza con locks en memoria porque corre una sola instancia ([ADR-009](ADR-009-consumo-de-rumbo.md)).

### Que nada se haga dos veces
| Qué se puede repetir | Por qué no duplica |
|---|---|
| El pedido de reserva | La reserva tiene como clave el id de la transferencia. Si llega dos veces, la segunda devuelve la reserva que ya existe |
| Un evento de RabbitMQ (`TrasladoCompletado` o `TrasladoCancelado`) | Inventario guarda el id de cada evento que procesó, en la misma transacción que el cambio de stock. Si llega repetido, ya está anotado y no hace nada. Si las dos instancias lo procesan a la vez, solo una lo aplica (CA-15) |
| La reserva o la cancelación en Rumbo | Van con `Idempotency-Key` ([ADR-009](ADR-009-consumo-de-rumbo.md)) |
| Un dato viejo de Rumbo | Transferencias guarda la fecha del último dato que aplicó y descarta los más viejos, así una transferencia nunca vuelve a un estado anterior |

### Que nada se pierda
| Qué falla | Qué pasa | Cómo se resuelve |
|---|---|---|
| Inventario no responde al reservar | No sabemos si reservó. La transferencia queda `FALLIDA` | Un proceso de fondo le pide a Inventario que libere hasta que responde. Si la liberación llega antes que la reserva, Inventario la anota como liberada y rechaza la reserva cuando llega tarde (CA-14) |
| Transferencias no puede guardar la transferencia después de reservar | Hay una reserva sin transferencia | La libera en el momento |
| Rumbo no responde | La transferencia sigue `RESERVADA` | Se reintenta con la misma clave. A las 24 horas queda `RECHAZADA` y se libera ([ADR-009](ADR-009-consumo-de-rumbo.md)) |
| Inventario se cae mientras procesa un evento | La transacción no se guardó | Inventario confirma el mensaje recién después de guardar. Como no lo confirmó, RabbitMQ se lo vuelve a entregar, y si ya lo había guardado, el id del evento evita repetirlo |
| RabbitMQ se reinicia | Los eventos que Inventario todavía no procesó estaban en las colas | Las colas son durables y los mensajes persistentes, así que los eventos pendientes siguen ahí |
| Un evento no se puede procesar (por ejemplo, confirmar una reserva ya liberada) | Reintentarlo no lo arregla | Va a la cola de errores y queda en el log, para revisarlo ([ADR-016](ADR-016-comunicacion-entre-servicios.md)) |
| RabbitMQ no está cuando hay que publicar | El cambio ya está guardado pero el evento no salió | Transferencias guarda la marca de eventos por publicar junto con el cambio de estado, y un proceso de fondo los vuelve a publicar cada 10 segundos hasta que salen. Si salen dos veces, Inventario ignora el repetido ([ADR-016](ADR-016-comunicacion-entre-servicios.md)) |

Salvo los eventos que van a la cola de errores, que revisamos nosotros, lo que no se pudo hacer en el momento lo reintenta un proceso de fondo. Mientras tanto las unidades quedan reservadas en el origen: no se pierden.

## Alternativas que descartamos
- **Una transacción distribuida entre Transferencias e Inventario.** Haría todo en un paso, pero ninguna de las herramientas que usamos la ofrece entre MongoDB y MySQL, y si un servicio se cae en el medio, las filas quedan bloqueadas hasta que vuelve.
- **Control optimista: guardar una versión en cada fila y reintentar si cambió.** No bloquea, pero con muchas transferencias del mismo producto al mismo tiempo la mayoría tendría que reintentar. Con `FOR UPDATE` la espera es corta, porque cada transacción toca una o dos filas.
- **Controlar el stock solo en el código.** Era más simple, pero un error nuestro podía dejar un stock negativo guardado. Con el `CHECK` y las columnas sin signo, la base lo rechaza.
- **Una tabla de eventos pendientes (outbox completo).** En Transferencias alcanza con una marca en el mismo documento de la transferencia ([ADR-016](ADR-016-comunicacion-entre-servicios.md)).

## Consecuencias
- Las unidades de una transferencia quedan reservadas hasta que termina, que puede ser varios días.
- Una consulta de stock puede mostrar el disponible de un instante antes; si alguien reserva en ese momento, la transferencia del otro puede quedar `SIN_STOCK`.
- Entre que Rumbo informa que el traslado llegó y el destino ve las unidades pasan hasta 60 segundos, más lo que tarde el evento en la cola.
- Si RabbitMQ está caído, las unidades de los traslados que terminan mientras tanto siguen reservadas hasta que vuelve. Después el proceso de fondo publica los eventos en su vuelta siguiente.
- Transferencias tiene que seguir corriendo en una sola instancia.
