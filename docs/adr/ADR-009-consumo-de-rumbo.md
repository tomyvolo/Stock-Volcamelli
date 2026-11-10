# ADR-009 — Consumo de Rumbo (D9)

## Estado
Aceptado.

## Contexto
Para trasladar mercadería entre sucursales usamos Rumbo, el servicio de traslados de otro grupo. Lo consumimos en su versión 4.0.1, publicada en `https://rumbo-gateway.onrender.com`. Su contrato ("Reservar traslado") funciona así: se pide un token, se consulta qué franjas horarias tienen lugar y se reserva una. La respuesta es definitiva en el momento: o el traslado queda `PROGRAMADO` o la franja no tiene lugar y hay que elegir otra. Nadie de Rumbo revisa el pedido ni propone otro horario. Después el traslado avanza hasta que llega a destino, y Rumbo no avisa los cambios: hay que consultarlos.

Las franjas son de 2 horas entre las 8 y las 20, hora de Argentina. Se reservan con al menos 2 horas de anticipación y hasta 30 días adelante, y cada franja tiene un cupo de traslados que comparten todos los grupos que usan Rumbo.

Queremos que las transferencias se comporten como en la realidad: el usuario puede elegir cuándo quiere que llegue la mercadería, sigue el viaje y puede cancelarlo. El stock pasa al destino recién cuando la mercadería llega.

## Decisión

### Qué usamos del contrato
| Operación de Rumbo | Para qué |
|---|---|
| `POST /auth/token` | Obtener el token. Dura 1 hora: lo guardamos en memoria y pedimos otro cuando está por vencer o si Rumbo responde `401` |
| `GET /disponibilidad?fecha=` | Ver qué franjas de un día tienen lugar |
| `POST /traslados` | Reservar el traslado en una franja |
| `GET /traslados?actualizado_desde=` | Consultar cada 60 segundos qué traslados cambiaron |
| `GET /traslados/{id}` | Ver un traslado puntual cuando Rumbo nos dice que su estado no permite lo que pedimos |
| `POST /traslados/{id}/cancelacion` | Cancelar una transferencia a pedido del usuario |

### Cómo elegimos la franja
Cuando el usuario crea la transferencia, además del producto, la cantidad y las sucursales, puede indicar dos cosas opcionales: a qué hora quiere que llegue y observaciones para el retiro o la entrega.

- **Si eligió una hora,** reservamos la franja que la contiene. Si esa franja no tiene lugar, la primera que tenga lugar después.
- **Si no eligió hora,** reservamos la primera franja con lugar.

Para saber qué franjas tienen lugar consultamos la disponibilidad del día, y si ese día no queda ninguna, la del día siguiente, hasta los 30 días que permite Rumbo. Si en esos 30 días no hay lugar, la transferencia queda `RECHAZADA` y se libera la reserva.

Elegimos la franja nosotros en lugar de mostrarle al usuario la lista de franjas para que la elija. Así crear una transferencia sigue siendo un solo paso, igual que antes, y el usuario que no tiene apuro no tiene que pensar en horarios.

### Cómo pedimos el traslado
Le mandamos a Rumbo:

- `cosa`: el nombre del producto y la cantidad;
- `origen` y `destino`: el nombre y la dirección de cada sucursal, que traemos de Sucursales;
- `franja_inicio`: el inicio de la franja elegida, tal como lo devolvió la disponibilidad;
- `observaciones`, si las hay;
- `referencia_externa`: el id de nuestra transferencia.

Como `Idempotency-Key` usamos el id de la transferencia junto con el inicio de la franja. Antes de mandar el pedido guardamos la franja elegida, así un reintento usa la misma franja y la misma clave, y Rumbo no reserva dos veces aunque el primer pedido le haya llegado y no nos hayamos enterado. Si la franja ya no tiene lugar, la siguiente es otra franja y por eso lleva otra clave, como pide el contrato.

Rumbo responde `201` con el traslado `PROGRAMADO`. Guardamos su id y la franja, y la transferencia pasa a `PROGRAMADA`.

### Cómo seguimos la transferencia
Cada 60 segundos consultamos los cambios desde la última fecha que procesamos y guardamos la nueva fecha. Cada traslado lo reconocemos por su `referencia_externa`. El adaptador de Rumbo traduce sus estados a los nuestros:

| En Rumbo | Nuestro estado | Qué pasa con el stock |
|---|---|---|
| `PROGRAMADO` o `ASIGNADO` | `PROGRAMADA` | Sigue reservado. Mostramos la franja en la que llega |
| `EN_CURSO` | `EN_CURSO` | Sigue reservado. Si Rumbo informa una demora, la mostramos |
| `COMPLETADO` | `COMPLETADA` | Publicamos `TrasladoCompletado` e Inventario pasa el stock al destino |
| `CANCELADO` | `CANCELADA` | Publicamos `TrasladoCancelado` e Inventario libera la reserva |
| `NO_REALIZADO` o `INTERRUMPIDO` | `NO_REALIZADA` | Publicamos `TrasladoCancelado` e Inventario libera la reserva |

