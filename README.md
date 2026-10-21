# Stock Volcamelli

Stock Volcamelli es un sistema para llevar el inventario de una empresa con varias sucursales. Permite ver qué productos hay, cuánto stock tiene cada sucursal y mover mercadería de una sucursal a otra.

## Qué resuelve

En una empresa con varias sucursales, la misma mercadería está repartida y se mueve todo el tiempo. Queremos que en cualquier momento se sepa cuánto hay disponible de cada producto en cada sucursal, y que una transferencia nunca deje el stock mal: que no se mande más de lo que hay, que no quede stock negativo y que, si el traslado no se puede hacer, la mercadería siga disponible en el origen.

## Flujo principal

El flujo principal es la transferencia entre sucursales:

1. Un usuario elige producto, cantidad, sucursal de origen y sucursal de destino, y si quiere, a qué hora quiere que llegue.
2. Las unidades quedan reservadas en el origen: siguen ahí, pero nadie más las puede usar. Si el origen no tiene suficiente, la transferencia se rechaza.
3. En segundo plano le pedimos el traslado a Rumbo, el servicio de transporte de otro grupo. Si el usuario pidió una hora y Rumbo propone otra, el usuario la acepta o la rechaza. Si no pidió ninguna, aceptamos la que proponga Rumbo.
4. Seguimos el traslado hasta que llega: programado, en curso, con demora si la hay.
5. Cuando la mercadería llega, el stock pasa del origen al destino. Si el traslado se rechaza, se cancela o no se realiza, la reserva se libera y las unidades vuelven a estar disponibles en el origen.

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

Todo el sistema se levanta con Docker, desde la raíz del repositorio:

```bash
docker compose up --build -d
```

La primera vez tarda un par de minutos, porque los MySQL cargan sus tablas y datos de prueba. Después queda:

| Qué | Dónde |
|---|---|
| Gateway (la única entrada a la API) | `http://localhost:8080` |
| Grafana (usuario `admin`, contraseña `admin`) | `http://localhost:3000`, tablero "Stock Volcamelli" |
| Prometheus | `http://localhost:9090` |

Hay un usuario de prueba por rol: `director@volcamelli.com`, `admin1@volcamelli.com`, `vendedor1@volcamelli.com` e `integracion@volcamelli.com`. Las contraseñas no están en el repositorio; las tiene el equipo.

Para empezar de cero (borra los datos de los MySQL): `docker compose down -v`.

Cuando Inventario esté desplegado, vamos a agregar acá su URL pública.

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

Cómo responde el sistema cuando algo falla:

| Prueba | Cómo hacerla | Qué tiene que pasar |
|---|---|---|
| Se cae una instancia de Inventario | `docker compose stop inventario1` y repetir las consultas | Siguen respondiendo, ahora todas desde `inventario2`. Se ve en `docker compose logs nginx`, en el campo `instancia` |
| Se caen las dos | `docker compose stop inventario1 inventario2` | El gateway responde `503 SERVICE_UNAVAILABLE`, nunca stock cero |
| Volver a la normalidad | `docker compose start inventario1 inventario2` | En menos de 30 segundos NGINX vuelve a repartir entre las dos |

Los tests unitarios usan los repositorios mock, así que no necesitan Docker. Se corren desde la carpeta de cada servicio, por ejemplo `cd servicios/inventario && go test ./...`.

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

Por ahora están el gateway, Usuarios e Inventario. Las carpetas de los demás servicios se crean cuando empecemos a implementarlos.
