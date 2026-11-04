# Validación del sistema

Pruebas sobre el sistema levantado con Docker Compose. Respaldan lo que dicen los ADR: la validación de D1, D3 y D5 ([ADR-014](adr/ADR-014-limites-de-servicios.md), [ADR-015](adr/ADR-015-persistencia.md) y [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md)), el balanceo ([ADR-012](adr/ADR-012-balanceo-de-carga.md)) y la capacidad ([ADR-013](adr/ADR-013-capacidad-y-costos.md)).

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
