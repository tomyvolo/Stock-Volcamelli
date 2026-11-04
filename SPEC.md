# Stock Volcamelli — Especificación

## 1. Descripción

Stock Volcamelli lleva el inventario de una empresa con varias sucursales. Registra los productos, cuánto stock tiene cada sucursal y cada movimiento de ese stock. El flujo principal es la consulta del stock disponible de cada sucursal, que es lo que usan todas las pantallas y lo que le ofrecemos a otro grupo. El stock se mueve con transferencias de mercadería entre sucursales.

Una transferencia no es solo cambiar dos números. No se puede mandar más de lo que hay disponible en el origen, el stock nunca puede quedar negativo y el traslado lo hace Rumbo, el servicio de transporte de otro grupo. Por eso, al crear una transferencia las unidades quedan reservadas en el origen y pasan al destino recién cuando la mercadería llega. Si el traslado se rechaza, se cancela o no se realiza, la reserva se libera.

## 2. Objetivo

Que en cualquier momento se sepa cuánto hay disponible de cada producto en cada sucursal, y que las transferencias entre sucursales dejen siempre el stock correcto, aunque algún servicio o Rumbo fallen en el medio.

## 3. Alcance

El sistema permite:

- Administrar usuarios con su rol y su sucursal, y hacer login.
- Administrar sucursales con su nombre y dirección.
- Ver todas las sucursales con sus datos y los usuarios que trabajan en cada una.
- Administrar el catálogo de productos.
- Consultar el stock disponible de cada producto en cada sucursal.
- Hacer ajustes manuales de stock (entradas y salidas), por ejemplo cuando llega mercadería o hay una rotura.
- Transferir productos entre sucursales, opcionalmente con una hora de llegada deseada.
- Seguir cada transferencia hasta que llega: horario comprometido, viaje en curso y demoras.
- Aceptar o rechazar el horario que propone el transporte, y cancelar una transferencia.
- Consultar el historial de movimientos de un producto en una sucursal.
- Buscar en el catálogo por texto y proveedor, con los resultados ordenados por relevancia.
- Ver los productos con stock bajo (5 unidades disponibles o menos) de una sucursal.
- Que otro grupo consulte nuestro catálogo y stock mediante un contrato.
- Consultar stock, buscar productos, crear transferencias y ver su estado conversando con un asistente desde el frontend. Las transferencias que pide el asistente se crean solo si el usuario las confirma.
- Ver los cambios de stock y de estado de las transferencias en el momento, sin recargar la pantalla.

## 4. Actores

| Actor | Qué hace |
|---|---|
| Director general (`DIRECTOR`) | Ve y opera todas las sucursales: transferencias, ajustes, usuarios, sucursales y productos. No tiene sucursal asignada |
| Admin de sucursal (`ADMIN_SUCURSAL`) | Lo mismo que el vendedor, más ajustes de stock en su sucursal y alta de vendedores de su sucursal |
| Vendedor (`VENDEDOR`) | Consulta productos, stock, sucursales y usuarios de todas las sucursales. Crea, cancela y responde los horarios de las transferencias que salen de su sucursal o llegan a ella |
| Grupo consumidor | Otro sistema que consulta nuestro catálogo y stock con una API key. Solo puede leer |
| Rumbo | Servicio de transporte de otro grupo. Recibe nuestros pedidos de traslado, los acepta, propone otro horario o los rechaza, y hace el viaje |

## 5. Entidades

### User
Un usuario del sistema.
- id, nombre, email, contraseña (guardada con hash), rol, sucursal.
- Rol: `DIRECTOR`, `ADMIN_SUCURSAL` o `VENDEDOR`.
- La sucursal es obligatoria para `ADMIN_SUCURSAL` y `VENDEDOR`, y queda vacía para `DIRECTOR`.
- Cualquier usuario puede ver nombre, email, rol y sucursal de los demás. La contraseña nunca se muestra.

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
- Estado: `ACTIVA` mientras la transferencia no termina, `CONFIRMADA` si las unidades pasaron al destino, `LIBERADA` si volvieron a estar disponibles.

### StockMovement
Cada cambio en el total de stock queda registrado como un movimiento.
- id, producto, sucursal, tipo (entrada o salida), cantidad, stock resultante, motivo, fecha.
- Motivo: `AJUSTE` o `TRANSFERENCIA`. Si viene de una transferencia, guarda su id.

