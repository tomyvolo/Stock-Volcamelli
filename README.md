# Stock Volcamelli

Stock Volcamelli es un sistema para llevar el inventario de una empresa con varias sucursales. Permite ver qué productos hay, cuánto stock tiene cada sucursal y mover mercadería de una sucursal a otra.

## Qué resuelve

En una empresa con varias sucursales, la misma mercadería está repartida y se mueve todo el tiempo. Queremos que en cualquier momento se sepa cuánto hay disponible de cada producto en cada sucursal, y que una transferencia nunca deje el stock mal: que no se mande más de lo que hay, que no quede stock negativo y que, si el traslado no se puede hacer, la mercadería siga disponible en el origen.

## Flujo principal

El flujo principal es la consulta del stock disponible de una sucursal. Es lo que le ofrecemos a otro grupo y lo que usan todas las pantallas:

1. El cliente hace login (`POST /api/auth/login`) y, con el token, consulta `GET /api/branches/{branchId}/stock`.
2. El gateway valida el token, le asigna al pedido un TrackID y lo reenvía a NGINX.
3. NGINX lo manda a una de las dos instancias de Inventario, que lee su MySQL y devuelve el disponible de cada producto: el total menos lo reservado.
4. El gateway devuelve la respuesta con el header `X-Track-ID`. Con ese valor se ve en Grafana el recorrido del pedido, y en el log de NGINX qué instancia lo atendió.

El disponible descuenta lo reservado porque una transferencia entre sucursales reserva las unidades en el origen hasta que llegan a destino. `branchId` es el id de la sucursal: el dueño de las sucursales es su propio servicio, e Inventario solo guarda ese id, así que la consulta no depende de él.

## Cómo está armado

El sistema son cuatro microservicios en Go detrás de un API Gateway, con un frontend en React:

| Servicio | Qué hace |
|---|---|
| Usuarios | Usuarios con su rol y su sucursal, y login |
| Sucursales | Sucursales y sus direcciones |
| Inventario | Productos, stock por sucursal, reservas y movimientos. Es lo que le ofrecemos al otro grupo |
| Transferencias | Transferencias entre sucursales e integración con Rumbo |

Usamos MySQL y MongoDB como bases, RabbitMQ para los eventos, NGINX para repartir la carga entre las dos instancias de Inventario, Solr para la búsqueda y Memcached como caché. Para ver qué pasa adentro, los logs y las trazas se guardan en un MySQL aparte y las métricas en Prometheus, y todo se ve en Grafana.

Además sumamos dos funcionalidades:

- **Asistente:** un chat en el frontend para consultar stock, buscar productos y crear transferencias conversando. Usa un modelo de lenguaje que opera el sistema a través de un servidor MCP.
- **Tiempo real:** las pantallas de stock y de transferencias se actualizan solas cuando algo cambia.

El detalle está en [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Integración con otros grupos

- **Ofrecemos** consultas del catálogo y del stock disponible de Inventario. El contrato y cómo probarlo están en [docs/contracts/README.md](docs/contracts/README.md).
- **Usamos** Rumbo, el servicio de traslados de otro grupo, en cada transferencia.

## Cómo ejecutarlo

Todo el sistema se levanta con Docker, desde la raíz del repositorio. Primero hay que crear el archivo `.env` con las claves, copiando el ejemplo y completándolo:

```bash
cp .env.example .env
docker compose up --build -d
```

El `.env` no se sube al repositorio. Si falta alguna clave, `docker compose` avisa cuál.

La primera vez tarda un par de minutos, porque los MySQL cargan sus tablas y datos de prueba. Después queda:

| Qué | Dónde |
|---|---|
| Gateway (la única entrada a la API) | `http://localhost:8080` |
| Grafana (usuario `admin`, contraseña `admin`) | `http://localhost:3000`, tablero "Stock Volcamelli" |
| Prometheus | `http://localhost:9090` |

Hay un usuario de prueba por rol: `director@volcamelli.com`, `admin1@volcamelli.com`, y `vendedor1@volcamelli.com`. Las contraseñas no están en el repositorio; las tiene el equipo.

Para empezar de cero (borra los datos de los MySQL): `docker compose down -v`.

### Cómo probarlo

Con todo levantado, se hace login y se usa el token en las consultas de Inventario:

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "vendedor1@volcamelli.com", "password": "..."}'

