# Arquitectura de Stock Volcamelli

## 1. Descripción general

Stock Volcamelli lleva el inventario de una empresa con varias sucursales: qué productos hay, cuánto stock tiene cada sucursal y las transferencias de mercadería entre sucursales.

Lo armamos con microservicios. El frontend en React habla únicamente con un API Gateway, y el gateway reparte cada pedido a uno de estos cuatro servicios:

| Servicio | Qué hace | Base |
|---|---|---|
| Usuarios | Usuarios, login y roles (ADMIN, OPERATOR e INTEGRACION) | MySQL |
| Sucursales | Alta, modificación y consulta de sucursales, con su dirección | MySQL |
| Inventario | Catálogo de productos, stock por sucursal, movimientos y ajustes manuales | MySQL |
| Transferencias | Transferencias entre sucursales y pedido del traslado a Rumbo | MongoDB |

Cada servicio es dueño de sus datos y nadie lee la base de otro. Los servicios se comunican por HTTP cuando necesitan la respuesta en el momento, y por eventos en RabbitMQ cuando no.

Además nos integramos con otros dos grupos. A uno le ofrecemos consultar nuestro catálogo y stock (sección 8) y del otro usamos Rumbo, su servicio de traslados, en cada transferencia (sección 7).

Los diagramas están en [diagramas/contexto.md](diagramas/contexto.md) y [diagramas/contenedores.md](diagramas/contenedores.md).

## 2. Tecnologías

| Parte | Qué usamos |
|---|---|
| Backend de los servicios | Go con Gin para la API HTTP y GORM para MySQL. Transferencias usa el driver oficial de MongoDB para Go |
| API Gateway | Go con Gin, hecho por nosotros |
| Frontend | React |
| Bases de datos | MySQL 8 (un contenedor por servicio) y MongoDB |
| Mensajería | RabbitMQ |
| Búsqueda del catálogo | Elasticsearch (Entrega 2, ver D6) |
| Caché | Redis (Entrega 2, ver D7) |
| Para levantar todo | Docker Compose |

Elegimos Go para todos los servicios y para el gateway, así todo el equipo trabaja con el mismo lenguaje y las mismas librerías.

## 3. API Gateway

Es la única puerta de entrada al sistema, tanto para nuestro frontend como para el grupo que consume nuestro catálogo. Lo hacemos nosotros en Go con Gin y no tiene lógica de negocio ni base de datos. Hace cuatro cosas:

1. **Valida el token.** Todos los pedidos, salvo el login, tienen que traer un JWT en `Authorization: Bearer <token>`. El gateway verifica la firma y que no esté vencido. Si falla, responde `401` y el pedido no llega a ningún servicio. El token lo emite Usuarios en el login.
2. **Controla los permisos por rol.** Por ejemplo, el rol `INTEGRACION` solo puede usar las consultas del contrato, y los ajustes de stock son solo para `ADMIN`.
3. **Redirige cada pedido al servicio que corresponde**, según la ruta:

   | Ruta | Servicio |
   |---|---|
   | `/api/auth/*`, `/api/users/*` | Usuarios |
   | `/api/branches/{id}/stock`, `/api/branches/{id}/products/*` | Inventario |
   | `/api/branches/*` (el resto) | Sucursales |
   | `/api/products/*` | Inventario |
   | `/api/transfers/*` | Transferencias |

4. **Limita la cantidad de pedidos** por usuario por minuto, para que un cliente no sature el sistema. Si se pasa, responde `429`. El número lo vamos a fijar con las pruebas de carga.

Inventario va a correr con dos instancias, y el gateway reparte los pedidos entre las dos. Cómo detecta que una instancia se cayó lo definimos en el ADR de D12.

Las llamadas entre servicios no pasan por el gateway: van directo de un servicio al otro.

## 4. Los servicios

### Usuarios
Guarda los usuarios con su email, contraseña (hasheada) y rol. Hace el login y emite el JWT, que dura 1 hora y lleva el id del usuario y su rol. Hay tres roles:

