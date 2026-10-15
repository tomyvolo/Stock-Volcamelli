# ADR-017 — Contrato propio (D8)

## Estado
Aceptado. Reemplaza a [ADR-008](ADR-008-contrato-de-integracion.md).

## Por qué revisamos ADR-008
En ADR-008 publicábamos toda la API de Inventario, incluidas las transferencias y los movimientos. Con [ADR-014](ADR-014-limites-de-servicios.md) las transferencias pasaron a su propio servicio, y además nos dimos cuenta de que no queremos que otro grupo pueda mover nuestro stock. Lo que el grupo consumidor necesita es traer el catálogo con el stock de cada sucursal, así que achicamos el contrato a eso.

## Decisión
Publicamos solamente consultas de Inventario:

| Operación | Ruta |
|---|---|
| Listar productos (paginado) | `GET /api/products` |
| Ver un producto | `GET /api/products/{productId}` |
| Ver el stock de una sucursal | `GET /api/branches/{branchId}/stock` |
| Ver el stock de un producto en una sucursal | `GET /api/branches/{branchId}/products/{productId}/stock` |

Además, el contrato incluye el login (`POST /api/auth/login`), que es de Usuarios y hace falta para obtener el token.

El stock que devolvemos (`quantity`) es el **disponible**: la cantidad total menos lo que está reservado para transferencias que esperan la respuesta de Rumbo. Es lo que el otro grupo realmente puede contar como existente en esa sucursal.

El contrato está en [openapi.yaml](../contracts/openapi.yaml). Mientras Inventario no esté implementado, el otro grupo puede probar contra un mock que se levanta con Prism desde ese mismo archivo. Cuando Inventario esté desplegado en la nube, va a tener una URL pública.

### Autenticación
Usamos el mismo JWT que usan nuestros usuarios. Al grupo consumidor le creamos una cuenta en Usuarios con el rol `INTEGRACION`, que solo puede hacer estas consultas. Con esa cuenta hace login, recibe un token que dura 1 hora y lo manda en `Authorization: Bearer <token>` en cada pedido. El gateway valida el token igual que con cualquier usuario. Cuando vence, vuelven a hacer login.

### Errores
Todos los errores devuelven el mismo formato, con un código y un mensaje:

| Código | Cuándo |
|---|---|
| `400` | Parámetros inválidos (por ejemplo, una página negativa) |
| `401` | Falta el token, no es válido o venció; o el usuario y la contraseña del login son incorrectos |
| `403` | El token es válido, pero el rol no tiene permiso para esa operación |
| `404` | El producto no existe |
| `429` | Se pasaron del límite de pedidos por minuto que pone el gateway |
| `500` | Error nuestro |
| `503` | Inventario no está disponible en ese momento |

### Compatibilidad
Para no romperle nada al otro grupo, solo hacemos cambios que no afecten lo que ya usa: podemos agregar operaciones nuevas o campos nuevos en las respuestas, pero no sacamos ni renombramos campos, ni cambiamos su tipo o su significado. Si alguna vez hiciera falta un cambio así, lo acordamos antes con el grupo consumidor.

## Alternativas que descartamos
- **Seguir publicando toda la API de Inventario, como en ADR-008.** Le daba al otro grupo la posibilidad de crear productos y mover stock en nuestro sistema, y no lo necesita.
- **No pedir autenticación.** Cuando Inventario esté en la nube con una URL pública, cualquiera podría consultar nuestro catálogo y stock.
- **Una API key fija para el otro grupo.** Es más simple para ellos, pero nos dejaba con dos formas de autenticación distintas, cuando el gateway ya valida JWT.
- **Devolver el stock total en vez del disponible.** El otro grupo vería unidades que ya están comprometidas en una transferencia.

## Consecuencias
- El otro grupo no puede modificar nada; si necesita algo más, hay que agregar operaciones al contrato.
- Como son solo lecturas, no hace falta `Idempotency-Key`.
- El stock que devolvemos es el del momento de la consulta y puede cambiar enseguida.
- El otro grupo tiene que hacer login y volver a pedir el token cada vez que vence.
- Usuarios suma el rol `INTEGRACION`, y el usuario y la contraseña de esa cuenta se los pasamos por un medio privado, no en el repositorio.
