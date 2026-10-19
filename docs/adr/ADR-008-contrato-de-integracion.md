# ADR-008 — Contrato propio (D8)

## Estado
Reemplazado por [ADR-017](ADR-017-contrato-propio.md). Lo dejamos como registro de lo que habíamos decidido antes.

## Contexto
Otro grupo va a usar una capacidad de nuestro sistema, y elegimos ofrecerle Inventario.

## Decisión
Publicamos toda la API de Inventario, descripta en [openapi.yaml](../contracts/openapi.yaml).

| Área | Operaciones |
|---|---|
| Productos | Listar, ver uno, crear y modificar nombre o descripción |
| Stock | Ver el stock de una sucursal o de un producto en una sucursal |
| Movimientos | Registrar entradas y salidas, y ver los movimientos de un producto en una sucursal |
| Transferencias | Transferir stock entre sucursales y ver el resultado |

Los POST (crear producto, registrar movimiento, transferir) llevaban el header `Idempotency-Key` para que un reintento no repitiera la operación. El contrato no tenía autenticación: la íbamos a acordar con el otro grupo antes de la implementación real.

## Alternativas que descartamos
- **Publicar solo la consulta de stock.** Nos parecía que dejaba afuera lo principal de Inventario.
- **Publicar también la gestión de sucursales.** Las sucursales son de otro servicio.

## Consecuencias
- El otro grupo podía crear productos y mover stock en nuestro sistema.
- Las transferencias tenían que ser atómicas dentro de Inventario.
- No se podían borrar productos, porque no habíamos definido qué pasaba con sus movimientos.
