# Stock Volcamelli

Stock Volcamelli es un sistema para llevar el inventario de una empresa con varias sucursales. Permite ver qué productos hay, cuánto stock tiene cada sucursal y mover mercadería de una sucursal a otra.

## Qué resuelve

En una empresa con varias sucursales, la misma mercadería está repartida y se mueve todo el tiempo. Queremos que en cualquier momento se sepa cuánto hay disponible de cada producto en cada sucursal, y que una transferencia nunca deje el stock mal: que no se mande más de lo que hay, que no quede stock negativo y que, si el traslado no se puede hacer, la mercadería siga disponible en el origen.

## Flujo principal

Nuestro flujo principal es la consulta de Inventario, en particular del stock disponible por sucursal. Es la capacidad que publicamos para otro grupo y el recorrido que tenemos implementado y trazado en esta entrega:

1. El cliente obtiene un token mediante `POST /api/auth/login` y consulta `GET /api/branches/{branchId}/stock` con ese token. El login es un pedido separado y tiene su propio TrackID.
2. El gateway valida el token, genera un `X-Track-ID` para la consulta y reenvía el pedido a NGINX.
3. NGINX distribuye el pedido a una de las dos instancias de Inventario. La instancia lee MySQL y devuelve el stock disponible de la sucursal: `total - reserved`.
4. El gateway devuelve la respuesta con el mismo `X-Track-ID`. Sus logs y los de Inventario permiten seguir ese pedido en Grafana. El registro de NGINX, consultado por separado, indica qué instancia atendió el pedido.

Por ahora, `branchId` es un identificador numérico. Los datos de prueba de Inventario usan los valores 1, 2 y 3; esta consulta no llama a un servicio de Sucursales ni verifica que el ID corresponda a una sucursal registrada. La transferencia entre sucursales sigue prevista para una etapa posterior. 

## Cómo está armado

La arquitectura prevista contempla cuatro microservicios en Go detrás de un API Gateway, con un frontend en React. En esta entrega están operativos Usuarios e Inventario; Sucursales y Transferencias aún no están implementados:

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
| Grafana (usuario `admin`, contraseña la de `GRAFANA_ADMIN_PASSWORD`) | `http://localhost:3000`, tablero "Stock Volcamelli" |
| Prometheus | `http://localhost:9090` |

Hay un usuario de prueba por rol: `director@volcamelli.com`, `admin1@volcamelli.com`, `vendedor1@volcamelli.com` e `integracion@volcamelli.com`. Las contraseñas no están en el repositorio; las tiene el equipo.

Para empezar de cero (borra los datos de los MySQL): `docker compose down -v`.

### Cómo probarlo

Con todo levantado, se hace login y se usa el token en las consultas del contrato ([docs/contracts/README.md](docs/contracts/README.md) tiene todas):

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "integracion@volcamelli.com", "password": "..."}'

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

Responde `201` con el usuario creado, sin la contraseña, que se guarda hasheada con bcrypt. Con un email repetido responde `409`, y si lo intenta un vendedor o la cuenta de integración, `403`.

Cómo responde el sistema cuando algo falla:

| Prueba | Cómo hacerla | Qué tiene que pasar |
|---|---|---|
| Se cae una instancia de Inventario | `docker compose stop inventario1` y repetir las consultas | Siguen respondiendo, ahora todas desde `inventario2`. Se ve en `docker compose logs nginx`, en el campo `instancia` |
| Se caen las dos | `docker compose stop inventario1 inventario2` | El gateway responde `503 SERVICE_UNAVAILABLE`, nunca stock cero |
| Volver a la normalidad | `docker compose start inventario1 inventario2` | En menos de 30 segundos NGINX vuelve a repartir entre las dos |

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
| Inventario | 3308 | `INVENTARIO_DB_USER` / `INVENTARIO_DB_PASSWORD` | `INVENTARIO_DB_NAME` |
| Logs | 3309 | `LOGS_DB_USER` / `LOGS_DB_PASSWORD` | `LOGS_DB_NAME` |

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
└── observabilidad/     → Prometheus, Grafana y el esquema del MySQL de logs
```
