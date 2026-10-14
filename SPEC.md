# Stock Volcamelli — Especificación

## 1. Descripción

Stock Volcamelli lleva el inventario de una empresa con varias sucursales. Primero se reserva stock en origen; recién cuando Rumbo acepta el traslado se confirma la salida y la entrada. El seguimiento físico del viaje no forma parte de esta versión.

Este documento describe el comportamiento a implementar. La Entrega 1 contiene el diseño y el contrato, no servicios funcionando.

## 2. Objetivo

Que las cantidades sean correctas aunque haya operaciones simultáneas, mensajes duplicados o respuestas que se pierdan. Un timeout no prueba que una operación no se ejecutó: su resultado se reconcilia antes de decidir.

## 3. Alcance

- Usuarios, login y roles.
- Sucursales con nombre y dirección.
- Catálogo de productos y consulta de stock disponible por sucursal.
- Ajustes manuales de entradas y salidas, con motivo y usuario responsable.
- Transferencias y consulta de su estado e historial.
- Historial de movimientos de stock por producto y sucursal.
- Búsqueda de catálogo con texto, proveedor, sucursal, stock bajo, paginación y ordenamiento.
- Consulta del catálogo y stock por otro grupo mediante el contrato publicado.

## 4. Actores

| Actor | Qué hace |
|---|---|
| Administrador (`ADMIN`) | Todo lo del operador, más administrar usuarios, sucursales y productos, y hacer ajustes manuales |
| Operador (`OPERATOR`) | Consulta productos, stock y movimientos; crea transferencias y consulta sus estados |
| Grupo consumidor (`INTEGRACION`) | Solo consulta catálogo y stock mediante el contrato; no accede a operaciones internas |
| Rumbo | Proveedor externo del traslado. Su contrato lo entrega el otro grupo |

## 5. Entidades

### User
Propiedad de Usuarios: id, nombre, email único, contraseña con hash y un rol (`ADMIN`, `OPERATOR` o `INTEGRACION`).

### Branch
Propiedad de Sucursales: id, nombre y dirección. Guardamos la dirección para el traslado; el formato que deba enviarse a Rumbo está pendiente de su contrato.

### Product
Propiedad de Inventario: id, nombre, descripción y proveedor. Proveedor es un texto opcional, no una entidad administrable.

### Stock
Propiedad de Inventario: producto, sucursal, cantidad total, cantidad reservada y versión del registro. La cantidad disponible se calcula como `total − reservada`, no se modifica por separado.

Hay un solo registro por producto y sucursal y siempre se cumple `0 ≤ reservada ≤ total`. El total es el saldo contable del inventario, no una medición de dónde está el camión. Pantallas y API llaman **stock disponible** a esa diferencia; el campo público `quantity` expresa exactamente esa cantidad. Las reservas no acreditan stock en destino.

### Reservation
Propiedad de Inventario: id de transferencia único, producto, origen, destino, cantidad, fecha, plazo de decisión y estado `ACTIVA`, `CONFIRMADA` o `LIBERADA`. Conserva el resultado de cada operación para responder a reintentos. No almacena el estado ni el historial del proceso de Transferencias ni el id de Rumbo.

`ACTIVA → CONFIRMADA` y `ACTIVA → LIBERADA` son las únicas transiciones normales. Los estados finales no se reabren. Una liberación que llegue antes de la reserva deja una marca `LIBERADA` para impedir que una solicitud demorada reserve después.

### StockMovement
Propiedad de Inventario: id, producto, sucursal, tipo, cantidad, total y reservado resultantes, motivo, fecha, usuario e id de transferencia cuando corresponda. Tipos: `ENTRADA`, `SALIDA`, `RESERVA`, `LIBERACION`. Motivos: `AJUSTE` o `TRANSFERENCIA`. Reservar y liberar modifican el disponible, pero no el total; confirmar registra una salida y una entrada vinculadas.

