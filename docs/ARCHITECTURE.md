# Arquitectura de Stock Volcamelli

## 1. Descripción general

Stock Volcamelli lleva el inventario de una empresa con varias sucursales: qué productos hay, cuánto stock tiene cada sucursal y las transferencias de mercadería entre sucursales.

Lo armamos con microservicios. El frontend en React habla únicamente con un API Gateway, y el gateway reparte cada pedido a uno de estos cuatro servicios:

| Servicio | Qué hace | Base |
|---|---|---|
| Usuarios | Usuarios con su rol y su sucursal, y login | MySQL |
| Sucursales | Alta, modificación y consulta de sucursales, con su dirección | MySQL |
| Inventario | Catálogo de productos, stock por sucursal, reservas, movimientos y ajustes manuales | MySQL |
| Transferencias | Transferencias entre sucursales y pedido del traslado a Rumbo | MongoDB |

Cada servicio es dueño de sus datos y nadie lee la base de otro. Los servicios se comunican por HTTP cuando necesitan la respuesta en el momento, y por eventos en RabbitMQ cuando no.

Además nos integramos con otros dos grupos. A uno le ofrecemos consultar nuestro catálogo y stock (sección 8). Del otro usamos Rumbo, su servicio de traslados, en cada transferencia (sección 7).

Por encima de esto sumamos dos funcionalidades: un asistente en el frontend que opera el sistema conversando, y actualizaciones de las pantallas en tiempo real. Son tres servicios más (Asistente, servidor MCP y Tiempo Real), descriptos en la sección 11.

Los diagramas están en [diagramas/contexto.md](diagramas/contexto.md) y [diagramas/contenedores.md](diagramas/contenedores.md).

## 2. Tecnologías

| Parte | Qué usamos |
|---|---|
| Backend de los servicios | Go con Gin para la API HTTP y GORM para MySQL. Transferencias usa el driver oficial de MongoDB para Go |
| API Gateway | Go con Gin, hecho por nosotros |
| Frontend | React |
| Bases de datos | MySQL 8 (un contenedor por servicio) y MongoDB |
| Mensajería | RabbitMQ |
| Balanceo de Inventario | NGINX |
| Búsqueda del catálogo | Solr ([ADR-006](adr/ADR-006-busqueda.md)) |
| Caché | Memcached ([ADR-007](adr/ADR-007-cache.md)) |
| Observabilidad | Logs guardados con GORM en un MySQL de logs, Prometheus y Grafana ([ADR-011](adr/ADR-011-observabilidad.md)) |
| Asistente | Servidor MCP con el SDK oficial para Go y un modelo de lenguaje por API compatible con OpenAI ([ADR-018](adr/ADR-018-asistente.md)) |
| Pruebas de carga | Un generador de carga propio en Go, con un worker pool ([ADR-013](adr/ADR-013-capacidad-y-costos.md)) |
| Para levantar todo | Docker Compose |

Elegimos Go para todos los servicios y para el gateway, así todo el equipo trabaja con el mismo lenguaje y las mismas librerías.

## 3. API Gateway

Es la única puerta de entrada al sistema, tanto para nuestro frontend como para el grupo que consume nuestro catálogo. Lo hacemos nosotros en Go con Gin y no tiene lógica de negocio ni base de datos. Hace tres cosas:

1. **Valida el token.** Todos los pedidos, salvo el login, tienen que traer un JWT en `Authorization: Bearer <token>`. El gateway verifica la firma y que no esté vencido. Si falla, responde `401` y el pedido no llega a ningún servicio. El token lo emite Usuarios en el login.
2. **Controla los permisos por rol.** Si el rol no puede hacer esa operación, responde `403`. Por ejemplo, `INTEGRACION` solo puede usar las consultas del contrato, y los ajustes de stock no los puede hacer un `VENDEDOR`. Al servicio le pasa el id, el rol y la sucursal del usuario, sacados del token, en los headers `X-User-ID`, `X-User-Role` y `X-User-Branch`. El control por sucursal (que la transferencia pase por la sucursal del vendedor, por ejemplo) lo hace cada servicio, porque necesita los datos del pedido.
3. **Redirige cada pedido al servicio que corresponde**, según la ruta:

   | Ruta | Servicio |
   |---|---|
   | `/api/auth/*`, `/api/users/*` | Usuarios |
   | `/api/branches/{id}/stock`, `/api/branches/{id}/products/*` | Inventario (por NGINX) |
   | `/api/branches/*` (el resto) | Sucursales |
   | `/api/products/*` | Inventario (por NGINX) |
   | `/api/transfers/*` | Transferencias |
   | `/api/assistant/*` | Asistente |
   | `/api/events` | Tiempo Real (conexión que queda abierta) |

