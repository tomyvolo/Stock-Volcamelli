# Validación del sistema

Pruebas sobre el sistema levantado con Docker Compose. Respaldan lo que dicen los ADR: la validación de D1, D3 y D5 ([ADR-014](adr/ADR-014-limites-de-servicios.md), [ADR-015](adr/ADR-015-persistencia.md) y [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md)), el balanceo ([ADR-012](adr/ADR-012-balanceo-de-carga.md)) y la capacidad ([ADR-013](adr/ADR-013-capacidad-y-costos.md)).

Las secciones 1 a 7 son de la primera validación. De la 8 en adelante están las pruebas de lo que sumamos después: los tests, la caída del MySQL de logs, tiempo real, el asistente, y Rumbo real. Lo que todavía no corrimos está en la sección 13, con lo que tiene que dar cada prueba.

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

## 12. Rumbo real

Validación de [ADR-009](adr/ADR-009-consumo-de-rumbo.md), contra `https://rumbo-gateway.onrender.com` (contrato 4.0.1), el 8/11/2026:

| Prueba | Resultado |
|---|---|
| `POST /auth/token` con credenciales equivocadas | `401` `NO_AUTENTICADO`: "Las credenciales no son válidas." |
| `POST /auth/token` con las credenciales del grupo | `200`, token `Bearer` por 3600 segundos |
| `GET /disponibilidad?fecha=2026-11-09` | Las 6 franjas, de 8 a 20, con lugar para 4 traslados cada una |

Estas pruebas no reservan nada. La reserva y la cancelación de punta a punta están en la sección 13.

## 13. Pruebas que faltan correr

| Prueba | Qué valida | Cómo | Qué tiene que dar |
|---|---|---|---|
| Rumbo de punta a punta | D9 | Crear una transferencia sin hora, esperar el proceso de fondo y cancelarla | `RESERVADA`, después `PROGRAMADA` con la franja en `slotStart` y `slotEnd`; al cancelar, `CANCELADA` y el origen vuelve a tener el disponible de antes |
| Rumbo caído | D9, D10 | Levantar Transferencias con una `RUMBO_URL` que no responde y crear transferencias | Quedan `RESERVADA`; después de 5 fallas sube `rumbo_circuit_opens_total` y no se llama a Rumbo por 30 segundos |
| Reservas que se cruzan | D4 (CA-13) | Con 5 disponibles, dos transferencias de 4 al mismo tiempo | Una `RESERVADA` y otra `SIN_STOCK`; el origen queda con 1 disponible |
| Inventario no responde | D4 (CA-14) | `docker compose stop inventario1 inventario2` y crear una transferencia | `FALLIDA`; al volver Inventario, la reserva se libera sola |
| Evento repetido | D4, D5 (CA-15) | Publicar a mano el mismo `TrasladoCompletado` dos veces desde el panel de RabbitMQ | El stock se mueve una sola vez |
| Mensaje que no se puede procesar | D5 | Publicar un `TrasladoCompletado` de una transferencia que no existe | Va a `q.inventario.TrasladoCompletado.dlq` y queda el log de error |
| Caché antes y después | D7 | La carga de sucursales y usuarios con `CACHE_ENABLED=false` y con la caché | El porcentaje de aciertos del tablero y cuánto baja el p95 |
| Búsqueda | D6 | Buscar con texto, proveedor, orden y página; cambiar un producto; correr la reconstrucción | El cambio aparece en menos de 5 segundos y la reconstrucción deja el índice igual a MySQL |
| Asistente de punta a punta | ADR-018 | Preguntas de stock y una transferencia desde el chat | Responde con datos de la API y la transferencia se crea recién al confirmar |
| Inventario publicado | D8 | Las cuatro consultas y aumentar y reducir contra `https://stock-volcamelli.onrender.com`, sin API key y con la key | Sin key, `401`; con la key, las respuestas del contrato |
| Carga | D13 | El generador de `pruebas/carga`, sin `RUMBO_URL` | Los mismos escenarios de ADR-013, para comparar con la primera medición |

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
| Inventario publicado no respondía: la base de Aiven se había apagado por inactividad, y sin base Inventario se cerraba al arrancar y Render lo reiniciaba sin parar | Inventario arranca sin base y responde `503` hasta que vuelve. Lo probamos con un MySQL apagado: `503` en 3 ms, y unos 30 segundos después de prender MySQL pasó solo a `200`. Para que Aiven no se apague, un cron-job consulta el catálogo cada 10 minutos |