### Transfer
Propiedad de Transferencias: id, clave de idempotencia del solicitante, huella del pedido, producto, cantidad, origen, destino, usuario, fechas, estado, versión, historial, fase a reconciliar, decisión final elegida y su motivo, referencia de reserva, id de Rumbo cuando exista y evidencia de su respuesta. También conserva el trabajo pendiente de publicar en su outbox.

| Estado | Qué significa |
|---|---|
| `RESERVA_PENDIENTE` | Pedido guardado; falta conocer el resultado de la reserva |
| `SIN_STOCK` | Inventario rechazó por disponible insuficiente; final |
| `PENDIENTE_RUMBO` | Reserva activa; falta una decisión del proveedor |
| `RECONCILIACION_PENDIENTE` | Resultado desconocido de una operación; se conserva su fase y referencia |
| `CONFIRMACION_PENDIENTE` | Aceptación de Rumbo registrada; falta comprobar la confirmación en Inventario |
| `LIBERACION_PENDIENTE` | Rechazo o vencimiento seguro registrado; falta comprobar la liberación |
| `CONFIRMADA` | Inventario confirmó la reserva y los dos movimientos; final |
| `RECHAZADA` | Rumbo rechazó y la reserva fue liberada; final |
| `VENCIDA` | Venció el plazo, se comprobó que no puede ejecutarse un traslado y la reserva fue liberada; final |

Transiciones permitidas:

- `RESERVA_PENDIENTE → PENDIENTE_RUMBO`, `SIN_STOCK` o `RECONCILIACION_PENDIENTE` según respuesta.
- `PENDIENTE_RUMBO → CONFIRMACION_PENDIENTE`, `LIBERACION_PENDIENTE` o `RECONCILIACION_PENDIENTE`.
- `RECONCILIACION_PENDIENTE` vuelve a la fase que corresponda al resultado comprobado. No permite cambiar una decisión final ya persistida. Si venció antes de reservar y se cierra de forma segura la intención, pasa a `LIBERACION_PENDIENTE` para impedir una reserva tardía.
- `CONFIRMACION_PENDIENTE → CONFIRMADA` solo con evidencia de Inventario. Un timeout conserva la decisión de confirmar y pasa a reconciliación de esa operación.
- `LIBERACION_PENDIENTE → RECHAZADA` o `VENCIDA` según el motivo, solo con evidencia de Inventario. Un timeout conserva la decisión de liberar y pasa a reconciliación.

Una respuesta repetida no agrega otra transición ni otro movimiento. Una respuesta contradictoria a una decisión final se registra como incidente y se consulta al proveedor; no reabre la transferencia ni altera el stock automáticamente.

## 6. Reglas de negocio