Inventario corre con dos instancias. Delante de ellas hay un NGINX interno que reparte los pedidos entre las dos, y si una falla, manda todo a la otra. Tanto el gateway como Transferencias llegan a Inventario a través de ese NGINX. El detalle está en [ADR-012](adr/ADR-012-balanceo-de-carga.md).

Las llamadas entre servicios no pasan por el gateway: van directo de un servicio al otro.

## 4. Los servicios

### Usuarios
Guarda los usuarios con su email, contraseña (hasheada), rol y sucursal. Hace el login y emite el JWT, que dura 1 hora y lleva el id del usuario, su rol y su sucursal. Si a un usuario le cambian el rol o la sucursal, se aplica cuando vuelve a hacer login. Hay cuatro roles:

| Rol | Sucursal | Qué puede hacer |
|---|---|---|
| `DIRECTOR` | Ninguna | Todo, en todas las sucursales: transferencias, ajustes, usuarios de cualquier rol, sucursales y productos |
| `ADMIN_SUCURSAL` | Una | Lo del vendedor, más ajustes de stock en su sucursal y alta de vendedores de su sucursal |
| `VENDEDOR` | Una | Consulta todo. Crea, cancela y responde los horarios de las transferencias que salen de su sucursal o llegan a ella |
| `INTEGRACION` | Ninguna | Es la cuenta del grupo que consume nuestro catálogo, y solo puede usar las consultas del contrato |

Todos los usuarios, salvo `INTEGRACION`, pueden ver la lista de usuarios de cada sucursal con su nombre, email y rol (`GET /api/users?branchId=3`). La contraseña nunca sale de Usuarios. Esa lista se guarda en caché ([ADR-007](adr/ADR-007-cache.md)).

### Sucursales
Guarda las sucursales con su nombre y dirección. La dirección hace falta porque Rumbo la pide para cada traslado. Los datos de las sucursales se guardan en caché, porque casi nunca cambian y los piden todas las pantallas y Transferencias en cada transferencia ([ADR-007](adr/ADR-007-cache.md)).

El frontend tiene una sección de sucursales que muestra cada una con sus datos y sus usuarios: los datos los trae de Sucursales y los usuarios, de Usuarios.

### Inventario
Es el servicio que publicamos. Guarda los productos, el stock de cada producto en cada sucursal y los movimientos (cada cambio de stock queda registrado, con su motivo).

Del stock guarda dos números: el total y lo reservado. El **disponible** es la diferencia, y es el que mostramos y el que se usa para decidir si una transferencia se puede hacer. Las unidades reservadas siguen estando en la sucursal, pero ya están comprometidas en una transferencia que todavía no llegó a destino.

Tiene tres tipos de operaciones:

- **Consultas** del catálogo y del stock, que usan nuestro frontend y el otro grupo.
- **Ajustes manuales** de stock, que hace el director o el admin de esa sucursal, por ejemplo cuando llega mercadería o hay una rotura.
- **Reservas**, que solo usa Transferencias: reservar unidades en el origen, confirmarlas cuando el traslado llega (se mueven al destino) y liberarlas si el traslado se rechaza, se cancela o no se realiza.

El stock nunca puede quedar negativo, y eso lo controlan tanto el código como la base.

### Transferencias
Guarda cada transferencia con su estado y su historial. Es el único servicio que habla con Rumbo: le pide los traslados, sigue cada uno hasta que termina y le reenvía las acciones de los usuarios (responder un horario propuesto o cancelar). El flujo completo está en la sección 7.

## 5. Arquitectura interna de cada servicio

