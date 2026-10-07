# ADR-001 — Límites de servicios y propiedad de datos

## Estado
Aceptado para la Entrega 1.

## Contexto
Stock Volcamelli administra usuarios, sucursales, inventario y notificaciones para una empresa con varias sucursales. La separación en servicios necesita límites de responsabilidad claros y un dueño único para cada conjunto de datos.

## Decisión
El sistema se organizará en cuatro microservicios de negocio y un API Gateway:

| Componente | Responsabilidad | Datos de los que es propietario |
|---|---|---|
| Servicio de Usuarios | Gestionar usuarios y roles (ADMIN, OPERATOR) | Usuarios y roles |
| Servicio de Sucursales | Crear, modificar y consultar sucursales | Sucursales |
| Servicio de Inventario | Gestionar productos, stock, movimientos y transferencias; aplicar reglas de inventario | Productos, stock, movimientos y transferencias |
| Servicio de Notificaciones | Crear y consultar notificaciones derivadas de cambios relevantes de stock | Notificaciones |
| API Gateway | Recibir solicitudes de clientes y enrutarlas a los servicios | No es dueño de datos del dominio |

Cada servicio administra su propia persistencia. Ningún servicio consulta o modifica directamente la base de datos de otro. Las dependencias se resuelven mediante interfaces o eventos documentados. Servicio de Inventario puede consultar Servicio de Sucursales para validar sucursales en operaciones que lo requieran y publica eventos que consume Servicio de Notificaciones.

## Justificación
Los límites separan capacidades de negocio y asignan un único responsable a cada dato. Servicio de Inventario mantiene juntas las reglas que preservan la consistencia del stock. Servicio de Notificaciones queda desacoplado del momento en que se registra un movimiento.

## Alternativas consideradas
- Aplicación monolítica con una base común: más simple al inicio, pero no refleja el requisito de arquitectura distribuida ni establece propiedad independiente de los datos.
- Un servicio por entidad: produciría más interacciones y límites demasiado pequeños para el alcance actual.
- Tres servicios: combinar notificaciones con inventario o usuarios con sucursales acoplaría responsabilidades con ciclos de cambio distintos.

## Consecuencias
- Cada servicio debe exponer una interfaz para los datos que posee.
- Las operaciones que cruzan servicios pueden fallar parcialmente y necesitan manejo explícito de errores.
- Una transferencia mantiene su consistencia dentro de Servicio de Inventario; las notificaciones se procesan después mediante eventos.
- La cantidad de servicios agrega trabajo operativo, aceptado como parte del objetivo académico.

## Alcance y revisión
Los límites son iniciales y podrán revisarse cuando se implementen casos de uso completos. Cualquier cambio debe conservar un dueño claro para cada dato y actualizar este ADR y los diagramas.
