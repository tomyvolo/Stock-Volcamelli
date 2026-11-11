# Validación del sistema

Pruebas sobre el sistema levantado con Docker Compose. Respaldan lo que dicen los ADR: la validación de D1, D3 y D5 ([ADR-014](adr/ADR-014-limites-de-servicios.md), [ADR-015](adr/ADR-015-persistencia.md) y [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md)), el balanceo ([ADR-012](adr/ADR-012-balanceo-de-carga.md)) y la capacidad ([ADR-013](adr/ADR-013-capacidad-y-costos.md)).

Las secciones 1 a 7 son de la primera validación. De la 8 en adelante están las pruebas de lo que sumamos después: los tests, la caída del MySQL de logs, tiempo real, el asistente, Rumbo real, el frontend, la concurrencia, los eventos, las caídas y la búsqueda, y la medición de la caché con el control de tráfico. Lo que todavía no corrimos está en la sección 16, con lo que tiene que dar cada prueba.

## 1. Login y alta de usuarios

Con un token de director creamos un usuario `INTEGRACION` y después hicimos login con ese usuario, como lo haría el grupo consumidor.

| Prueba | Pedido | Resultado |
|---|---|---|
| Login con contraseña incorrecta | `POST /api/auth/login` | `401` `INVALID_CREDENTIALS` |
| El director crea un usuario `INTEGRACION` | `POST /api/create-users` | `201`, devuelve el usuario sin la contraseña |
| El mismo email otra vez | `POST /api/create-users` | `409` `EMAIL_TAKEN` |
| Un vendedor intenta crear un usuario | `POST /api/create-users` | `403` `FORBIDDEN` (lo frena el gateway) |
| Un admin de sucursal intenta crear otro admin | `POST /api/create-users` | `403` `FORBIDDEN` (lo frena Usuarios) |
| Un admin de sucursal crea un vendedor de su sucursal | `POST /api/create-users` | `201` |
| Login con el usuario `INTEGRACION` creado | `POST /api/auth/login` | `200`, con `token` y `expiresIn: 3600` |
| La cuenta `INTEGRACION` intenta crear un usuario | `POST /api/create-users` | `403` `FORBIDDEN` |

## 2. Las cuatro consultas del contrato

Con el token de la cuenta `INTEGRACION`:

| Consulta | Resultado |
|---|---|
| `GET /api/products?page=1&size=3` | `200`, los productos 1, 2 y 3 con `page`, `size` y `total` |
| `GET /api/products/1` | `200`, `Harina 000` de `Molinos del Sur` |
| `GET /api/branches/1/stock` | `200`, el stock disponible de cada producto de la sucursal 1, ordenado por nombre |
| `GET /api/branches/1/stock?lowStock=true` | `200`, solo `Leche entera` con 0 unidades, el único de la sucursal 1 con 5 o menos |
| `GET /api/branches/1/products/1/stock` | `200`, `quantity: 42`: la base tiene 46 en total y 4 reservadas, y la consulta devuelve el disponible |

Los nombres con tilde llegan bien (`Azúcar`, `Fideos tirabuzón`). Un usuario creado por la API con el nombre `José Pérez` quedó guardado en UTF-8 (`4A6F73C3A92050C3A972657A`).

## 3. Errores del contrato

| Prueba | Resultado |
|---|---|
| Producto que no existe (`GET /api/products/99`) | `404` `PRODUCT_NOT_FOUND`: "No existe el producto 99." |
| Sin token | `401` `UNAUTHORIZED`: "token no provisto" |
| Token vencido | `401` `UNAUTHORIZED`: "token inválido o expirado" |
| Página inválida (`?page=abc`) | `400` `INVALID_REQUEST` |

Todos responden con el formato `{code, message}` del contrato.

## 4. Traza de un pedido

Pedido `GET /api/branches/1/products/1/stock`, que devolvió `X-Track-ID: b3286fb0051f3505bcd12c387ad3c4c5`.

En la tabla `app_logs`, que es lo que muestra el panel "Logs del recorrido" de Grafana con ese TrackID:

| created_at | stage | level | message |
|---|---|---|---|
| 18:22:15.293 | gateway | info | pedido recibido GET /api/branches/1/products/1/stock |
| 18:22:15.300 | inventario | info | stock del producto 1 en la sucursal 1: 42 disponibles (0 ms) |
| 18:22:15.308 | gateway | info | respuesta 200 (15 ms) |

En el log de NGINX (`docker compose logs nginx`), con el mismo TrackID y la instancia que atendió el pedido:

```text
track_id=b3286fb0051f3505bcd12c387ad3c4c5 instancia=172.19.0.6:8082 status=200 tiempo=0.008 "GET /api/branches/1/products/1/stock HTTP/1.1"
```

NGINX registra la IP de la instancia, no su nombre. Para saber cuál es cada una: `docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' <contenedor>`. En esta corrida, `172.19.0.6` era `inventario1`.

## 5. Prometheus y Grafana

| Qué | Resultado |
|---|---|
| Prometheus lee las métricas de todos los servicios (`up`) | gateway, usuarios, inventario1 e inventario2 en `1` |
| Fuente "Prometheus" en Grafana | "Successfully queried the Prometheus API" |
| Fuente "MySQL Logs" en Grafana | "Database Connection OK" |
| Tablero | "Stock Volcamelli", cargado solo al arrancar |

## 6. Balanceo y caída de instancias

Todas las consultas son `GET /api/branches/1/stock` con el token de `INTEGRACION`.

| Paso | Qué hicimos | Resultado |
|---|---|---|
| A | Las dos instancias arriba, 10 consultas | 10 respuestas `200`. NGINX mandó 5 a `inventario1` y 5 a `inventario2`. Prometheus: las dos en `up = 1` |
| B | `docker compose stop inventario1`, 10 consultas | 10 respuestas `200`. 7 fueron directo a `inventario2`; las otras 3 NGINX las intentó primero en `inventario1`, no pudo conectar y las reintentó en `inventario2`. Prometheus: `inventario1` en `0`, `inventario2` en `1` |
| C | `docker compose stop inventario2` (las dos caídas), 3 consultas | 3 respuestas `503` `SERVICE_UNAVAILABLE`: "El servicio no está disponible. Probá de nuevo en unos minutos." Nunca stock cero ni lista vacía |
| C | Login con las dos instancias caídas | `200`: Usuarios no depende de Inventario |
| D | `docker compose start inventario1 inventario2` | A los 31 segundos NGINX vuelve a repartir entre las dos (`fail_timeout` de 30 segundos). 10 consultas `200`. Prometheus: las dos en `1` |

## 7. Carga

Los resultados completos están en [ADR-013](adr/ADR-013-capacidad-y-costos.md). En resumen:

- Con hasta 50 workers (unas 520 a 590 consultas por segundo) el p95 queda debajo de 130 ms, sin errores.
- Con 100 workers algunas corridas tienen entre 1,5 % y 1,8 % de pedidos que pasan los 2 segundos.
- El límite es la escritura de logs: el MySQL de logs llega a su máximo de 151 conexiones y responde `Too many connections`.

## 8. Tests unitarios

Corrimos `go test ./...` en cada módulo el 8/11/2026:

| Módulo | Tests | Qué cubren, entre otras cosas |
|---|---|---|
| gateway | 3 | El proxy de tiempo real y que un MySQL de logs colgado no frene los pedidos |
| usuarios | 18 | Alta y permisos por rol, login y la caché |
| sucursales | 8 | Alta, consultas y la caché |
| inventario | 73 | Reservas, confirmación y liberación, eventos repetidos, ajustes, stock bajo, la búsqueda y el contrato publicado |
| transferencias | 36 | Elegir la franja de Rumbo, reservar sin duplicar, cancelar, traslados sueltos, el circuit breaker y el adaptador contra un servidor de prueba que responde como el contrato de Rumbo |
| tiempo-real | 6 | El reparto de avisos y que un navegador lento no frene a los demás |
| mcp | 10 | Las 7 herramientas sobre MCP de verdad, sin token y con errores del sistema |
| asistente | 16 | El ciclo con un modelo falso: límite de vueltas, confirmación antes de crear y que el token nunca llegue al modelo |

Pasaron los 170, ninguno falló.

## 9. MySQL de logs caído