| Servicio | Estilo | Por qué |
|---|---|---|
| Usuarios | Capas | Es casi todo alta, baja y consulta |
| Sucursales | Capas | Igual que Usuarios |
| Inventario | Capas | Tiene reglas (stock no negativo, reservas, transacciones), pero solo habla con su base, RabbitMQ y Solr |
| Transferencias | Hexagonal | Depende de tres cosas externas que pueden cambiar o fallar: Inventario, Sucursales y Rumbo |
| Asistente, servidor MCP y Tiempo Real | Sin capas | Son chicos, no tienen base ni reglas de negocio propias: reciben, llaman a otro y devuelven |

En capas, cada servicio se divide en controllers (reciben el HTTP), services (la lógica) y repositories (la base), y los repositories se usan a través de interfaces con un mock para los tests.

En Transferencias, la lógica de la transferencia (los estados y cuándo se confirma o se libera) queda en el centro y no conoce a nadie de afuera. Lo externo se conecta con adaptadores: uno para Rumbo, uno para Inventario, uno para Sucursales, uno para MongoDB y uno para RabbitMQ. Así, si Rumbo cambia su contrato, solo se toca su adaptador. Además, en los tests podemos reemplazar a Rumbo por uno falso.

La decisión completa, con la estructura de carpetas de cada servicio, está en [ADR-002](adr/ADR-002-arquitectura-interna.md).

## 6. Datos y comunicación

Cada MySQL corre en su propio contenedor, y las dos instancias de Inventario comparten el MySQL de Inventario. Transferencias guarda cada transferencia en MongoDB como un solo documento, con su historial de estados. El detalle está en [ADR-015](adr/ADR-015-persistencia.md).

Las comunicaciones entre servicios son:

| De | A | Cómo | Para qué |
|---|---|---|---|
| Transferencias | Sucursales | HTTP | Traer nombre y dirección de origen y destino |
| Transferencias | Inventario | HTTP, por NGINX | Reservar las unidades y, si falló, liberar |
| Inventario | Sucursales | HTTP | Validar la sucursal en un ajuste |
| Transferencias | Inventario | Eventos `TrasladoCompletado` y `TrasladoCancelado` (RabbitMQ) | Confirmar o liberar la reserva |
| Transferencias | Rumbo | HTTP, en segundo plano | Pedir el traslado, seguirlo, responder horarios y cancelar |
| Inventario | Su indexador | Evento `ProductoActualizado` (RabbitMQ) | Actualizar Solr |
| Inventario | Tiempo Real | Evento `StockActualizado` (RabbitMQ) | Actualizar las pantallas |
| Transferencias | Tiempo Real | Evento `TransferenciaActualizada` (RabbitMQ) | Actualizar las pantallas |
| Asistente | Servidor MCP | HTTP (MCP) | Ejecutar las herramientas que pide el modelo |
| Servidor MCP | Gateway | HTTP, con el token del usuario | Consultar y crear como lo haría el usuario |

Todas las llamadas HTTP internas tienen un timeout de 2 segundos. Cada servicio publica sus eventos apenas guarda el cambio. Inventario los procesa una sola vez aunque lleguen repetidos, los confirma con ACK manual y, si un mensaje no se puede procesar, lo manda a una cola de errores. El detalle está en [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md).

## 7. Flujo principal: transferencia entre sucursales

Cuando un usuario crea la transferencia, las unidades quedan reservadas en el origen. Pasan al destino recién cuando Rumbo informa que el traslado llegó. Si el traslado se rechaza, se cancela o no se realiza, la reserva se libera y el origen vuelve a tenerlas disponibles.

1. El usuario elige producto, cantidad, sucursal de origen y de destino. Si quiere, indica a qué hora quiere que llegue, con cuánta flexibilidad, y observaciones para el retiro o la entrega.
2. Transferencias le pide a Sucursales el nombre y la dirección de las dos sucursales.
3. Transferencias le pide a Inventario que reserve las unidades en el origen. Si no hay stock disponible suficiente, Inventario lo rechaza.
4. El usuario ve el resultado enseguida: la transferencia reservada o el motivo por el que no se pudo.
5. En segundo plano, Transferencias le pide el traslado a Rumbo.
6. Cada 60 segundos, Transferencias le consulta a Rumbo qué cambió:
   - si Rumbo propone otro horario y no se había pedido hora, lo aceptamos solos; si se había pedido una, la transferencia espera a que un usuario acepte o rechace la propuesta;
   - si Rumbo acepta, la transferencia queda programada con la hora comprometida;
   - mientras el camión viaja, mostramos que está en curso y, si hay, la demora.