1. Cada dato tiene un único dueño. Solo Inventario modifica total, reservado, reservas y movimientos. Ningún servicio accede a la base de otro.
2. La cantidad solicitada es un entero positivo. Origen y destino son diferentes y existen; el producto existe.
3. Reservar requiere `disponible ≥ cantidad`. La reserva y el incremento de reservado se guardan juntos, con bloqueo de las filas afectadas. Dos instancias de Inventario usan la misma base y las mismas restricciones.
4. Mientras no haya aceptación de Rumbo y confirmación de Inventario, el destino no recibe unidades de esa transferencia.
5. Confirmar descuenta total y reservado del origen, suma total en destino, cierra la reserva y guarda movimientos, idempotencia y eventos en una sola transacción MySQL. Ocurre todo o nada.
6. Liberar descuenta solo el reservado del origen. No descuenta nada del destino ni necesita un ajuste manual compensatorio.
7. El plazo para obtener una decisión es de **24 horas desde la creación durable**. Se deja de intentar crear el traslado al vencer. Si nunca se envió y ningún trabajador puede enviarlo, o el proveedor confirma un cierre definitivo sin aceptación, se libera. Una consulta puntual que diga “no encontrado” no basta si un pedido sigue en vuelo.
8. Si al vencer hay un resultado desconocido, se mantiene la reserva en reconciliación, incluso más de 24 horas, y se alerta. No hay borrado por TTL de reservas. Sin garantías del proveedor no se pueden prometer a la vez una liberación puntual y ausencia de traslados tardíos.
9. Si se comprueba una aceptación durante la reconciliación y no hay una decisión de liberar ya cerrada de forma segura, se confirma. Si llega después de un rechazo/cierre definitivo, se trata como contradicción del proveedor. No se inventa una cancelación que Rumbo no haya documentado.
10. Transferencias guarda una única decisión de confirmar o liberar mediante actualización condicionada por estado y versión. Inventario verifica además el estado de la reserva: confirmar una liberada o liberar una confirmada produce conflicto, nunca un segundo efecto.
11. Crear una transferencia requiere una clave de idempotencia por usuario e intención; el mismo pedido devuelve el mismo id y estado. Reusar la clave con otros parámetros se rechaza. Los reintentos internos conservan id, operación y parámetros. Las claves y los estados finales se conservan durante toda la evaluación; no se eliminan mientras pueda haber reenvíos.
12. Los ajustes los hace solo `ADMIN` con motivo. Una salida manual no puede consumir unidades reservadas: el total resultante debe seguir siendo mayor o igual al reservado.
13. Stock bajo significa **disponible ≤ 5**, incluido cero. El filtro requiere sucursal y considera productos con registro de existencias en esa sucursal, igual que el listado de stock. La búsqueda puede tener hasta 5 segundos de atraso en condiciones normales; la consulta puntual y toda escritura validan contra MySQL.
14. El rol `INTEGRACION` solo lee. `ADMIN` y `OPERATOR` también pueden usar las consultas del contrato. La autorización se aplica antes de consultar caché o base.
15. La aceptación de Rumbo es suficiente para el cierre contable; no esperamos la entrega física. No administramos horarios: cualquier adaptación a propuestas del proveedor dependerá de su contrato, todavía pendiente.

Los timeouts, reintentos, eventos y respuestas tardías se detallan en [ADR-016](docs/adr/ADR-016-comunicacion-entre-servicios.md).

## 7. Integraciones