curl -i http://localhost:8080/api/branches/1/stock \
  -H "Authorization: Bearer <token>"
```

Con `-i`, la respuesta muestra el header `X-Track-ID`. Para ver la traza, pegá ese valor en el campo **Track ID** del tablero: aparecen los logs del gateway y de Inventario de ese pedido, en orden.

El director y el admin de sucursal pueden dar de alta usuarios con `POST /api/create-users`. El director crea usuarios de cualquier rol; el admin, solo vendedores de su sucursal. Con el token de un director:

```bash
curl -X POST http://localhost:8080/api/create-users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "Vendedor 2", "email": "vendedor2@volcamelli.com", "password": "...", "role": "VENDEDOR", "branchId": 1}'
```

Responde `201` con el usuario creado, sin la contraseña, que se guarda hasheada con bcrypt. Con un email repetido responde `409`, y si lo intenta un vendedor, `403`. El director también puede cambiarle el rol o la sucursal a un usuario con `PUT /api/users/{id}` y un body como `{"role": "VENDEDOR", "branchId": 2}`. El cambio se aplica cuando ese usuario vuelve a hacer login. Para borrar un usuario, el director usa `DELETE /api/users/{id}`, que responde `204`; no puede borrarse a sí mismo.

Cualquier usuario puede ver las sucursales con su nombre y dirección (`GET /api/branches` y `GET /api/branches/{branchId}`). Solo el director puede crearlas (`POST /api/branches`) o cambiarlas (`PUT /api/branches/{branchId}`), con un body `{"name": "...", "address": "..."}`. También pueden ver a todos los usuarios de la empresa con su rol y su sucursal (`GET /api/users`, o `GET /api/users?branchId=1` para una sola sucursal), nunca con la contraseña. Las consultas de sucursales y de usuarios pasan por Memcached: un cambio se ve como mucho 60 segundos después.

El director y el admin de sucursal pueden ajustar el stock, por ejemplo cuando llega mercadería o hay una rotura. El admin, solo en su sucursal:

```bash
curl -X POST http://localhost:8080/api/branches/1/products/1/adjustments \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"type": "SALIDA", "quantity": 2, "reason": "Rotura"}'
```

Responde el total, lo reservado y el disponible que quedaron. Una salida que dejaría el total por debajo de lo reservado responde `409`. Cada ajuste queda en el historial del producto en esa sucursal: `GET /api/branches/1/products/1/movements`. El director también da de alta productos con `POST /api/products` y los modifica con `PUT /api/products/{id}`, con un body como `{"name": "Polenta", "description": "Paquete de 500 g", "supplier": null}`.

Cualquier rol busca productos por texto, en el nombre y la descripción, ordenados por relevancia:

```bash
curl "http://localhost:8080/api/products/search?q=harina&supplier=Molinos%20del%20Sur&page=1&size=20" \
  -H "Authorization: Bearer <token>"
