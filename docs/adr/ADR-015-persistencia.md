# ADR-015 — Persistencia (D3)

## Estado
Aceptado. Reemplaza a [ADR-003](ADR-003-persistencia-inicial.md). Diseño actualizado con reservas; implementación pendiente.

## Por qué revisamos ADR-003
Para las bases relacionales pasamos de PostgreSQL a MySQL por la familiaridad del equipo con Go, GORM y el driver de MySQL. Ambos motores podían cubrir las transacciones necesarias. Con los servicios de [ADR-014](ADR-014-limites-de-servicios.md), elegimos MongoDB en Transferencias.

## Decisión
Cada servicio tiene su propio servidor de base de datos y es el único que lo usa.

| Servicio | Base | Qué guarda y cómo se consulta |
|---|---|---|
| Usuarios | MySQL | Usuarios, credenciales y roles `ADMIN`, `OPERATOR`, `INTEGRACION`; búsquedas por id/email único |
| Sucursales | MySQL | Sucursales con nombre y dirección; consulta por id |
| Inventario | MySQL | Productos, existencias, reservas y movimientos; por producto/sucursal e id de reserva |
| Transferencias | MongoDB | Documento por transferencia con estado, historial e id externo; por id, estado y sucursal/fecha |

Cada MySQL corre en su contenedor, con credenciales propias. Las dos instancias de Inventario usan el mismo MySQL del servicio. Esto aísla accesos y recursos, pero no elimina fallas del host compartido.

### Inventario
El stock almacena `cantidad_total` y `cantidad_reservada`; el disponible se calcula como su diferencia. MySQL con InnoDB debe hacer cumplir `UNIQUE(producto, sucursal)` y `CHECK(0 <= reservada AND reservada <= total)`. Se elegirá una versión de MySQL 8 que aplique esos CHECK.

Reservar bloquea la fila de stock y crea una reserva única por transferencia. Solo incrementa reservado si hay disponible suficiente. El estado de la reserva es `ACTIVA`, `CONFIRMADA` o `LIBERADA`; los dos últimos son finales.

Confirmar bloquea la reserva y las filas de origen/destino en orden estable. En **una sola transacción**:

1. Comprueba reserva activa, parámetros y decisión de aceptación recibida del servicio autorizado.
2. Resta cantidad de total y reservado en origen.
3. Suma cantidad al total en destino.
4. Cierra la reserva y registra salida y entrada, resultado idempotente y outbox.

Si falta la fila destino, se crea de manera segura bajo la restricción única antes de aplicar los cambios. Un deadlock o conflicto de unicidad aborta la transacción completa; el reintento usa la misma identidad y relee las reglas. No hay locks en memoria como única protección: deben funcionar entre dos instancias.

Liberar descuenta solo reservado en origen y cierra la reserva, con movimiento, resultado y outbox en la misma transacción. Si el pedido de reserva todavía no llegó, se registra una marca final `LIBERADA` para impedir una reserva tardía. Confirmar una liberada o liberar una confirmada devuelve conflicto sin modificar cantidades. Las salidas manuales no pueden reducir el total por debajo del reservado.

Además del stock hay registros de operaciones idempotentes (clave, tipo, huella, resultado), outbox y mensajes consumidos. Se deduplica por evento y por efecto de negocio (`transferencia + operación`), con restricciones únicas y transacción conjunta. **No hay una tabla con los estados de negocio de Transferencias ni una segunda copia de su historial.** Las reservas y resultados finales se conservan durante toda la evaluación; no se vencen por TTL.

### Transferencias
Cada transferencia es un documento con pedido, clave del solicitante, estado, versión, historial, decisión, fase a reconciliar, referencia de reserva, id que le dio Rumbo y eventos pendientes. Estado y outbox se guardan en una actualización atómica condicionada a la versión/estado anterior. Un índice único por usuario y clave de idempotencia evita dos transferencias para la misma intención.

Índices por estado y próxima revisión permiten recuperar trabajo después de un reinicio; por sucursal de origen y fecha permiten consultar el historial. Las escrituras del proceso deben ser durables antes de responder o enviar mensajes.

MongoDB permite leer el proceso como una unidad y guardar evidencia externa con forma variable. No depende de que Rumbo tenga una contrapropuesta u otros campos particulares: eso se adapta al contrato recibido. Una alternativa relacional también es viable, pero requiere coordinar transferencia, historial y outbox en varias tablas.

El historial guarda transiciones de negocio, no cada intento técnico. Los eventos publicados pueden depurarse después de confirmar su envío y conservar la evidencia necesaria; los pendientes nunca se eliminan. Se deberá controlar el tamaño del documento y definir archivado si crece el uso.

### Copias de lectura
Solr indexa catálogo y disponible por sucursal; Memcached cachea fichas de producto sin stock. Ambos son reconstruibles y no deciden reservas, ajustes ni confirmaciones. Las claves de caché y documentos de búsqueda no sustituyen MySQL. La sincronización y los desfases están definidos en [arquitectura](../ARCHITECTURE.md).

## Alternativas que descartamos
- **Seguir con PostgreSQL.** Cubría las mismas garantías, pero priorizamos herramientas conocidas por el equipo.
- **Guardar cantidades en Transferencias.** Duplicaría la propiedad del stock y haría más difícil una confirmación atómica.
- **Usar caché o índice para validar disponibilidad.** Pueden estar atrasados o ausentes.
- **Usar solo MySQL también para el proceso.** Es viable; se elige el documento por sus consultas como unidad, aceptando operar otro motor.

## Consecuencias
- Hay cuatro bases de negocio (tres MySQL y un MongoDB) y copias de lectura adicionales.
- No hay rollback global entre servicios: Inventario protege las cantidades y Transferencias reconcilia el proceso.
- Las reservas retenidas ante incertidumbre reducen disponibilidad; priorizamos no duplicar ni liberar unidades inseguramente.
- Hay costo de almacenamiento de movimientos, idempotencia y outbox; retención y mantenimiento deberán medirse.
