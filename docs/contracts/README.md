# Contrato de Inventario

Le ofrecemos al otro grupo consultar catálogo y **stock disponible**. Las consultas no permiten crear ni modificar datos. [OpenAPI](openapi.yaml) describe las operaciones y [ADR-017](../adr/ADR-017-contrato-propio.md) explica las decisiones.

El campo de versión vigente sigue en **1.0.0**. Es un diseño pendiente de implementación; el mock devuelve ejemplos, no cantidades reales.

## Operaciones

| Método y ruta | Para qué |
|---|---|
| `POST /api/auth/login` | Obtener el token desde Usuarios |
| `GET /api/products?page=1&size=20` | Listar productos; página desde 1, tamaño 1–100 (default 20) |
| `GET /api/products/{productId}` | Ficha del producto, sin cantidades |
| `GET /api/branches/{branchId}/stock` | Stock disponible de los productos registrados en esa sucursal |
| `GET /api/branches/{branchId}/products/{productId}/stock` | Stock disponible de un producto en esa sucursal |

La búsqueda avanzada del frontend y las operaciones internas de reserva, confirmación y liberación no se agregan a esta capacidad pública.

## Qué significa el stock

`quantity = cantidad total − cantidad reservada`. Siempre es no negativa. Una reserva reduce el disponible en origen, sin modificar su total ni acreditar unidades en destino. Solo después de la aceptación de Rumbo, Inventario confirma la salida y la entrada de forma atómica. Un rechazo o vencimiento seguro libera la reserva; un timeout mantiene resultado desconocido hasta reconciliarlo.

Las pantallas muestran “stock disponible”. El filtro interno de stock bajo usa esa misma cantidad y selecciona disponible ≤ 5, incluido cero, en una sucursal. La búsqueda puede estar hasta 5 s atrasada con dependencias sanas; **las consultas de stock de este contrato leen MySQL**, sin caché ni Solr.

Se mantiene el comportamiento existente:

- Sin registros de stock en el id de sucursal, el listado devuelve una lista vacía.
- Si el producto existe pero no tiene registro en ese id de sucursal, la consulta puntual devuelve `quantity: 0`.
- Estas consultas no verifican ni certifican la existencia de la sucursal. Las operaciones internas la validan con Sucursales.
- Una consulta no reserva unidades y su resultado puede cambiar enseguida.
- La ficha de producto puede usar Memcached, con hasta 60 s de desfase de nombre/descripción/proveedor; no contiene stock. El listado básico del catálogo consulta MySQL.

## Autenticación y permisos

`ADMIN` y `OPERATOR` pueden leer estas consultas. Al grupo consumidor se le crea una cuenta `INTEGRACION`, que solo puede hacer login y usar las consultas publicadas; no modifica datos ni crea transferencias. Email y contraseña se entregan por privado.

1. Hacer login y guardar el `token`.
2. Enviar `Authorization: Bearer <token>` en cada consulta.
3. El token dura 1 hora (`expiresIn: 3600`). Cuando vence, volver a hacer login.

Gateway y servicio aplican permisos antes de consultar datos o caché. Los GET son idempotentes y no requieren `Idempotency-Key`; ese encabezado pertenece a operaciones de escritura internas, fuera de este contrato.

## Errores

Formato:

```json
{ "code": "PRODUCT_NOT_FOUND", "message": "No existe el producto 15." }
```

| HTTP | code | Qué hacer |
|---|---|---|
| 400 | `INVALID_REQUEST` | Corregir los parámetros |
| 401 | `UNAUTHORIZED` | Obtener un token válido |
| 401 | `INVALID_CREDENTIALS` | Revisar credenciales del login |
| 403 | `FORBIDDEN` | Verificar permisos; no insistir con la misma identidad |
| 404 | `PRODUCT_NOT_FOUND` | Revisar el id de producto |
| 429 | `RATE_LIMITED` | Respetar `Retry-After` cuando esté presente y espaciar pedidos |
| 500 | `INTERNAL_ERROR` | Error interno; reintentar las lecturas con límite |
| 503 | `SERVICE_UNAVAILABLE` | Dependencia necesaria no disponible; reintentar más tarde |

