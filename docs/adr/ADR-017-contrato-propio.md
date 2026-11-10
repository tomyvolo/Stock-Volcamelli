# ADR-017 — Contrato propio (D8)

## Estado
Aceptado. Reemplaza a [ADR-008](ADR-008-contrato-de-integracion.md).

## Por qué revisamos ADR-008
En ADR-008 publicábamos toda la API de Inventario, incluidas las transferencias y los movimientos. Con [ADR-014](ADR-014-limites-de-servicios.md) las transferencias pasaron a su propio servicio, y además nos dimos cuenta de que no queremos que otro grupo pueda mover nuestro stock. Lo que el grupo consumidor necesita es traer el catálogo con el stock de cada sucursal y poder aumentar o reducir ese stock, así que achicamos el contrato a eso, sobre una base que es solo suya.

Como lo único que publicamos es Inventario, en la nube va solo Inventario con su base: no hace falta publicar el gateway ni Usuarios para que el otro grupo consulte o cambie stock.

## Decisión

### Qué se publica
Cuatro consultas de Inventario y dos cambios de stock:

| Operación | Ruta |
|---|---|
| Listar productos (paginado) | `GET /api/products` |
| Ver un producto | `GET /api/products/{productId}` |
| Ver el stock de una sucursal (con `?lowStock=true`, solo los productos con 5 unidades o menos) | `GET /api/branches/{branchId}/stock` |
| Ver el stock de un producto en una sucursal | `GET /api/branches/{branchId}/products/{productId}/stock` |
| Aumentar el stock de un producto en una sucursal | `POST /api/branches/{branchId}/products/{productId}/stock/increase` |
| Reducir el stock de un producto en una sucursal | `POST /api/branches/{branchId}/products/{productId}/stock/decrease` |

El stock que devolvemos (`quantity`) es el **disponible**: la cantidad total menos lo que está reservado para transferencias que todavía no llegaron a destino. Es lo que el otro grupo realmente puede contar como existente en esa sucursal.

El grupo consumidor llama directo a la URL pública de Inventario. Nuestro frontend usa las mismas cuatro consultas por el gateway, con el JWT de cada usuario, como cualquier otra ruta. El contrato está en [openapi.yaml](../contracts/openapi.yaml).

### La base del grupo consumidor
Inventario publicado usa su propia base en Aiven, separada de la de nuestro sistema en Docker Compose. Los aumentos y reducciones del otro grupo cambian el stock de esa base y no el nuestro, y nuestros ajustes y transferencias no cambian el suyo. Las dos bases arrancan con los mismos datos de prueba de `001_schema.sql`.

### Aumentar y reducir
Los dos pedidos llevan `quantity` (un entero mayor a 0) y `reason` (el motivo, de hasta 200 caracteres) y responden el stock disponible después del cambio, con el mismo formato que la consulta de stock de un producto.

- Por dentro hacen lo mismo que un ajuste manual de nuestro frontend: en una transacción bloquean la fila del stock con `SELECT ... FOR UPDATE`, la cambian y anotan el movimiento, con el motivo que mandó el otro grupo. Si dos pedidos llegan a la vez sobre el mismo producto y sucursal, el segundo espera al primero.
- El aumento crea la fila si el producto no tenía stock en esa sucursal.
- La reducción no puede dejar el disponible en negativo: si no alcanza, responde `409 INSUFFICIENT_STOCK` y no cambia nada.
- El producto tiene que existir. Como en la nube no está Sucursales, las sucursales válidas son una lista fija en Inventario publicado: 1, 2 y 3. Para otra, la respuesta es `404 BRANCH_NOT_FOUND`.

No piden `Idempotency-Key`. No la vimos en la materia y suma una tabla más para guardar cada clave con su resultado. A cambio, un pedido repetido se aplica dos veces, así que en el README del contrato le pedimos al otro grupo que, si no le llega la respuesta, consulte el stock antes de repetir.

### Autenticación
Cada pedido lleva una API key en el header `X-API-Key`. Inventario la compara con la variable de entorno `INVENTARIO_API_KEY` y, si falta o no coincide, responde `401 UNAUTHORIZED`. La comparación tarda lo mismo acierte o no, para no dar pistas sobre la clave.

- La clave la generamos nosotros, se la pasamos al grupo consumidor por un medio privado y no está en el repositorio.
- Si se filtra, cambiamos la variable de entorno y les pasamos la nueva.
- Dentro de Docker Compose la variable no se define: Inventario no tiene puerto abierto, solo le llegan pedidos del gateway y el gateway ya validó el JWT. Al arrancar sin clave, Inventario avisa por consola que solo se puede usar detrás del gateway.

### TrackID
Como en la nube el pedido no pasa por el gateway, Inventario crea el TrackID cuando no le llega uno y lo devuelve en `X-Track-ID`, igual que el gateway. Si el grupo consumidor nos reporta un problema con ese valor, encontramos el pedido en `app_logs`.

### Errores
Todos los errores devuelven el mismo formato, con un código y un mensaje:

| Código | Cuándo |
|---|---|
| `400` | Parámetros o cuerpo inválidos (por ejemplo, una página negativa o una cantidad en 0) |
| `401` | Falta la API key o no es válida |
| `404` | El producto no existe, o la sucursal no existe al aumentar o reducir |
| `409` | La reducción pide más que el stock disponible |
| `500` | Error nuestro |

No hay `503` propio: en el sistema completo lo devuelve el gateway, y en la nube no hay gateway. Si Inventario no responde, Render devuelve su propio `502` o `503`, sin nuestro formato.

