# ADR-015 — Persistencia (D3)

## Estado
Aceptado. Reemplaza a [ADR-003](ADR-003-persistencia-inicial.md).

## Por qué revisamos ADR-003
Cuando empezamos a pensar la implementación revisamos dos cosas. Para las bases relacionales pasamos de PostgreSQL a MySQL, porque las herramientas que usamos en el backend (Go con GORM y el driver de MySQL) nos dan más facilidad para conectarnos, definir los modelos y armar las migraciones. Además, con los servicios nuevos de [ADR-014](ADR-014-limites-de-servicios.md), decidimos usar MongoDB en Transferencias.

## Decisión
Cada servicio tiene su propio servidor de base de datos y es el único que lo usa.

| Servicio | Base | Qué guarda y cómo se consulta |
|---|---|---|
| Usuarios | MySQL | Usuarios con email único, rol y el id de su sucursal. Se buscan por id, por email al hacer login y por sucursal para la sección de usuarios |
| Sucursales | MySQL | Sucursales con nombre y dirección. Se buscan por id |
| Inventario | MySQL | Productos, stock por sucursal, reservas y movimientos |
| Transferencias | MongoDB | Una transferencia por documento, con su historial de estados |

Cada MySQL corre en su propio contenedor. Así, si una base se cae o se llena, los otros servicios siguen funcionando. Las dos instancias de Inventario usan el mismo MySQL de Inventario.

### Inventario
De cada producto en cada sucursal guardamos dos números: la cantidad total y la cantidad reservada. El stock disponible es la diferencia, y es el que mostramos y el que usamos para decidir si una transferencia se puede hacer. La base hace cumplir dos reglas: la combinación producto y sucursal no se repite (`UNIQUE`) y lo reservado nunca es negativo ni mayor que el total (`CHECK`, que MySQL respeta desde la 8.0.16).

Las tres operaciones de una transferencia son transacciones de esta base:

- **Reservar.** Lee la fila del origen con `SELECT ... FOR UPDATE`. Si el disponible alcanza, suma a lo reservado y crea la reserva, identificada por el id de la transferencia, en estado `ACTIVA`.
- **Confirmar** (cuando el traslado se completa). En una sola transacción resta la cantidad del total y de lo reservado en el origen, la suma al total del destino (si el destino no tenía fila, la crea), pasa la reserva a `CONFIRMADA` y registra un movimiento de salida y uno de entrada.
- **Liberar** (cuando el traslado se rechaza, se cancela o no se realiza, o cuando la reserva falló). Resta de lo reservado en el origen y pasa la reserva a `LIBERADA`. Si la reserva todavía no existía, la crea directamente como `LIBERADA`, así un pedido de reserva que llega tarde se rechaza.

Bloqueamos con `FOR UPDATE` en la base y no con locks en memoria, porque hay dos instancias de Inventario y un lock en memoria solo protege a una. Los ajustes manuales siguen la misma regla: una salida no puede bajar el total por debajo de lo reservado.

En esta misma base va una tabla más: la de eventos ya procesados, para no confirmar ni liberar dos veces la misma reserva ([ADR-016](ADR-016-comunicacion-entre-servicios.md)).

### Transferencias
Cada transferencia es un documento con lo que se pidió (incluida la hora de llegada deseada y las observaciones, si las hay), su estado actual, la lista de estados por los que pasó, el id que le dio Rumbo, el horario que propuso o comprometió Rumbo, la demora informada y el motivo si se rechazó o se canceló. En MongoDB escribir un documento es atómico, así que un cambio de estado y su historial se guardan juntos.

Las consultas son tres: por id, por estado (los procesos de fondo buscan las que todavía no se mandaron a Rumbo o esperan respuesta) y por sucursal y fecha (las pantallas de los usuarios). Para eso ponemos índices en `estado` y en `sucursal_origen` + `fecha`.

Elegimos MongoDB porque una transferencia siempre se lee y se escribe entera, y no se cruza con datos de otras tablas. En una base relacional la tendríamos repartida en dos tablas (transferencia e historial de estados) y habría que juntarlas en cada lectura. Además, lo que guardamos de las respuestas de Rumbo cambia según el caso: un rechazo trae un motivo, una contrapropuesta trae un horario y su vencimiento, un traslado en curso puede traer una demora. En un documento eso entra sin agregar columnas que quedan vacías la mayoría de las veces.

## Alternativas que descartamos
- **Seguir con PostgreSQL.** Funcionaba igual, pero con las herramientas que usamos en Go nos resulta más fácil trabajar con MySQL.

## Consecuencias
- Hay que levantar cuatro bases (tres MySQL y un MongoDB) y configurar cada una.
- Una transferencia queda guardada en Transferencias y el stock que mueve queda en Inventario, en bases distintas. Por eso no podemos confirmar las dos cosas juntas con una transacción: Transferencias avisa con un evento e Inventario confirma o libera por su cuenta.
- Las reservas pueden durar días, mientras el traslado no termina.
- Las reservas y los eventos procesados se guardan siempre, así que esas tablas crecen con el tiempo.
