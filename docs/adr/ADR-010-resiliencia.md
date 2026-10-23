# ADR-010 — Resiliencia (D10)

## Estado
Aceptado.

## Contexto
El sistema tiene varias piezas que se pueden caer o ponerse lentas: cuatro servicios, tres MySQL, MongoDB, RabbitMQ, Solr, Memcached, NGINX y Rumbo, que además es de otro grupo y no controlamos. Si no hacemos nada, una pieza lenta puede trabar a las que la llaman, y esas a las suyas, hasta que se cae todo.

Queremos que cada falla afecte solo lo que depende de esa pieza, y que lo más importante (consultar stock y lo que le ofrecemos al otro grupo) dependa de la menor cantidad de piezas posible: solo de Inventario y su MySQL.

## Decisión

### Timeouts
Ninguna llamada espera para siempre:

| Llamada | Timeout |
|---|---|
| Entre nuestros servicios (HTTP) | 2 segundos |
| NGINX a Inventario | 1 segundo para conectar con una instancia; si no conecta, prueba la otra. Para la respuesta, el que trae NGINX: corta antes el timeout de 2 segundos de quien llama |
| Usuarios y Sucursales a Memcached | El que trae el cliente de Memcached para Go (500 ms) |
| Inventario a Solr | 2 segundos |
| Transferencias a Rumbo | 5 segundos |
| Consultas a MySQL y MongoDB | 2 segundos |

### Reintentos
- **Lo que pide el usuario no se reintenta solo.** Si una llamada interna falla mientras el usuario espera, le respondemos el error enseguida. Reintentar ahí solo alarga la espera y suma carga a una pieza que ya está mal.
- **Las consultas que pasan por NGINX** se reintentan una vez en la otra instancia de Inventario ([ADR-012](ADR-012-balanceo-de-carga.md)).
- **Los procesos de fondo sí reintentan** con backoff exponencial: hasta 4 intentos, esperando 1 segundo, después 2 y después 4 entre uno y otro. Si los 4 fallan, el proceso lo deja pendiente y lo vuelve a intentar en su próxima vuelta. Son: liberar una reserva después de un timeout y mandarle pedidos y respuestas a Rumbo. Con los mismos 4 intentos se reintenta publicar un evento (si fallan, queda en el log de error para publicarlo a mano), actualizar Solr (si fallan, el mensaje va a la cola de errores) y conectarse a RabbitMQ al arrancar cada servicio. Todos se pueden repetir sin efectos duplicados, porque usan el id de la transferencia o del evento ([ADR-016](ADR-016-comunicacion-entre-servicios.md), [ADR-009](ADR-009-consumo-de-rumbo.md)).
- **Los mensajes de RabbitMQ** que no se pueden procesar se rechazan sin reencolar y van a la cola de errores ([ADR-016](ADR-016-comunicacion-entre-servicios.md)).

### Circuit breaker con Rumbo
Rumbo es la única dependencia que no es nuestra, así que es la que más chances tiene de estar caída un buen rato. Para no insistir contra un servicio caído, Transferencias usa un circuit breaker en el adaptador de Rumbo:

- **Cerrado** (normal): las llamadas pasan.
- Después de **5 fallas seguidas** (timeout, error de red o `5xx`), se **abre**: durante **30 segundos** no llamamos a Rumbo. Los pedidos quedan guardados y se mandan después.
- Pasados los 30 segundos queda **medio abierto**: dejamos pasar una sola llamada de prueba, que siempre es la consulta de cambios (nunca un pedido nuevo). Si responde, se cierra; si falla, se abre otros 30 segundos.

Los errores de negocio (`422`, `409`) no cuentan como fallas: Rumbo está andando, el problema es el pedido.

### Qué pasa si se cae cada pieza
| Si se cae… | Qué sigue andando | Qué no |
|---|---|---|
| **Una instancia de Inventario** | Todo. NGINX manda los pedidos a la otra | — |
| **Las dos instancias o el MySQL de Inventario** | Login, sucursales y ver transferencias. Las transferencias en curso se siguen consultando a Rumbo, y sus eventos esperan en la cola | Stock, catálogo, búsqueda, contrato y crear transferencias: responden `503` |
| **Memcached** | Todo. Usuarios y Sucursales leen directo de MySQL ([ADR-007](ADR-007-cache.md)) | — |
| **Solr** | Todo menos la búsqueda | La búsqueda responde `502` ([ADR-006](ADR-006-busqueda.md)) |
| **RabbitMQ** | Todo lo que es HTTP | Los eventos no se pueden publicar: se reintentan y, si siguen fallando, quedan en el log de error para publicarlos a mano. Mientras tanto no se mueve stock de traslados terminados y la búsqueda no se actualiza |
| **Rumbo** | Todo. Las transferencias se crean y quedan `RESERVADA` | Nada avanza con Rumbo hasta que vuelve ([ADR-009](ADR-009-consumo-de-rumbo.md)) |
| **Transferencias o MongoDB** | Stock, catálogo, búsqueda, contrato, ajustes | Crear y ver transferencias: responden `503` |
| **Sucursales** | Stock, catálogo, contrato y ver transferencias | Crear transferencias y ajustes (necesitan validar la sucursal): responden `503` |
| **Usuarios** | Todo para quien ya tiene token: el gateway valida el JWT solo, con la firma, sin consultar a Usuarios | Iniciar sesión |
| **Asistente, servidor MCP o el proveedor del modelo** | Todo | El chat responde que el asistente no está disponible ([ADR-018](ADR-018-asistente.md)) |
| **Tiempo Real** | Todo | Las pantallas no se actualizan solas; el usuario recarga ([ADR-019](ADR-019-tiempo-real.md)) |
| **El MySQL de logs** | Todo | Los logs de ese rato no aparecen en Grafana; quedan solo en la consola de Docker ([ADR-011](ADR-011-observabilidad.md)). Si en vez de caerse se satura, los pedidos se demoran porque esperan su `INSERT`, y con mucha carga algunos pasan los 2 segundos ([ADR-013](ADR-013-capacidad-y-costos.md)) |
| **NGINX** | Todo lo que no es Inventario | Lo mismo que si se cayeran las dos instancias de Inventario |
| **Gateway** | Los procesos de fondo (Rumbo y eventos) | Todo lo que viene de afuera: el frontend y el otro grupo |

Cuando algo responde `503`, el frontend muestra "no disponible por el momento" solo en esa parte de la pantalla, y el resto sigue andando.

## Alternativas que descartamos
- **Reintentar automáticamente las llamadas del usuario** (lo decidimos en [ADR-016](ADR-016-comunicacion-entre-servicios.md)). Con un timeout de 2 segundos y tres intentos, el usuario podía esperar 6 segundos para recibir el mismo error, y en el caso de la reserva un reintento podía reservar dos veces.

## Consecuencias
- Con Rumbo caído, las transferencias se acumulan como `RESERVADA` y esas unidades no se pueden usar. Si pasa más de 24 horas, se rechazan y se liberan.
- Cuando el circuit breaker está abierto, nos enteramos de los cambios de Rumbo con hasta 30 segundos más de atraso.
- El gateway, NGINX y RabbitMQ son una sola instancia cada uno. Si se caen, se pierde una parte grande del sistema hasta que se vuelven a levantar.
- Los timeouts se validaron con las caídas de instancias ([ADR-012](ADR-012-balanceo-de-carga.md)) y con la prueba de carga ([ADR-013](ADR-013-capacidad-y-costos.md)): hasta unas 500 consultas por segundo ningún pedido llega a los 2 segundos. Por encima, los que los pasan son los que esperan al MySQL de logs.
