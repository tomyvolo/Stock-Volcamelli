# Arquitectura de Stock Volcamelli

## 1. Descripción general

Stock Volcamelli lleva el inventario de una empresa con varias sucursales: qué productos hay, cuánto stock tiene cada sucursal y las transferencias de mercadería entre sucursales.

Lo armamos con microservicios. El frontend en React habla únicamente con un API Gateway, y el gateway reparte cada pedido a uno de estos cuatro servicios:

| Servicio | Qué hace | Base |
|---|---|---|
| Usuarios | Usuarios, login y roles (`ADMIN`, `OPERATOR` e `INTEGRACION`) | MySQL |
| Sucursales | Alta, modificación y consulta de sucursales, con su dirección | MySQL |
| Inventario | Catálogo de productos, stock por sucursal, reservas, movimientos y ajustes manuales | MySQL |
| Transferencias | Transferencias entre sucursales y pedido del traslado a Rumbo | MongoDB |

Cada servicio es dueño de sus datos y nadie lee la base de otro. Los servicios se comunican por HTTP cuando necesitan la respuesta en el momento, y por eventos en RabbitMQ cuando no.

Además nos integramos con otros dos grupos. A uno le ofrecemos consultar nuestro catálogo y stock (sección 8). Del otro usamos Rumbo, su servicio de traslados, en cada transferencia (sección 7).

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
| Búsqueda del catálogo | Solr (detalle en el ADR de D6) |
| Caché | Memcached (detalle en el ADR de D7) |
| Para levantar todo | Docker Compose |

Elegimos Go para todos los servicios y para el gateway, así todo el equipo trabaja con el mismo lenguaje y las mismas librerías. RabbitMQ, NGINX, Solr y Memcached son las herramientas que usamos en las prácticas de la materia.

## 3. API Gateway

Es la única puerta de entrada al sistema, tanto para nuestro frontend como para el grupo que consume nuestro catálogo. Lo hacemos nosotros en Go con Gin y no tiene lógica de negocio ni base de datos. Hace cuatro cosas:

1. **Valida el token.** Todos los pedidos, salvo el login, tienen que traer un JWT en `Authorization: Bearer <token>`. El gateway verifica la firma y que no esté vencido. Si falla, responde `401` y el pedido no llega a ningún servicio. El token lo emite Usuarios en el login.
2. **Controla los permisos por rol.** Si el rol no puede hacer esa operación, responde `403`. Por ejemplo, `INTEGRACION` solo puede usar las consultas del contrato, y los ajustes de stock son solo para `ADMIN`.
3. **Redirige cada pedido al servicio que corresponde**, según la ruta:

   | Ruta | Servicio |
   |---|---|
   | `/api/auth/*`, `/api/users/*` | Usuarios |
   | `/api/branches/{id}/stock`, `/api/branches/{id}/products/*` | Inventario (por NGINX) |
   | `/api/branches/*` (el resto) | Sucursales |
   | `/api/products/*` | Inventario (por NGINX) |
   | `/api/transfers/*` | Transferencias |

4. **Limita la cantidad de pedidos** por usuario por minuto, para que un cliente no sature el sistema. Si se pasa, responde `429`. El número lo vamos a fijar con las pruebas de carga.

Inventario corre con dos instancias. Delante de ellas hay un NGINX interno que reparte los pedidos entre las dos, y si una falla, manda todo a la otra. Tanto el gateway como Transferencias llegan a Inventario a través de ese NGINX. El detalle va en el ADR de D12.

Las llamadas entre servicios no pasan por el gateway: van directo de un servicio al otro.

## 4. Los servicios

### Usuarios
Guarda los usuarios con su email, contraseña (hasheada) y rol. Hace el login y emite el JWT, que dura 1 hora y lleva el id del usuario y su rol. Hay tres roles:

- `ADMIN`: puede hacer todo, incluso administrar usuarios, sucursales y productos, y hacer ajustes manuales de stock.
- `OPERATOR`: consulta el stock y crea transferencias.
- `INTEGRACION`: es la cuenta del grupo que consume nuestro catálogo, y solo puede consultar.

### Sucursales
Guarda las sucursales con su nombre y dirección. La dirección hace falta porque Rumbo la pide para cada traslado.

### Inventario
Es el servicio que publicamos. Guarda los productos, el stock de cada producto en cada sucursal y los movimientos (cada cambio de stock queda registrado, con su motivo).

