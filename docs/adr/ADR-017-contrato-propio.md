# ADR-017 — Contrato propio (D8)

## Estado
Aceptado. Reemplaza a [ADR-008](ADR-008-contrato-de-integracion.md).

## Por qué revisamos ADR-008
En ADR-008 publicábamos toda la API de Inventario, incluidas las transferencias y los movimientos. Con [ADR-014](ADR-014-limites-de-servicios.md) las transferencias pasaron a su propio servicio, y además nos dimos cuenta de que no queremos que otro grupo pueda mover nuestro stock. Lo que el grupo consumidor necesita es traer el catálogo con el stock de cada sucursal, así que achicamos el contrato a eso.

Como lo único que publicamos es Inventario, en la nube va solo Inventario con su base: no hace falta publicar el gateway ni Usuarios para que el otro grupo consulte stock.

## Decisión

### Qué se publica
Solamente consultas de Inventario:

| Operación | Ruta |
|---|---|
| Listar productos (paginado) | `GET /api/products` |
| Ver un producto | `GET /api/products/{productId}` |
| Ver el stock de una sucursal (con `?lowStock=true`, solo los productos con 5 unidades o menos) | `GET /api/branches/{branchId}/stock` |
| Ver el stock de un producto en una sucursal | `GET /api/branches/{branchId}/products/{productId}/stock` |

El stock que devolvemos (`quantity`) es el **disponible**: la cantidad total menos lo que está reservado para transferencias que todavía no llegaron a destino. Es lo que el otro grupo realmente puede contar como existente en esa sucursal.

El grupo consumidor llama directo a la URL pública de Inventario. Nuestro frontend usa las mismas consultas por el gateway, con el JWT de cada usuario, como cualquier otra ruta. El contrato está en [openapi.yaml](../contracts/openapi.yaml).

### Autenticación
Cada consulta lleva una API key en el header `X-API-Key`. Inventario la compara con la variable de entorno `INVENTARIO_API_KEY` y, si falta o no coincide, responde `401 UNAUTHORIZED`. La comparación tarda lo mismo acierte o no, para no dar pistas sobre la clave.

- La clave la generamos nosotros, se la pasamos al grupo consumidor por un medio privado y no está en el repositorio.
- Si se filtra, cambiamos la variable de entorno y les pasamos la nueva.
- Dentro de Docker Compose la variable no se define: Inventario no tiene puerto abierto, solo le llegan pedidos del gateway y el gateway ya validó el JWT. Al arrancar sin clave, Inventario avisa por consola que solo se puede usar detrás del gateway.

### TrackID
Como en la nube el pedido no pasa por el gateway, Inventario crea el TrackID cuando no le llega uno y lo devuelve en `X-Track-ID`, igual que el gateway. Si el grupo consumidor nos reporta un problema con ese valor, encontramos el pedido en `app_logs`.

### Errores
Todos los errores devuelven el mismo formato, con un código y un mensaje:

| Código | Cuándo |
|---|---|
| `400` | Parámetros inválidos (por ejemplo, una página negativa) |
| `401` | Falta la API key o no es válida |
| `404` | El producto no existe |
| `500` | Error nuestro |

No hay `503` propio: en el sistema completo lo devuelve el gateway, y en la nube no hay gateway. Si Inventario no responde, Render devuelve su propio `502` o `503`, sin nuestro formato.

### Dónde corre
| Pieza | Dónde | Por qué |
|---|---|---|
| Inventario | Un Web Service de Render, construido con el `Dockerfile` de `servicios/inventario` | Corre contenedores y da una URL pública con HTTPS. Las horas gratis del mes alcanzan para un servicio prendido todo el tiempo |
| MySQL | Un MySQL gratis de Aiven | Render no tiene MySQL, y el disco de su plan gratis se borra en cada reinicio. Aiven no pide tarjeta ni vence. Cargamos `servicios/inventario/mysql/001_schema.sql` y la tabla `app_logs` de `observabilidad/mysql/001_schema.sql` |
| Logs | La misma base de Aiven, en la tabla `app_logs` | `MYSQL_DSN` apunta a la misma base que `DB_DSN`. No hay un MySQL aparte para logs |
| Que no se duerma | Una consulta a `/api/products?size=1` con la API key cada 10 minutos desde cron-job.org | Render duerme los servicios gratis después de 15 minutos sin pedidos, y despertar tarda cerca de un minuto. Usamos una consulta de verdad y no `/health` porque así también se usa la base, y Aiven no la apaga por falta de uso |

Las variables de entorno del servicio en Render son `DB_DSN`, `MYSQL_DSN` e `INVENTARIO_API_KEY`. La conexión a Aiven lleva `tls=skip-verify` en el DSN, porque Aiven exige conexiones cifradas.

### Compatibilidad
Para no romperle nada al otro grupo, solo hacemos cambios que no afecten lo que ya usa: podemos agregar operaciones nuevas o campos nuevos en las respuestas, pero no sacamos ni renombramos campos, ni cambiamos su tipo o su significado. Si alguna vez hiciera falta un cambio así, lo acordamos antes con el grupo consumidor.

### Versionado
La versión del contrato es la de `info.version` en `openapi.yaml`. Agregar una operación o un campo sube el número del medio, una corrección que solo toca la documentación sube el último, y un cambio que rompa lo que ya usa el otro grupo sube el primero, después de acordarlo con ellos. La versión vigente es la 2.0.0: el paso del login a la API key cambió la forma de autenticarse. La lista de cambios está en el [README del contrato](../contracts/README.md).

No ponemos la versión en la URL (`/v1/...`): como solo hacemos cambios compatibles, salvo los acordados, el otro grupo siempre usa las mismas rutas.

## Alternativas que descartamos
- **Seguir publicando toda la API de Inventario, como en ADR-008.** Le daba al otro grupo la posibilidad de crear productos y mover stock en nuestro sistema, y no lo necesita.
- **Login en Usuarios y JWT validado por el gateway.** Obliga a publicar también el gateway y Usuarios. En Render gratis no alcanzan las horas para tres servicios prendidos, y en una máquina virtual hay que mantener todo el sistema solo para que el otro grupo consulte stock.
- **Un JWT fijo de larga duración.** Evita el login, pero un token que no vence es igual de sensible que una API key, y además obliga a Inventario a validar firmas y roles.
- **No pedir autenticación.** Con una URL pública, cualquiera podría consultar nuestro catálogo y stock.
- **Devolver el stock total en vez del disponible.** El otro grupo vería unidades que ya están comprometidas en una transferencia.

## Consecuencias
- El otro grupo no puede modificar nada; si necesita algo más, hay que agregar operaciones al contrato.
- Como son solo lecturas, no hace falta `Idempotency-Key`.
- El stock que devolvemos es el del momento de la consulta y puede cambiar enseguida.
- El grupo consumidor no hace login ni renueva tokens: manda siempre la misma clave. Es una sola clave para todo el grupo, así que no sabemos qué integrante hizo cada consulta.
- En la nube Inventario corre en una sola instancia y sin gateway, así que el balanceo con NGINX ([ADR-012](ADR-012-balanceo-de-carga.md)) y el control de permisos por rol quedan en el sistema completo, levantado con Docker Compose.
- Si la consulta de cron-job.org deja de llegar, Render duerme el servicio y la primera consulta tarda cerca de un minuto. Cada una de esas consultas deja una fila en `app_logs`.
- Aiven puede apagar un servicio gratis que no se usa por un tiempo, avisando antes por mail.