### Transfer
Una transferencia entre dos sucursales.
- id, producto, cantidad, sucursal de origen, sucursal de destino, usuario que la creó, fecha, estado, historial de estados.
- Opcionales: hora de llegada deseada, flexibilidad en minutos y observaciones.
- Lo que informa Rumbo: id del traslado, horario propuesto y hasta cuándo se puede responder, hora comprometida, demora y motivo del rechazo o la cancelación.

Estados:

| Estado | Qué significa |
|---|---|
| `SIN_STOCK` | El origen no tenía stock disponible suficiente. No se reservó nada |
| `FALLIDA` | Inventario no respondió a tiempo. Si llegó a reservar, la reserva se libera |
| `RESERVADA` | Las unidades están reservadas y esperamos que Rumbo revise el pedido |
| `ESPERANDO_CONFIRMACION` | Se había pedido una hora, Rumbo propuso otra y hay 24 horas para aceptarla o rechazarla |
| `PROGRAMADA` | Rumbo aceptó y hay una hora de llegada comprometida |
| `EN_CURSO` | El camión está viajando |
| `COMPLETADA` | El traslado llegó y el stock pasa al destino |
| `RECHAZADA` | Rumbo rechazó el traslado, un usuario rechazó el horario o nadie lo respondió a tiempo |
| `CANCELACION_PENDIENTE` | Un usuario pidió cancelar con menos de 24 horas de anticipación y Rumbo todavía no lo aprobó |
| `CANCELADA` | La transferencia se canceló |
| `NO_REALIZADA` | Rumbo no pudo hacer el traslado o lo interrumpió |

Son finales `SIN_STOCK`, `FALLIDA`, `COMPLETADA`, `RECHAZADA`, `CANCELADA` y `NO_REALIZADA`. En todos los finales salvo `COMPLETADA`, las unidades vuelven a estar disponibles en el origen.

## 6. Reglas de negocio

1. Cada usuario tiene un solo rol. El admin de sucursal y el vendedor tienen una sola sucursal.
2. El stock de un producto en una sucursal nunca puede ser negativo, y lo reservado nunca puede superar el total.
3. Todo cambio en el total de stock genera un movimiento.
4. Los ajustes manuales de stock los hace el director en cualquier sucursal o el admin de sucursal en la suya, y siempre con un motivo. Una salida no puede dejar el total por debajo de lo reservado.
5. En una transferencia, el origen y el destino tienen que ser sucursales distintas que existan, y la cantidad tiene que ser mayor a 0.
6. No se puede transferir más de lo que hay disponible en el origen.
7. Si se indica una hora de llegada, tiene que ser futura.
8. Al crear una transferencia, las unidades se reservan en el origen. El destino no recibe nada todavía.
9. Si Rumbo propone otro horario y no se había pedido hora, el sistema lo acepta solo. Si se había pedido una, hay 24 horas para aceptar o rechazar la propuesta. Si la rechaza o no responde, la transferencia queda `RECHAZADA`.
10. Cuando el traslado llega, en un solo paso se resta del origen y se suma al destino.
11. Si el traslado se rechaza, se cancela o no se realiza, la reserva se libera y el origen vuelve a tener esas unidades disponibles.
12. Una transferencia se puede cancelar mientras el traslado no esté en curso. Si faltan menos de 24 horas para la llegada, la cancelación depende de que Rumbo la apruebe.
13. Si no pudimos mandarle el pedido a Rumbo en 24 horas, la transferencia queda `RECHAZADA`.
14. Un producto tiene stock bajo cuando le quedan 5 unidades disponibles o menos en una sucursal.
15. El grupo consumidor solo puede usar las consultas del contrato, con su API key.
16. El vendedor y el admin de sucursal solo pueden crear, cancelar y responder los horarios de las transferencias que salen de su sucursal o llegan a ella. El director puede hacerlo con cualquiera.
17. El director da de alta usuarios de cualquier rol, sucursales y productos. El admin de sucursal solo da de alta vendedores de su sucursal.
18. La sucursal y el rol de un usuario van dentro de su token. Si cambian, se aplican cuando vuelve a hacer login.

## 7. Integraciones

- **Lo que ofrecemos:** consultas del catálogo y del stock disponible de Inventario. El contrato está en [docs/contracts/README.md](docs/contracts/README.md).
- **Lo que usamos:** Rumbo, para cada transferencia. Cómo lo usamos está en [docs/adr/ADR-009-consumo-de-rumbo.md](docs/adr/ADR-009-consumo-de-rumbo.md).

## 8. Fuera de alcance

