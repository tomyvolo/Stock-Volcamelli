# ADR-001 — Límites de los servicios (D1)

## Estado
Reemplazado por [ADR-014](ADR-014-limites-de-servicios.md). Lo dejamos como registro de lo que habíamos decidido antes.

## Contexto
El sistema maneja usuarios, sucursales, productos con su stock y avisos de stock bajo. Teníamos que repartir eso en servicios de forma que cada dato tenga un solo dueño.

## Decisión
Separamos el sistema en cuatro servicios detrás de un API Gateway:

| Servicio | Qué hace | Datos de los que es dueño |
|---|---|---|
| Usuarios | Usuarios y roles (ADMIN y OPERATOR) | Usuarios |
| Sucursales | Alta, modificación y consulta de sucursales | Sucursales |
| Inventario | Productos, stock, movimientos y transferencias entre sucursales | Productos, stock, movimientos y transferencias |
| Notificaciones | Avisos cuando un producto queda con poco stock o sin stock | Notificaciones |

Ningún servicio lee la base de otro. Inventario consulta a Sucursales para validar las sucursales y publica eventos de stock que consume Notificaciones.

Pusimos las transferencias dentro de Inventario para que restar en origen y sumar en destino quedara en una sola transacción.

## Alternativas que descartamos
- **Separar stock y transferencias en dos servicios.** Lo descartamos porque una transferencia resta en una sucursal y suma en otra, y con todo en Inventario eso quedaba en una sola transacción. Después volvimos a esta idea (ver ADR-014).
- **Un microservicio de métricas.** Las métricas no son parte del negocio: cada servicio puede exponer las suyas y no hace falta un servicio solo para eso.

## Consecuencias
- Las operaciones que pasan por más de un servicio pueden fallar a la mitad, y eso hay que manejarlo.
- Las notificaciones se generan después del movimiento de stock, no en el mismo momento.