```

`q` es obligatorio y `supplier` filtra por el nombre exacto del proveedor. Cuando se crea o se modifica un producto, aparece en la búsqueda en unos segundos.

Cualquier usuario puede crear una transferencia que salga de su sucursal o llegue a ella. El director, con cualquier sucursal:

```bash
curl -X POST http://localhost:8080/api/transfers \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"productId": 1, "quantity": 4, "originBranchId": 1, "destinationBranchId": 2}'
```

Responde enseguida con la transferencia `RESERVADA` (o `SIN_STOCK` si no alcanza el disponible), y en segundo plano se reserva el traslado en Rumbo. Opcionalmente lleva `desiredArrival` (la hora de llegada, en ISO 8601) y `notes`. Transferencias reserva la franja de Rumbo que contiene esa hora o, si no tiene lugar o no se pidió hora, la primera con lugar. Cuando Rumbo la reserva, la transferencia pasa a `PROGRAMADA` y muestra la franja de llegada en `slotStart` y `slotEnd`. El estado y su historial se ven con `GET /api/transfers/{id}`, y la lista con `GET /api/transfers?branchId=1&status=PROGRAMADA`. Se cancela con `POST /api/transfers/{id}/cancel` y `{"reason": "..."}`.

Transferencias habla con el Rumbo real, así que para probar las transferencias hace falta internet y tener en el `.env` su URL (`RUMBO_URL`) y las credenciales que entrega su equipo (`RUMBO_CLIENT_ID` y `RUMBO_CLIENT_SECRET`). Sin `RUMBO_URL`, Transferencias arranca igual pero no le manda nada a Rumbo, y las transferencias quedan `RESERVADA`. Así corremos las pruebas de carga. Transferencias consulta los cambios cada 60 segundos. `EN_CURSO` y `COMPLETADA` dependen de que un operador de Rumbo marque el viaje, así que el recorrido completo no avanza solo. Las franjas tienen un cupo que comparten todos los grupos, así que las transferencias de prueba las cancelamos al terminar.

Cuando la transferencia termina, Inventario recibe el evento por RabbitMQ y mueve el stock: si quedó `COMPLETADA`, las unidades pasan al destino y se ven en `GET /api/branches/2/products/1/movements`; si quedó `RECHAZADA`, `CANCELADA` o `NO_REALIZADA`, vuelven al disponible del origen. Un evento que no se puede procesar queda en su cola de errores (`q.inventario.TrasladoCompletado.dlq` o `q.inventario.TrasladoCancelado.dlq`), que se ve en el panel de RabbitMQ.

Cómo responde el sistema cuando algo falla:

| Prueba | Cómo hacerla | Qué tiene que pasar |
|---|---|---|
| Se cae una instancia de Inventario | `docker compose stop inventario1` y repetir las consultas | Siguen respondiendo, ahora todas desde `inventario2`. Se ve en `docker compose logs nginx`, en el campo `instancia` |
| Se caen las dos | `docker compose stop inventario1 inventario2` | El gateway responde `503 SERVICE_UNAVAILABLE`, nunca stock cero |
| Volver a la normalidad | `docker compose start inventario1 inventario2` | En menos de 30 segundos NGINX vuelve a repartir entre las dos |
| Se cae la caché | `docker compose stop memcached` y consultar `GET /api/branches` y `GET /api/users` | Siguen respondiendo, desde MySQL. En `docker compose logs sucursales` aparece que Memcached no respondió |
| Inventario no está cuando termina una transferencia | Con una transferencia `EN_CURSO`, `docker compose stop inventario1 inventario2`; esperar a que quede `COMPLETADA` y `docker compose start inventario1 inventario2` | El evento espera en `q.inventario.TrasladoCompletado` y, cuando Inventario vuelve, el stock pasa al destino una sola vez |
| Se cae Solr | `docker compose stop solr` y buscar | La búsqueda responde `502` ("La búsqueda no está disponible.") o, si Solr no contesta en 2 segundos, el `503` del gateway. Nunca una lista vacía. El catálogo y el stock siguen respondiendo desde MySQL. Con `docker compose start solr` vuelve a andar |
| Llega un evento que no se puede leer | En el panel de RabbitMQ, publicar `hola` en el exchange `ex.TrasladoCompletado` | Inventario lo rechaza y queda en `q.inventario.TrasladoCompletado.dlq`. Los demás eventos se siguen procesando |

Los tests unitarios usan los repositorios mock, así que no necesitan Docker. Se corren desde la carpeta de cada servicio, por ejemplo `cd servicios/inventario && go test ./...`.

### Comandos útiles

| Comando | Qué hace |
|---|---|
| `docker compose ps` | Muestra qué contenedores están corriendo |
| `docker compose logs inventario1 --tail 20` | Muestra los últimos logs de un servicio |
| `docker compose logs nginx --tail 5` | Muestra a qué instancia de Inventario mandó NGINX cada pedido |
| `docker compose stop inventario1` / `docker compose start inventario1` | Apaga o prende un servicio |
| `docker stats` | Muestra la CPU y la memoria de cada contenedor |
| `docker compose down` | Apaga todo sin borrar los datos |

Las bases se pueden abrir desde la compu con un cliente de MySQL, como Workbench, en `127.0.0.1`:

| Base | Puerto | Usuario y contraseña (del `.env`) | Esquema |
|---|---|---|---|
| Usuarios | 3310 | `USUARIOS_DB_USER` / `USUARIOS_DB_PASSWORD` | `USUARIOS_DB_NAME` |
| Sucursales | 3311 | `SUCURSALES_DB_USER` / `SUCURSALES_DB_PASSWORD` | `SUCURSALES_DB_NAME` |
| Inventario | 3308 | `INVENTARIO_DB_USER` / `INVENTARIO_DB_PASSWORD` | `INVENTARIO_DB_NAME` |
| Logs | 3309 | `LOGS_DB_USER` / `LOGS_DB_PASSWORD` | `LOGS_DB_NAME` |

MongoDB, con las transferencias, se abre con MongoDB Compass en `mongodb://<MONGO_USER>:<MONGO_PASSWORD>@127.0.0.1:27017`, base `transferencias`. Las colas y los mensajes de RabbitMQ se ven en `http://localhost:15672`, con `RABBITMQ_USER` y `RABBITMQ_PASSWORD`. El índice de búsqueda se ve en el panel de Solr, `http://localhost:8983/solr/#/productos/query`.

