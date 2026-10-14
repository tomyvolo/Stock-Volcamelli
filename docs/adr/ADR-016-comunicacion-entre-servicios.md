# ADR-016 — Comunicación entre servicios (D5)

## Estado
Aceptado. Reemplaza a [ADR-005](ADR-005-comunicacion-inicial.md). Diseño con reservas e incertidumbre explícita; implementación pendiente.

## Por qué revisamos ADR-005
El flujo de [ADR-014](ADR-014-limites-de-servicios.md) pasa por Transferencias, Sucursales, Inventario y Rumbo. Necesitamos definir tiempos, reintentos y mensajes fallidos. Un timeout no puede traducirse en “no se hizo”: el efecto puede haber ocurrido y la respuesta haberse perdido.

## Decisión

### Llamadas sincrónicas (HTTP)
Las usamos cuando el servicio necesita una respuesta para continuar.

| Quién llama | A quién | Para qué | Si falla |
|---|---|---|---|
| Transferencias | Sucursales | Validar origen/destino y obtener direcciones | No iniciar una nueva reserva; informar indisponibilidad |
| Transferencias | Inventario por NGINX | Reservar y consultar el resultado por id | Conservar el pedido como `RECONCILIACION_PENDIENTE` si el resultado es desconocido |
| Inventario | Sucursales | Validar sucursal en un ajuste | No aplicar el ajuste sin validación |

Cada llamada interna tiene hasta 2 segundos y respeta el deadline restante propagado con context. Presupuesto inicial de creación: 6 segundos en Transferencias para persistencia, consultas y reserva; el gateway deja un margen hasta 7 segundos. No se espera a Rumbo. Una operación durable aún pendiente responde con su id y estado (previsto `202`); si no se pudo persistir, se informa error sin afirmar aceptación.

No hay reintentos de escrituras en frontend o proxy con claves nuevas. El cliente reusa su clave por intención y puede consultar el estado. Inventario guarda `transferencia + operación + huella + resultado`; mismos parámetros recuperan el resultado, parámetros distintos producen conflicto. Retener esas claves durante toda la evaluación evita que un mensaje antiguo vuelva a producir efectos.

En el proceso de fondo, lecturas y operaciones internas idempotentes tienen hasta 3 intentos por ciclo (inicial y dos reintentos), con esperas de 1 y 2 segundos más jitter, timeout de 2 segundos por intento y presupuesto máximo de 10 segundos por ciclo. Después se persiste la próxima revisión a 60 segundos y se mantiene el estado pendiente. No se reintentan errores de validación, falta de stock, permisos o conflicto terminal. El servicio que inicia la llamada es el único dueño del reintento.

Si se perdió una respuesta de reserva/confirmación/liberación, se consulta Inventario por el mismo id. Un resultado ausente no prueba que una petición en vuelo no pueda ejecutarse: se reintenta la misma intención idempotente o, si corresponde un cierre seguro, se guarda la marca de liberación que bloquea reservas tardías. Nunca se inventa un fracaso definitivo por timeout.

El frontend y el grupo consumidor entran por el gateway con los roles `ADMIN`, `OPERATOR` o `INTEGRACION` según sus permisos. Las llamadas entre servicios no pasan por el gateway. Las rutas internas de reserva y reconciliación requieren identidad de servicio y no se publican al consumidor.

### Eventos (RabbitMQ)
Los eventos propios describen hechos ya registrados de nuestro proceso; no presuponen nombres ni formatos del proveedor.

| Evento | Publica | Consume | Efecto |
|---|---|---|---|
| `TrasladoAceptado` | Transferencias | Inventario | Confirmar reserva y ambos movimientos |
| `TrasladoRechazado` | Transferencias | Inventario | Liberar reserva |
| `SolicitudVencida` | Transferencias | Inventario | Liberar tras cierre seguro por plazo |
| `ReservaConfirmada` | Inventario | Transferencias | Cerrar como `CONFIRMADA` |
| `ReservaLiberada` | Inventario | Transferencias | Cerrar como `RECHAZADA` o `VENCIDA`, según decisión guardada |
| `ProductoActualizado` | Inventario | Trabajadores de Inventario | Actualizar Solr e invalidar ficha |
| `StockDisponibleActualizado` | Inventario | Indexador de Inventario | Actualizar disponible en Solr |

Metadatos mínimos: id de evento, tipo, fecha, id de agregado, revisión monotónica del agregado, id de correlación y contexto de traza; datos suficientes para identificar el efecto. En eventos del proceso se incluye transferencia y decisión. Los esquemas internos procesables se completarán en la implementación; no se cambia aquí la política de versionado del contrato público.