7. Cuando el traslado llega, Transferencias publica `TrasladoCompletado` e Inventario pasa el stock: resta en el origen y suma en el destino. Si el traslado se rechaza, se cancela o no se realiza, publica `TrasladoCancelado` e Inventario libera la reserva.

Mientras el traslado no esté en curso, se puede cancelar la transferencia. Si falta poco para la llegada, Rumbo tiene que aprobar la cancelación.

Un ejemplo con 4 unidades, de una sucursal con 10 a otra con 2:

| Momento | Origen: total / reservado / disponible | Destino |
|---|---|---|
| Se crea la transferencia | 10 / 4 / 6 | 2 |
| Mientras el traslado está programado o en curso | 10 / 4 / 6 | 2 |
| Si el traslado llega | 6 / 0 / 6 | 6 |
| Si se rechaza, se cancela o no se realiza | 10 / 0 / 10 | 2 |

Los estados de una transferencia son:

| Estado | Qué significa |
|---|---|
| `SIN_STOCK` | El origen no tenía stock disponible suficiente. No se reservó nada |
| `FALLIDA` | Inventario no respondió a tiempo. Si llegó a reservar, la reserva se libera |
| `RESERVADA` | Las unidades están reservadas y esperamos que Rumbo revise el pedido |
| `ESPERANDO_CONFIRMACION` | Se había pedido una hora, Rumbo propuso otra y hay 24 horas para aceptarla o rechazarla |
| `PROGRAMADA` | Rumbo aceptó y hay una hora de llegada comprometida |
| `EN_CURSO` | El camión está viajando |
| `COMPLETADA` | El traslado llegó. Inventario pasa el stock al destino al procesar el evento |
| `RECHAZADA` | Rumbo rechazó el traslado, un usuario rechazó el horario o nadie lo respondió a tiempo. La reserva se liberó |
| `CANCELACION_PENDIENTE` | Un usuario pidió cancelar con poca anticipación y Rumbo todavía no lo aprobó |
| `CANCELADA` | La transferencia se canceló. La reserva se liberó |
| `NO_REALIZADA` | Rumbo no pudo hacer el traslado o lo interrumpió. La reserva se liberó |

## 8. Integración con otros grupos

**Lo que ofrecemos.** El otro grupo puede consultar el catálogo y el stock disponible de Inventario: listar productos, ver un producto, el stock de una sucursal y el stock de un producto en una sucursal. Entra por el gateway con la cuenta `INTEGRACION`. El contrato está en [contracts/README.md](contracts/README.md) y la decisión en [ADR-017](adr/ADR-017-contrato-propio.md).

**Lo que usamos.** De Rumbo usamos toda la funcionalidad de su contrato:
- pedir el token;
- pedir un traslado, con hora de llegada si el usuario la eligió;
- consultar qué cambió, tanto de la solicitud como del viaje;
- responder el horario que proponen;
- cancelar un traslado.

Solo le habla Transferencias, siempre en segundo plano, así que si Rumbo no responde nadie se queda esperando. Cómo traducimos sus estados y qué hacemos con cada error está en [ADR-009](adr/ADR-009-consumo-de-rumbo.md).

## 9. Observabilidad

Cada pedido tiene un único TrackID desde que entra al gateway. Lo genera el gateway, se lo pasa a los servicios en el header `X-Track-ID` y lo devuelve en la respuesta. Los servicios no generan uno propio: lo pasan igual en cada llamada a otro servicio y en los eventos de RabbitMQ.

Todos los servicios guardan sus logs con GORM en la tabla `app_logs` de un MySQL aparte, solo para logs. Cada fila tiene el TrackID, el servicio que la escribió (`stage`), el nivel y el mensaje, y se inserta en el momento. Grafana lee esa tabla con un usuario de solo lectura. La traza de un pedido son sus logs: filtrando por su TrackID se ve por qué servicios pasó, en qué orden, cuánto tardó cada paso y en cuál falló.

Cada servicio expone sus métricas en `GET /metrics`: el contador `requests_total` y el histograma `request_duration_seconds`, los dos por ruta y código de respuesta. Además, Usuarios y Sucursales cuentan los aciertos y fallos de la caché, Inventario cuenta los mensajes que manda a la cola de errores, y Transferencias cuenta los errores de Rumbo y las aperturas del circuit breaker.

