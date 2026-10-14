# ADR-014 — Límites de los servicios (D1)

## Estado
Aceptado. Reemplaza a [ADR-001](ADR-001-limites-de-servicios.md). Esta revisión completa el diseño con reservas; la implementación está pendiente.

## Por qué revisamos ADR-001
En la Entrega 1 habíamos separado el sistema en Usuarios, Sucursales, Inventario y Notificaciones. Cuando lo repasamos contra la consigna encontramos dos problemas.

El primero es que Inventario hacía demasiado: productos, stock, movimientos y transferencias. Inventario es el servicio que le publicamos al otro grupo, y las transferencias dependen de Rumbo, el servicio de traslados de otro grupo que nos asignaron. Con todo junto, si Rumbo se cae o cambia su contrato, el problema llega hasta lo que publicamos. Separar stock y transferencias ya lo habíamos pensado y lo habíamos descartado para no partir la transacción. Ahora separamos el proceso, pero mantenemos todas las modificaciones de cantidades en Inventario.

El segundo es que el servicio anterior de avisos no aportaba una funcionalidad necesaria. El stock bajo se consulta en el catálogo; Transferencias cubre una necesidad real y justifica su almacenamiento documental y consumo de eventos.

## Decisión
Quedan cuatro servicios detrás de un API Gateway:

| Servicio | Qué hace | Datos de los que es dueño |
|---|---|---|
| Usuarios | Usuarios, login y roles `ADMIN`, `OPERATOR` e `INTEGRACION` | Usuarios y credenciales |
| Sucursales | Alta, modificación y consulta de sucursales | Sucursales, con su dirección |
| Inventario | Catálogo, stock disponible, reservas, ajustes y movimientos. Publica las consultas | Productos, existencias totales y reservadas, reservas y movimientos |
| Transferencias | Administra la transferencia y consume Rumbo | Proceso, estados, historial, decisión e id del traslado en Rumbo |

El gateway es la entrada del frontend y del grupo consumidor; no guarda datos ni coordina el negocio. `ADMIN` administra y opera; `OPERATOR` consulta y crea transferencias; `INTEGRACION` solo consulta catálogo y stock. Los servicios también validan permisos.

Cada dato tiene un solo dueño. Solo Inventario modifica cantidades. Sus claves de idempotencia y estados de reserva protegen operaciones locales; no son otra copia del proceso de Transferencias. Nadie lee la base de otro.

Las relaciones entre servicios son:

- Transferencias consulta nombres y direcciones a Sucursales; el formato externo lo determinará el contrato que entregue el grupo de Rumbo.
- Transferencias pide a Inventario reservar y consulta el resultado por id. Inventario reserva solo si alcanza `total − reservado`; el destino no cambia.
- Transferencias pide el traslado a Rumbo y registra su resultado. No se asumen operaciones externas aún no documentadas.
- Transferencias publica aceptación, rechazo o vencimiento seguro. Inventario confirma o libera una única vez y publica el resultado para que Transferencias cierre su estado.
- Inventario valida sucursales al hacer ajustes y publica cambios para sus proyecciones de búsqueda e invalidación de caché.

Las llamadas internas salen del servicio responsable. Las dirigidas a Inventario pasan por su NGINX interno, sin volver al gateway. Cómo se comunican y reconcilian se explica en [ADR-016](ADR-016-comunicacion-entre-servicios.md).

## Alternativas que descartamos
- **Dejar el proceso de transferencias dentro de Inventario.** El servicio publicado dependería de Rumbo.
- **Mantener un servicio aparte de avisos.** Agregaba operación sin una necesidad dentro del alcance.
- **Acreditar destino antes de la aceptación.** Permitiría consumir esas unidades y luego impediría deshacer el movimiento ante un rechazo. Reservar evita ese problema.

## Consecuencias
- La confirmación de salida y entrada sigue siendo una transacción local de Inventario. El estado del proceso se completa después, mediante eventos o reconciliación.
- El disponible baja al reservar; el total se mueve solo al confirmar. Rechazar libera origen sin tocar destino.
- El stock bajo se ve en la búsqueda por sucursal con disponible de 5 o menos.
- La aceptación de Rumbo habilita el cierre contable; el seguimiento físico del viaje queda fuera de alcance.
- Si Rumbo no está disponible, catálogo y stock siguen consultables; las transferencias pueden quedar pendientes y retener reservas.
