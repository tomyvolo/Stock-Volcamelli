# Contrato de Inventario

Le ofrecemos al otro grupo consultar nuestro catálogo de productos y el stock disponible de cada sucursal. Son solo lecturas: no se puede crear ni modificar nada. El contrato completo está en [openapi.yaml](openapi.yaml) y las decisiones detrás están en [ADR-017](../adr/ADR-017-contrato-propio.md).

**Versión vigente: 1.0.0**, la misma que figura en `info.version` de [openapi.yaml](openapi.yaml).

## Operaciones

| Método y ruta | Para qué |
|---|---|
| `POST /api/auth/login` | Obtener el token |
| `GET /api/products?page=1&size=20` | Listar productos, de a una página |
| `GET /api/products/{productId}` | Ver un producto |
| `GET /api/branches/{branchId}/stock` | Ver el stock disponible de todos los productos de una sucursal. Con `?lowStock=true`, solo los que tienen 5 unidades o menos |
| `GET /api/branches/{branchId}/products/{productId}/stock` | Ver el stock disponible de un producto en una sucursal |

Algunas aclaraciones:
- Las sucursales se identifican por su `branchId`. Con los datos de prueba hay tres: 1, 2 y 3. Si sumamos o sacamos sucursales, actualizamos esta lista y le avisamos al grupo consumidor, así no tiene que usar ningún otro servicio nuestro para conocerlas.
- El stock que devolvemos (`quantity`) es el **disponible**: el total menos las unidades reservadas para transferencias a otras sucursales que todavía no llegaron a destino. Por ejemplo, si una sucursal tiene 46 unidades y 4 están reservadas, devolvemos 42.
- Si una sucursal no tiene stock cargado, la consulta de su stock devuelve una lista vacía.
- Si un producto existe pero no tiene stock en esa sucursal, la cantidad viene en 0.
- El stock es el del momento de la consulta y puede cambiar enseguida.

## Autenticación

Le creamos al grupo consumidor una cuenta con el rol `INTEGRACION`, que solo puede hacer estas consultas. El email y la contraseña los pasamos por privado, no están en el repositorio.

Todo entra por nuestro gateway. El login lo resuelve el servicio de Usuarios y el token lo valida el gateway antes de pasarle la consulta a Inventario, así que Inventario no maneja usuarios ni tokens. Lo incluimos en este contrato porque el consumidor necesita el token para hacer las consultas.

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
| 500 | `INTERNAL_ERROR` | Error de nuestro lado | Probar de nuevo más tarde |
| 503 | `SERVICE_UNAVAILABLE` | Inventario no está disponible en ese momento | Probar de nuevo más tarde |

## Ejemplos

Login:

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "integracion@grupo-consumidor.com", "password": "una-clave-de-ejemplo"}'
```

Listar productos:

```bash
curl "http://localhost:8080/api/products?page=1&size=20" \
  -H "Authorization: Bearer <token>"
```

Stock de un producto en una sucursal:

```bash
curl http://localhost:8080/api/branches/1/products/1/stock \
  -H "Authorization: Bearer <token>"
```

Respuesta:

```json
{ "branchId": 1, "productId": 1, "productName": "Harina 000", "quantity": 42 }
```

## Dónde probarlo

Los ejemplos usan `http://localhost:8080`, que es el gateway cuando el sistema se levanta con `docker compose up`.

## Cambios en el contrato

Solo hacemos cambios que no rompan lo que ya usa el otro grupo, como agregar operaciones o campos nuevos en las respuestas. No sacamos ni renombramos campos, ni cambiamos su tipo o su significado. Si alguna vez hiciera falta un cambio así, lo acordamos antes con el grupo consumidor. Conviene que el consumidor ignore los campos que no conoce, así un campo nuevo no le rompe nada.

Cada cambio sube la versión de `info.version` y queda anotado en la tabla de abajo:

- si agregamos una operación o un campo, sube el número del medio (de 1.0.0 a 1.1.0);
- si solo corregimos la documentación, sin cambiar lo que responde la API, sube el último (de 1.0.0 a 1.0.1);
- si alguna vez tuviéramos que hacer un cambio que rompa lo que ya usa el otro grupo, primero lo acordamos con ellos y el contrato pasa a la 2.0.0.

| Versión | Qué cambió |
|---|---|
| 1.0.0 | Versión inicial: login, listado y detalle de productos, stock de una sucursal y stock de un producto en una sucursal |