- **Outbox:** estado y evento juntos en MongoDB; cantidades/movimientos y evento juntos en la transacción MySQL. El relay revisa cada segundo, publica mensajes persistentes a colas durables y marca enviados solo después de confirmación del broker y verificación de enrutamiento. Si se cae entre publicar y marcar, puede duplicar. Si el broker falla, conserva el pendiente y reintenta con backoff hasta 60 segundos, sin borrar la intención.
- **Consumidores:** una cola por suscriptor lógico; dos instancias compiten dentro de la cola. ACK manual después del efecto durable. Inventario inserta id procesado y efecto en la misma transacción. Transferencias actualiza condición, historial y deduplicación de manera atómica en su documento. Repetir un resultado no agrega otra transición.
- **Mensajes tardíos:** el estado de reserva y la decisión guardada impiden efectos opuestos. No se descarta un evento útil solo por ser viejo; se comprueba qué efecto sigue pendiente. Los resultados contradictorios se registran y alertan sin cambiar stock.
- **Mensajes fallidos:** 3 intentos totales, con esperas de 1 y 2 segundos más jitter mediante colas de reintento; no reencolar inmediatamente en bucle. Payload inválido va a DLQ sin insistir. Un conflicto terminal se registra como incidente sin repetirlo.
- **DLQ:** alerta si hay mensajes; responsable inicial es el integrante a cargo del servicio consumidor. Conserva mensaje, causa e intentos durante la evaluación. Se inspecciona, corrige la causa y reprocesa con el mismo id; nunca se borran mensajes para simular recuperación.

El indexador usa ids/versiones y actualización condicional en Solr para tolerar reentrega; no puede hacer una transacción global MySQL+Solr. La invalidación de caché puede repetirse sin daño. [Arquitectura](../ARCHITECTURE.md) detalla sus desfases.

### Rumbo
Solo Transferencias habla con Rumbo, por HTTP y en segundo plano. **El contrato lo entrega el otro grupo**: están pendientes endpoints, credenciales, estados, idempotencia, consulta de resultados y posibles operaciones de cierre. No se inventan.

Presupuesto propuesto por intento externo: 5 segundos. Revisión tentativa cada 60 segundos; ambos se ajustan al contrato. Solo se reintentan altas si el proveedor garantiza idempotencia con la misma clave y parámetros. Si una creación da timeout y no hay esa garantía, se consulta/reconcilia; no se manda otra alta. Si tampoco hay consulta segura, queda pendiente de resolución con el proveedor. Las lecturas reintentables usan hasta 3 intentos por ciclo y backoff, con presupuesto de 20 segundos.

A las **24 horas desde la creación durable** se suspenden nuevas altas. Se libera solo si se prueba que no hubo ni podrá haber aceptación: ningún envío iniciado/posible, o cierre definitivo confirmado por el proveedor. Transferencias serializa esa decisión con sus trabajadores mediante estado y versión. Un trabajador que ya inició un envío deja el caso incierto hasta verificarlo; “no encontrado” no es prueba suficiente mientras haya envíos en vuelo.

Un timeout mantiene `RECONCILIACION_PENDIENTE` y la reserva, incluso después del plazo. Una aceptación comprobada, mientras no exista decisión final opuesta, guarda `CONFIRMACION_PENDIENTE` y publica el hecho. Un rechazo definitivo o vencimiento seguro guarda `LIBERACION_PENDIENTE`. Son decisiones mutuamente excluyentes; solo después del efecto de Inventario se llega a estado final. Respuestas duplicadas no producen efectos; una aceptación posterior a un cierre definitivo ya aplicado genera incidente y consulta al proveedor.

No se sigue el viaje físico. Las garantías del contrato externo que necesitamos confirmar están en [arquitectura](../ARCHITECTURE.md#8-integración-con-otros-grupos); D9 y D10 siguen pendientes.

## Alternativas que descartamos
- **Publicar después de guardar sin outbox.** La caída entre ambos pasos puede perder la intención.
- **Anular automáticamente por timeout.** Confunde resultado desconocido con operación fallida.
- **Liberar por TTL sin reconciliar.** Puede dejar un traslado aceptado sin unidades reservadas.
- **Acreditar destino antes de Rumbo.** Haría posible consumir stock que luego habría que quitar.

## Consecuencias
- Hay consistencia eventual entre el efecto de Inventario y el estado visible de Transferencias; las pantallas muestran pendientes de forma explícita.
- Las reservas inciertas pueden superar 24 horas; se alertan y reconcilian sin comprometer cantidades.
- Se retienen identificadores, outbox y resultados; habrá que medir crecimiento y definir mantenimiento.
- RabbitMQ y DLQ requieren operación y pruebas de recuperación.
- Si Sucursales está caído no se inician transferencias; una caída de Rumbo no impide consultar catálogo.