- `ADMIN`: puede hacer todo, incluso administrar usuarios y sucursales y hacer ajustes manuales de stock.
- `OPERATOR`: consulta el stock y crea transferencias.
- `INTEGRACION`: es la cuenta del grupo que consume nuestro catálogo, y solo puede consultar.

### Sucursales
Guarda las sucursales con su nombre y dirección. La dirección hace falta porque Rumbo la pide para cada traslado.

### Inventario
Es el servicio que publicamos. Guarda los productos, el stock de cada producto en cada sucursal y los movimientos (cada cambio de stock queda registrado, con su motivo). Tiene tres tipos de operaciones:

- **Consultas** del catálogo y del stock, que usan nuestro frontend y el otro grupo.
- **Ajustes manuales** de stock, que hace un ADMIN, por ejemplo cuando llega mercadería o hay una rotura.
- **Operaciones internas** que solo usa Transferencias: mover stock de una sucursal a otra, anular una transferencia y devolver el stock cuando Rumbo rechaza un traslado.

El stock nunca puede quedar negativo, y eso lo controlan tanto el código como la base.

### Transferencias
Guarda cada transferencia con su estado y su historial. Es el único servicio que habla con Rumbo. El flujo completo está en la sección 7.

## 5. Arquitectura interna de cada servicio

| Servicio | Estilo | Por qué |
|---|---|---|
| Usuarios | Capas | Es casi todo alta, baja y consulta |
| Sucursales | Capas | Igual que Usuarios |
| Inventario | Capas | Tiene reglas (stock no negativo, transacciones), pero solo habla con su base y con RabbitMQ |
| Transferencias | Hexagonal | Depende de tres cosas externas que pueden cambiar o fallar: Inventario, Sucursales y Rumbo |

En capas, cada servicio se divide en handlers (reciben el HTTP), servicios (la lógica) y repositorios (la base).

En Transferencias, la lógica de la transferencia (los estados y cuándo se revierte) queda en el centro y no conoce a nadie de afuera. Lo externo se conecta con adaptadores: uno para Rumbo, uno para Inventario, uno para Sucursales, uno para MongoDB y uno para RabbitMQ. Así, si Rumbo cambia su contrato, solo se toca su adaptador. Además, en los tests podemos reemplazar a Rumbo por uno falso.

La decisión completa va en el ADR de D2 (Entrega 2).

## 6. Datos y comunicación

Cada MySQL corre en su propio contenedor. Transferencias guarda cada transferencia en MongoDB como un solo documento, con su historial de estados y los eventos pendientes de publicar. El detalle está en [ADR-015](adr/ADR-015-persistencia.md).

Las comunicaciones entre servicios son:

| De | A | Cómo | Para qué |
|---|---|---|---|
| Transferencias | Sucursales | HTTP | Traer nombre y dirección de origen y destino |
| Transferencias | Inventario | HTTP | Mover el stock y, si hace falta, anular |
| Inventario | Sucursales | HTTP | Validar la sucursal en un ajuste |
| Transferencias | Inventario | Evento `TransferenciaRevertida` (RabbitMQ) | Devolver el stock cuando Rumbo rechaza |
| Transferencias | Rumbo | HTTP, en segundo plano | Pedir el traslado y ver si lo acepta |

Todas las llamadas HTTP internas tienen un timeout de 2 segundos. Los eventos se publican con outbox, Inventario los procesa una sola vez aunque lleguen repetidos y, si un mensaje falla 3 veces, va a una cola de errores. El detalle está en [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md).

## 7. Flujo principal: transferencia entre sucursales

La transferencia se hace en el momento en que el operador la crea. Rumbo después la aprueba o la rechaza; si la rechaza, se deshace.

