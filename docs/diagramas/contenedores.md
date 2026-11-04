# Diagrama de contenedores

Muestra las piezas del sistema y cómo se comunican.

```mermaid
flowchart TB
    U["Director / Admin de sucursal / Vendedor"]
    C["Grupo consumidor<br/>con API key"]
    F["Frontend<br/>React"]
    G["API Gateway<br/>Go + Gin"]
    US["Usuarios<br/>Go, capas"]
    BS["Sucursales<br/>Go, capas"]
    TS["Transferencias<br/>Go, hexagonal"]
    N["NGINX"]
    I["Inventario<br/>Go, capas, 2 instancias"]
    A["Asistente<br/>Go"]
    MCP["Servidor MCP<br/>Go"]
    TR["Tiempo Real<br/>Go"]
    DU[("MySQL<br/>Usuarios")]
    DB[("MySQL<br/>Sucursales")]
    DI[("MySQL<br/>Inventario")]
    DT[("MongoDB<br/>Transferencias")]
    Q["RabbitMQ"]
    S[("Solr")]
    M[("Memcached")]
    R["Rumbo<br/>otro grupo"]
    LLM["Modelo de lenguaje<br/>proveedor externo"]

    U --> F --> G
    C -->|"Consultas con X-API-Key"| I
    G --> US
    G --> BS
    G --> TS
    G --> N
    G --> A
    G -->|"/api/events"| TR
    N --> I
    US --> DU
    BS --> DB
    I --> DI
    TS --> DT
    TS -->|"Nombre y dirección"| BS
    TS -->|"Reservar"| N
    I -->|"Validar sucursal en ajustes"| BS
    TS -->|"TrasladoCompletado / TrasladoCancelado<br/>TransferenciaActualizada"| Q
    I -->|"ProductoActualizado / StockActualizado"| Q
    Q --> I
    Q --> TR
    I -->|"Búsqueda del catálogo"| S
    US -->|"Caché de usuarios por sucursal"| M
    BS -->|"Caché de sucursales"| M
    TS -->|"Pedir, seguir, responder horario y cancelar"| R
    A -->|"Pregunta y herramientas"| LLM
    A -->|"Ejecutar herramientas"| MCP
    MCP -->|"Con el token del usuario"| G
```

## Qué es cada cosa

| Contenedor | Qué hace |
|---|---|
| Frontend | La aplicación web que usan el director, los admins de sucursal y los vendedores, con el chat del asistente |
| API Gateway | La única entrada. Valida el token, controla permisos y manda cada pedido a su servicio |
| Usuarios | Usuarios con su rol y su sucursal, y login |
| Sucursales | Sucursales con su dirección |
| Inventario | Catálogo, stock, reservas y movimientos. Corre en dos instancias |
| NGINX | Reparte los pedidos entre las dos instancias de Inventario |
| Transferencias | Transferencias y comunicación con Rumbo |
| Asistente | Recibe los mensajes del chat y coordina el modelo de lenguaje con las herramientas |
| Servidor MCP | Expone las operaciones del sistema como herramientas para el modelo. Cada una es una llamada al gateway |
| Tiempo Real | Le manda al frontend los cambios de stock y de transferencias apenas pasan |
| RabbitMQ | Lleva los eventos entre servicios |
| Solr | Índice para buscar en el catálogo |
| Memcached | Caché de los datos de cada sucursal y de sus usuarios |

Cada servicio usa solo su propia base. Solr y Memcached no son fuente de verdad: si se pierden, Solr se reconstruye desde el MySQL de Inventario y Memcached se vuelve a llenar desde los MySQL de Usuarios y Sucursales.

Los pedidos a Inventario pasan siempre por NGINX, vengan del gateway o de Transferencias. Las demás llamadas entre servicios van directo, sin pasar por el gateway, salvo las del servidor MCP: pasan por el gateway a propósito, para que el asistente tenga los mismos permisos que el usuario. A Rumbo solo le habla Transferencias.

Las piezas de observabilidad (el MySQL de logs, Prometheus y Grafana) no están en el diagrama para no cargarlo: leen los logs, las métricas y las trazas de todos los servicios ([ADR-011](../adr/ADR-011-observabilidad.md)).
