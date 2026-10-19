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

El gateway queda en `http://localhost:8080`. Cuando Inventario esté desplegado, vamos a agregar acá su URL pública.

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

Las carpetas de los servicios se crean cuando empecemos a implementarlos.

```text
Stock-Volcamelli/
├── README.md
├── SPEC.md
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
└── pruebas/
    └── carga/
```