### Dónde corre
| Pieza | Dónde | Por qué |
|---|---|---|
| Inventario | Un Web Service de Render, construido con el `Dockerfile` de `servicios/inventario` | Corre contenedores y da una URL pública con HTTPS. Las horas gratis del mes alcanzan para un servicio prendido todo el tiempo |
| MySQL | Un MySQL gratis de Aiven | Render no tiene MySQL, y el disco de su plan gratis se borra en cada reinicio. Aiven no pide tarjeta ni vence. Cargamos `servicios/inventario/mysql/001_schema.sql` y la tabla `app_logs` de `observabilidad/mysql/001_schema.sql` |
| Logs | La misma base de Aiven, en la tabla `app_logs` | `MYSQL_DSN` apunta a la misma base que `DB_DSN`. No hay un MySQL aparte para logs |
| Que no se duerma | Una consulta a `/api/products?size=1` con la API key cada 10 minutos desde cron-job.org | Render duerme los servicios gratis después de 15 minutos sin pedidos, y despertar tarda cerca de un minuto. Usamos una consulta de verdad y no `/health` porque así también se usa la base, y Aiven no la apaga por falta de uso |

Las variables de entorno del servicio en Render son `DB_DSN`, `MYSQL_DSN` e `INVENTARIO_API_KEY`. Cuando `INVENTARIO_API_KEY` está definida, Inventario solo registra las seis operaciones del contrato: el ajuste de nuestro frontend, el alta de productos y las reservas dependen de que el gateway controle el rol, y en la nube no hay gateway. La conexión a Aiven lleva `tls=skip-verify` en el DSN, porque Aiven exige conexiones cifradas.

### Compatibilidad
Para no romperle nada al otro grupo, solo hacemos cambios que no afecten lo que ya usa: podemos agregar operaciones nuevas o campos nuevos en las respuestas, pero no sacamos ni renombramos campos, ni cambiamos su tipo o su significado. Si alguna vez hiciera falta un cambio así, lo acordamos antes con el grupo consumidor.

### Versionado
La versión del contrato es la de `info.version` en `openapi.yaml`. Agregar una operación o un campo sube el número del medio, una corrección que solo toca la documentación sube el último, y un cambio que rompa lo que ya usa el otro grupo sube el primero, después de acordarlo con ellos. La versión vigente es la 2.1.0: la 2.0.0 cambió el login por la API key, y la 2.1.0 sumó aumentar y reducir stock. La lista de cambios está en el [README del contrato](../contracts/README.md).

No ponemos la versión en la URL (`/v1/...`): como solo hacemos cambios compatibles, salvo los acordados, el otro grupo siempre usa las mismas rutas.

## Alternativas que descartamos
- **Seguir publicando toda la API de Inventario, como en ADR-008.** Le daba al otro grupo la posibilidad de crear productos y mover stock en nuestro sistema, y no lo necesita.
- **Que el otro grupo cambie el stock de nuestra base.** Su stock y el nuestro se mezclarían, y una venta de ellos nos dejaría sin unidades para una transferencia.
- **Publicar el mismo ajuste que usa nuestro frontend, con tipo `ENTRADA` o `SALIDA`.** Ese ajuste controla el rol y la sucursal de quien lo pide con los headers que pone el gateway. Dos operaciones separadas, sin roles, son más fáciles de usar para el otro grupo.
- **Pedir `Idempotency-Key` en los cambios de stock.** Evita que un pedido repetido se aplique dos veces, pero no la vimos en la materia y suma una tabla. Con consultar el stock antes de repetir alcanza.
- **Login en Usuarios y JWT validado por el gateway.** Obliga a publicar también el gateway y Usuarios. En Render gratis no alcanzan las horas para tres servicios prendidos, y en una máquina virtual hay que mantener todo el sistema solo para que el otro grupo consulte stock.
- **Un JWT fijo de larga duración.** Evita el login, pero un token que no vence es igual de sensible que una API key, y además obliga a Inventario a validar firmas y roles.
- **No pedir autenticación.** Con una URL pública, cualquiera podría consultar nuestro catálogo y stock.
- **Devolver el stock total en vez del disponible.** El otro grupo vería unidades que ya están comprometidas en una transferencia.

## Consecuencias
- El otro grupo puede aumentar y reducir el stock de su base, pero no crear productos ni sucursales; si necesita algo más, hay que agregar operaciones al contrato.
- Su stock y el nuestro no tienen relación: los dos arrancan con los mismos datos de prueba y después cada uno cambia el suyo.
- Sin `Idempotency-Key`, un aumento o una reducción repetidos se aplican dos veces. Si no le llega la respuesta, el otro grupo tiene que consultar el stock antes de repetir.
- Si sumamos sucursales de prueba, hay que cambiar la lista fija de Inventario publicado y avisarle al otro grupo.
- El stock que devolvemos es el del momento de la consulta y puede cambiar enseguida.
- El grupo consumidor no hace login ni renueva tokens: manda siempre la misma clave. Es una sola clave para todo el grupo, así que no sabemos qué integrante hizo cada consulta.
- En la nube Inventario corre en una sola instancia y sin gateway, así que el balanceo con NGINX ([ADR-012](ADR-012-balanceo-de-carga.md)) y el control de permisos por rol quedan en el sistema completo, levantado con Docker Compose.
- Si la consulta de cron-job.org deja de llegar, Render duerme el servicio y la primera consulta tarda cerca de un minuto. Cada una de esas consultas deja una fila en `app_logs`.
- Aiven puede apagar un servicio gratis que no se usa por un tiempo, avisando antes por mail.
