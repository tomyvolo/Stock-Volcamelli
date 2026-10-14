# Stock Volcamelli

Stock Volcamelli es un sistema para llevar el inventario de una empresa con varias sucursales. Permite consultar productos y stock, registrar ajustes y transferir mercadería entre sucursales.

## Qué resuelve

Queremos saber cuánto stock está disponible en cada sucursal y evitar que dos operaciones usen las mismas unidades. El **stock disponible es la cantidad total menos la cantidad reservada**. Esa es la cantidad que muestran las pantallas, las consultas que ofrecemos al otro grupo y el filtro de stock bajo (5 unidades disponibles o menos).

## Flujo principal

1. Un operador elige producto, cantidad, origen y destino.
2. Transferencias le pide a Inventario que reserve las unidades en origen. Si no hay disponible suficiente, se rechaza. El total no cambia y el destino todavía no recibe unidades.
3. Transferencias pide el traslado a Rumbo, el sistema de otro grupo, en segundo plano.
4. Si Rumbo acepta, Inventario confirma en una sola transacción: descuenta el total y la reserva del origen, suma el total del destino y registra los movimientos. Repetir el pedido no repite el efecto.
5. Si Rumbo rechaza, o vence el plazo de 24 horas y se puede comprobar que no hay una aceptación ni una solicitud que todavía pueda ejecutarse, Inventario libera la reserva. El destino no cambia.

Un timeout significa que el resultado es desconocido. La transferencia queda en reconciliación hasta comprobar qué ocurrió; no se libera stock ni se vuelve a crear un traslado a ciegas. La aceptación de Rumbo habilita la confirmación; el seguimiento físico del viaje queda fuera de alcance.

## Cómo está armado

El diseño contempla cuatro microservicios en Go con Gin, un API Gateway y un frontend en React:

| Servicio | Qué hace | Base propia |
|---|---|---|
| Usuarios | Usuarios, login y roles `ADMIN`, `OPERATOR` e `INTEGRACION` | MySQL |
| Sucursales | Sucursales y direcciones | MySQL |
| Inventario | Productos, existencias, reservas, ajustes y movimientos; único escritor de cantidades | MySQL |
| Transferencias | Proceso, estados, historial e identificador del traslado en Rumbo | MongoDB |

`ADMIN` administra y también opera; `OPERATOR` consulta y crea transferencias; `INTEGRACION` solo consulta la capacidad publicada. Cada servicio accede únicamente a su propia base.

RabbitMQ comunica los hechos del proceso. Solr permite buscar productos y filtrar stock bajo por sucursal. Memcached guarda temporalmente fichas del catálogo. NGINX se ubica entre el gateway y dos instancias de Inventario; también recibe las llamadas internas dirigidas a Inventario, sin hacerlas pasar por el gateway. Todo esto está **planificado, todavía no implementado**.

## Integración con otros grupos

- **Ofrecemos** consultas del catálogo y del stock disponible. El [contrato OpenAPI y su mock](docs/contracts/README.md) describen cómo consumirlas por el gateway con el rol `INTEGRACION`.
- **Usamos** Rumbo en cada transferencia. Su contrato lo proporciona el otro grupo. Las operaciones concretas y las garantías por confirmar están en [arquitectura](docs/ARCHITECTURE.md#8-integración-con-otros-grupos); no damos por existentes endpoints ni mecanismos de cancelación.

## Cómo ejecutarlo

Esta entrega contiene documentación, contrato y la estructura inicial de carpetas. No hay servicios ejecutables ni un sistema desplegado. Para probar las respuestas de ejemplo del contrato, con Docker desde la raíz:

```bash
docker run --rm -p 4010:4010 -v "${PWD}/docs/contracts:/contrato" stoplight/prism:5 mock -h 0.0.0.0 /contrato/openapi.yaml
```

El mock queda en `http://localhost:4010`. No demuestra reservas, autenticación real ni consistencia. Las instrucciones y limitaciones están en [el contrato](docs/contracts/README.md).

Para la implementación se prevé un único `docker compose up --build`, con bases, migraciones, datos de demostración, broker, índice, caché y observabilidad inicializados automáticamente. El archivo Compose, la configuración de ejemplo sin secretos y la URL pública de la capacidad propia están pendientes.

## Forma de trabajo: Gitflow

| Rama | Uso |
|---|---|
| `main` | Versiones estables y entrega evaluable |
| `develop` | Integración del trabajo del equipo |
| `feature/*` | Funcionalidades o documentación, desde `develop` hacia `develop` |
| `release/*` | Preparación de una entrega, desde `develop` |
| `hotfix/*` | Correcciones urgentes de una versión estable, desde `main` |

Todo cambio se integra mediante un pull request revisado por otro integrante. Una entrega se prepara en `release/*`, se verifican contrato, documentación y las pruebas disponibles, y se integra mediante PR a `main`. Sus correcciones vuelven también a `develop`. Los `hotfix/*` se integran en `main` y en `develop` (y en la release abierta si corresponde). La versión que evalúa la cátedra debe quedar en `main`. Falta aplicar esta estrategia y las protecciones de ramas en el repositorio remoto; describirla no prueba que ya se use.

## Documentación

| Documento | Contenido |
|---|---|
| [SPEC.md](SPEC.md) | Alcance, roles, entidades, estados, reglas y criterios de aceptación |
| [Arquitectura](docs/ARCHITECTURE.md) | Componentes, dependencias, flujo, búsqueda, caché y pendientes |
| [Contexto](docs/diagramas/contexto.md) y [contenedores](docs/diagramas/contenedores.md) | Diagramas del diseño vigente |
| [ADR](docs/adr/) | Vigentes: 014, 015, 016 y 017; los anteriores se conservan como historia |
| [Contrato](docs/contracts/README.md) | Operaciones publicadas, stock disponible, ejemplos y mock |

## Estructura del repositorio

Estructura inicial; las carpetas de código contienen solo archivos `.gitkeep` para que Git las conserve:

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