- Gestión de proveedores y compras a proveedores. Del proveedor solo guardamos el nombre en el producto.
- Facturación, ventas y contabilidad.
- Gestión de la logística: los vehículos, los choferes y las rutas los maneja Rumbo. Nosotros pedimos el traslado, coordinamos el horario y seguimos su estado.
- Avisos automáticos de stock bajo. El stock bajo se consulta con el filtro de la consulta de stock de cada sucursal.
- Reposición automática y predicción de demanda.
- Reportes o estadísticas.

## 9. Criterios de aceptación

En los ejemplos, "un vendedor" es un vendedor de la sucursal de origen o de destino de la transferencia.

### Transferencias

**CA-01 — Transferencia válida.** Dado que el origen tiene 10 unidades disponibles de un producto y el destino tiene 2, cuando un vendedor transfiere 4, entonces la transferencia queda `RESERVADA`, el origen tiene 6 disponibles (10 en total y 4 reservadas) y el destino sigue con 2.

**CA-02 — Stock insuficiente.** Dado que el origen tiene 3 unidades disponibles, cuando se intenta transferir 5, entonces la transferencia queda `SIN_STOCK` y el stock de las dos sucursales no cambia.

**CA-03 — Misma sucursal.** Cuando se intenta transferir con la misma sucursal como origen y destino, entonces la transferencia se rechaza.

**CA-04 — Sucursal inexistente.** Cuando se intenta transferir desde o hacia una sucursal que no existe, entonces la transferencia se rechaza y el stock no cambia.

**CA-05 — Hora pasada.** Cuando se intenta crear una transferencia con una hora de llegada que ya pasó, entonces se rechaza y no se reserva nada.

**CA-06 — Horario propuesto sin hora pedida.** Dada la transferencia de CA-01, creada sin hora de llegada, cuando Rumbo propone un horario, entonces el sistema lo acepta sin preguntarle a nadie, la transferencia pasa a `PROGRAMADA` cuando Rumbo lo confirma, y las 4 unidades siguen reservadas.

**CA-07 — Horario propuesto con hora pedida.** Dada una transferencia como la de CA-01 pero con hora de llegada pedida, cuando Rumbo propone otro horario, entonces la transferencia queda `ESPERANDO_CONFIRMACION`. Si un vendedor acepta, pasa a `PROGRAMADA` cuando Rumbo lo confirma. Si pasan 24 horas sin respuesta, queda `RECHAZADA` y el origen vuelve a tener 10 disponibles.

**CA-08 — Traslado completado.** Dada la transferencia de CA-01 `EN_CURSO`, cuando Rumbo informa que el traslado llegó, entonces la transferencia queda `COMPLETADA`, el origen queda con 6 (sin reservas), el destino con 6, y se registra un movimiento en cada sucursal.

**CA-09 — Traslado rechazado.** Dada la transferencia de CA-01, cuando Rumbo rechaza el traslado, entonces la transferencia queda `RECHAZADA` con el motivo de Rumbo, el origen vuelve a tener 10 disponibles y el destino sigue con 2.

**CA-10 — Cancelación.** Dada la transferencia de CA-01 `PROGRAMADA` para dentro de 3 días, cuando un vendedor la cancela, entonces queda `CANCELADA` y el origen vuelve a tener 10 disponibles.

**CA-11 — Cancelación de un viaje en curso.** Dada una transferencia `EN_CURSO`, cuando un vendedor intenta cancelarla, entonces la cancelación se rechaza y la transferencia sigue `EN_CURSO`.

**CA-12 — Traslado no realizado.** Dada una transferencia `PROGRAMADA`, cuando Rumbo informa que el traslado no se pudo hacer, entonces la transferencia queda `NO_REALIZADA` y se libera la reserva.

**CA-13 — Reservas que se cruzan.** Dado que el origen tiene 5 unidades disponibles, cuando dos vendedores crean al mismo tiempo transferencias de 4 unidades cada una, entonces solo una queda `RESERVADA` y la otra queda `SIN_STOCK`.

**CA-14 — Inventario no responde.** Cuando Inventario no responde a tiempo al crear una transferencia, entonces la transferencia queda `FALLIDA` y, al final, el origen tiene el mismo stock disponible que antes de crearla.

**CA-15 — Aviso repetido.** Cuando Inventario recibe dos veces el mismo aviso de que un traslado se completó, entonces mueve el stock una sola vez.

### Stock

**CA-16 — Ajuste que dejaría stock negativo.** Dado que una sucursal tiene 2 unidades, cuando su admin registra una salida de 3, entonces el ajuste se rechaza y el stock sigue en 2.

**CA-17 — Ajuste sobre unidades reservadas.** Dado que una sucursal tiene 10 unidades en total y 4 reservadas, cuando su admin registra una salida de 8, entonces el ajuste se rechaza, porque solo hay 6 disponibles.

