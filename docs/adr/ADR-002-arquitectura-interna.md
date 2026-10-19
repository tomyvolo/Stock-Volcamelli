# ADR-002 — Arquitectura interna de los servicios (D2)

## Estado
Aceptado.

## Contexto
Los cuatro servicios no tienen el mismo problema. Usuarios y Sucursales son casi todo alta, baja y consulta, con una caché adelante. Inventario tiene reglas importantes (el stock no puede quedar negativo, las reservas, las transacciones), pero todo lo que usa es suyo: su base, RabbitMQ y Solr. Transferencias, en cambio, coordina un proceso que depende de tres servicios que no controlamos del todo: Inventario, Sucursales y Rumbo, que además es de otro grupo y puede cambiar su contrato.

Queríamos usar en cada servicio el estilo más simple que alcance para su problema.

## Decisión

| Servicio | Estilo |
|---|---|
| Usuarios | Capas |
| Sucursales | Capas |
| Inventario | Capas |
| Transferencias | Hexagonal |

### Capas: Usuarios, Sucursales e Inventario
Cada servicio se divide en tres capas, y cada una solo llama a la de abajo:

1. **Controllers.** Reciben el pedido HTTP con Gin, leen los parámetros y el body, validan que tengan el formato correcto, llaman al service y devuelven el JSON. No tienen reglas de negocio ni tocan la base.
2. **Services.** Tienen las reglas de negocio y abren las transacciones. Por ejemplo, en Inventario, "no reservar más de lo disponible".
3. **Repositories.** Hablan con MySQL a través de GORM. Cada repositorio maneja una sola entidad.

Los services no usan los repositorios directamente, sino interfaces que no saben nada de MySQL. Cada interfaz tiene la implementación real y un mock que guarda los datos en memoria, para los tests. En `main.go` se arma todo: se crea el repositorio, se le pasa al service y el service al controller.

Inventario suma una pieza que se ubica igual que los controllers, porque es otra forma de entrar al servicio: el consumidor de RabbitMQ (recibe `TrasladoCompletado` y `TrasladoCancelado` y llama al service que confirma o libera). Después de cada cambio, el service publica los eventos en RabbitMQ a través de una interfaz, igual que usa los repositorios. Solr se usa desde la capa de repositorios, igual que MySQL.

```text
servicios/inventario/
├── main.go         → arranque: arma repositorios, services y controllers
├── controllers/    → HTTP, una carpeta por entidad (products, stock, adjustments)
├── consumers/      → consumidor de RabbitMQ
├── services/       → reglas de negocio
├── repositories/   → MySQL, Solr y la publicación en RabbitMQ, cada uno con su interfaz y su mock
└── models/         → structs de productos, stock, reservas y movimientos
```

Usuarios y Sucursales tienen la misma estructura, sin la carpeta de consumers. En los dos, Memcached se usa desde la capa de repositories: el repositorio con caché implementa la misma interfaz que el de MySQL y lo tiene adentro, así busca primero en la caché y, si no está, le pide el dato al de MySQL ([ADR-007](ADR-007-cache.md)).

### Hexagonal: Transferencias
En el centro está lo que es propio de la transferencia: sus estados, qué cambios de estado se permiten (por ejemplo, una transferencia `EN_CURSO` ya no se puede cancelar, y una `COMPLETADA` no cambia más) y los casos de uso, como crear una transferencia, responder un horario propuesto, cancelar o procesar los cambios que informa Rumbo. El centro no conoce Gin, ni el driver de MongoDB, ni el formato de Rumbo.

Lo que el centro necesita de afuera lo pide mediante **puertos**, que son interfaces definidas por el propio centro:

| Puerto | Para qué |
|---|---|
| Inventario | Reservar y liberar unidades |
| Sucursales | Traer nombre y dirección |
| Transporte | Pedir un traslado, consultar qué cambió, responder un horario propuesto y cancelar |
| Repositorio | Guardar y buscar transferencias |
| Eventos | Publicar los eventos |

Cada puerto tiene un **adaptador** que lo implementa: un cliente HTTP para Inventario, otro para Sucursales, otro para Rumbo, uno para MongoDB y uno para RabbitMQ. Al puerto lo llamamos "Transporte" y no "Rumbo" a propósito: si algún día cambiara el proveedor, se escribe otro adaptador y el centro queda igual.

Los adaptadores también pueden ser de entrada: los controllers HTTP que recibe el frontend y los procesos de fondo (el que le manda los pedidos a Rumbo, el que consulta cada 60 segundos y el que libera reservas después de un timeout). Todos llaman a los casos de uso del centro.

```text
servicios/transferencias/
├── main.go         → arranque: arma los adaptadores y se los pasa al centro
├── dominio/        → la transferencia, sus estados y reglas
├── aplicacion/     → casos de uso
├── puertos/        → interfaces que necesita el centro
└── adaptadores/
    ├── http/         → controllers de entrada
    ├── procesos/     → procesos de fondo
    ├── inventario/
    ├── sucursales/
    ├── rumbo/
    ├── mongo/
    └── rabbitmq/
```

### Gateway
El gateway no es un servicio de negocio, así que no sigue ninguno de los dos estilos. Es una cadena de middlewares (`track_id`, token y permisos) seguida del reenvío al servicio que corresponde.

### Asistente, servidor MCP y Tiempo Real
Tampoco siguen estos estilos. No tienen base ni reglas de negocio: el Asistente recibe un mensaje y coordina el modelo con las herramientas, el servidor MCP traduce cada herramienta a una llamada al gateway, y Tiempo Real pasa eventos de RabbitMQ al navegador. Separarlos en capas sería agregar carpetas sin nada adentro. Cada uno tiene un paquete con su lógica y otro para sus conexiones (el modelo, el gateway, RabbitMQ), para poder reemplazarlas por versiones falsas en los tests. Están descriptos en [ADR-018](ADR-018-asistente.md) y [ADR-019](ADR-019-tiempo-real.md).

## Alternativas que descartamos
- **Hexagonal en Inventario, como teníamos antes.** Lo habíamos elegido cuando Inventario también hacía las transferencias y tenía que hablar con otros servicios. Al separar Transferencias, esa parte se fue con ella: ahora Inventario solo usa dependencias propias y capas alcanza. El hexagonal pasó a Transferencias, que es donde quedaron las dependencias externas.

## Consecuencias
- En Inventario, tanto los pedidos HTTP como los eventos de RabbitMQ pasan por la misma capa de servicios. Así, las reglas (no reservar más de lo disponible, no liberar una reserva confirmada) se aplican igual sin importar por dónde entre la operación.
- Hay dos formas de organizar el código en el mismo repositorio, y todo el equipo tiene que conocer las dos.
- Transferencias tiene más archivos e interfaces que los demás. A cambio, el centro se prueba con adaptadores falsos, por ejemplo un Rumbo falso que acepta o rechaza, sin red ni base.
- Que nadie se saltee capas (un controller que use un repositorio directo, por ejemplo) no lo controla el compilador: lo revisamos entre nosotros antes de subir los cambios.
