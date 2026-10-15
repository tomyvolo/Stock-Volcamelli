# Contrato de Inventario

Le ofrecemos al otro grupo consultar nuestro catálogo de productos y el stock disponible de cada sucursal. Son solo lecturas: no se puede crear ni modificar nada. El contrato completo está en [openapi.yaml](openapi.yaml) y las decisiones detrás están en [ADR-017](../adr/ADR-017-contrato-propio.md).

## Operaciones

| Método y ruta | Para qué |
|---|---|
| `POST /api/auth/login` | Obtener el token |
| `GET /api/products?page=1&size=20` | Listar productos, de a una página |
| `GET /api/products/{productId}` | Ver un producto |
| `GET /api/branches/{branchId}/stock` | Ver el stock disponible de todos los productos de una sucursal |
| `GET /api/branches/{branchId}/products/{productId}/stock` | Ver el stock disponible de un producto en una sucursal |

Algunas aclaraciones:
- El stock que devolvemos (`quantity`) es el **disponible**: el total menos las unidades reservadas para transferencias que todavía esperan la respuesta del transporte. Por ejemplo, si una sucursal tiene 46 unidades y 4 están reservadas, devolvemos 42.
- Si una sucursal no tiene stock cargado, la consulta de su stock devuelve una lista vacía.
- Si un producto existe pero no tiene stock en esa sucursal, la cantidad viene en 0.
- El stock es el del momento de la consulta y puede cambiar enseguida.

## Autenticación

Le creamos al grupo consumidor una cuenta con el rol `INTEGRACION`, que solo puede hacer estas consultas. El email y la contraseña los pasamos por privado, no están en el repositorio.

1. Hacer login con esa cuenta y guardar el `token` de la respuesta.
2. Mandarlo en todas las consultas en el header `Authorization: Bearer <token>`.
3. El token dura 1 hora (`expiresIn` viene en segundos). Cuando vence, las consultas devuelven `401` y hay que volver a hacer login.

## Errores

Todos los errores tienen el mismo formato:

```json
{ "code": "PRODUCT_NOT_FOUND", "message": "No existe el producto 15." }
```

| HTTP | `code` | Qué pasó | Qué hacer |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | Algún parámetro es inválido | Corregir el pedido |
| 401 | `UNAUTHORIZED` | Falta el token, no es válido o venció | Volver a hacer login |
| 401 | `INVALID_CREDENTIALS` | Email o contraseña incorrectos en el login | Revisar los datos de la cuenta |
| 403 | `FORBIDDEN` | La cuenta no tiene permiso para esa operación | Usar solo las operaciones de este contrato |
| 404 | `PRODUCT_NOT_FOUND` | El producto no existe | Verificar el id |
| 429 | `RATE_LIMITED` | Se pasaron del límite de pedidos por minuto | Esperar los segundos que indica el header `Retry-After` |
| 500 | `INTERNAL_ERROR` | Error de nuestro lado | Probar de nuevo más tarde |
| 503 | `SERVICE_UNAVAILABLE` | Inventario no está disponible en ese momento | Probar de nuevo más tarde |

## Ejemplos

Login:

```bash
curl -X POST http://localhost:4010/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "integracion@grupo-consumidor.com", "password": "una-clave-de-ejemplo"}'
```

Listar productos:

```bash
curl "http://localhost:4010/api/products?page=1&size=20" \
  -H "Authorization: Bearer <token>"
```

Stock de un producto en una sucursal:

```bash
curl http://localhost:4010/api/branches/3/products/15/stock \
  -H "Authorization: Bearer <token>"
```

Respuesta:

```json
{ "branchId": 3, "productId": 15, "productName": "Harina 000", "quantity": 42 }
```

## Probar con el mock

Mientras Inventario no esté implementado, se puede probar contra un mock que se arma solo a partir de `openapi.yaml`. Hace falta Docker. Desde la raíz del repositorio:

```bash
docker run --rm -p 4010:4010 -v "${PWD}/docs/contracts:/contrato" stoplight/prism:5 mock -h 0.0.0.0 /contrato/openapi.yaml
```

El mock queda en `http://localhost:4010`:
- Devuelve las respuestas de ejemplo del contrato. No tiene datos reales ni guarda nada.
- Valida los pedidos: si falta un parámetro obligatorio o tiene un valor inválido, responde `400`.
- Pide el header `Authorization`. Sin el header responde `401`, pero acepta cualquier token.

Para ver un error en particular se usa el header `Prefer`, por ejemplo `Prefer: code=404`.

## Cambios en el contrato

Solo hacemos cambios que no rompan lo que ya usa el otro grupo, como agregar operaciones o campos nuevos en las respuestas. No sacamos ni renombramos campos, ni cambiamos su tipo o su significado. Si alguna vez hiciera falta un cambio así, lo acordamos antes con el grupo consumidor. Conviene que el consumidor ignore los campos que no conoce, así un campo nuevo no le rompe nada.