`ASIGNADO`, `NO_REALIZADO` e `INTERRUMPIDO` están en el contrato pero esta versión de Rumbo no los produce. Los traducimos igual, porque no cuesta nada y no tenemos que tocar el adaptador el día que empiecen a aparecer. `EN_CURSO` lo marca a mano un operador de Rumbo.

En cada consulta nos quedamos con el estado actual de cada traslado, aunque entre una consulta y otra haya pasado por varios. Por ejemplo, si el camión salió y llegó en el mismo minuto, pasamos de `PROGRAMADA` a `COMPLETADA` directamente. No perdemos nada importante, porque el stock solo se mueve en los estados finales, que son los últimos. Una transferencia nunca vuelve a un estado anterior: si llega un dato más viejo que el que ya tenemos, lo ignoramos.

Un traslado `INTERRUMPIDO` empezó pero no terminó. Suponemos que la mercadería vuelve al origen, así que liberamos la reserva. Si se perdió algo en el camino, un ADMIN lo corrige con un ajuste.

Si Rumbo informa un estado que no está en esta tabla, lo dejamos registrado en los logs y la transferencia no cambia hasta que actualicemos el adaptador.

Si aparece un traslado de una transferencia que de nuestro lado ya terminó, lo cancelamos en Rumbo. Pasa cuando el usuario cancela mientras no sabíamos si el pedido le había llegado a Rumbo: la cancelamos de nuestro lado y, si resulta que Rumbo sí lo había reservado, liberamos ese lugar de la franja para los demás grupos.

Antes de tomarlo como suelto miramos la fecha del dato. La consulta de cambios y el envío de las cancelaciones corren por separado, así que la consulta puede traer el traslado todavía `PROGRAMADO` justo cuando el envío ya lo estaba cancelando. Ese dato es más viejo que la cancelación confirmada y lo ignoramos. Por eso guardamos la fecha del último dato de Rumbo también cuando la transferencia ya terminó. Lo encontramos probando contra el Rumbo real: mandábamos una segunda cancelación con la misma clave y otro motivo, y Rumbo la rechazaba con `409`.

### Cancelación
El usuario cancela desde nuestro frontend y escribe un motivo:

| Momento | Qué hacemos | Nuestro estado |
|---|---|---|
| `RESERVADA`, todavía sin traslado en Rumbo | La cancelamos de nuestro lado, sin avisarle a Rumbo | `CANCELADA` |
| `PROGRAMADA` | Le mandamos la cancelación a Rumbo con el motivo. Rumbo la aplica en el momento | `CANCELADA` |
| `EN_CURSO` | No se puede cancelar | No cambia. El usuario ve que ya no se puede |

La cancelación a Rumbo se manda en segundo plano, con el id de la transferencia seguido de `-cancelacion` como `Idempotency-Key`, para que no choque con la clave de la reserva. Hasta que Rumbo la confirma, la transferencia sigue `PROGRAMADA` y el usuario ve que la cancelación está pedida. Si Rumbo responde que el estado no lo permite (el traslado salió mientras tanto), consultamos el traslado y la transferencia sigue su estado actual.

Si la cancelación es con menos de 24 horas de anticipación, Rumbo la aplica igual y la marca como tardía para su operación. Nosotros no hacemos nada distinto.

### Errores de Rumbo
| Respuesta | Qué hacemos |
|---|---|
| `401` | Pedimos un token nuevo y reintentamos una vez |
| `409 SIN_DISPONIBILIDAD` | Otro grupo tomó el último lugar entre la consulta y la reserva. Buscamos la franja siguiente con lugar y la reservamos con otra clave |
| `422` porque la franja ya empezó o empieza en menos de 2 horas | Pasa si reintentamos tarde, por ejemplo después de que Rumbo estuvo caído. Rumbo no reservó nada, así que buscamos la siguiente franja con lugar |
| Otro `422` (datos inválidos o la carga excede la capacidad) | No reintentamos. La transferencia queda `RECHAZADA` con el motivo de Rumbo y se libera la reserva |
| `409` porque la clave ya se usó con otro pedido | Es un error nuestro. No reintentamos: la transferencia queda `RECHAZADA`, se libera la reserva y lo dejamos registrado en los logs para revisarlo |
| `409` porque el estado no permite la operación | Consultamos el traslado y actualizamos la transferencia según su estado actual |
| `429` | Esperamos los segundos que indica `Retry-After` |
| `502`, `503`, `504`, timeout o error de red | Reintentamos más tarde con la misma `Idempotency-Key` |