Validación de [ADR-010](adr/ADR-010-resiliencia.md) y [ADR-011](adr/ADR-011-observabilidad.md). Apagamos `mysql-logs` con todo lo demás andando.

| Momento | Resultado |
|---|---|
| Antes del arreglo | Cada pedido esperaba su `INSERT` de log: pasaba los 2 segundos del gateway y respondía `503` a los 8 segundos |
| Con el arreglo (cada log espera como mucho 200 ms) | Los pedidos responden en unos 0,6 segundos y se pudo crear una transferencia. Los logs que no se guardaron aparecen completos en `docker compose logs` |
| Con el MySQL de logs andando | Cada pedido tarda unos 30 ms, y se guardaron las 45 filas de logs de la prueba |

## 10. Tiempo real

Validación de [ADR-019](adr/ADR-019-tiempo-real.md). Abrimos `GET /api/events` con `curl` a través del gateway, con un token, y en otra terminal creamos una transferencia y la hicimos avanzar.

| Qué | Resultado |
|---|---|
| Crear la transferencia | Llegaron `TransferenciaActualizada` con el estado y `StockActualizado` con el nuevo disponible del origen |
| Cada cambio de estado | Llegó un `TransferenciaActualizada` por cada uno |
| Conexión abierta sin eventos | Cada 25 segundos llega un comentario para que no se corte |

Una primera corrida pareció perder eventos, pero era el límite de tiempo que le habíamos puesto a `curl`. Con un límite más largo llegaron todos.

## 11. Asistente

Validación de [ADR-018](adr/ADR-018-asistente.md).

**Servidor MCP contra el sistema levantado.** Llamamos a las herramientas como lo hace el Asistente, con el token de un vendedor de la sucursal 1:

| Prueba | Resultado |
|---|---|
| Las herramientas de consulta y de estado | Responden con los datos de la API |
| Crear una transferencia de la sucursal 2 a la 3 | La rechaza Transferencias, porque no sale de la sucursal del vendedor ni llega a ella. El stock del origen siguió en 13 |
| Sin token | Error, y el servidor MCP no llama al gateway |

**Modelo.** Probamos la clave de Groq el 8/11/2026. El modelo que habíamos elegido (`llama-3.3-70b-versatile`) ya no estaba disponible. De los que sí estaban, probamos los tres que pueden usar herramientas (`openai/gpt-oss-120b`, `openai/gpt-oss-20b` y `qwen/qwen3.8-27b`) con la pregunta "¿Qué stock hay en la sucursal Centro?". Los tres pidieron primero `listar_sucursales`, en lugar de inventar el id, en menos de un segundo. Elegimos `openai/gpt-oss-120b`. Con el resultado de la herramienta, respondió "Las sucursales son: Centro, Nueva Córdoba y Cerro".

**De punta a punta.** El 9/11/2026, con todo el sistema levantado y el token del director, por `/api/assistant/chat` a través del frontend:

| Prueba | Resultado |
|---|---|
| "¿Cuánta Harina 000 hay en Córdoba Centro?" | "En la sucursal Córdoba Centro hay 313 unidades de Harina 000", que es el disponible real. Tardó 3 segundos |
| "Pasá 2 Harina 000 de Córdoba Centro a Nueva Córdoba" | Devolvió la tarjeta con producto, cantidad, origen y destino, y no creó nada |
| Confirmar la tarjeta en `/api/assistant/confirm` | Creó la transferencia, que pasó a `PROGRAMADA` con el proceso de fondo |

La primera vez que lo probamos contestó que la búsqueda no estaba disponible: Solr tardó en responder las primeras consultas después de arrancar e Inventario cortó por tiempo. Unos minutos después la misma pregunta anduvo.

## 12. Rumbo real

Validación de [ADR-009](adr/ADR-009-consumo-de-rumbo.md), contra `https://rumbo-gateway.onrender.com` (contrato 4.0.1), el 8/11/2026:

| Prueba | Resultado |
|---|---|
| `POST /auth/token` con credenciales equivocadas | `401` `NO_AUTENTICADO`: "Las credenciales no son válidas." |
| `POST /auth/token` con las credenciales del grupo | `200`, token `Bearer` por 3600 segundos |
| `GET /disponibilidad?fecha=2026-11-09` | Las 6 franjas, de 8 a 20, con lugar para 4 traslados cada una |

