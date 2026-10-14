# ADR-003 — Persistencia (D3)

## Estado
Reemplazado por [ADR-015](ADR-015-persistencia.md). Lo dejamos como registro de lo que decidimos en la Entrega 1.

## Contexto
Usuarios, sucursales e inventario son datos con relaciones y reglas (un email no se repite, el stock no puede ser negativo). Las notificaciones son registros sueltos que se escriben una vez y casi no se modifican.

## Decisión
Cada servicio tiene su propia base y es el único que la lee y la escribe.

| Servicio | Base | Por qué |
|---|---|---|
| Usuarios | PostgreSQL | Pocos datos, email único, consultas por id |
| Sucursales | PostgreSQL | Pocos datos, consultas por id |
| Inventario | PostgreSQL | Una transferencia resta en una sucursal y suma en otra; necesitamos transacciones y bloqueo de filas |
| Notificaciones | MongoDB | Cada notificación es un documento suelto que se consulta por sucursal y fecha |

En local, los tres servicios relacionales comparten un servidor PostgreSQL, pero cada uno tiene su base y su usuario.

## Alternativas que descartamos
- **Una sola base para todos los servicios.** Cualquier servicio podría leer las tablas de otro.
- **MongoDB para Inventario.** La transferencia tiene que tocar dos registros de stock juntos, y eso es más directo en una base relacional.

## Consecuencias
- Hay que levantar y configurar dos motores de base distintos.
- Lo que cruza servicios no tiene transacción; si algún caso lo necesita, se compensa.
