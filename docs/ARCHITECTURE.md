# Arquitectura de Stock Volcamelli

## 1. Descripción general

Stock Volcamelli administra inventario multisucursal. El stock disponible es `total − reservado`; se reserva en origen y se acredita en destino solo después de la aceptación de Rumbo y la confirmación atómica de Inventario.

**Estado de esta entrega:** diseño, contrato OpenAPI y estructura inicial de carpetas. Todos los componentes y mecanismos de ejecución descriptos abajo están planificados; no hay servicios, pruebas de negocio ni despliegue funcionando.

| Servicio | Responsabilidad y datos propios | Base |
|---|---|---|
| Usuarios | Usuarios, credenciales y roles `ADMIN`, `OPERATOR`, `INTEGRACION` | MySQL |
| Sucursales | Sucursales y direcciones | MySQL |
| Inventario | Productos, existencias totales y reservadas, reservas, ajustes y movimientos | MySQL |
| Transferencias | Proceso, estados, historial, decisión, id del traslado en Rumbo y trabajo pendiente | MongoDB |

Cada servicio accede solo a su base. Las dos instancias de Inventario comparten la base **del mismo servicio**. Ningún otro servicio la consulta directamente. Solr y Memcached son copias derivadas administradas por Inventario.

Diagramas vigentes: [contexto](diagramas/contexto.md) y [contenedores](diagramas/contenedores.md).

## 2. Tecnologías

| Parte | Elección | Motivo |
|---|---|---|
| Backend y gateway | Go con Gin; GORM para MySQL y driver oficial de MongoDB | Continuidad con las prácticas e interfaces testeables |
| Frontend | React | Interfaz web común para las funcionalidades |
| Persistencia | MySQL 8 con InnoDB y MongoDB | Transacciones de cantidades; documento por proceso de transferencia |
| Mensajería | RabbitMQ | ACK manual, consumidores, reintentos y DLQ trabajados en clase |
| Búsqueda | Solr | Texto, filtros y ordenamiento vistos en la práctica 5 |
| Caché | Memcached | Cache-aside distribuida vista en la práctica 3, compartida entre instancias |
| Balanceo | NGINX | Round-robin de la práctica 7; detección pasiva de fallas |
| Inicio local | Docker Compose | Dependencias y configuración reproducibles |

Las versiones exactas e imágenes fijadas quedan para la implementación. MySQL debe soportar restricciones CHECK efectivas. Estas elecciones no prueban rendimiento: D13 deberá apoyarse en mediciones.

## 3. API Gateway y balanceo

El frontend y el grupo consumidor entran por el gateway. Valida firma, vencimiento, emisor y audiencia del JWT, emitido por Usuarios con vigencia de 1 hora. Aplica permisos por rol y control de tráfico; los servicios vuelven a autorizar las acciones sobre sus recursos. Se descartan headers de identidad aportados por el cliente y se propaga la identidad por el canal interno confiable.

| Ruta | Destino |
|---|---|
| `/api/auth/*`, `/api/users/*` | Usuarios |
| `/api/branches/{id}/stock`, `/api/branches/{id}/products/*` | NGINX → Inventario |
| `/api/branches/*` restante | Sucursales |
| `/api/products`, `/api/products/*` | NGINX → Inventario |
| `/api/transfers/*` | Transferencias |

Las rutas específicas de stock se resuelven antes que la ruta general de sucursales. El gateway no tiene base ni lógica de transferencias. Se prevé un límite por identidad y minuto; su valor se fija con carga. El exceso devuelve `429`; la falta de identidad válida, `401`; los permisos insuficientes, `403`.

**Ubicación:** cliente → gateway → NGINX interno → Inventario A/B. Transferencias también llama al NGINX interno para acceder a Inventario, sin volver al gateway. Los otros servicios se llaman directamente por sus nombres de red de Compose. Rumbo recibe llamadas únicamente de Transferencias.

NGINX distribuye en round-robin entre dos instancias sin estado local de negocio. Se planifica `max_fails=3` y `fail_timeout=30s`: tres fallas de comunicación en esa ventana excluyen temporalmente la instancia. La detección es pasiva, basada en tráfico; un healthcheck de Docker no retira por sí solo un servidor del upstream. Tras el intervalo se vuelve a intentar y se registra su recuperación. Las verificaciones activas de readiness sirven al monitoreo, sin atribuirlas al balanceador.

