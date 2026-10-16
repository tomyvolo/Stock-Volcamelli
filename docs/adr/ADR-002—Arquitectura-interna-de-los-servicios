# ADR-002 — Arquitectura interna de los servicios (D2)

## Estado
Aceptado.

## Contexto
Los cuatro servicios no tienen el mismo problema. Usuarios y Sucursales son casi todo alta, baja y consulta. Inventario tiene reglas importantes (el stock no puede quedar negativo, las reservas, las transacciones), pero todo lo que usa es suyo: su base, RabbitMQ, Solr y Memcached. Transferencias, en cambio, coordina un proceso que depende de tres servicios que no controlamos del todo: Inventario, Sucursales y Rumbo, que además es de otro grupo y puede cambiar su contrato.

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

1. **Handlers.** Reciben el pedido HTTP con Gin, validan que los datos tengan el formato correcto y devuelven el JSON. No tienen reglas de negocio ni tocan la base.
2. **Servicios.** Tienen las reglas de negocio y abren las transacciones. Por ejemplo, en Inventario, "no reservar más de lo disponible".
3. **Repositorios.** Hablan con MySQL a través de GORM.

Los servicios no usan los repositorios directamente, sino interfaces. Así en los tests podemos reemplazar la base por un repositorio falso.

Inventario suma dos piezas que se ubican igual que los handlers, porque son otra forma de entrar al servicio: el consumidor de RabbitMQ (recibe `TrasladoCompletado` y `TrasladoCancelado` y llama al servicio que confirma o libera) y el proceso que publica el outbox. Solr y Memcached se usan desde la capa de abajo, igual que MySQL.

```text
servicios/inventario/
├── cmd/            → arranque del servicio
└── internal/
    ├── handlers/   → HTTP
    ├── mensajeria/ → consumidor de RabbitMQ y publicación del outbox
    ├── servicios/  → reglas de negocio
    ├── repositorios/ → MySQL, Solr y Memcached
    └── modelos/    → structs de productos, stock, reservas y movimientos
```

Usuarios y Sucursales tienen la misma estructura, sin la carpeta de mensajería.

### Hexagonal: Transferencias
En el centro está lo que es propio de la transferencia: sus estados, qué cambios de estado se permiten (por ejemplo, una transferencia `EN_CURSO` ya no se puede cancelar, y una `COMPLETADA` no cambia más) y los casos de uso, como crear una transferencia, responder un horario propuesto, cancelar o procesar los cambios que informa Rumbo. El centro no conoce Gin, ni el driver de MongoDB, ni el formato de Rumbo.

Lo que el centro necesita de afuera lo pide mediante **puertos**, que son interfaces definidas por el propio centro:

| Puerto | Para qué |
|---|---|
| Inventario | Reservar y liberar unidades |
| Sucursales | Traer nombre y dirección |
| Transporte | Pedir un traslado, consultar qué cambió, responder un horario propuesto y cancelar |
| Repositorio | Guardar y buscar transferencias |
| Eventos | Dejar los eventos para publicar |

Cada puerto tiene un **adaptador** que lo implementa: un cliente HTTP para Inventario, otro para Sucursales, otro para Rumbo, uno para MongoDB y uno para RabbitMQ. Al puerto lo llamamos "Transporte" y no "Rumbo" a propósito: si algún día cambiara el proveedor, se escribe otro adaptador y el centro queda igual.

Los adaptadores también pueden ser de entrada: los handlers HTTP que recibe el frontend y los procesos de fondo (el que le manda los pedidos a Rumbo, el que consulta cada 60 segundos, el que publica el outbox y el que libera reservas después de un timeout). Todos llaman a los casos de uso del centro.

```text
servicios/transferencias/
├── cmd/
└── internal/
    ├── dominio/      → la transferencia, sus estados y reglas
    ├── aplicacion/   → casos de uso
    ├── puertos/      → interfaces que necesita el centro
    └── adaptadores/
        ├── http/         → handlers de entrada
        ├── procesos/     → procesos de fondo
        ├── inventario/
        ├── sucursales/
        ├── rumbo/
        ├── mongo/
        └── rabbitmq/
```

### Gateway
El gateway no es un servicio de negocio, así que no sigue ninguno de los dos estilos. Es una cadena de middlewares (token, permisos, límite de pedidos y `trace_id`) seguida del reenvío al servicio que corresponde.

## Alternativas que descartamos
- **Hexagonal en Inventario, como teníamos antes.** Lo habíamos elegido cuando Inventario también hacía las transferencias y tenía que hablar con otros servicios. Al separar Transferencias, esa parte se fue con ella: ahora Inventario solo usa dependencias propias y capas alcanza. El hexagonal pasó a Transferencias, que es donde quedaron las dependencias externas.

## Consecuencias
- En Inventario, tanto los pedidos HTTP como los eventos de RabbitMQ pasan por la misma capa de servicios. Así, las reglas (no reservar más de lo disponible, no liberar una reserva confirmada) se aplican igual sin importar por dónde entre la operación.
- Hay dos formas de organizar el código en el mismo repositorio, y todo el equipo tiene que conocer las dos.
- Transferencias tiene más archivos e interfaces que los demás. A cambio, el centro se prueba con adaptadores falsos, por ejemplo un Rumbo falso que acepta o rechaza, sin red ni base.
- Que nadie se saltee capas (un handler que use un repositorio directo, por ejemplo) no lo controla el compilador: lo revisamos en los pull requests.
