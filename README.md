# Stock Volcamelli

Stock Volcamelli es un sistema para llevar el inventario de una empresa con varias sucursales. Permite ver qué productos hay, cuánto stock tiene cada sucursal y mover mercadería de una sucursal a otra.

## Qué resuelve

En una empresa con varias sucursales, la misma mercadería está repartida y se mueve todo el tiempo. Queremos que en cualquier momento se sepa cuánto hay disponible de cada producto en cada sucursal, y que una transferencia nunca deje el stock mal: que no se mande más de lo que hay, que no quede stock negativo y que, si el traslado no se puede hacer, la mercadería siga disponible en el origen.

## Flujo principal

El flujo principal es la transferencia entre sucursales:

1. Un operador elige producto, cantidad, sucursal de origen y sucursal de destino.
2. Las unidades quedan reservadas en el origen: siguen ahí, pero nadie más las puede usar. Si el origen no tiene suficiente, la transferencia se rechaza.
3. En segundo plano le pedimos el traslado a Rumbo, el servicio de transporte de otro grupo.
4. Si Rumbo acepta, el stock pasa del origen al destino. Si rechaza, la reserva se libera y las unidades vuelven a estar disponibles en el origen.

## Cómo está armado

El sistema son cuatro microservicios en Go detrás de un API Gateway, con un frontend en React:

| Servicio | Qué hace |
|---|---|
| Usuarios | Usuarios, login y roles |
| Sucursales | Sucursales y sus direcciones |
| Inventario | Productos, stock por sucursal, reservas y movimientos. Es lo que le ofrecemos al otro grupo |
| Transferencias | Transferencias entre sucursales e integración con Rumbo |

Usamos MySQL y MongoDB como bases, RabbitMQ para los eventos, NGINX para repartir la carga entre las dos instancias de Inventario, Solr para la búsqueda y Memcached como caché. El detalle está en [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Integración con otros grupos

- **Ofrecemos** consultas del catálogo y del stock disponible de Inventario. El contrato y cómo probarlo están en [docs/contracts/README.md](docs/contracts/README.md).
- **Usamos** Rumbo, el servicio de traslados de otro grupo, en cada transferencia.

## Cómo ejecutarlo

Los servicios todavía no están implementados. Lo que ya se puede levantar es el mock del contrato que ofrecemos, para probar las consultas sin el servicio real. Con Docker, desde la raíz del repositorio:

```bash
docker run --rm -p 4010:4010 -v "${PWD}/docs/contracts:/contrato" stoplight/prism:5 mock -h 0.0.0.0 /contrato/openapi.yaml
```

Cuando estén los servicios, todo el sistema se va a levantar con `docker compose up`, y cuando Inventario esté desplegado, vamos a agregar acá su URL pública.

## Documentación

| Documento | Qué tiene |
|---|---|
| [SPEC.md](SPEC.md) | Alcance, actores, reglas de negocio y criterios de aceptación |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Servicios, tecnologías, gateway, comunicación y flujo de transferencia |
| [docs/diagramas/contexto.md](docs/diagramas/contexto.md) | Diagrama de contexto |
| [docs/diagramas/contenedores.md](docs/diagramas/contenedores.md) | Diagrama de contenedores |
| [docs/adr/](docs/adr/) | Decisiones de arquitectura |
| [docs/contracts/README.md](docs/contracts/README.md) | Contrato de lo que ofrecemos y cómo levantar el mock |

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
└── servicios/
    ├── usuarios/
    ├── sucursales/
    ├── inventario/
    └── transferencias/
```