**CA-18 — Stock bajo.** Dado que una sucursal tiene 5 unidades disponibles de un producto y 6 de otro, cuando se consulta el stock de esa sucursal con el filtro de stock bajo, entonces aparece solo el primero.

### Usuarios y permisos

**CA-19 — Permisos del vendedor.** Un vendedor de la sucursal 3 puede consultar productos, stock, sucursales y usuarios de todas las sucursales, y crear, cancelar y responder horarios de transferencias que salen de la 3 o llegan a ella. No puede hacer ajustes de stock ni crear usuarios, sucursales o productos.

**CA-20 — Transferencia de otra sucursal.** Dado un vendedor de la sucursal 3, cuando intenta crear una transferencia de la sucursal 5 a la 7, o cancelar una que no pasa por la 3, entonces se rechaza y no cambia nada.

**CA-21 — Permisos del admin de sucursal.** Un admin de la sucursal 3 puede todo lo del vendedor, más hacer ajustes de stock en la 3 y dar de alta vendedores de la 3. Cuando intenta ajustar el stock de otra sucursal, crear un admin o crear una sucursal, entonces se rechaza.

**CA-22 — Permisos del director.** El director puede crear, cancelar y responder cualquier transferencia, hacer ajustes en cualquier sucursal y dar de alta usuarios de cualquier rol, sucursales y productos.

**CA-23 — Usuarios por sucursal.** Cuando cualquier usuario abre la sección de sucursales, entonces ve cada sucursal con su nombre y dirección, y sus usuarios con nombre, email y rol. Nunca ve contraseñas.

**CA-24 — Cambio de sucursal.** Dado un vendedor que pasa de la sucursal 3 a la 5, cuando vuelve a hacer login, entonces puede operar las transferencias de la 5 y ya no las de la 3.

**CA-25 — Acceso del grupo consumidor.** Con su API key, el grupo consumidor puede consultar productos y stock. Sin la API key, o con una incorrecta, recibe `401`. No puede ver usuarios, crear transferencias ni modificar nada.

### Búsqueda, caché y balanceo

**CA-26 — Búsqueda al día.** Dado un producto llamado "Harina 000", cuando se le cambia el nombre a "Harina 0000", entonces en menos de 5 segundos la búsqueda lo encuentra con el nombre nuevo.

**CA-27 — Búsqueda caída.** Cuando Solr no responde, entonces la búsqueda informa que no está disponible (no una lista vacía) y la consulta de stock de una sucursal sigue respondiendo.

**CA-28 — Caché con vencimiento.** Dado que los usuarios de una sucursal ya se consultaron y quedaron en la caché, cuando su admin da de alta un vendedor, entonces el vendedor nuevo aparece en la sección como mucho 60 segundos después. Lo mismo con los datos de la sucursal: si se cambia la dirección, se ve la nueva como mucho 60 segundos después.

**CA-29 — Caché caída.** Cuando Memcached no responde, entonces la sección de sucursales y la creación de transferencias siguen andando, con los datos de MySQL y sin errores.

**CA-30 — Una instancia de Inventario caída.** Cuando se apaga una de las dos instancias de Inventario, entonces las consultas de stock siguen respondiendo a través de la otra.

### Asistente

**CA-31 — Consulta por chat.** Cuando un vendedor le pregunta al asistente cuánto hay de un producto en una sucursal, entonces responde el mismo disponible que muestra la pantalla de stock.

**CA-32 — Transferencia con confirmación.** Cuando un vendedor le pide al asistente transferir 4 unidades de un producto entre dos sucursales, entonces el asistente muestra los datos para confirmar y no crea nada. Si el vendedor confirma, la transferencia queda creada como si la hubiera cargado a mano; si cancela, no se crea.

**CA-33 — Permisos del asistente.** Cuando un vendedor le pide al asistente algo que su rol no puede hacer, como un ajuste de stock o una transferencia que no pasa por su sucursal, entonces el asistente responde que no puede hacerlo y no cambia nada.

### Tiempo real

**CA-34 — Pantalla que se actualiza sola.** Dado que un vendedor tiene abierta una transferencia `RESERVADA`, cuando Rumbo la acepta, entonces la pantalla muestra `PROGRAMADA` sin que el vendedor recargue.

**CA-35 — Reconexión.** Cuando se corta la conexión de tiempo real y en ese rato cambia el stock de la sucursal que el usuario está mirando, entonces al reconectarse la pantalla muestra el stock actual.
