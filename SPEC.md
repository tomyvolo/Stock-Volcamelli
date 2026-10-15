# Stock Volcamelli — Especificación

## 1. Descripción

Stock Volcamelli lleva el inventario de una empresa con varias sucursales. Registra los productos, cuánto stock tiene cada sucursal y cada movimiento de ese stock. La operación principal es la transferencia de mercadería entre sucursales.

Una transferencia no es solo cambiar dos números. No se puede mandar más de lo que hay disponible en el origen, el stock nunca puede quedar negativo y el traslado depende de Rumbo, el servicio de transporte de otro grupo. Por eso, al crear una transferencia las unidades quedan reservadas en el origen, y pasan al destino recién cuando Rumbo acepta el traslado. Si Rumbo no lo puede hacer, la reserva se libera.

## 2. Objetivo

Que en cualquier momento se sepa cuánto hay disponible de cada producto en cada sucursal, y que las transferencias entre sucursales dejen siempre el stock correcto, aunque algún servicio o Rumbo fallen en el medio.

## 3. Alcance

El sistema permite:

- Administrar usuarios con sus roles, y hacer login.
- Administrar sucursales con su nombre y dirección.
- Administrar el catálogo de productos.
- Consultar el stock disponible de cada producto en cada sucursal.
- Hacer ajustes manuales de stock (entradas y salidas), por ejemplo cuando llega mercadería o hay una rotura.
- Transferir productos entre sucursales y consultar en qué estado está cada transferencia.
- Consultar el historial de movimientos de un producto en una sucursal.
- Buscar en el catálogo, incluido un filtro de productos con stock bajo (5 unidades disponibles o menos).
- Que otro grupo consulte nuestro catálogo y stock mediante un contrato.

## 4. Actores

| Actor | Qué hace |
|---|---|
| Administrador (`ADMIN`) | Todo lo del operador, más administrar usuarios, sucursales y productos, y hacer ajustes manuales de stock |
| Operador (`OPERATOR`) | Consulta productos y stock, crea transferencias y ve su estado |
| Grupo consumidor (`INTEGRACION`) | Otro sistema que consulta nuestro catálogo y stock. Solo puede leer |
| Rumbo | Servicio de transporte de otro grupo. Recibe nuestros pedidos de traslado y los acepta o rechaza |

## 5. Entidades

### User
Un usuario del sistema.
- id, nombre, email, contraseña (guardada con hash), rol.
- Rol: `ADMIN`, `OPERATOR` o `INTEGRACION`.

### Branch
Una sucursal.
- id, nombre, dirección.
- La dirección es obligatoria porque Rumbo la pide para cada traslado.

### Product
Un producto del catálogo.
- id, nombre, descripción, proveedor.
- El proveedor es solo un texto con el nombre de a quién le compramos el producto. Puede quedar vacío.

### Stock
Cuántas unidades hay de un producto en una sucursal.
- producto, sucursal, cantidad total, cantidad reservada.
- El disponible es el total menos lo reservado. Es el que se muestra y el que se usa para transferir.
- Hay un solo registro por producto y sucursal. Ningún número puede ser negativo, y lo reservado nunca supera el total.

### Reservation
Las unidades apartadas en el origen para una transferencia.
- id de la transferencia, producto, sucursal de origen, sucursal de destino, cantidad, estado.
- Estado: `ACTIVA` mientras Rumbo decide, `CONFIRMADA` si las unidades pasaron al destino, `LIBERADA` si volvieron a estar disponibles.

### StockMovement
Cada cambio en el total de stock queda registrado como un movimiento.
- id, producto, sucursal, tipo (entrada o salida), cantidad, stock resultante, motivo, fecha.
- Motivo: `AJUSTE` o `TRANSFERENCIA`. Si viene de una transferencia, guarda su id.

### Transfer
Una transferencia entre dos sucursales.
- id, producto, cantidad, sucursal de origen, sucursal de destino, usuario que la creó, fecha, estado, historial de estados, id del traslado en Rumbo, motivo del rechazo si lo hubo.

Estados:

| Estado | Qué significa |
|---|---|
| `SIN_STOCK` | El origen no tenía stock disponible suficiente. No se reservó nada |
| `FALLIDA` | Inventario no respondió a tiempo. Si llegó a reservar, la reserva se libera |
| `RESERVADA` | Las unidades están reservadas y esperamos la respuesta de Rumbo |
| `CONFIRMADA` | Rumbo aceptó el traslado. Inventario mueve el stock al destino al procesar el aviso |
| `RECHAZADA` | Rumbo rechazó el traslado, o no pudimos pedírselo en 24 horas. La reserva se liberó |

Todos son finales salvo `RESERVADA`.

## 6. Reglas de negocio