Estas pruebas no reservan nada.

**Reserva de punta a punta.** El 9/11/2026 a las 15:16, con todo el sistema levantado, creamos una transferencia de 3 Harina 000 de Córdoba Centro a Nueva Córdoba, sin hora pedida:

| Qué | Resultado |
|---|---|
| Al crearla | `201` en estado `RESERVADA`, y el disponible del origen bajó de 316 a 313 |
| A los 5 segundos | `PROGRAMADA` en la franja de 18 a 20 del mismo día, la primera que respeta las 2 horas de anticipación |
| Tiempo Real | Llegaron `StockActualizado` con 313 y un `TransferenciaActualizada` por cada estado |

La dejamos programada.

**Cancelación de punta a punta.** Un vendedor de Villa Carlos Paz (sucursal 3) creó una transferencia de 2 Harina 000 a Córdoba Centro, que quedó `PROGRAMADA` en 5 segundos, y la canceló:

| Qué | Resultado |
|---|---|
| Stock del origen | 10 antes, 8 con la transferencia reservada y 10 otra vez después de cancelar |
| Rumbo | Confirmó la cancelación y la transferencia quedó `CANCELADA` |
| El pedido de cancelar | Respondió `503` aunque la cancelación se aplicó: el usuario esperaba el lock que tenía el proceso de fondo mientras le hablaba a Rumbo, y el gateway corta a los 2 segundos |
| Los logs | Mandamos una segunda cancelación que Rumbo rechazó con `409 CLAVE_IDEMPOTENCIA_REUTILIZADA`: la consulta de cambios había traído el traslado todavía programado mientras el envío lo cancelaba |

Los dos errores eran nuestros. Pasamos a un lock por transferencia ([ADR-004](adr/ADR-004-consistencia-y-concurrencia.md)) e ignoramos los datos de Rumbo más viejos que la cancelación confirmada ([ADR-009](adr/ADR-009-consumo-de-rumbo.md)), cada uno con su test.

**Con los arreglos.** Reconstruimos Transferencias y repetimos la prueba con el mismo vendedor:

| Qué | Resultado |
|---|---|
| El pedido de cancelar | `200` en 0,08 segundos, con la cancelación pedida |
| Rumbo | Respondió `503` tres veces seguidas. Reintentamos cada 10 segundos con la misma clave y a la cuarta la aceptó: `CANCELADA` unos 30 segundos después de pedirla |
| Stock del origen | 10 antes, 8 con la reserva y 10 después |
| Los logs | Una sola cancelación aceptada, sin traslados sueltos ni `409` |

## 13. Frontend

El 9/11/2026, con `docker compose up --build -d` y entrando por `http://localhost:8000`:

| Prueba | Resultado |
|---|---|
| La página y una ruta interna (`/transferencias`) | `200`, NGINX devuelve la aplicación y los archivos de `assets` |
| `/api` a través del frontend | Llega al gateway: sin token `401`, con token responde |
| Login del director | Token con el rol y sin sucursal |
| Ajuste de entrada de 10 unidades | `200` y el evento `StockActualizado` llegó en el momento por `/api/events` |
| Transferencia y asistente | Las de las secciones 11 y 12 se hicieron por acá |

**Permisos por rol.** Con el director creamos un admin y un vendedor de la sucursal 3, entramos con cada uno y probamos lo que pide la especificación:

| Rol | Lo que pudo | Lo que se rechazó con `403` |
|---|---|---|
| Vendedor de la 3 (CA-19) | Ver productos, el stock de otra sucursal, las sucursales y los usuarios; crear y cancelar una transferencia de la 3 a la 1 | Ajustar stock, crear un producto, una sucursal o un usuario, y crear una transferencia de la 1 a la 2 |
| Admin de la 3 (CA-21) | Ajustar stock en la 3 y crear un vendedor de la 3 | Ajustar stock en la 1, crear un vendedor de la 1, crear un admin, una sucursal o un producto |

Las 18 pruebas dieron lo esperado.

## 14. Concurrencia, eventos, caídas y búsqueda

