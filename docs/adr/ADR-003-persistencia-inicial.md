# ADR-003 — Persistencia inicial

## Estado
Aceptado para la Entrega 1 como versión inicial; se validará durante la implementación (Entrega 2).

## Contexto
Los servicios necesitan persistir información con necesidades distintas. Usuarios, sucursales e inventario manejan datos con relaciones y restricciones de integridad; en particular, una transferencia debe descontar stock en origen y acreditarlo en destino sin duplicar ni perder unidades. Las notificaciones, en cambio, se escriben una vez, se consultan por usuario o sucursal y casi no se modifican. El proyecto exige al menos una persistencia relacional y una no relacional.

## Decisión
Cada microservicio tendrá su propia base de datos y será el único que la lea y la escriba.

| Servicio | Almacenamiento | Motivo |
|---|---|---|
| Servicio de Usuarios | PostgreSQL (base propia) | Usuarios con email único y rol; integridad y consultas simples por clave |
| Servicio de Sucursales | PostgreSQL (base propia) | Datos estructurados, pocos registros y lecturas frecuentes por identificador |
| Servicio de Inventario | PostgreSQL (base propia) | Transacciones ACID, restricciones (stock ≥ 0, unicidad producto–sucursal) y bloqueo de filas para transferencias concurrentes |
| Servicio de Notificaciones | MongoDB | Documentos independientes, escritura por evento, consulta por destinatario y fecha, esquema que puede variar según el tipo de notificación |

Los tres servicios relacionales pueden compartir un mismo servidor PostgreSQL en el entorno local, pero cada uno usa una base y credenciales propias; ningún servicio tiene acceso a la base de otro.

## Justificación
- La transferencia es la operación que requiere garantías de consistencia. Al estar todo el stock dentro de la base de Servicio de Inventario, se resuelve con una única transacción local, sin coordinación distribuida.
- PostgreSQL permite expresar las reglas del dominio como restricciones de la base (CHECK de stock no negativo, claves únicas), lo que agrega una segunda barrera además de la validación en el código.
- Las notificaciones no tienen relaciones con otras entidades ni requieren transacciones entre documentos; un almacenamiento documental se ajusta a su patrón de escritura por evento y lectura por destinatario, y cumple el requisito de persistencia no relacional con un caso de uso real.
- La propiedad independiente de los datos sostiene los límites definidos en ADR-001.

## Alternativas consideradas
- Una base compartida por todos los servicios: descartada porque permite acoplamiento por esquema y acceso directo a datos ajenos.
- MongoDB para Servicio de Inventario: descartada porque la transferencia necesita actualizar varios registros de forma atómica con restricciones de integridad, algo más natural y verificable en una base relacional.
- PostgreSQL también para Servicio de Notificaciones: viable, pero no aporta ventajas para documentos independientes y dejaría sin un caso real la persistencia no relacional.
- Una base en memoria (por ejemplo Redis) para notificaciones: descartada como almacenamiento principal por las garantías de durabilidad; se podrá evaluar como caché (D7).

## Consecuencias y temas pendientes
- Los servicios no compartirán tablas ni credenciales de escritura.
- Las operaciones que cruzan servicios no usarán transacciones distribuidas; si un caso real lo requiere se definirán flujos de compensación.
- El equipo deberá operar dos motores distintos, lo que suma configuración al procedimiento de puesta en marcha local.
- El motor de búsqueda (D6) y la caché (D7) se decidirán en sus propios ADR y no reemplazan a estas persistencias como fuente de verdad.
- Durante la implementación se documentarán versiones, migraciones de esquema y datos iniciales de prueba.
