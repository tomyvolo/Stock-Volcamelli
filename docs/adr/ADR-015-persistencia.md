# ADR-015 — Persistencia (D3)

## Estado
Aceptado. Reemplaza a [ADR-003](ADR-003-persistencia-inicial.md).

## Por qué revisamos ADR-003
Cuando empezamos a pensar la implementación revisamos dos cosas. Para las bases relacionales pasamos de PostgreSQL a MySQL, porque las herramientas que usamos en el backend (Go con GORM y el driver de MySQL) nos dan más facilidad para conectarnos, definir los modelos y armar las migraciones. Además, con los servicios nuevos de [ADR-014](ADR-014-limites-de-servicios.md), decidimos usar MongoDB en Transferencias.

## Decisión
Cada servicio tiene su propio servidor de base de datos y es el único que lo usa.

| Servicio | Base | Qué guarda y cómo se consulta |
|---|---|---|
| Usuarios | MySQL | Usuarios con email único. Se buscan por id o por email al hacer login |
| Sucursales | MySQL | Sucursales con nombre y dirección. Se buscan por id |
| Inventario | MySQL | Productos, stock por sucursal y movimientos |
| Transferencias | MongoDB | Una transferencia por documento, con su historial de estados |

Cada MySQL corre en su propio contenedor. Así, si una base se cae o se llena, los otros servicios siguen funcionando.

### Inventario
Es donde más importa la consistencia. Una transferencia resta en origen y suma en destino en una sola transacción, y lee las dos filas con `SELECT ... FOR UPDATE` para que dos transferencias al mismo tiempo no dejen el stock mal. Además, la base hace cumplir dos reglas: la combinación producto y sucursal no se repite (`UNIQUE`) y el stock no puede ser negativo (`CHECK`, que MySQL respeta desde la 8.0.16).

En esta misma base van tres tablas que necesitamos para la comunicación con Transferencias ([ADR-016](ADR-016-comunicacion-entre-servicios.md)): la de transferencias hechas o anuladas, la de eventos a publicar (outbox) y la de eventos ya procesados, para no aplicar dos veces la misma reversión.

### Transferencias
Cada transferencia es un documento con lo que se pidió, su estado actual, la lista de estados por los que pasó, el id que le dio Rumbo y los eventos pendientes de publicar. Un cambio de estado y su evento se guardan en el mismo documento, y en MongoDB escribir un documento es atómico, así que no se puede guardar uno sin el otro.

Las consultas son tres: por id, por estado (los procesos de fondo buscan las que todavía no se mandaron a Rumbo o esperan respuesta) y por sucursal y fecha (las pantallas del operador). Para eso ponemos índices en `estado` y en `sucursal_origen` + `fecha`.

Elegimos MongoDB porque una transferencia siempre se lee y se escribe entera, y no se cruza con datos de otras tablas. En una base relacional la tendríamos repartida en tres tablas (transferencia, historial de estados y eventos pendientes) y habría que juntarlas en cada lectura. Además, lo que guardamos de las respuestas de Rumbo cambia según el caso: un rechazo trae un motivo, una contrapropuesta trae un horario. En un documento eso entra sin agregar columnas que quedan vacías la mayoría de las veces.

## Alternativas que descartamos
- **Seguir con PostgreSQL.** Funcionaba igual, pero con las herramientas que usamos en Go nos resulta más fácil trabajar con MySQL.

## Consecuencias
- Hay que levantar cuatro bases (tres MySQL y un MongoDB) y configurar cada una.
- Una transferencia queda guardada en Transferencias y el stock que mueve queda en Inventario, en bases distintas. Por eso no podemos deshacer las dos cosas juntas con un rollback: si Rumbo rechaza el traslado, Transferencias avisa con un evento e Inventario devuelve el stock por su cuenta.
