# Diagrama de contenedores

Muestra las piezas del sistema y cómo se comunican.

```mermaid
flowchart TB
    U["Administrador / Operador"]
    C["Grupo consumidor<br/>INTEGRACION"]
    F["Frontend<br/>React"]
    G["API Gateway<br/>Go + Gin"]
    US["Usuarios<br/>Go, capas"]
    BS["Sucursales<br/>Go, capas"]
    TS["Transferencias<br/>Go, hexagonal"]
    N["NGINX"]
    I["Inventario<br/>Go, capas, 2 instancias"]
    DU[("MySQL<br/>Usuarios")]
    DB[("MySQL<br/>Sucursales")]
    DI[("MySQL<br/>Inventario")]
    DT[("MongoDB<br/>Transferencias")]
    Q["RabbitMQ"]
    S[("Solr")]
    M[("Memcached")]
    R["Rumbo<br/>otro grupo"]

    U --> F --> G
    C -->|"Login y consultas"| G
    G --> US
    G --> BS
    G --> TS
    G --> N
    N --> I
    US --> DU
    BS --> DB
    I --> DI
    TS --> DT
    TS -->|"Nombre y dirección"| BS
    TS -->|"Reservar"| N
    I -->|"Validar sucursal en ajustes"| BS
    TS -->|"TrasladoCompletado / TrasladoCancelado"| Q
    Q --> I
    I -->|"Búsqueda del catálogo"| S
    I -->|"Caché"| M
    TS -->|"Pedir, seguir, responder horario y cancelar"| R
```

## Qué es cada cosa

| Contenedor | Qué hace |
|---|---|
| Frontend | La aplicación web que usan el administrador y el operador |
| API Gateway | La única entrada. Valida el token, controla permisos, limita pedidos y manda cada pedido a su servicio |
| Usuarios | Usuarios, login y roles |
| Sucursales | Sucursales con su dirección |
| Inventario | Catálogo, stock, reservas y movimientos. Corre en dos instancias |
| NGINX | Reparte los pedidos entre las dos instancias de Inventario |
| Transferencias | Transferencias y comunicación con Rumbo |
| RabbitMQ | Lleva los eventos de Transferencias a Inventario |
| Solr | Índice para buscar en el catálogo |
| Memcached | Caché de las consultas del catálogo |

Cada servicio usa solo su propia base. Solr y Memcached no son fuente de verdad: si se pierden, se reconstruyen desde el MySQL de Inventario.

Los pedidos a Inventario pasan siempre por NGINX, vengan del gateway o de Transferencias. Las demás llamadas entre servicios van directo, sin pasar por el gateway. A Rumbo solo le habla Transferencias.