- **Ofrecemos:** catálogo y stock disponible de Inventario, por el gateway. [Contrato](docs/contracts/README.md).
- **Consumimos:** la capacidad de traslados de Rumbo desde Transferencias. El otro grupo entrega el contrato; sus URLs, autenticación, estados, operaciones y garantías siguen pendientes. [Información necesaria](docs/ARCHITECTURE.md#8-integración-con-otros-grupos).

## 8. Fuera de alcance

Gestión de proveedores y compras, facturación, ventas, contabilidad, seguimiento físico de viajes, gestión de horarios, avisos automáticos, reposición automática, predicción de demanda y reportes avanzados. La observabilidad técnica sí forma parte de los requisitos.

## 9. Criterios de aceptación

### Transferencias

**CA-01 — Reserva válida.** Con origen total 10, reservado 0, y destino total 2, reservado 0, reservar 4 deja origen total 10, reservado 4, disponible 6; destino disponible 2. Se registra `RESERVA` y la transferencia pasa a `PENDIENTE_RUMBO`.

**CA-02 — Disponible insuficiente.** Con total 10 y reservado 8, solicitar 3 devuelve `SIN_STOCK`. No cambian cantidades ni se solicita traslado.

**CA-03 — Misma sucursal.** Se rechaza sin reserva ni llamada a Rumbo.

**CA-04 — Datos inválidos.** Producto o sucursal inexistentes, cantidad no entera o no positiva se rechazan sin modificar stock. Una falla de Sucursales no se interpreta como sucursal inexistente.

**CA-05 — Rumbo acepta.** Sobre CA-01, se registra `CONFIRMACION_PENDIENTE`. Inventario confirma atómicamente: origen total 6, reservado 0, disponible 6; destino total 6, reservado 0. Solo entonces queda `CONFIRMADA`, con una salida y una entrada.

**CA-06 — Rumbo rechaza.** Sobre CA-01, queda `LIBERACION_PENDIENTE` hasta que Inventario libere. Después: origen total 10, reservado 0, disponible 10; destino total 2; estado `RECHAZADA` con motivo.

**CA-07 — Unidades no anticipadas.** Mientras CA-01 espera a Rumbo, una salida de 3 en destino se rechaza porque solo tiene 2. No puede usar las 4 unidades reservadas en origen.

**CA-08 — Respuesta de reserva perdida.** Si Inventario reserva pero su respuesta se pierde, se muestra `RECONCILIACION_PENDIENTE`; se consulta por el mismo id y se recupera la reserva existente. No se duplica, no se libera por timeout y no se informa fracaso definitivo.

**CA-09 — Duplicados y concurrencia.** Repetir la creación con la misma clave devuelve la misma transferencia. Dos consumidores que confirman simultáneamente producen un único par de movimientos; dos liberaciones producen una sola liberación. Confirmar una reserva liberada o liberar una confirmada se rechaza.

### Stock

**CA-10 — Ajuste sobre reservas.** Con total 10 y reservado 8, una salida manual de 3 se rechaza; una salida de 2 deja total 8, reservado 8 y disponible 0.

**CA-11 — Stock bajo coherente.** Con producto A total 10 y reservado 5, y B total 6 y reservado 0, el filtro por esa sucursal incluye solo A. En un máximo de 5 segundos con dependencias sanas, el índice refleja el cambio; la consulta puntual devuelve `quantity: 5` y la pantalla lo rotula “stock disponible”.

### Usuarios y permisos

**CA-12 — Operador.** `OPERATOR` consulta y crea transferencias, pero no administra usuarios, sucursales o productos ni ajusta stock.

**CA-13 — Integración.** `INTEGRACION` consulta catálogo y stock, pero no crea transferencias ni invoca reservar, confirmar o liberar. Token inválido produce `401`; rol sin permiso produce `403`.

### Recuperación y lecturas

**CA-14 — Vencimiento seguro.** A las 24 horas, sin envío posible o con cierre definitivo comprobado en Rumbo, se libera una vez y queda `VENCIDA`. Una reserva retrasada después del cierre no se aplica.

**CA-15 — Vencimiento incierto.** Si el alta en Rumbo termina en timeout, al vencer el plazo queda en reconciliación y conserva la reserva. No se vuelve a crear el traslado sin idempotencia garantizada. Una aceptación luego comprobada permite confirmar; una respuesta contradictoria posterior a una liberación final no mueve stock y genera alerta.

**CA-16 — Dos reservas simultáneas.** Con disponible 5, dos transferencias de 4 atendidas por instancias distintas no pueden reservar ambas; una se acepta y otra queda `SIN_STOCK`.

**CA-17 — Confirmación sin respuesta.** Si MySQL confirma y se pierde la respuesta o el evento, la consulta/reentrega con el mismo id recupera el resultado sin otra salida ni entrada. Se completa el estado de Transferencias.

**CA-18 — Caché caída.** La ficha del producto se obtiene de MySQL si Memcached no responde, con concurrencia acotada. No se usa caché ni Solr para autorizar stock. Se comparan latencias y consultas a MySQL con y sin caché.

**CA-19 — Búsqueda.** Texto, proveedor, sucursal y stock bajo se combinan; hay página, tamaño y orden estable. Los parámetros inválidos se rechazan. Si Solr cae o el índice excede el retraso permitido, se informa indisponibilidad de búsqueda; no se responde una lista vacía como éxito.

**CA-20 — Eventos y reinicio.** Caer luego de guardar y antes de publicar no pierde la intención: el outbox reanuda. Los duplicados no repiten efectos; los mensajes que agotan reintentos van a DLQ y se reprocesan conservando su id tras corregir la causa.
