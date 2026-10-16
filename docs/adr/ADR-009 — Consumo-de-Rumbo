# ADR-009 — Consumo de Rumbo (D9)

## Estado
Aceptado.

## Contexto
Para trasladar mercadería entre sucursales usamos Rumbo, el servicio de traslados de otro grupo. Su contrato ("Solicitar traslado") funciona así: se pide un token y se crea una solicitud de traslado. Un operador de Rumbo la revisa y la acepta, propone otro horario o la rechaza. Si la acepta, el traslado pasa por sus etapas hasta que llega a destino o falla. Rumbo no avisa los cambios: hay que consultarlos.

Decidimos usar toda la funcionalidad de Rumbo para que las transferencias se comporten como en la realidad: el operador puede elegir cuándo quiere que llegue la mercadería, coordina el horario con Rumbo, sigue el viaje y puede cancelarlo. El stock pasa al destino recién cuando la mercadería llega.

## Decisión

### Qué usamos del contrato
| Operación de Rumbo | Para qué |
|---|---|
| `POST /v2/auth/token` | Obtener el token. Dura 1 hora: lo guardamos en memoria y pedimos otro cuando está por vencer o si Rumbo responde `401` |
| `POST /v2/traslados` | Pedir el traslado |
| `GET /v2/traslados?actualizado_desde=` | Consultar cada 60 segundos qué solicitudes cambiaron |
| `GET /v2/traslados/{id}` | Ver una solicitud puntual cuando Rumbo nos dice que su estado no permite lo que pedimos |
| `POST /v2/traslados/{id}/respuesta-contrapropuesta` | Mandar la respuesta del operador al horario que propone Rumbo |
| `POST /v2/traslados/{id}/cancelacion` | Cancelar una transferencia a pedido del operador |

### Cómo pedimos el traslado
Cuando el operador crea la transferencia, además del producto, la cantidad y las sucursales, puede indicar tres cosas opcionales: a qué hora quiere que llegue, cuánta flexibilidad acepta alrededor de esa hora y observaciones para el retiro o la entrega. Le mandamos a Rumbo:

- `cosa`: el nombre del producto y la cantidad;
- `origen` y `destino`: el nombre y la dirección de cada sucursal, que traemos de Sucursales;
- `hora_llegada_solicitada` y `tolerancia_minutos`, si el operador los cargó;
- `observaciones`, si las hay;
- `referencia_externa`: el id de nuestra transferencia.

Como `Idempotency-Key` usamos el id de la transferencia. Si un pedido falla o no sabemos si llegó, lo reintentamos con la misma clave y Rumbo no crea una solicitud repetida. Rumbo responde `202` y guardamos el id que le asignó.

### Cómo seguimos la transferencia
Cada 60 segundos consultamos los cambios desde la última fecha que procesamos y guardamos la nueva fecha. Cada solicitud la reconocemos por su `referencia_externa`. El adaptador de Rumbo traduce sus estados a los nuestros:

| En Rumbo | Nuestro estado | Qué pasa con el stock |
|---|---|---|
| Solicitud `RECIBIDA` | `RESERVADA` | Sigue reservado en el origen |
| Solicitud `CONTRAPROPUESTA` | `RESERVADA` o `ESPERANDO_CONFIRMACION` (ver abajo) | Sigue reservado |
| Solicitud `ACEPTADA`, traslado `PROGRAMADO` o `ASIGNADO` | `PROGRAMADA` | Sigue reservado. Mostramos la hora comprometida |
| Traslado `EN_CURSO` | `EN_CURSO` | Sigue reservado. Si Rumbo informa una demora, la mostramos |
| Traslado `COMPLETADO` | `COMPLETADA` | Publicamos `TrasladoCompletado` e Inventario pasa el stock al destino |
| Solicitud `RECHAZADA` o `RECHAZADA_POR_CLIENTE` | `RECHAZADA` | Publicamos `TrasladoCancelado` e Inventario libera la reserva |
| Solicitud `CANCELADA` o traslado `CANCELADO` | `CANCELADA` | Publicamos `TrasladoCancelado` e Inventario libera la reserva |
| Traslado `NO_REALIZADO` o `INTERRUMPIDO` | `NO_REALIZADA` | Publicamos `TrasladoCancelado` e Inventario libera la reserva |

