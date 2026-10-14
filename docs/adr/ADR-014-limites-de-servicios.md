# ADR-014 — Límites de los servicios (D1)

## Estado
Aceptado. Reemplaza a [ADR-001](ADR-001-limites-de-servicios.md).

## Por qué revisamos ADR-001
En la Entrega 1 habíamos separado el sistema en Usuarios, Sucursales, Inventario y Notificaciones. Cuando lo repasamos contra la consigna encontramos dos problemas.

El primero es que Inventario hacía demasiado: productos, stock, movimientos y transferencias. Inventario es el servicio que le publicamos al otro grupo, y las transferencias dependen de Rumbo, el servicio de traslados de otro grupo que nos asignaron. Con todo junto, si Rumbo se cae o cambia su contrato, el problema llega hasta lo que publicamos. Separar stock y transferencias ya lo habíamos pensado en la Entrega 1 y lo habíamos descartado para no partir la transacción. Ahora volvemos a esa idea: preferimos resolver la consistencia entre los dos servicios antes que tener el servicio publicado atado a Rumbo.

El segundo es que Notificaciones no aportaba nada al negocio. Lo habíamos puesto más que nada para tener una base no relacional y alguien que consuma eventos, y las dos cosas las cubrimos con las transferencias.

## Decisión
Quedan cuatro servicios detrás de un API Gateway:

| Servicio | Qué hace | Datos de los que es dueño |
|---|---|---|
| Usuarios | Usuarios, login y roles (ADMIN y OPERATOR) | Usuarios |
| Sucursales | Alta, modificación y consulta de sucursales | Sucursales, con su dirección |
| Inventario | Catálogo de productos, stock por sucursal, movimientos y ajustes manuales. Es la capacidad que publicamos | Productos, stock y movimientos |
| Transferencias | Registra las transferencias entre sucursales y le pide el traslado a Rumbo | Transferencias y su estado |

El gateway es la entrada del frontend y no guarda datos.

El criterio fue que cada dato tenga un solo dueño. Las cantidades de stock las cambia únicamente Inventario: Transferencias le pide que mueva unidades, pero nunca las toca. Ningún servicio lee la base de otro.

Las relaciones entre servicios son:

- Transferencias consulta a Sucursales el nombre y la dirección de origen y destino, porque Rumbo los pide.
- Transferencias le pide a Inventario que mueva el stock. Inventario lo hace en una sola transacción y, si no hay stock suficiente, lo rechaza.
- Transferencias le pide el traslado a Rumbo y sigue si lo acepta o lo rechaza.
- Si Rumbo rechaza el traslado, Transferencias publica el evento `TransferenciaRevertida` e Inventario devuelve el stock al origen.
- Inventario consulta a Sucursales que la sucursal exista cuando un ADMIN hace un ajuste.

Cómo se comunican (sincrónico, eventos, reintentos) lo explicamos en el ADR de D5.

## Alternativas que descartamos
- **Dejar las transferencias dentro de Inventario, como en ADR-001.** Es lo que había y lo que queremos evitar: el servicio publicado dependería de Rumbo.
- **Tener un servicio de Notificaciones aparte.** Era un servicio más para levantar y mantener, y no cubría ninguna funcionalidad que alguien necesitara.

## Consecuencias
- Una transferencia pasa por dos servicios. Si Rumbo la rechaza, hay que deshacer el movimiento con un evento en vez de un rollback.
- Ya no hay notificaciones de stock bajo. El stock bajo se ve como un filtro en la búsqueda del catálogo.
- Sucursales tiene que guardar la dirección de cada sucursal, porque sin eso no podemos pedirle traslados a Rumbo.
- Si Rumbo no está disponible, el catálogo y el stock que publicamos siguen funcionando igual.