1. Cada usuario tiene un solo rol.
2. El stock de un producto en una sucursal nunca puede ser negativo, y lo reservado nunca puede superar el total.
3. Todo cambio en el total de stock genera un movimiento.
4. Los ajustes manuales de stock los hace solo un ADMIN, y siempre con un motivo. Una salida no puede dejar el total por debajo de lo reservado.
5. En una transferencia, el origen y el destino tienen que ser sucursales distintas que existan, y la cantidad tiene que ser mayor a 0.
6. No se puede transferir más de lo que hay disponible en el origen.
7. Al crear una transferencia, las unidades se reservan en el origen. El destino no recibe nada todavía.
8. Si Rumbo acepta el traslado, en un solo paso se resta del origen y se suma al destino.
9. Si Rumbo rechaza el traslado, o no pudimos pedírselo en 24 horas, la reserva se libera y el origen vuelve a tener esas unidades disponibles.
10. Si Rumbo propone otro horario, se acepta: el horario no nos importa.
11. Una vez que Rumbo acepta, la transferencia queda confirmada; no seguimos el viaje.
12. Un producto tiene stock bajo cuando le quedan 5 unidades disponibles o menos en una sucursal.
13. La cuenta `INTEGRACION` solo puede usar las consultas del contrato.

## 7. Integraciones

- **Lo que ofrecemos:** consultas del catálogo y del stock disponible de Inventario. El contrato está en [docs/contracts/README.md](docs/contracts/README.md).
- **Lo que usamos:** Rumbo, para cada transferencia. Cómo lo usamos está en [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## 8. Fuera de alcance

- Gestión de proveedores y compras a proveedores. Del proveedor solo guardamos el nombre en el producto.
- Facturación, ventas y contabilidad.
- Gestión de envíos: el traslado lo hace Rumbo. Nosotros solo se lo pedimos y vemos si lo acepta; no seguimos el viaje ni manejamos horarios.
- Avisos automáticos de stock bajo. El stock bajo se consulta con el filtro del catálogo.
- Reposición automática y predicción de demanda.
- Reportes o estadísticas.

## 9. Criterios de aceptación

### Transferencias

**CA-01 — Transferencia válida.** Dado que el origen tiene 10 unidades disponibles de un producto y el destino tiene 2, cuando un operador transfiere 4, entonces la transferencia queda `RESERVADA`, el origen tiene 6 disponibles (10 en total y 4 reservadas) y el destino sigue con 2.

**CA-02 — Stock insuficiente.** Dado que el origen tiene 3 unidades disponibles, cuando se intenta transferir 5, entonces la transferencia queda `SIN_STOCK` y el stock de las dos sucursales no cambia.

**CA-03 — Misma sucursal.** Cuando se intenta transferir con la misma sucursal como origen y destino, entonces la transferencia se rechaza.

**CA-04 — Sucursal inexistente.** Cuando se intenta transferir desde o hacia una sucursal que no existe, entonces la transferencia se rechaza y el stock no cambia.

**CA-05 — Rumbo acepta.** Dada la transferencia de CA-01, cuando Rumbo acepta el traslado, directamente o después de proponer otro horario, entonces la transferencia queda `CONFIRMADA`, el origen queda con 6 (sin reservas), el destino con 6, y se registra un movimiento en cada sucursal.

**CA-06 — Rumbo rechaza.** Dada la transferencia de CA-01, cuando Rumbo rechaza el traslado, entonces la transferencia queda `RECHAZADA` con el motivo de Rumbo, el origen vuelve a tener 10 disponibles y el destino sigue con 2.

**CA-07 — Reserva que se cruza con otra.** Dado que el origen tiene 5 unidades disponibles, cuando dos operadores crean al mismo tiempo transferencias de 4 unidades cada una, entonces solo una queda `RESERVADA` y la otra queda `SIN_STOCK`.

**CA-08 — Inventario no responde.** Cuando Inventario no responde a tiempo al crear una transferencia, entonces la transferencia queda `FALLIDA` y, al final, el origen tiene el mismo stock disponible que antes de crearla.

**CA-09 — Aviso repetido.** Cuando Inventario recibe dos veces el mismo aviso de que Rumbo aceptó un traslado, entonces mueve el stock una sola vez.

### Stock

**CA-10 — Ajuste que dejaría stock negativo.** Dado que una sucursal tiene 2 unidades, cuando un ADMIN registra una salida de 3, entonces el ajuste se rechaza y el stock sigue en 2.

**CA-11 — Ajuste sobre unidades reservadas.** Dado que una sucursal tiene 10 unidades en total y 4 reservadas, cuando un ADMIN registra una salida de 8, entonces el ajuste se rechaza, porque solo hay 6 disponibles.

**CA-12 — Stock bajo.** Dado que una sucursal tiene 5 unidades disponibles de un producto y 6 de otro, cuando se filtra el catálogo por stock bajo en esa sucursal, entonces aparece solo el primero.

### Usuarios y permisos

**CA-13 — Permisos del operador.** Un `OPERATOR` puede consultar stock y crear transferencias, pero no puede crear usuarios ni sucursales, ni hacer ajustes de stock.

**CA-14 — Cuenta de integración.** La cuenta `INTEGRACION` puede consultar productos y stock, pero no puede crear transferencias ni modificar nada.
