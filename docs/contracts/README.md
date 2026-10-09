# Contrato del Servicio de Inventario

La capacidad publicada para la integración abarca la API funcional de Servicio de Inventario: productos, stock por sucursal, movimientos y transferencias. La especificación OpenAPI 3.1 está en openapi.yaml; los cuatro ADR están en ../adr/.

## Operaciones de v1

| Área | Método y ruta | Uso |
|---|---|---|
| Productos | GET /api/v1/products | Listar el catálogo |
| Productos | POST /api/v1/products | Crear un producto |
| Productos | GET /api/v1/products/{productId} | Consultar un producto |
| Productos | PATCH /api/v1/products/{productId} | Modificar nombre o descripción |
| Stock | GET /api/v1/branches/{branchId}/stock | Consultar existencias de una sucursal |
| Stock | GET /api/v1/branches/{branchId}/products/{productId}/stock | Consultar una existencia puntual |
| Movimientos | GET /api/v1/branches/{branchId}/products/{productId}/movements | Consultar el historial de movimientos |
| Movimientos | POST /api/v1/branches/{branchId}/products/{productId}/movements/adjustments | Registrar una entrada o salida |
| Transferencias | GET /api/v1/transfers | Listar transferencias |
| Transferencias | POST /api/v1/transfers | Transferir stock entre sucursales |
| Transferencias | GET /api/v1/transfers/{transferId} | Consultar una transferencia |

Las sucursales siguen siendo propiedad de Servicio de Sucursales. Servicio de Inventario solo recibe sus identificadores. Una transferencia válida descuenta en origen y acredita en destino como una operación atómica. El stock nunca puede ser negativo. Las operaciones POST para crear productos, registrar ajustes y transferir stock requieren Idempotency-Key. PATCH de producto asigna valores y puede repetirse con seguridad.

## Ejecutar el mock

El mock de Prism se deriva de esta misma especificación, usa los ejemplos declarados y no necesita código de servicio ni datos reales.

Requiere Docker. Desde la raíz del repositorio:

    docker run --rm -p 4010:4010 -v "${PWD}/docs/contracts:/contrato" stoplight/prism:5 mock -h 0.0.0.0 /contrato/openapi.yaml

El mock escucha en http://localhost:4010. Por ejemplo:

    curl http://localhost:4010/api/v1/branches/3/products/15/stock
    curl -H "Idempotency-Key: demo-ajuste-001" -H "Content-Type: application/json" -d "{\"direction\":\"INBOUND\",\"quantity\":10,\"reason\":\"Reposición\"}" http://localhost:4010/api/v1/branches/3/products/15/movements/adjustments

Las operaciones de escritura del mock muestran las respuestas de ejemplo; no modifican inventario real ni conservan estado entre llamadas.

## Versionado y compatibilidad

La versión mayor está en la ruta /api/v1 y la versión de la especificación figura en info.version. Agregar operaciones o campos opcionales dentro de v1 puede ser compatible. Cambiar tipos, quitar campos obligatorios o cambiar las reglas de una operación requiere una nueva versión mayor y una nueva ruta. Los consumidores deben ignorar campos nuevos que no reconozcan.

Este mock es una herramienta de diseño, no un servicio de producción. Antes de una exposición real se deben acordar autenticación, autorización, límites de uso y disponibilidad con los consumidores. La eliminación de productos queda fuera de v1 hasta definir cómo afecta el historial de movimientos.
