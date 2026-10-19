# ADR-012 — Balanceo de carga (D12)

## Estado
Aceptado.

## Contexto
Inventario es el servicio que más trabajo recibe: lo consultan nuestro frontend, el otro grupo a través del contrato y Transferencias para reservar. Además es lo que publicamos, así que si se cae, el otro grupo se queda sin servicio. Por eso lo corremos en dos instancias, y necesitamos algo que reparta los pedidos entre ellas y deje de mandarle pedidos a una que se cayó.

Las dos instancias pueden atender cualquier pedido porque Inventario es stateless, no guarda nada propio en memoria: el stock está en MySQL y los bloqueos se hacen en la base con `SELECT ... FOR UPDATE` ([ADR-015](ADR-015-persistencia.md)).

## Decisión

### Dónde va
Un NGINX interno delante de las dos instancias de Inventario. Le llegan los pedidos del gateway y los de Transferencias, así que todos los que van a Inventario pasan por el mismo lugar.

### Cómo reparte
Round-robin: un pedido a cada instancia, alternando. Como las dos son iguales y no tienen estado propio, no hace falta nada más elaborado.

### Cómo detecta que una se cayó
Usamos la detección de fallas de NGINX con `max_fails=3` y `fail_timeout=30s`: si una instancia falla 3 veces (no conecta o se corta la conexión) dentro de 30 segundos, NGINX deja de mandarle pedidos por 30 segundos. Pasado ese tiempo le vuelve a probar con un pedido real, y si responde, vuelve a repartir entre las dos.

Esta detección se basa en los pedidos que pasan. No usamos chequeos activos porque la versión gratuita de NGINX no los tiene.

Cada instancia expone además `GET /health`, que responde bien solo si llega a su MySQL. Lo usa Docker Compose para saber si el contenedor está sano y para reiniciarlo si se cae.

### La configuración
El `nginx.conf` tiene solo lo necesario: el grupo `upstream` con las dos instancias (cada una con `max_fails=3 fail_timeout=30s`), `keepalive 32` para que NGINX mantenga hasta 32 conexiones abiertas con las instancias y no abra una por cada pedido, y un `proxy_pass` hacia ese grupo. Todo lo demás queda como viene en NGINX.

### Qué pasa con el pedido que falla
Por cómo viene configurado NGINX:

- **Consultas (`GET`).** Si una instancia no conecta o corta la conexión, NGINX prueba el mismo pedido en la otra. El que consulta no se entera.
- **Reservas y ajustes (`POST`).** NGINX no los reintenta en la otra instancia: si la primera llegó a hacer la reserva y se cortó la respuesta, reintentarla podría reservar dos veces. Devuelve el error, y lo resuelve quien llamó, como dice [ADR-016](ADR-016-comunicacion-entre-servicios.md): Transferencias marca la transferencia `FALLIDA` y libera en segundo plano.

No cambiamos los timeouts de NGINX. Quien llama a Inventario (el gateway o Transferencias) corta a los 2 segundos con su propio timeout ([ADR-010](ADR-010-resiliencia.md)), así que nadie espera más que eso.

### Si se caen las dos
NGINX responde `502` si no pudo conectarse con ninguna instancia, o `504` si se conectó pero la respuesta tardó más que su timeout. En los dos casos, o si se cumplen los 2 segundos de quien llama, el gateway responde `503` (servicio no disponible). Nunca respondemos stock cero ni una lista vacía, porque parecería un dato real.

### Los demás servicios
| Servicio | Instancias | Por qué |
|---|---|---|
| Inventario | 2, detrás de NGINX | Es el que más carga recibe y el que publicamos |
| Transferencias | 1 | Sus procesos de fondo (consultar a Rumbo, mandar pedidos) no se pueden duplicar ([ADR-009](ADR-009-consumo-de-rumbo.md)) |
| Usuarios, Sucursales | 1 | Reciben poca carga: el login y algunas consultas |
| Gateway | 1 | Es una sola puerta de entrada; no tiene estado y se reinicia rápido |

### Cómo mostramos que funciona
NGINX registra en su log, por cada pedido, a qué instancia fue (`upstream_addr`), cuánto tardó, el código de respuesta y el `track_id`. Para demostrarlo:

1. Con las dos instancias arriba, el log muestra los pedidos repartidos entre A y B.
2. Apagamos A (`docker compose stop inventario-a`): después de unas pocas fallas, todos los pedidos van a B y las consultas siguen respondiendo.
3. Prendemos A de nuevo: a los 30 segundos vuelve a recibir pedidos.

## Alternativas que descartamos
- **Repartir la carga desde el gateway.** El gateway ya hace autenticación, permisos y ruteo, y sumarle el balanceo lo cargaba de otra responsabilidad. Además Transferencias no pasa por el gateway, así que el gateway tendría que repartir solo una parte de los pedidos y Transferencias necesitaría su propio reparto.

## Consecuencias
- NGINX y el gateway son una sola instancia cada uno: si se caen, Inventario no se puede usar aunque las dos instancias estén bien. Lo aceptamos porque son piezas simples, sin estado, que Docker reinicia en segundos.
- Mientras NGINX no detecta que una instancia se cayó, algunos pedidos fallan. Las consultas se salvan porque se reintentan en la otra; las reservas no.
- Una instancia que está viva pero muy lenta no cuenta como falla para NGINX, porque no cambiamos su timeout: los pedidos que le tocan fallan por el timeout de 2 segundos de quien llama. Lo vemos en el tablero, con los pedidos por instancia y sus tiempos.
- El MySQL de Inventario es uno solo para las dos instancias. Tener dos instancias reparte el trabajo de Go, pero no el de la base: si el límite está en MySQL, agregar instancias no ayuda. Eso lo vemos con las pruebas de carga de D13.
