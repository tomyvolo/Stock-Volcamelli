# Diagrama de contexto

Muestra quién usa Stock Volcamelli y con qué sistemas externos se comunica.

```mermaid
flowchart LR
    A["Administrador<br/>ADMIN"]
    O["Operador<br/>OPERATOR"]
    C["Grupo consumidor<br/>INTEGRACION"]
    SV["Stock Volcamelli"]
    R["Rumbo<br/>Servicio de traslados de otro grupo"]

    A -->|"Administra usuarios, sucursales y productos;<br/>hace ajustes y transferencias"| SV
    O -->|"Consulta stock, crea y sigue transferencias,<br/>responde horarios y cancela"| SV
    C -->|"Consulta catálogo y stock disponible"| SV
    SV -->|"Pide traslados, sigue el viaje,<br/>responde horarios y cancela"| R
```

- **Administrador y operador** usan el sistema desde el frontend.
- **El grupo consumidor** es otro sistema que consulta nuestro catálogo y stock con la cuenta `INTEGRACION`. Solo puede leer.
- **Rumbo** es el servicio de transporte de otro grupo. Le pedimos cada traslado y lo seguimos hasta que termina. Si llega, el stock pasa al destino. Si se rechaza, se cancela o no se realiza, vuelve a estar disponible en el origen.

El detalle por dentro está en el [diagrama de contenedores](contenedores.md).