El 9/11/2026, con todo el sistema levantado y el token del director.

**Reservas que se cruzan (CA-13).** Con un ajuste dejamos 5 disponibles de Azúcar en Nueva Córdoba y mandamos al mismo tiempo dos transferencias de 4 a Córdoba Centro. Una quedó `RESERVADA` y la otra `SIN_STOCK`, y el origen quedó con 1 disponible. El `FOR UPDATE` de la reserva hace esperar a la segunda hasta que termina la primera ([ADR-004](adr/ADR-004-consistencia-y-concurrencia.md)).

**Evento repetido (CA-15).** Cancelamos la transferencia reservada, Transferencias publicó `TrasladoCancelado` e Inventario liberó la reserva. Después publicamos a mano el mismo mensaje, con el mismo `id`, desde la API del panel de RabbitMQ, como si RabbitMQ lo entregara dos veces. Inventario lo recibió y lo confirmó, pero no cambió nada: el disponible siguió en 5 y los movimientos del producto en 6. Usamos `TrasladoCancelado` y no `TrasladoCompletado` porque completar un traslado lo decide Rumbo, y publicar un `TrasladoCompletado` inventado dejaba a Inventario y a Transferencias diciendo cosas distintas. Los dos eventos pasan por el mismo control, que anota el `id` del evento en la misma transacción que el cambio de stock.

**Mensaje que no se puede procesar (DLQ).** Publicamos un `TrasladoCompletado` de una transferencia que no existe. Quedó en `q.inventario.TrasladoCompletado.dlq` con el log de error "va a la cola de errores: no hay una reserva para esa transferencia".

**Inventario no responde (CA-14).** Apagamos las dos instancias de Inventario y creamos una transferencia:

| Qué | Resultado |
|---|---|
| La transferencia | No se creó: Transferencias pide el producto a Inventario antes de reservar, y esa consulta no respondió |
| Stock del origen | 5 antes y 5 después de prender Inventario de nuevo |
| Lo que vio el usuario | `503` del gateway a los 2,07 segundos. Transferencias respondió su propio error a los 2,3 segundos, cuando el gateway ya había cortado |

Cambiamos dos cosas. El gateway ahora espera 5 segundos al crear o cancelar una transferencia, así llega la respuesta de Transferencias ([ADR-010](adr/ADR-010-resiliencia.md)). Y CA-14 ahora distingue los dos casos: si Inventario deja de responder justo al reservar, la transferencia queda `FALLIDA`; si ya no responde antes, no se crea. En los dos el stock no cambia.

Con el gateway reconstruido repetimos la prueba: el usuario recibió el `503` de Transferencias a los 2,2 segundos, con el mensaje "No se pudo crear la transferencia porque Sucursales o Inventario no respondió. Probá de nuevo en unos minutos.", y el stock siguió en 5.

**Rumbo caído.** Reiniciamos Transferencias con una `RUMBO_URL` que rechaza la conexión y creamos dos transferencias:

| Qué | Resultado |
|---|---|
| Las transferencias | Quedaron `RESERVADA` |
| A los 20 segundos | 5 fallas seguidas, `rumbo_circuit_opens_total` en 1 y el log "Rumbo falló 5 veces seguidas: no lo llamamos por 30s" |
| Durante esos 30 segundos | `rumbo_errors_total` siguió en 5: no se llamó a Rumbo |
| Medio abierto | La llamada de prueba fue la consulta de cambios. Falló, y el circuit breaker se volvió a abrir (2 aperturas, 6 errores) |
| Con la `RUMBO_URL` real de nuevo | Las dos pasaron solas a `PROGRAMADA` en menos de 15 segundos. Después las cancelamos |

**Búsqueda (D6).**

| Prueba | Resultado |
|---|---|
| `q=harina` | Harina 000 y Harina integral |
| `q=aceite` con proveedor Aceitera Central / Molinos del Sur | Aceite de girasol / ninguno |
| `q=harina` de a 1 por página | Página 1 Harina 000, página 2 Harina integral, total 2 |
| Sin `q` | `400` "El parámetro q es obligatorio" |
| Renombrar Harina integral | Apareció en la búsqueda a los 3,5 segundos; al volver el nombre, a los 1,9 |
| `docker compose restart inventario1` (reconstrucción) | Solr con 13 productos, los mismos nombres que MySQL |