No se habilitan reintentos transparentes de escrituras en el proxy; Transferencias controla los reintentos idempotentes. Las lecturas pueden probar una segunda instancia dentro del presupuesto de tiempo. Se registrarán `upstream_addr`, estado, tiempos, `instance_id` y `trace_id` para demostrar distribución, exclusión y recuperación. Si fallan ambas instancias se informa indisponibilidad, nunca stock cero.

Referencia de la configuración pasiva: [módulo upstream de NGINX](https://nginx.org/en/docs/http/ngx_http_upstream_module.html).

## 4. Los servicios y las pantallas

### Usuarios
Administra login y los tres roles. `ADMIN` administra usuarios, sucursales y productos, ajusta stock y también opera. `OPERATOR` consulta y crea transferencias. `INTEGRACION` solo accede al contrato de lectura y su login; no accede a rutas internas.

### Sucursales
Administra nombre y dirección. Transferencias obtiene las direcciones por HTTP y guarda en su pedido los datos usados; el formato a enviar a Rumbo depende del contrato externo.

### Inventario
Es el único escritor de cantidades. Provee catálogo, stock disponible e historial; valida ajustes y ejecuta reservar, confirmar y liberar. Sus registros de idempotencia guardan clave, huella y resultado de la operación de inventario, no otra copia de los estados e historial de Transferencias. La reserva tiene su propio ciclo `ACTIVA / CONFIRMADA / LIBERADA`.

### Transferencias
Orquesta el proceso y es el único consumidor de Rumbo. Persiste el pedido antes de llamar a Inventario. Mantiene estado, historial, fase incierta, decisión final e id externo. Las decisiones se guardan por comparación de estado y versión para evitar que dos trabajadores elijan finales opuestos.

### Frontend
Habla únicamente con el gateway. Incluye login, administración según rol, catálogo/búsqueda, stock por sucursal, ajustes, movimientos y creación/seguimiento de transferencias. Rotula `quantity` como **stock disponible**. Si muestra total o reservado como detalle, los identifica por separado. La búsqueda indica su carácter eventual; la consulta puntual obtiene el disponible de MySQL. En un resultado desconocido muestra “verificando resultado” con el id para consultar, sin invitar a duplicar el pedido.

## 5. Arquitectura interna y estructura inicial

| Servicio | Estilo | Motivo |
|---|---|---|
| Usuarios | Capas | Login y administración con reglas localizadas |
| Sucursales | Capas | Gestión y consulta simples |
| Inventario | Capas | Servicios de aplicación concentran invariantes y transacciones; repositorios encapsulan SQL, Solr y Memcached |
| Transferencias | Hexagonal | Aísla estados y decisiones de Inventario, Sucursales, Rumbo, MongoDB y RabbitMQ |

En capas: handlers → servicios de aplicación → repositorios mediante interfaces. Las reglas no van en handlers. En Transferencias: dominio y aplicación definen puertos; adaptadores HTTP, MongoDB y RabbitMQ dependen de esos puertos. El núcleo no importa Gin, drivers ni tipos del proveedor. D2 deberá demostrar esta separación con código y pruebas.

Estructura inicial creada para la implementación; las carpetas contienen solo archivos `.gitkeep`:

```text
frontend/src/
gateway/{handlers,middleware,proxy}/
servicios/
  usuarios/{handlers,aplicacion,repositorios}/
  sucursales/{handlers,aplicacion,repositorios}/
  inventario/{handlers,aplicacion,repositorios,mensajeria}/
  transferencias/{dominio,aplicacion,puertos,adaptadores}/
```

| Componente | Dependencias iniciales |
|---|---|
| Frontend | Gateway |
| Gateway | Usuarios, Sucursales, Transferencias, NGINX interno |
| Usuarios | Su MySQL |
| Sucursales | Su MySQL |
| Inventario | Su MySQL, Sucursales, RabbitMQ, Solr y Memcached |
| Transferencias | Su MongoDB, Sucursales, Inventario por NGINX, RabbitMQ y Rumbo |
| NGINX | Dos instancias de Inventario |

## 6. Datos y comunicación

MySQL protege `0 ≤ reservado ≤ total`, unicidad por producto/sucursal y una reserva por transferencia. Las transacciones bloquean las filas afectadas en orden estable; también registran movimientos, resultados idempotentes y outbox. La confirmación mueve ambos totales y consume la reserva en una sola transacción. Una liberación modifica solo la reserva del origen. Detalles: [ADR-015](adr/ADR-015-persistencia.md).

MongoDB guarda cada transferencia, historial y eventos pendientes en un documento. Actualizar estado, versión y outbox es atómico dentro de ese documento; no existe una transacción global con MySQL. Se conserva el trabajo pendiente ante reinicios.

| Origen | Destino | Comunicación | Uso |
|---|---|---|---|
| Transferencias | Sucursales | HTTP | Validar y obtener direcciones |
| Transferencias | Inventario por NGINX | HTTP interno | Reservar y consultar resultado por id |
| Inventario | Sucursales | HTTP | Validar sucursal en ajustes |
| Transferencias | Rumbo | HTTP en segundo plano, sujeto a su contrato | Solicitar traslado y reconciliar resultado |
| Transferencias | Inventario | RabbitMQ: `TrasladoAceptado`, `TrasladoRechazado`, `SolicitudVencida` | Confirmar o liberar la reserva |
| Inventario | Transferencias | RabbitMQ: `ReservaConfirmada`, `ReservaLiberada` | Completar el estado con evidencia del efecto |
| Inventario | Sus trabajadores de indexación e invalidación | RabbitMQ: `ProductoActualizado`, `StockDisponibleActualizado` | Actualizar Solr e invalidar fichas |

Estos eventos son **internos propios**, no nombres de eventos de Rumbo. Cada suscriptor lógico tiene su cola y las instancias compiten dentro de ella. Hay entrega al menos una vez, outbox, ACK después del efecto durable, deduplicación y DLQ. [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md) define timeout, presupuesto, reintentos y respuestas tardías.

## 7. Flujo principal: transferencia con reserva

1. El operador envía producto, cantidad, origen y destino con una clave de idempotencia. Transferencias valida sucursales y persiste el pedido como `RESERVA_PENDIENTE`.
2. Inventario reserva en MySQL si alcanza el disponible; registra el resultado por id. Un rechazo por stock lleva a `SIN_STOCK`; un timeout, a `RECONCILIACION_PENDIENTE`.
3. Con reserva comprobada, pasa a `PENDIENTE_RUMBO`. Un trabajador persiste la intención de envío y consulta al proveedor según el contrato que entregue su grupo.
4. Ante aceptación comprobada, Transferencias guarda `CONFIRMACION_PENDIENTE` y `TrasladoAceptado` en su outbox. Inventario consume, confirma atómicamente y publica `ReservaConfirmada`. Transferencias termina en `CONFIRMADA`.
5. Ante rechazo definitivo, guarda `LIBERACION_PENDIENTE` y `TrasladoRechazado`. Inventario libera y publica `ReservaLiberada`; Transferencias termina en `RECHAZADA`.
6. A las 24 horas desde la creación se cierran nuevos intentos de alta. Si no hubo envío posible o hay un cierre definitivo comprobado del proveedor sin aceptación, se publica `SolicitudVencida`, se libera y termina en `VENCIDA`.
7. Un resultado desconocido mantiene la reserva, también después del plazo. Se consulta el resultado y se alerta, sin interpretar “no encontrado” como garantía de que un pedido en vuelo no se ejecutará. Una aceptación comprobada antes de cerrar la decisión lleva a confirmar; un resultado contradictorio después del cierre se registra como incidente.

Los estados y transiciones completos están en [SPEC](../SPEC.md#5-entidades). Inventario no expira reservas por su cuenta. Transferencias serializa el vencimiento con los trabajadores de envío: primero cierra nuevos envíos, luego verifica los ya iniciados. Si no puede probar su cierre, conserva la incertidumbre. No hay compensación que quite stock ya disponible en destino.

Ejemplo: origen total 10/reservado 0 y destino total 2/reservado 0. Reservar 4 deja disponibles 6 y 2. Aceptar y confirmar deja totales 6 y 6, ambos sin reserva; rechazar y liberar deja totales 10 y 2. La confirmación depende de la aceptación del traslado, no de su entrega física.

## 8. Integración con otros grupos

**Publicamos** las consultas de catálogo y stock disponible por gateway con JWT. `ADMIN`, `OPERATOR` e `INTEGRACION` pueden consultar; solo el último es la cuenta del consumidor. [Contrato](contracts/README.md) y [ADR-017](adr/ADR-017-contrato-propio.md).

**Consumimos** Rumbo exclusivamente desde Transferencias y dentro del flujo principal. Su contrato lo define y entrega el otro grupo. Antes de implementar el adaptador necesitamos:

- URL del entorno, contrato formal y versión vigente, autenticación y renovación.
- Operación para solicitar traslado, campos obligatorios y formato de direcciones.
- Estados y significado de aceptación/rechazo definitivo; mecanismo de consulta o avisos.
- Identificador del traslado y forma de buscar un pedido cuya respuesta de creación se perdió.
- Garantías de idempotencia, alcance y retención de claves, y errores reintentables.
- Si permite cerrar/cancelar una solicitud y qué garantiza ante una aceptación concurrente o tardía.
- Límites de tráfico, tiempos y posibles propuestas de horario.

No damos por existentes login, polling de cambios, aceptación de horario ni cancelación del proveedor. El intervalo interno tentativo de reconciliación es 60 segundos, ajustable a su contrato. Si no ofrece deduplicación y consulta/cierre seguros, no reintentamos altas inciertas: dejamos el caso pendiente y lo resolvemos con ese grupo. El adaptador, el test de contrato y D9 se completan cuando recibamos el contrato.

## 9. Logs, métricas y tablero

Plan: logs JSON correlacionados con `trace_id`, `request_id`, servicio, instancia, operación y resultado. Propagar contexto de traza por HTTP y metadatos de eventos; las tareas de fondo enlazan su traza a la operación original. Ids de transferencia y evento sirven para buscar logs, no como etiquetas de métricas de alta cardinalidad. Nunca registrar credenciales o tokens.

Cada servicio expondrá métricas. El tablero deberá reunir tasas y errores HTTP, p95/p99 de latencia, disponibilidad por instancia, circuitos abiertos, antigüedad de reservas, resultados desconocidos, tiempo hasta estado final, profundidad/edad de colas, outbox pendiente, DLQ, retraso visible de Solr e impacto de caché.

Objetivos iniciales a validar: 95 % de lecturas de catálogo en menos de 500 ms bajo la carga de ensayo; índice visible en hasta 5 s con dependencias sanas; decisiones internas aplicadas en hasta 10 s con base y broker sanos (sin contar la espera a Rumbo). Alertas: DLQ no vacía, reserva incierta que supera 24 h, índice atrasado más de 5 s y ausencia de instancias listas. D11 fijará herramientas, ventanas y umbrales finales. Se exige una traza inicial para Entrega 2 y tablero, trazas y alerta demostrados para la defensa.

## 10. Decisiones de arquitectura

| Decisión | ADR vigente | Antecedente reemplazado |
|---|---|---|
| D1 Límites | [ADR-014](adr/ADR-014-limites-de-servicios.md) | [ADR-001](adr/ADR-001-limites-de-servicios.md) |
| D3 Persistencia | [ADR-015](adr/ADR-015-persistencia.md) | [ADR-003](adr/ADR-003-persistencia-inicial.md) |
| D5 Comunicación | [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md) | [ADR-005](adr/ADR-005-comunicacion-inicial.md) |
| D8 Contrato propio | [ADR-017](adr/ADR-017-contrato-propio.md) | [ADR-008](adr/ADR-008-contrato-de-integracion.md) |

Las correcciones de esta revisión se incorporan a esos cuatro ADR. D2, D4, D6, D7, D9, D10, D11, D12 y D13 todavía no tienen ADR propio; sus bases y evidencia pendiente quedan aquí. No se renumeran ni se borran antecedentes.

## 11. Búsqueda con Solr (D6 pendiente)

Inventario administra dos proyecciones: `catalogo`, un documento por producto (id, nombre, descripción, proveedor, versión), y `stock_sucursal`, un documento por par producto/sucursal con esos datos, ids, disponible, versión y fecha de origen. La segunda contiene solo pares con existencias registradas, incluidos los de disponible cero; no inventa pares sin registro. Sin sucursal se busca en catálogo; con sucursal, en su proyección, sin duplicar productos en una página.

La búsqueda interna prevista es `GET /api/products/search`, para `ADMIN` y `OPERATOR`; se resuelve antes de `/{productId}`. No se agrega al contrato público actual. Acepta:

| Parámetro | Regla |
|---|---|
| `q` | Texto en nombre y descripción; vacío permite listar con filtros |
| `supplier` | Proveedor exacto, opcional |
| `branchId` | Sucursal, opcional |
| `lowStock=true` | Requiere `branchId`; disponible entre 0 y 5 inclusive |
| `page`, `size` | Página desde 1; tamaño 1–100, por defecto 20 |
| `sort` | `name_asc`, `name_desc` o `relevance`; por defecto `name_asc` |

Paginación por desplazamiento y tamaño; devuelve resultados y total. Ordena con id como desempate; relevancia exige texto. Las páginas son sobre el índice del momento, no un snapshot entre pedidos. Filtros y orden se validan contra una lista permitida. La respuesta de búsqueda usa `quantity` como disponible solo si hay sucursal y señala la fecha de actualización. El cliente no envía sintaxis Solr libre. Consultar stock puntual o confirmar una operación siempre vuelve a MySQL.

**Sincronización:** la transacción de producto/reserva/liberación/confirmación/ajuste guarda eventos en outbox. El trabajador indexador de Inventario consume cambios, lee su fuente propia y reconstruye el documento afectado. Un cambio de producto actualiza catálogo y sus pares registrados. Los ids estables y versiones monotónicas impiden que un evento viejo sobrescriba uno nuevo; lectura y actualización condicional en Solr usan su control de concurrencia. Si hay conflicto se relee y reevalúa. El ACK llega después de la actualización; no se marca procesado antes del efecto externo.

**Frescura:** retraso máximo tolerable de **5 segundos desde el commit hasta que la búsqueda refleja el cambio**, incluyendo publicación, consumo y visibilidad del índice. Se mide con eventos y una comprobación de visibilidad, no solo con el éxito de escritura. Si se supera o no se puede determinar la frescura, la búsqueda con filtros afectados se declara temporalmente no disponible; catálogo básico y stock puntual siguen por MySQL.

**Reconstrucción:** cargar desde un snapshot consistente de MySQL en nuevos índices, retener y reproducir los cambios concurrentes mediante outbox/checkpoint y versiones, verificar conteos y ejemplos (incluidas reservas), alcanzar el punto de actualización y cambiar el destino de consulta. No borrar el índice anterior antes de validar. El procedimiento automatizado y sus pruebas son pendientes. Referencia técnica: [actualización y control de concurrencia de Solr](https://solr.apache.org/guide/solr/latest/indexing-guide/partial-document-updates.html).

## 12. Caché con Memcached (D7 pendiente)

Flujo elegido: `GET /api/products/{productId}`, la ficha del catálogo que se repite entre pantallas y consultas del grupo consumidor. Guarda nombre, descripción y proveedor, **sin cantidades ni permisos**. La necesidad a medir es la repetición de lecturas de fichas y su costo en MySQL; no se atribuyen mejoras antes de medirlas.

Cache-aside: autorizar → consultar `producto:ficha:v1:{id}` → hit devuelve ficha; miss consulta MySQL, almacena y devuelve. La respuesta es igual para los tres roles autorizados. No se cachean errores, login, stock, reservas ni búsquedas. TTL fijo de 60 s, sin renovarlo en hits; desfase tolerado de ficha hasta 60 s. No se sirve vencida.

Al confirmar una modificación de producto se invalida la clave; un evento del mismo outbox repite la invalidación si falla el borrado inmediato. El TTL cubre también la carrera entre una lectura antigua y la invalidación. Para no extender esa carrera, una carga usa como límite de vencimiento el instante inicial de su lectura más 60 s y descarta una carga que ya venció. La versión `v1` de la clave es interna al formato de caché, no cambia la política del contrato público.

Timeout de caché 50 ms. Error, vencimiento o expulsión llevan a MySQL, con pool y concurrencia acotados y agrupación de cargas simultáneas por clave dentro de cada instancia. Si la base no soporta el fallback, se informa indisponibilidad en vez de acumular trabajo ilimitado. Fallar al llenar la caché no convierte una lectura válida en error.

Comparar el mismo conjunto y tasa de pedidos con caché desactivada, fría y caliente: p50/p95/p99, throughput, errores, consultas/segundo y tiempo en MySQL, hits, misses, tasa de aciertos, errores/timeouts y expulsiones de Memcached. Registrar recursos, duración y carga; sin latencia artificial en la medición final. Probar actualización, TTL, caída y recuperación. Una ganancia que no compense la complejidad se registra como limitación en D7/D13.

## 13. Resiliencia, pruebas y puesta en marcha

| Dependencia que falla | Comportamiento previsto y evidencia |
|---|---|
| MySQL de Usuarios | Login falla explícitamente; tokens aún válidos se verifican según política vigente |
| MySQL de Sucursales / Sucursales | No iniciar transferencia ni ajuste que no pueda validarse; error reintentable |
| MySQL de Inventario / ambas instancias | No afirmar cantidades ni éxito; escrituras inciertas a reconciliación |
| MongoDB de Transferencias | No aceptar un pedido sin persistencia; retomar procesos guardados al recuperar |
| Rumbo | Circuit breaker y trabajadores acotados; conservar reserva y resultado desconocido |
| RabbitMQ | Outbox retiene eventos; estados pendientes visibles hasta reanudar |
| Solr | Búsqueda no disponible; lecturas puntuales y escrituras siguen contra MySQL |
| Memcached | Fallback acotado a MySQL; medir degradación |
| Una instancia de Inventario | NGINX la excluye por fallas; demostrar atención por la otra |

Circuit breaker propuesto por operación de Rumbo: abrir tras 5 fallas transitorias consecutivas, esperar 30 s y permitir una consulta segura de prueba; si falla, reabrir. No usar un alta incierta como prueba. Máximo inicial 4 trabajadores externos; ajustar con carga y contrato. Los errores de negocio no abren circuito. Los límites son decisiones iniciales, pendientes de validar en D10/D13.

Pruebas pendientes: unitarias de invariantes y transiciones (CA de SPEC), integración por servicio con su base real y con RabbitMQ para los consumidores, contrato del adaptador de Rumbo contra el contrato recibido, y carga sobre catálogo y reservas concurrentes por dos instancias. Medir cobertura y justificarla: la consigna no da un porcentaje concreto. Incluir pérdida de respuesta después del commit, confirmación/liberación duplicadas y vencimiento en carrera con aceptación.

La caída controlada tendrá hipótesis, tiempo de inicio, impacto, detección, respuesta, recuperación y acciones de mejora. Su evidencia irá a `docs/POSTMORTEM.md` después del ensayo; todavía no se crea un informe como si la prueba hubiera ocurrido.

Compose deberá crear dependencias, esquemas, índices, colas, datos de demo y configuración automáticamente, con healthchecks y secretos fuera de Git. La capacidad publicada requiere gateway/login e Inventario accesibles en una URL pública operativa hasta terminar la evaluación. El proveedor externo se configura por entorno.

## 14. Limitaciones y deuda técnica

No hay implementación ni evidencia operativa. Quedan pendientes el contrato de Rumbo, la URL pública, automatización de arranque, pruebas y observabilidad. El cierre seguro de pedidos externos inciertos depende de garantías que todavía no conocemos. Solr y caché aceptan los desfases declarados; ninguna escritura los usa como fuente de verdad.

NGINX y gateway únicos, las bases y el broker son puntos de falla: dos instancias de Inventario no garantizan alta disponibilidad de todo el sistema. El documento de Transferencias no debe crecer sin límite: se guardan transiciones de negocio, no cada intento técnico; eventos publicados se depuran de forma segura y los pendientes nunca se borran. Retención y archivado definitivo se justificarán antes de ampliar el uso.

**Observación separada sobre contrato:** se conserva la política vigente de ADR-017 y `info.version: 1.0.0`. La consigna exige contrato formal y versionado; la política actual de no manejar versiones sigue siendo un punto a resolver con el equipo/cátedra. Además, precisar `quantity` como disponible puede cambiar lo que un consumidor entendía por cantidad total; debe comunicarse y comprobarse con él antes de publicar. Esta revisión no redefine la política.

## 15. Base en la materia

Se usaron las fuentes locales de solo lectura: teóricos 1–8 (límites, datos, caché, asincronismo, búsqueda, gateway/balanceo, resiliencia y estilos) y prácticas 1–7 y TP integrador 1–4 (Go/Gin, interfaces, MongoDB, Memcached, RabbitMQ, Solr, concurrencia y NGINX). Se aplican transacciones locales, outbox, idempotencia y timeout como resultado desconocido. Las fuentes de clases no se modifican ni se copian como implementación del proyecto.

## 16. Cobertura de la consigna y evidencia pendiente

**Entrega 1:** README, SPEC, arquitectura y diagramas describen dominio, alcance, cuatro límites, propiedad de datos, estructura inicial y dependencias. ADR-014 y ADR-017 cubren D1 y D8; ADR-015 y ADR-016 cubren la versión inicial de D3 y D5. El contrato OpenAPI y el comando de Prism ya existían; se corrigió el significado del disponible. Las carpetas iniciales están creadas, pero todavía no hay código de servicios; no se ejecutó el mock porque Docker no estaba iniciado. El dominio aprobado, integrantes y acceso de la cátedra al repositorio requieren confirmación del equipo.

| Requisito posterior | Flujo previsto | Evidencia pendiente |
|---|---|---|
| Microservicios, gateway y frontend | Cuatro servicios, entrada única y pantallas de stock/transferencias | Rutas y flujo de punta a punta funcionando |
| Dos patrones internos | Capas y hexagonal | D2 y código con dependencias reconocibles |
| Almacenamientos y consistencia | MySQL, MongoDB, reserva y confirmación atómica | D4, bases reales y concurrencia entre dos instancias |
| Comunicación síncrona y asíncrona | HTTP interno y eventos de transferencia/stock | Outbox, consumidor idempotente, reintentos, DLQ y pruebas |
| Integración entre grupos | Catálogo/stock publicado y Rumbo consumido | Contrato de Rumbo, test de contrato, flujo real y URL pública |
| Búsqueda y caché | Solr y Memcached en Inventario | D6/D7, reconstrucción, frescura y comparación con/sin caché |
| Balanceo y resiliencia | NGINX sobre dos Inventarios; circuito y reconciliación de Rumbo | D10/D12, caída provocada, distribución y recuperación observadas |
| Observabilidad | Logs JSON, trazas, métricas y tablero | D11, primera traza en Entrega 2; tablero y alerta en defensa |
| Pruebas y capacidad | Reglas de SPEC, bases/broker reales, contrato externo, carga | Resultados unitarios, integración, contrato, carga, D13 y costos |
| Puesta en marcha y documentación | Compose automatizado y Gitflow por PR | Ejecución limpia, secretos externos, main evaluable y POSTMORTEM tras ensayo |

Para **Entrega 2 (23/10)** deben funcionar al menos un servicio, la capacidad propia, una persistencia, logs correlacionados y una primera traza; faltan D2, D6, D7, D9, D10, D12, D13 y una primera D11. Para la **presentación (11 o 13/11)** se requieren frontend, gateway, tres servicios, integración real, nube, dos almacenamientos, caché medida, evento idempotente con DLQ, búsqueda sincronizada y limitaciones explícitas. Para la **defensa individual** faltan pruebas, balanceo comprobado, carga con análisis, fallas controladas, tablero/alertas y el informe de incidente `docs/POSTMORTEM.md` después del ensayo.

La consigna no fija porcentaje de cobertura ni aporta la sección de formato ADR que menciona. Se conserva la estructura de los ADR vigentes y se justificará la calidad con resultados. Se mantiene como observación la política de versionado vigente del contrato frente al requisito de la consigna. Las garantías de Rumbo quedan pendientes de su contrato, que entrega el otro grupo.