## Cómo trabajamos con Git

- `main` tiene siempre una versión que funciona y es la que se evalúa. No se trabaja directo sobre ella.
- Cada cambio se hace en una rama propia que sale de `main`: `feat/<tema>` para funcionalidades (por ejemplo `feat/inventario`) y `docs/<tema>` para documentación.
- Cuando la rama está lista, se abre un pull request hacia `main`. Otro integrante lo revisa (que compile, que pasen los tests y que respete las capas) y recién ahí se mergea.
- Después del merge levantamos todo con `docker compose up --build` y probamos el recorrido completo.

## Documentación

| Documento | Qué tiene |
|---|---|
| [SPEC.md](SPEC.md) | Alcance, actores, reglas de negocio y criterios de aceptación |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Servicios, tecnologías, gateway, comunicación y flujo de transferencia |
| [docs/diagramas/contexto.md](docs/diagramas/contexto.md) | Diagrama de contexto |
| [docs/diagramas/contenedores.md](docs/diagramas/contenedores.md) | Diagrama de contenedores |
| [docs/adr/](docs/adr/) | Decisiones de arquitectura |
| [docs/contracts/README.md](docs/contracts/README.md) | Contrato de lo que ofrecemos y cómo probarlo |
| [docs/validacion.md](docs/validacion.md) | Pruebas sobre el sistema levantado que respaldan las decisiones |

## Estructura del repositorio

```text
Stock-Volcamelli/
├── README.md
├── SPEC.md
├── docker-compose.yml
├── .env.example        → las claves que hay que completar en el .env
├── docs/
│   ├── ARCHITECTURE.md
│   ├── adr/
│   ├── contracts/
│   └── diagramas/
├── frontend/
├── gateway/
├── servicios/
│   ├── usuarios/
│   ├── sucursales/
│   ├── inventario/
│   ├── transferencias/
│   ├── asistente/
│   ├── mcp/
│   └── tiempo-real/
├── nginx/              → balanceo entre las dos instancias de Inventario
├── pruebas/
│   └── carga/          → generador de carga para las pruebas de capacidad
└── observabilidad/     → Prometheus, Grafana y el esquema del MySQL de logs
```