Del stock guarda dos números: el total y lo reservado. El **disponible** es la diferencia, y es el que mostramos y el que se usa para decidir si una transferencia se puede hacer. Las unidades reservadas siguen estando en la sucursal, pero ya están comprometidas en una transferencia que todavía no llegó a destino.

Tiene tres tipos de operaciones:

- **Consultas** del catálogo y del stock, que usan nuestro frontend y el otro grupo.
- **Ajustes manuales** de stock, que hace un ADMIN, por ejemplo cuando llega mercadería o hay una rotura.
- **Reservas**, que solo usa Transferencias: reservar unidades en el origen, confirmarlas cuando el traslado llega (se mueven al destino) y liberarlas si el traslado se rechaza, se cancela o no se realiza.

El stock nunca puede quedar negativo, y eso lo controlan tanto el código como la base.

### Transferencias
Guarda cada transferencia con su estado y su historial. Es el único servicio que habla con Rumbo: le pide los traslados, sigue cada uno hasta que termina y le reenvía las acciones del operador (responder un horario propuesto o cancelar). El flujo completo está en la sección 7.

## 5. Arquitectura interna de cada servicio

| Servicio | Estilo | Por qué |
|---|---|---|
| Usuarios | Capas | Es casi todo alta, baja y consulta |
| Sucursales | Capas | Igual que Usuarios |
| Inventario | Capas | Tiene reglas (stock no negativo, reservas, transacciones), pero solo habla con su base, RabbitMQ, Solr y Memcached |
| Transferencias | Hexagonal | Depende de tres cosas externas que pueden cambiar o fallar: Inventario, Sucursales y Rumbo |

En capas, cada servicio se divide en handlers (reciben el HTTP), servicios (la lógica) y repositorios (la base).

En Transferencias, la lógica de la transferencia (los estados y cuándo se confirma o se libera) queda en el centro y no conoce a nadie de afuera. Lo externo se conecta con adaptadores: uno para Rumbo, uno para Inventario, uno para Sucursales, uno para MongoDB y uno para RabbitMQ. Así, si Rumbo cambia su contrato, solo se toca su adaptador. Además, en los tests podemos reemplazar a Rumbo por uno falso.

La decisión completa, con la estructura de carpetas de cada servicio, está en [ADR-002](adr/ADR-002-arquitectura-interna.md).

## 6. Datos y comunicación

Cada MySQL corre en su propio contenedor, y las dos instancias de Inventario comparten el MySQL de Inventario. Transferencias guarda cada transferencia en MongoDB como un solo documento, con su historial de estados y los eventos pendientes de publicar. El detalle está en [ADR-015](adr/ADR-015-persistencia.md).

Las comunicaciones entre servicios son:

| De | A | Cómo | Para qué |
|---|---|---|---|
| Transferencias | Sucursales | HTTP | Traer nombre y dirección de origen y destino |
| Transferencias | Inventario | HTTP, por NGINX | Reservar las unidades y, si falló, liberar |
| Inventario | Sucursales | HTTP | Validar la sucursal en un ajuste |
| Transferencias | Inventario | Eventos `TrasladoCompletado` y `TrasladoCancelado` (RabbitMQ) | Confirmar o liberar la reserva |
| Transferencias | Rumbo | HTTP, en segundo plano | Pedir el traslado, seguirlo, responder horarios y cancelar |

Todas las llamadas HTTP internas tienen un timeout de 2 segundos. Los eventos se publican con outbox. Inventario los procesa una sola vez aunque lleguen repetidos y, si un mensaje falla 3 veces, va a una cola de errores. El detalle está en [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md).

## 7. Flujo principal: transferencia entre sucursales

Cuando el operador crea la transferencia, las unidades quedan reservadas en el origen. Pasan al destino recién cuando Rumbo informa que el traslado llegó. Si el traslado se rechaza, se cancela o no se realiza, la reserva se libera y el origen vuelve a tenerlas disponibles.

1. El operador elige producto, cantidad, sucursal de origen y de destino. Si quiere, indica a qué hora quiere que llegue, con cuánta flexibilidad, y observaciones para el retiro o la entrega.
2. Transferencias le pide a Sucursales el nombre y la dirección de las dos sucursales.
3. Transferencias le pide a Inventario que reserve las unidades en el origen. Si no hay stock disponible suficiente, Inventario lo rechaza.
4. El operador ve el resultado enseguida: la transferencia reservada o el motivo por el que no se pudo.
5. En segundo plano, Transferencias le pide el traslado a Rumbo.
6. Cada 60 segundos, Transferencias le consulta a Rumbo qué cambió:
   - si Rumbo propone otro horario y el operador no había pedido hora, lo aceptamos solos; si había pedido una, la transferencia espera a que el operador acepte o rechace la propuesta;
   - si Rumbo acepta, la transferencia queda programada con la hora comprometida;
   - mientras el camión viaja, mostramos que está en curso y, si hay, la demora.
