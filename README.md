# Stock Volcamelli

Stock Volcamelli es un sistema distribuido orientado a empresas con múltiples sucursales que permite administrar y consultar el inventario de sus establecimientos.

El sistema permite gestionar usuarios, sucursales y productos, consultar el stock disponible en cada sucursal, realizar transferencias de productos entre establecimientos y generar notificaciones relacionadas con los niveles de stock.

## Dominio

El dominio del sistema es la gestión de inventario multisucursal.

La operación principal del dominio es la gestión y transferencia de productos entre sucursales, considerando las reglas de disponibilidad del stock y los niveles de inventario.

## Objetivo

El objetivo de Stock Volcamelli es facilitar la gestión del inventario de empresas con múltiples sucursales, permitiendo conocer el stock disponible en cada establecimiento y administrar los movimientos de productos entre ellos.

El sistema utiliza una arquitectura distribuida basada en microservicios, donde cada servicio es responsable de una parte específica del dominio y de sus propios datos.

## Arquitectura

Stock Volcamelli utiliza una arquitectura basada en microservicios.

El sistema cuenta con cuatro microservicios de negocio:

- Servicio de Usuarios
- Servicio de Sucursales
- Servicio de Inventario
- Servicio de Notificaciones

Los servicios se encuentran detrás de un API Gateway.

La comunicación entre los servicios podrá realizarse de forma síncrona o asíncrona según las necesidades de cada operación.

Cada microservicio es responsable de sus propios datos y no accede directamente a la base de datos de otro servicio.

La descripción detallada de la arquitectura se encuentra en [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Microservicios

### Servicio de Usuarios

Responsable de la gestión de usuarios y roles del sistema.

Roles principales:

- `ADMIN`
- `OPERATOR`

### Servicio de Sucursales

Responsable de la gestión de las sucursales de la empresa.

### Servicio de Inventario

Responsable de la gestión del inventario.

Incluye:

- Productos.
- Stock por sucursal.
- Movimientos de inventario.
- Transferencias entre sucursales.

El Servicio de Inventario será la capacidad publicada por el grupo para la integración con otro grupo. La capacidad publicada cubre la API funcional de Servicio de Inventario: gestión básica de productos, consulta de stock por sucursal, registro y consulta de movimientos, y transferencias entre sucursales. El contrato versionado enumera todas las operaciones. El contrato OpenAPI y las instrucciones del mock están en [docs/contracts/README.md](docs/contracts/README.md).

### Servicio de Notificaciones

Responsable de la gestión de las notificaciones relacionadas con el inventario.

Tipos principales de notificación:

- `LOW_STOCK`
- `OUT_OF_STOCK`

## Funcionalidades principales

Stock Volcamelli contempla las siguientes funcionalidades:

- Gestión de usuarios.
- Gestión de roles.
- Gestión de sucursales.
- Gestión de productos.
- Consulta de stock por producto y sucursal.
- Gestión del inventario.
- Transferencia de productos entre sucursales.
- Registro de movimientos de inventario.
- Generación de notificaciones relacionadas con los niveles de stock.
- Búsqueda de información del inventario.
- Comunicación síncrona y asíncrona entre servicios.
- Observabilidad del sistema.

## Reglas de negocio principales

Entre las principales reglas del dominio se encuentran:

1. Un usuario posee un rol dentro del sistema.
2. Un producto puede tener stock en distintas sucursales.
3. El stock de un producto se administra por sucursal.
4. Las transferencias se realizan entre una sucursal de origen y una sucursal de destino.
5. Una transferencia no puede superar el stock disponible en la sucursal de origen.
6. El stock no puede quedar en valores negativos.
7. Cuando el stock alcanza las 5 unidades se genera una notificación `LOW_STOCK`.
8. Cuando el stock llega a 0 unidades se genera una notificación `OUT_OF_STOCK`.

## Integración

El grupo publicará el `Servicio de Inventario` como capacidad de integración para que otro grupo pueda consumir funcionalidades relacionadas con el inventario.

La capacidad publicada contará con un contrato formal y versionado.

Los contratos de integración se documentarán en:

[docs/contracts/README.md](docs/contracts/README.md).

## Estructura del repositorio

El proyecto utiliza un único repositorio que contendrá todos los componentes del sistema. La estructura prevista es la siguiente; las carpetas de los servicios se crearán al comenzar su implementación.

```text
Stock-Volcamelli/
├── README.md
├── SPEC.md
├── docs/
│   ├── ARCHITECTURE.md
│   ├── adr/
│   ├── contracts/
│   │   └── openapi.yaml
│   └── diagramas/
├── gateway/
└── servicios/
    ├── servicio-usuarios/
    ├── servicio-sucursales/
    ├── servicio-inventario/
    └── servicio-notificaciones/
```

## Ejecución

En esta primera entrega el sistema todavía no es ejecutable: la entrega comprende el diseño y el contrato de integración. La implementación de los servicios comienza en la Entrega 2.

## Documentación

| Documento | Contenido |
|---|---|
| [SPEC.md](SPEC.md) | Alcance, actores, reglas de negocio y criterios de aceptación |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Arquitectura general, servicios, datos y comunicación |
| [docs/diagramas/contexto.md](docs/diagramas/contexto.md) | Diagrama de contexto |
| [docs/diagramas/contenedores.md](docs/diagramas/contenedores.md) | Diagrama de contenedores |
| [docs/adr/](docs/adr/) | Decisiones de arquitectura (D1, D3, D5 y D8) |
| [docs/contracts/README.md](docs/contracts/README.md) | Contrato de la capacidad publicada y cómo levantar su mock |