Las primeras búsquedas después de un rato sin uso respondieron `503`: Solr tardó más de los 2 segundos que le da Inventario. Ya nos había pasado con el asistente (sección 11). Antes de una demo conviene hacer una búsqueda para que Solr cargue sus cachés.

**Inventario publicado (D8).** Contra `https://stock-volcamelli.onrender.com`:

| Prueba | Resultado |
|---|---|
| Sin API key o con una equivocada | `401` "Falta la API key o no es válida", también en aumentar |
| Las cuatro consultas con la key | `200` con los productos, un producto, el stock de la sucursal y el de un producto |
| Aumentar 2 y reducir 2 | `200`: el disponible pasó de 0 a 2 y volvió a 0 |
| Producto que no existe | `404` `PRODUCT_NOT_FOUND` |
| Reducir más de lo disponible | `409` `INSUFFICIENT_STOCK` |
| Cantidad 0 | `400` `INVALID_REQUEST` |

**Carga y caché: la corrida no sirvió.** Corrimos los escenarios de ADR-013 y la comparación con y sin caché, pero la notebook estaba saturada: la máquina virtual de Docker en su límite de memoria y menos de 1 GB libre en Windows. Con 10 workers el p95 ya fue de 1,5 segundos, y con 100 workers fallaron casi todos los pedidos (en la primera medición, con 50 workers el p95 quedaba debajo de 130 ms). Con o sin caché, el p95 quedó en los 2 segundos del gateway, así que no se puede comparar. Lo único que sí sirve es el porcentaje de aciertos: 98 % en Usuarios y 97 % en Sucursales. No cambiamos los números de ADR-013; repetimos las dos pruebas con la máquina libre.

## 15. Caché, control de tráfico, orden de la búsqueda y RabbitMQ caído

Corrido el 10/11/2026 con `develop` en `8c05016` más los cambios de ese día, en la misma notebook de ADR-013.

**Caché con y sin (D7).** Cada consulta, 20 segundos con 20 workers, por el gateway y con el control de tráfico apagado (`RATE_LIMIT_PER_MINUTE=0`). Primero con `CACHE_ENABLED=false` y después con la caché prendida, recreando solo Usuarios y Sucursales. Las consultas a MySQL son los `SELECT` que contó cada base durante la corrida (`Com_select`).

| Consulta | Caché | Pedidos por segundo | p50 | p95 | p99 | Consultas a MySQL |
|---|---|---|---|---|---|---|
| `GET /api/branches/1` | No | 80,6 | 198 ms | 578 ms | 894 ms | 1626 |
| `GET /api/branches/1` | Sí | 75,1 | 216 ms | 664 ms | 896 ms | 11 |
| `GET /api/branches` | No | 51,9 | 225 ms | 1107 ms | 1341 ms | 1094 |
| `GET /api/branches` | Sí | 99,4 | 174 ms | 416 ms | 488 ms | 10 |
| `GET /api/users?branchId=1` | No | 77,2 | 200 ms | 583 ms | 770 ms | 1566 |
| `GET /api/users?branchId=1` | Sí | 117,8 | 145 ms | 320 ms | 517 ms | 10 |

Ningún error en las seis corridas. El análisis está en [ADR-013](adr/ADR-013-capacidad-y-costos.md).

**Control de tráfico (D10).** Con el límite en 300 pedidos por minuto, mandamos 320 pedidos a `GET /api/branches/1` en el mismo minuto, 40 a la vez:

| Resultado | Pedidos |
|---|---|
| `200` | 299 |
| `503` (uno que pasó los 2 segundos, con la máquina cargada) | 1 |
| `429` `TOO_MANY_REQUESTS` | 20 |

El `429` trae `Retry-After: 17` (los segundos que faltaban para el minuto siguiente) y el mensaje "demasiados pedidos, probá de nuevo en 17 segundos". Pasado el límite, el login también responde `429`. Que otra IP siga entrando lo prueba un test del gateway.

**Orden de la búsqueda (D6).** Con `q=paquete` (7 resultados):

