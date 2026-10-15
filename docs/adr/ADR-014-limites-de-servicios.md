# ADR-014 — Límites de los servicios (D1)

## Estado
Aceptado. Reemplaza a [ADR-001](ADR-001-limites-de-servicios.md).

## Por qué revisamos ADR-001
Al principio habíamos separado el sistema en Usuarios, Sucursales, Inventario y Notificaciones. Cuando lo repasamos encontramos dos problemas.

El primero es que Inventario hacía demasiado: productos, stock, movimientos y transferencias. Inventario es el servicio que le publicamos al otro grupo, y las transferencias dependen de Rumbo, el servicio de traslados de otro grupo que nos asignaron. Con todo junto, si Rumbo se cae o cambia su contrato, el problema llega hasta lo que publicamos. Separar stock y transferencias ya lo habíamos pensado y lo habíamos descartado para no partir la transacción. Ahora volvemos a esa idea: preferimos coordinar dos servicios antes que tener el servicio publicado atado a Rumbo.

El segundo es que Notificaciones no aportaba nada al negocio. Lo habíamos puesto más que nada para tener una base no relacional y alguien que consuma eventos, y las dos cosas las cubrimos con las transferencias.

## Decisión
Quedan cuatro servicios detrás de un API Gateway:

| Servicio | Qué hace | Datos de los que es dueño |
|---|---|---|
| Usuarios | Usuarios, login y roles (`ADMIN`, `OPERATOR` e `INTEGRACION`) | Usuarios |
| Sucursales | Alta, modificación y consulta de sucursales | Sucursales, con su dirección |
| Inventario | Catálogo de productos, stock por sucursal, reservas, movimientos y ajustes manuales. Es la capacidad que publicamos | Productos, stock, reservas y movimientos |
| Transferencias | Registra las transferencias, le pide el traslado a Rumbo y decide si se confirman o se liberan | Transferencias y su estado |

El gateway es la entrada del frontend y del grupo que consume nuestro catálogo, y no guarda datos.

El criterio fue que cada dato tenga un solo dueño. Las cantidades de stock las cambia únicamente Inventario: Transferencias le pide que reserve, confirme o libere unidades, pero nunca las toca. Ningún servicio lee la base de otro.

Las relaciones entre servicios son:

- Transferencias consulta a Sucursales el nombre y la dirección de origen y destino, porque Rumbo los pide.
- Transferencias le pide a Inventario que reserve las unidades en el origen. Si no hay stock disponible suficiente, Inventario lo rechaza.
- Transferencias le pide el traslado a Rumbo y sigue si lo acepta o lo rechaza.
- Si Rumbo acepta, Transferencias publica `TrasladoAceptado` e Inventario mueve el stock del origen al destino. Si Rumbo rechaza, publica `TrasladoRechazado` e Inventario libera la reserva.
- Inventario consulta a Sucursales que la sucursal exista cuando un ADMIN hace un ajuste.

Cómo se comunican lo explicamos en [ADR-016](ADR-016-comunicacion-entre-servicios.md).

## Alternativas que descartamos
- **Dejar las transferencias dentro de Inventario, como en ADR-001.** Es lo que había y lo que queremos evitar: el servicio publicado dependería de Rumbo.
- **Tener un servicio de Notificaciones aparte.** Era un servicio más para levantar y mantener, y no cubría ninguna funcionalidad que alguien necesitara.
- **Mover el stock en el momento y devolverlo si Rumbo rechaza.** Fue lo primero que pensamos, porque la transferencia se veía hecha enseguida. El problema es que, mientras Rumbo decide, el destino puede vender esas unidades, y si después Rumbo rechaza no hay cómo devolverlas. Con la reserva eso no pasa: el destino recibe el stock recién cuando Rumbo acepta.

## Consecuencias
- Una transferencia pasa por dos servicios. La confirmación o la liberación llega por un evento, no por un rollback.
- Mientras Rumbo decide, las unidades reservadas no se pueden usar en el origen, y el destino todavía no las tiene.
- Ya no hay notificaciones de stock bajo. El stock bajo se ve como un filtro en la búsqueda del catálogo.
- Sucursales tiene que guardar la dirección de cada sucursal, porque sin eso no podemos pedirle traslados a Rumbo.
- Si Rumbo no está disponible, el catálogo y el stock que publicamos siguen funcionando. Las transferencias nuevas quedan reservadas esperando.