En cada consulta nos quedamos con el estado actual de cada solicitud, aunque entre una consulta y otra haya pasado por varios. Por ejemplo, si Rumbo la aceptó y el camión salió en el mismo minuto, pasamos de `RESERVADA` a `EN_CURSO` directamente. No perdemos nada importante: el stock solo se mueve en los estados finales, que son los últimos, y una `CONTRAPROPUESTA` no la podemos saltear porque Rumbo espera nuestra respuesta. Una transferencia nunca vuelve a un estado anterior: si llega un dato más viejo que el que ya tenemos, lo ignoramos.

`RECHAZADA_POR_CLIENTE` aparece cuando el operador rechaza el horario propuesto o cuando nadie lo responde en 24 horas.

Un traslado `INTERRUMPIDO` empezó pero no terminó. Suponemos que la mercadería vuelve al origen, así que liberamos la reserva. Si se perdió algo en el camino, un ADMIN lo corrige con un ajuste.

Si Rumbo informa un estado que no está en esta tabla (por ejemplo, porque agregó uno nuevo a su contrato), lo dejamos registrado en los logs y la transferencia no cambia hasta que actualicemos el adaptador.

### Horario propuesto
Cuando Rumbo propone otro horario, lo que hacemos depende de si el operador eligió una hora al crear la transferencia:

- **No eligió hora.** Le daba igual cuándo llegara, así que aceptamos el horario solos, en segundo plano, y la transferencia sigue `RESERVADA` hasta que Rumbo la programe.
- **Eligió una hora.** Rumbo no puede cumplirla, así que le preguntamos. La transferencia pasa a `ESPERANDO_CONFIRMACION` y el operador ve el horario propuesto y lo acepta o lo rechaza antes de que venza (24 horas).

En los dos casos le mandamos a Rumbo `{ "acepta": true }` o `{ "acepta": false }`.

### Cancelación
El operador cancela desde nuestro frontend, escribe un motivo y Transferencias se lo reenvía a Rumbo, que aplica sus reglas:

| Momento | Resultado en Rumbo | Nuestro estado |
|---|---|---|
| Antes de que Rumbo acepte | Solicitud cancelada | `CANCELADA` |
| Aceptada, con 24 horas o más antes de la llegada | Traslado cancelado | `CANCELADA` |
| Aceptada, con menos de 24 horas | Pendiente de que Rumbo lo apruebe | `CANCELACION_PENDIENTE` hasta que Rumbo decida |
| En curso | No se puede cancelar (`409`) | No cambia. El operador ve que ya no se puede |

Si Rumbo aprueba una cancelación pendiente, la transferencia queda `CANCELADA` y se libera la reserva. Si no la aprueba, vuelve al estado que tenga el traslado (normalmente `PROGRAMADA`) y el operador ve en la transferencia que la cancelación no se aceptó.

Si el operador cancela antes de que hayamos llegado a mandarle el pedido a Rumbo, la cancelamos de nuestro lado y no le avisamos nada a Rumbo.

### Errores de Rumbo
| Respuesta | Qué hacemos |
|---|---|
| `401` | Pedimos un token nuevo y reintentamos una vez |
| `422` (datos inválidos, hora en el pasado, poca anticipación o excede la capacidad) | No reintentamos. La transferencia queda `RECHAZADA` con el motivo de Rumbo y se libera la reserva. Para evitar los casos de horario, el frontend ya no deja elegir una hora pasada |
| `409` porque la clave ya se usó con otro pedido | Es un error nuestro. No reintentamos y lo dejamos registrado en los logs para revisarlo |
| `409` porque el estado no permite la operación | Consultamos la solicitud y actualizamos la transferencia según su estado actual |
| `429` | Esperamos los segundos que indica `Retry-After` |
| `503`, timeout o error de red | Reintentamos más tarde con la misma `Idempotency-Key` |