Si Rumbo devuelve un `codigo` que no conocemos, decidimos por el status, como pide su contrato: un `4xx` es un problema del pedido y la transferencia queda `RECHAZADA`; un `5xx` se reintenta.

Si a las 24 horas de creada la transferencia todavía no pudimos reservar, dejamos de intentar: queda `RECHAZADA` y se libera la reserva. Si el pedido sí le había llegado a Rumbo aunque no nos enteramos, el traslado aparece en la consulta de cambios y lo cancelamos, como explicamos arriba.

Cuántas veces reintentamos, cada cuánto y cuándo dejamos de llamar a Rumbo por un rato lo definimos en [ADR-010](ADR-010-resiliencia.md).

### Si Rumbo cambia o no está
- **Cambios en su contrato.** Toda la traducción está en el adaptador de Rumbo ([ADR-002](ADR-002-arquitectura-interna.md)), así que un cambio de Rumbo solo toca ese adaptador. Guardamos una copia de su contrato en el repositorio. Los tests del adaptador levantan, dentro de cada test, un servidor de prueba que responde como dice esa copia, así probamos cada caso (reserva, franja sin lugar, timeout, token vencido, límite de pedidos) sin depender de que Rumbo esté andando.
- **Rumbo caído.** El catálogo, el stock y las consultas que publicamos siguen funcionando. Las transferencias nuevas se crean igual y quedan `RESERVADA` hasta que Rumbo vuelve. Si el usuario cancela una transferencia `PROGRAMADA` mientras Rumbo está caído, lo guardamos y se lo mandamos cuando vuelve.
- **Rumbo dormido.** Rumbo corre en el plan gratis de Render y se duerme si nadie lo usa. El primer pedido puede tardar o devolver `502` o `504`, y lo reintentamos como cualquier otra falla.
- **Sin Rumbo configurado.** Si Transferencias arranca sin la URL de Rumbo, no le manda nada y las transferencias quedan `RESERVADA`. Lo usamos en las pruebas de carga, para no ocupar el cupo de los otros grupos ([ADR-013](ADR-013-capacidad-y-costos.md)).
- **Una sola instancia de Transferencias.** Si hubiera dos, las dos harían la consulta de cambios y mandarían los mismos pedidos. Para evitarlo habría que coordinarlas, y no ganaríamos nada porque Rumbo limita los pedidos igual. Por eso Transferencias corre en una sola instancia.
- **Límite de pedidos.** Rumbo permite 60 pedidos por minuto. La consulta de cambios es uno por minuto, y si hay muchos pedidos para mandar, los que no entran se mandan en la vuelta siguiente.
- **Credenciales.** La URL, el `client_id` y el `client_secret` van en variables de entorno, nunca en el repositorio.

## Alternativas que descartamos
- **Mover el stock en el momento, sin esperar a Rumbo.** La transferencia se veía hecha enseguida, pero si Rumbo después no podía hacer el traslado, había que devolver unidades que el destino quizás ya había vendido.
- **Mover el stock cuando Rumbo reserva, sin seguir el viaje.** Era más simple, pero el destino veía mercadería que todavía no había llegado, y si el viaje fallaba nuestro sistema no se enteraba.
- **Mostrarle al usuario las franjas para que elija una.** Le daba más control, pero agregaba un paso a cada transferencia y una pantalla más. Además la lista puede quedar vieja: otro grupo puede tomar el último lugar mientras el usuario mira, y tendríamos que volver a preguntarle.
- **Rechazar la transferencia si la franja de la hora pedida no tiene lugar.** El usuario tenía que volver a crearla probando otra hora. Preferimos la siguiente franja con lugar: el usuario ve en la transferencia a qué hora llega y, si no le sirve, la cancela.
- **Seguir probando contra un Rumbo falso nuestro.** Lo usamos mientras no estaba el real. Ahora que está publicado, mantener el falso al día con cada versión de Rumbo era trabajo de más, y los tests con un servidor dentro de cada test cubren los mismos casos.

## Consecuencias
- Las unidades quedan reservadas en el origen hasta que el traslado se completa, se cancela o falla. Puede ser varios días.
- La transferencia puede llegar después de la hora que pidió el usuario si esa franja ya estaba llena. Lo ve en la transferencia y puede cancelarla.
- Nos enteramos de los cambios con hasta 60 segundos de atraso.
- Cada franja tiene un cupo que compartimos con los otros grupos. Las transferencias que creamos para probar ocupan lugar, así que las cancelamos al terminar.
- `EN_CURSO` y `COMPLETADO` los marca un operador de Rumbo, así que el viaje completo no avanza solo.
- Para probar las transferencias de punta a punta hace falta internet, porque Rumbo es un servicio publicado.
- Hay que mantener el adaptador y la copia del contrato de Rumbo al día cada vez que publiquen un cambio.