Nunca se devuelve stock cero o lista vacía para ocultar una falla de base. Propuesta para el consumidor: timeout de 3 s por lectura y hasta dos reintentos de GET ante 429/503 o fallas transitorias, con backoff y jitter, respetando `Retry-After`. Las validaciones y errores de permiso no se reintentan. Los límites definitivos se comunicarán con las pruebas de carga.

## Ejemplos

Comandos para Bash; en PowerShell puede usarse `curl.exe` en una sola línea con el escape de JSON correspondiente. Credenciales y token son ejemplos ficticios.

```bash
curl -X POST http://localhost:4010/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"integracion@grupo-consumidor.com","password":"una-clave-de-ejemplo"}'

curl "http://localhost:4010/api/products?page=1&size=20" \
  -H "Authorization: Bearer <token>"

curl http://localhost:4010/api/branches/3/products/15/stock \
  -H "Authorization: Bearer <token>"
```

Con total 46 y reservado 4, la última consulta devuelve:

```json
{ "branchId": 3, "productId": 15, "productName": "Harina 000", "quantity": 42 }
```

Ejemplo completo del flujo (cantidades internas para explicar el cálculo; los campos de respuesta siguen siendo los del contrato):

| Momento | Origen total/reservado | quantity origen | Destino total/reservado | quantity destino |
|---|---|---|---|---|
| Inicial | 10 / 0 | 10 | 2 / 0 | 2 |
| Reserva de 4 | 10 / 4 | 6 | 2 / 0 | 2 |
| Si Rumbo acepta e Inventario confirma | 6 / 0 | 6 | 6 / 0 | 6 |
| Si Rumbo rechaza e Inventario libera, como alternativa a confirmar | 10 / 0 | 10 | 2 / 0 | 2 |

## Probar con el mock

Con Docker iniciado, desde la raíz:

```bash
docker run --rm -p 4010:4010 -v "${PWD}/docs/contracts:/contrato" stoplight/prism:5 mock -h 0.0.0.0 /contrato/openapi.yaml
```

El mock queda en `http://localhost:4010` y devuelve los ejemplos del archivo. Se puede seleccionar una respuesta documentada con `Prefer: code=404`, por ejemplo sobre la consulta de un producto.

Prism permite verificar formas de pedidos/respuestas. No guarda stock, no ejecuta transferencias, no verifica credenciales reales ni demuestra los permisos JWT, expiración, límites de tráfico, TTL de caché o tiempos de sincronización. Sus errores de validación propios pueden tener un formato distinto al de la aplicación planificada. La ejecución del mock debe verificarse con Docker; describir el comando no demuestra que esté funcionando.

## Publicación y pruebas pendientes

La URL pública todavía no está disponible. Antes de compartirla se deben implementar gateway/login y consultas, entregar credenciales en privado, verificar las respuestas y mantener la capacidad operativa hasta terminar la evaluación. El consumidor debe incorporarla a un flujo de negocio relevante.

Verificar contra el contrato todos los ejemplos, límites de paginación, errores y permisos. Incluir el caso total 10/reservado 5 → `quantity: 5`. El mock no reemplaza esas pruebas contra la implementación real.

Rumbo es otra integración: su grupo nos entrega el contrato. No se agregan supuestos endpoints de Rumbo a este OpenAPI. Las dependencias por confirmar están en [arquitectura](../ARCHITECTURE.md#8-integración-con-otros-grupos).

## Cambios en el contrato

No manejamos versiones. Solo hacemos cambios que no rompan lo que ya usa el otro grupo, como agregar operaciones o campos nuevos en las respuestas. No sacamos ni renombramos campos, ni cambiamos su tipo o significado. Si alguna vez hiciera falta, lo acordamos antes con el grupo consumidor. Conviene que el consumidor ignore los campos que no conoce.

**Observaciones separadas:** se mantiene esta política y el campo `1.0.0`, pero queda pendiente resolver su relación con la exigencia de contrato versionado de la consigna. También debe coordinarse la interpretación de `quantity` como disponible si el consumidor esperaba total. Las nuevas respuestas documentadas 403/429/503 deben comunicarse y probarse con él antes de publicar.