Si Rumbo nunca recibió el pedido, a las 24 horas de creada la transferencia dejamos de intentar: queda `RECHAZADA` y se libera la reserva. Elegimos ese plazo porque es lo que Rumbo guarda la `Idempotency-Key`: después de 24 horas, un reintento podría crear una solicitud nueva. Si el pedido sí le había llegado aunque no nos enteramos, la solicitud aparece en la consulta de cambios con nuestra referencia y la seguimos normalmente.

La respuesta al horario propuesto, sea automática o del operador, se reintenta igual, pero solo hasta que vence el plazo de 24 horas que da Rumbo. Si vence sin que se la hayamos podido mandar, Rumbo marca la solicitud como `RECHAZADA_POR_CLIENTE` y la transferencia queda `RECHAZADA`.

Cuántas veces reintentamos, cada cuánto y cuándo dejamos de llamar a Rumbo por un rato lo definimos en el ADR de D10.

### Si Rumbo cambia o no está
- **Cambios en su contrato.** Toda la traducción está en el adaptador de Rumbo ([ADR-002](ADR-002-arquitectura-interna.md)), así que un cambio de Rumbo solo toca ese adaptador. Guardamos una copia de su contrato en el repositorio y probamos nuestros pedidos contra un mock que se levanta desde esa copia.
- **Rumbo caído.** El catálogo, el stock y las consultas que publicamos siguen funcionando. Las transferencias nuevas se crean igual y quedan `RESERVADA` hasta que Rumbo vuelve. Si el operador cancela o responde un horario mientras Rumbo está caído, lo guardamos y se lo mandamos cuando vuelve.
- **Una sola instancia de Transferencias.** Si hubiera dos, las dos harían la consulta de cambios y mandarían los mismos pedidos. Para evitarlo habría que coordinarlas, y no ganaríamos nada porque Rumbo limita los pedidos igual. Por eso Transferencias corre en una sola instancia.
- **Límite de pedidos.** Rumbo permite 60 pedidos por minuto. La consulta de cambios es uno por minuto, y si hay muchos pedidos para mandar, los que no entran se mandan en la vuelta siguiente.
- **Credenciales.** El `client_id` y el `client_secret` van en variables de entorno, nunca en el repositorio.

## Alternativas que descartamos
- **Mover el stock en el momento, sin esperar a Rumbo.** La transferencia se veía hecha enseguida, pero si Rumbo después no podía hacer el traslado, había que devolver unidades que el destino quizás ya había vendido.
- **Mover el stock cuando Rumbo acepta, sin seguir el viaje.** Era más simple, pero el destino veía mercadería que todavía no había llegado, y si el viaje fallaba nuestro sistema no se enteraba.
- **Aceptar siempre el horario que propone Rumbo.** Era lo que teníamos antes, cuando nunca mandábamos hora. Ahora que el operador puede pedir una, aceptar cualquier otra sin avisarle podía comprometer una entrega en un horario en el que nadie la iba a recibir.
- **Preguntarle siempre al operador.** Como sin hora pedida Rumbo siempre propone una, el operador iba a tener que confirmar todas las transferencias, incluso las que no le importaba cuándo llegaban. Y si se olvidaba, a las 24 horas se caían.

## Consecuencias
- Las unidades quedan reservadas en el origen hasta que el traslado se completa, se cancela o falla. Puede ser varios días.
- Cuando el operador pide una hora y Rumbo propone otra, tiene que responder dentro de las 24 horas. Si no lo hace, la transferencia se cae.
- Nos enteramos de los cambios con hasta 60 segundos de atraso.
- Las transferencias tienen más estados, y el frontend tiene que mostrar cada uno con sus acciones posibles (responder el horario, cancelar).
- Hay que mantener el adaptador y la copia del contrato de Rumbo al día cada vez que publiquen un cambio.