1. El operador elige producto, cantidad, sucursal de origen y de destino, y la crea.
2. Transferencias le pide a Sucursales el nombre y la dirección de las dos sucursales.
3. Transferencias le pide a Inventario que mueva el stock. Inventario resta en origen y suma en destino en una sola transacción, y si no alcanza el stock, lo rechaza.
4. El operador ve el resultado enseguida.
5. En segundo plano, Transferencias le pide el traslado a Rumbo, sin indicar horario.
6. Cada 60 segundos, Transferencias le consulta a Rumbo qué cambió. Si Rumbo propone un horario, lo acepta automáticamente: para nosotros el horario no importa.
7. Si Rumbo acepta, la transferencia queda confirmada. Si la rechaza, Transferencias publica `TransferenciaRevertida` e Inventario devuelve el stock al origen.

Los estados de una transferencia son:

| Estado | Qué significa | Stock |
|---|---|---|
| `SIN_STOCK` | El origen no tenía stock suficiente | No se movió |
| `FALLIDA` | Inventario no respondió a tiempo; se anula en segundo plano | No se movió, o se devuelve |
| `REALIZADA` | El stock se movió y esperamos la respuesta de Rumbo | Movido |
| `CONFIRMADA` | Rumbo aceptó el traslado | Movido |
| `REVERTIDA` | Rumbo rechazó el traslado o no pudimos pedírselo en 24 horas | Devuelto al origen |
| `REVERSION_PENDIENTE` | Había que devolver el stock, pero el destino ya no lo tenía | Lo corrige un ADMIN con un ajuste |

Una vez que Rumbo acepta, no seguimos el viaje del camión: para el inventario la mercadería ya está en el destino.

## 8. Integración con otros grupos

**Lo que ofrecemos.** El otro grupo puede consultar el catálogo y el stock de Inventario: listar productos, ver un producto, el stock de una sucursal y el stock de un producto en una sucursal. Entra por el gateway con la cuenta `INTEGRACION`. El contrato está en [contracts/README.md](contracts/README.md) y la decisión en [ADR-017](adr/ADR-017-contrato-propio.md).

**Lo que usamos.** De Rumbo usamos solo cuatro operaciones:
- pedir el token;
- pedir un traslado;
- consultar qué cambió;
- aceptar el horario que proponen.

Solo le habla Transferencias, siempre en segundo plano, así que si Rumbo no responde nadie se queda esperando. Cómo lo manejamos en detalle va en los ADRs de D9 y D10 (Entrega 2).

## 9. Logs y métricas

Todos los servicios escriben logs en JSON con un `trace_id`. El gateway genera el `trace_id` y lo pasa a cada servicio en un header, así podemos seguir un mismo pedido por todos los servicios por los que pasa.

Cada servicio expone sus métricas en `GET /metrics`: cantidad de pedidos, tiempo de respuesta y errores. Además medimos algunas cosas del negocio:

| Servicio | Métrica |
|---|---|
| Transferencias | Transferencias creadas y cuántas terminan en cada estado (confirmadas, revertidas, fallidas) |
| Transferencias | Tiempo que tarda Rumbo en responder y errores al llamarlo |
| Inventario | Ajustes manuales hechos |
| Inventario | Eventos en la cola de errores |

Con qué herramienta juntamos y mostramos todo esto se decide en el ADR de D11 (Entrega 2).

## 10. Decisiones de arquitectura

| Decisión | ADR vigente | Reemplazó a |
|---|---|---|
| D1 Límites de los servicios | [ADR-014](adr/ADR-014-limites-de-servicios.md) | [ADR-001](adr/ADR-001-limites-de-servicios.md) |
| D3 Persistencia | [ADR-015](adr/ADR-015-persistencia.md) | [ADR-003](adr/ADR-003-persistencia-inicial.md) |
| D5 Comunicación entre servicios | [ADR-016](adr/ADR-016-comunicacion-entre-servicios.md) | [ADR-005](adr/ADR-005-comunicacion-inicial.md) |
| D8 Contrato propio | [ADR-017](adr/ADR-017-contrato-propio.md) | [ADR-008](adr/ADR-008-contrato-de-integracion.md) |
