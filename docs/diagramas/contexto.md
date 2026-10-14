# Diagrama de Contexto

Diseño planificado para Stock Volcamelli. El grupo consumidor usa nuestro catálogo y stock; Rumbo es el proveedor de traslados. Su contrato lo entrega el otro grupo.

```mermaid
flowchart LR
    A["Administrador<br/>ADMIN"]
    O["Operador<br/>OPERATOR"]
    C["Grupo consumidor<br/>INTEGRACION"]
    SV["Stock Volcamelli<br/>Inventario multisucursal"]
    R["Rumbo<br/>Proveedor externo de traslados"]

    A -->|"Administra, ajusta y transfiere"| SV
    O -->|"Consulta y crea transferencias"| SV
    C -->|"Login, catálogo y stock disponible"| SV
    SV -->|"Transferencias solicita traslado<br/>y reconcilia el resultado"| R
    R -->|"Aceptación o rechazo<br/>según contrato pendiente"| SV
```

`ADMIN` administra usuarios, sucursales, productos y ajustes, además de operar. `OPERATOR` consulta inventario y crea transferencias. `INTEGRACION` solo lee las consultas del contrato.

El disponible es total menos reservado. La aceptación de Rumbo habilita la confirmación contable por Inventario; no seguimos el viaje físico. Las llamadas al proveedor salen de Transferencias. La interfaz y el consumidor entran por el API Gateway; los detalles internos están en el [diagrama de contenedores](contenedores.md).