| `sort` | Resultado |
|---|---|
| `relevancia` (o sin `sort`) | Arroz largo fino (0,33), Azúcar (0,28) y los demás con 0,26 |
| `nombre` | Arroz largo fino, Azúcar, Café molido, Fideos tirabuzón, Harina 000, Harina integral, Sal fina |
| `otro` | `400` "El parámetro sort tiene que ser relevancia o nombre." |

El campo `nombre_orden` se creó solo al arrancar Inventario, sobre el índice que ya existía. El primer intento de crearlo pasó los 2 segundos y la reconstrucción lo reintentó a los 5 segundos.

**Crear con RabbitMQ caído (D10).** Con `docker compose stop rabbitmq`, crear una transferencia de 1 Harina 000 respondió `201` en 0,74 segundos (antes del arreglo, `503` a los 5,1). El disponible bajó de 311 a 310 y los eventos que no se pudieron publicar quedaron en el log de error ("no se pudo publicar TransferenciaActualizada… después de 4 intentos"). Con RabbitMQ de vuelta, cancelarla respondió `200` en 0,03 segundos, quedó `CANCELADA` y el disponible volvió a 311.

**Cancelar con RabbitMQ caído (D10).** Lo probó una revisión externa el 10/11/2026: con 5 disponibles en el origen, reservó 1 (quedaron 4) y canceló la transferencia con RabbitMQ apagado. Transferencias la guardó `CANCELADA`, pero al volver RabbitMQ el disponible siguió en 4: el `TrasladoCancelado` había quedado solo en el log de error y nadie lo volvía a publicar. Publicando de nuevo el evento el disponible volvió a 5, y repetirlo lo dejó en 5. Lo arreglamos con la marca de eventos por publicar ([ADR-016](adr/ADR-016-comunicacion-entre-servicios.md)). Los tests de Transferencias cubren cancelar y completar con RabbitMQ caído, que la marca se saque al publicar, que con RabbitMQ todavía caído cada vuelta pruebe con una sola transferencia y que cancelar no espere los reintentos del proceso de fondo.

Con el arreglo, el 11/11/2026 repetimos el ensayo con el sistema levantado y Rumbo real:

| Paso | Resultado |
|---|---|
| `docker compose stop rabbitmq` y crear una transferencia de 1 Harina 000 | `201` en 0,39 segundos, el disponible bajó de 311 a 310. Rumbo la programó a los 3 segundos |
| Cancelar mientras el proceso de fondo reintentaba publicar | `200` en 0,6 segundos. Rumbo canceló el traslado y la transferencia quedó `CANCELADA` con `eventos_por_publicar: true`. El disponible siguió en 310 |
| `docker compose start rabbitmq` | A los 21 segundos el log dice "eventos de la transferencia … (CANCELADA) publicados en el reintento", Inventario procesó el `TrasladoCancelado` y el disponible volvió solo a 311. La marca quedó en `false` |

En una primera corrida del arreglo, cancelar respondió `503` a los 5 segundos aunque la cancelación se aplicó 26 segundos después: el proceso de fondo reintentaba publicar con la transferencia tomada y el pedido esperaba detrás. Lo corregimos publicando siempre sin el lock ([ADR-010](adr/ADR-010-resiliencia.md)), y es la corrida de la tabla.

## 16. Pruebas que faltan correr

| Prueba | Qué valida | Cómo | Qué tiene que dar |
|---|---|---|---|
| Pantallas por rol | CA-19, CA-21 | Entrar al frontend con el vendedor y el admin de prueba | Cada uno ve solo los botones y pantallas de su rol |
| Carga | D13 | El generador de `pruebas/carga`, con la máquina libre | Los mismos escenarios de ADR-013, para comparar con la primera medición |

## Cómo repetir estas pruebas

1. `docker compose down -v` y `docker compose up --build -d`.
2. Login con el director y con la cuenta `INTEGRACION`. Las contraseñas las tiene el equipo.
3. Las consultas de las secciones 1 a 3 con `curl -i`, anotando el código y el `X-Track-ID`.
4. Pegar un `X-Track-ID` en el campo **Track ID** del tablero y buscar el mismo valor en `docker compose logs nginx`.
5. Los pasos A a D de la sección 6 con `docker compose stop` y `docker compose start`.
6. La carga con el generador de [`pruebas/carga`](../pruebas/carga/main.go), como dice ADR-013.