7. Cuando el traslado llega, Transferencias publica `TrasladoCompletado` e Inventario pasa el stock: resta en el origen y suma en el destino. Si el traslado se rechaza, se cancela o no se realiza, publica `TrasladoCancelado` e Inventario libera la reserva.

Mientras el traslado no esté en curso, el operador puede cancelar la transferencia. Si falta poco para la llegada, Rumbo tiene que aprobar la cancelación.

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
| `ESPERANDO_CONFIRMACION` | El operador había pedido una hora, Rumbo propuso otra y el operador tiene 24 horas para aceptarla o rechazarla |
| `PROGRAMADA` | Rumbo aceptó y hay una hora de llegada comprometida |
| `EN_CURSO` | El camión está viajando |
| `COMPLETADA` | El traslado llegó. Inventario pasa el stock al destino al procesar el evento |
| `RECHAZADA` | Rumbo rechazó el traslado, el operador rechazó el horario o nadie lo respondió a tiempo. La reserva se liberó |
| `CANCELACION_PENDIENTE` | El operador pidió cancelar con poca anticipación y Rumbo todavía no lo aprobó |
| `CANCELADA` | La transferencia se canceló. La reserva se liberó |
| `NO_REALIZADA` | Rumbo no pudo hacer el traslado o lo interrumpió. La reserva se liberó |

## 8. Integración con otros grupos

**Lo que ofrecemos.** El otro grupo puede consultar el catálogo y el stock disponible de Inventario: listar productos, ver un producto, el stock de una sucursal y el stock de un producto en una sucursal. Entra por el gateway con la cuenta `INTEGRACION`. El contrato está en [contracts/README.md](contracts/README.md) y la decisión en [ADR-017](adr/ADR-017-contrato-propio.md).

**Lo que usamos.** De Rumbo usamos toda la funcionalidad de su contrato:
- pedir el token;
- pedir un traslado, con hora de llegada si el operador la eligió;
- consultar qué cambió, tanto de la solicitud como del viaje;
- responder el horario que proponen;
- cancelar un traslado.

Solo le habla Transferencias, siempre en segundo plano, así que si Rumbo no responde nadie se queda esperando. Cómo traducimos sus estados y qué hacemos con cada error está en [ADR-009](adr/ADR-009-consumo-de-rumbo.md).

## 9. Logs y métricas

Todos los servicios escriben logs en JSON con un `trace_id`. El gateway genera el `trace_id` y lo pasa a cada servicio en un header, así podemos seguir un mismo pedido por todos los servicios por los que pasa.

Cada servicio expone sus métricas en `GET /metrics`: cantidad de pedidos, tiempo de respuesta y errores. Además medimos algunas cosas del negocio:

| Servicio | Métrica |
|---|---|
| Transferencias | Transferencias creadas y cuántas terminan en cada estado |
| Transferencias | Tiempo que tarda Rumbo en responder y errores al llamarlo |
| Inventario | Unidades reservadas en este momento |
| Inventario | Mensajes en la cola de errores |

Con qué herramienta juntamos y mostramos todo esto se decide en el ADR de D11.

## 10. Decisiones de arquitectura

| Decisión | ADR vigente | Reemplazó a |
|---|---|---|
| D1 Límites de los servicios | [ADR-014](adr/ADR-014-limites-de-servicios.md) | [ADR-001](adr/ADR-001-limites-de-servicios.md) |
| D2 Arquitectura interna | [ADR-002](adr/ADR-002-arquitectura-interna.md) | — |
| D3 Persistencia | [ADR-015](adr/ADR-015-persistencia.md) | [ADR-003](adr/ADR-003-persistencia-inicial.md) |
| D5 Comunicación entre servicios | [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md) | [ADR-005](adr/ADR-005-comunicacion-inicial.md) |
| D8 Contrato propio | [ADR-017](adr/ADR-017-contrato-propio.md) | [ADR-008](adr/ADR-008-contrato-de-integracion.md) |
| D9 Consumo de Rumbo | [ADR-009](adr/ADR-009-consumo-de-rumbo.md) | — |