Prometheus junta las métricas y Grafana las muestra en un tablero junto con los logs y las trazas, donde también están las alertas. El objetivo de servicio está puesto sobre lo que publicamos: 99 % de las consultas del contrato sin error y 95 % en menos de 500 ms. El detalle está en [ADR-011](adr/ADR-011-observabilidad.md).

## 10. Qué pasa cuando algo falla

Cada falla afecta solo lo que depende de esa pieza. Lo más importante, consultar stock y lo que publicamos, depende solo de Inventario y su MySQL:

- Todas las llamadas tienen timeout, y las de los usuarios no se reintentan solas: si algo falla, lo ve enseguida.
- Los procesos de fondo reintentan esperando cada vez más, y las operaciones se pueden repetir sin duplicar efectos.
- Con Rumbo usamos un circuit breaker: después de 5 fallas seguidas dejamos de llamarlo por 30 segundos.
- Si se cae Memcached, Usuarios y Sucursales leen de MySQL. Si se cae Solr, solo deja de andar la búsqueda. Si se cae Usuarios, los que ya tienen token siguen trabajando.

La tabla completa de qué sigue andando cuando se cae cada pieza está en [ADR-010](adr/ADR-010-resiliencia.md). Cuánta carga aguanta el sistema y cuánto cuesta tenerlo andando está en [ADR-013](adr/ADR-013-capacidad-y-costos.md).

## 11. Asistente y tiempo real

**Asistente.** El frontend tiene un chat donde el usuario pregunta en lenguaje natural ("¿qué productos tienen stock bajo en Nueva Córdoba?") o pide crear una transferencia. El servicio Asistente le manda la pregunta a un modelo de lenguaje junto con las herramientas disponibles, y las ejecuta a través de un servidor MCP. El servidor MCP no tiene IA: cada herramienta es una llamada a nuestra API por el gateway, con el token del usuario, así que el asistente puede hacer exactamente lo mismo que el usuario. Antes de crear una transferencia, el usuario la confirma en el chat. El detalle está en [ADR-018](adr/ADR-018-asistente.md).

**Tiempo real.** El servicio Tiempo Real consume de RabbitMQ los cambios de stock y de estado de las transferencias, y se los manda al frontend por una conexión que queda abierta (Server-Sent Events en `/api/events`). Así, cuando Rumbo informa que un traslado salió o llegó, la pantalla se actualiza sola. El detalle está en [ADR-019](adr/ADR-019-tiempo-real.md).

## 12. Decisiones de arquitectura

| Decisión | ADR vigente | Reemplazó a |
|---|---|---|
| D1 Límites de los servicios | [ADR-014](adr/ADR-014-limites-de-servicios.md) | [ADR-001](adr/ADR-001-limites-de-servicios.md) |
| D2 Arquitectura interna | [ADR-002](adr/ADR-002-arquitectura-interna.md) | — |
| D3 Persistencia | [ADR-015](adr/ADR-015-persistencia.md) | [ADR-003](adr/ADR-003-persistencia-inicial.md) |
| D5 Comunicación entre servicios | [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md) | [ADR-005](adr/ADR-005-comunicacion-inicial.md) |
| D6 Búsqueda | [ADR-006](adr/ADR-006-busqueda.md) | — |
| D7 Caché | [ADR-007](adr/ADR-007-cache.md) | — |
| D8 Contrato propio | [ADR-017](adr/ADR-017-contrato-propio.md) | [ADR-008](adr/ADR-008-contrato-de-integracion.md) |
| D9 Consumo de Rumbo | [ADR-009](adr/ADR-009-consumo-de-rumbo.md) | — |
| D10 Resiliencia | [ADR-010](adr/ADR-010-resiliencia.md) | — |
| D11 Observabilidad | [ADR-011](adr/ADR-011-observabilidad.md) | — |
| D12 Balanceo de carga | [ADR-012](adr/ADR-012-balanceo-de-carga.md) | — |
| D13 Capacidad y costos | [ADR-013](adr/ADR-013-capacidad-y-costos.md) | — |
| Asistente con MCP | [ADR-018](adr/ADR-018-asistente.md) | — |
| Actualizaciones en tiempo real | [ADR-019](adr/ADR-019-tiempo-real.md) | — |