## Problemas encontrados

| Problema | Cómo se resolvió |
|---|---|
| Los datos de prueba con tilde se guardaban mal en MySQL (`AzÃºcar`) | Los archivos SQL se cargan en `utf8mb4` (PR #7). Con una base ya creada hay que correr `docker compose down -v` una vez para recargarla |
| Bajo carga, el MySQL de logs se queda sin conexiones y se pierden logs | Es el límite de capacidad; las acciones están en [ADR-013](adr/ADR-013-capacidad-y-costos.md) |
| NGINX registra la IP de la instancia y no el nombre | Se averigua con `docker inspect` (sección 4) |
| Con el MySQL de logs apagado, todos los pedidos respondían `503` | Cada log espera como mucho 200 ms y, si no se guarda, sale por consola (sección 9) |
| Rumbo publicó la versión 4 de su contrato, que reserva franjas en lugar de proponer horarios | Reescribimos el adaptador y ADR-009, y dejamos de usar el Rumbo falso |
| El modelo elegido para el asistente ya no estaba en Groq | Cambiamos a `openai/gpt-oss-120b` con la variable `MODEL_NAME` (sección 11) |
| Inventario publicado no respondía: la base de Aiven se había apagado por inactividad, y sin base Inventario se cerraba al arrancar y Render lo reiniciaba sin parar | Un cron-job consulta el catálogo cada 10 minutos para que Aiven y Render no se apaguen. Probamos también que Inventario arrancara sin base y respondiera `503` hasta que volviera, y anduvo, pero lo descartamos para que Inventario funcione igual que los demás servicios ([ADR-010](adr/ADR-010-resiliencia.md)) |
| El frontend en `localhost:8000` respondía `400 Request Header Or Cookie Too Large` | El navegador mandaba las cookies de otros proyectos que habían corrido en `localhost`. Le dimos a NGINX más lugar para los encabezados; nosotros no usamos cookies |
| Recién levantado el sistema, el asistente decía que la búsqueda no estaba disponible | Solr tardaba en responder las primeras consultas e Inventario cortaba por tiempo. A los pocos minutos anduvo (sección 11) |
| Con la máquina cargada, Transferencias se cerraba al arrancar: el MySQL de logs tardó más de 2 segundos en decirle si existía la tabla, intentó crearla y falló porque ya estaba | Los 8 servicios abren la conexión de logs sin probarla y crean la tabla en segundo plano, como ya hacía Inventario ([ADR-011](adr/ADR-011-observabilidad.md)) |
| `docker compose up` fallaba con "mongo-transferencias is unhealthy" aunque Mongo respondía | El chequeo de salud con `mongosh` pasaba los 3 segundos con la máquina cargada. Lo subimos a 10 |
| Cancelar respondía `503` aunque la cancelación se aplicaba, y se mandaba a Rumbo una cancelación de más | Lock por transferencia, y los datos de Rumbo más viejos que la cancelación se ignoran (sección 12) |
| Con RabbitMQ caído, cancelar una transferencia la dejaba `CANCELADA` pero la reserva seguía activa cuando RabbitMQ volvía: el evento quedaba solo en el log de error. Lo encontró una revisión externa | Transferencias guarda la marca de eventos por publicar junto con el cambio de estado, y el proceso de fondo los vuelve a publicar hasta que salen, sin tener la transferencia tomada ([ADR-016](adr/ADR-016-comunicacion-entre-servicios.md)). Probado con el sistema levantado: el disponible volvió solo a los 21 segundos de levantar RabbitMQ (sección 15) |
| Con RabbitMQ caído, crear una transferencia respondía `503` a los 5 segundos aunque quedaba creada y reservada; si el usuario volvía a intentar, la creaba dos veces. Lo encontró una revisión externa | Crear y cancelar publican los eventos en segundo plano, como ya hacía Inventario ([ADR-010](adr/ADR-010-resiliencia.md)). Lo probamos con RabbitMQ apagado: `201` en 0,74 segundos (sección 15) |
