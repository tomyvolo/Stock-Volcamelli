# ADR-013 — Capacidad y costos (D13)

## Estado
Propuesto. Definimos cómo vamos a medir; los números los agregamos cuando corramos las pruebas sobre el sistema levantado.

## Contexto
Necesitamos saber cuánta carga aguanta el sistema, qué se satura primero y cuánto cuesta tenerlo andando. También queremos comprobar con números dos decisiones que tomamos: que la caché de sucursales y usuarios sirve ([ADR-007](ADR-007-cache.md)) y que tener dos instancias de Inventario ayuda ([ADR-012](ADR-012-balanceo-de-carga.md)).

## Decisión

### Herramienta
Escribimos un generador de carga propio en Go. Usa un worker pool: una cantidad fija de goroutines que toman pedidos de un channel y los mandan al gateway, cada uno con un `context.WithTimeout` para que un pedido colgado no frene la prueba. Al terminar informa cuántos pedidos hizo, cuántos fallaron y cuánto tardaron.

Los tiempos que tomamos como resultado son los de Prometheus (el histograma de duración de cada servicio, del que sale el p95), así medimos con la misma herramienta con la que monitoreamos el sistema.

Todas las pruebas entran por el gateway, con un token, igual que un usuario real.

### Escenarios
| Escenario | Qué simula | Para qué |
|---|---|---|
| **Consultas del contrato** | El otro grupo y nuestras pantallas consultando el stock de varias sucursales y productos | Ver cuántas consultas por segundo aguanta Inventario dentro del objetivo de 500 ms de p95 ([ADR-011](ADR-011-observabilidad.md)) |
| **Sucursales y usuarios, con y sin caché** | Usuarios abriendo la sección de sucursales y creando transferencias (cada una pide dos sucursales), una vez con `CACHE_ENABLED=false` y otra con la caché prendida | Medir cuánto mejora la caché y cuántas consultas les saca a los MySQL de Usuarios y Sucursales |
| **Una y dos instancias** | El mismo escenario con una sola instancia de Inventario y con dos | Ver si la segunda instancia sube la capacidad o si el límite está en MySQL |
| **Reservas concurrentes** | Muchas transferencias del mismo producto y la misma sucursal al mismo tiempo, mientras siguen las consultas | Ver cuánto aguantan las reservas con `FOR UPDATE` sobre la misma fila, y comprobar que el stock nunca queda negativo |
| **Búsqueda** | Búsquedas con texto y filtro de proveedor | Ver la respuesta de Solr y su atraso mientras se modifican productos |

Cada escenario sube la carga de a poco (por ejemplo, de 10 a 200 workers en 5 minutos) hasta que el p95 pasa los 500 ms o aparecen errores. Ese punto es la capacidad máxima.

Rumbo no participa de las pruebas: usamos un Rumbo falso que responde enseguida, porque no queremos cargar el servicio de otro grupo y porque lo que medimos es nuestro sistema.

### Qué registramos
Por cada corrida: p95, pedidos por segundo y porcentaje de errores (de Prometheus y del generador), y CPU y memoria de cada contenedor, consultas a MySQL, porcentaje de aciertos de la caché y pedidos por instancia (de Grafana). También anotamos la máquina en la que corrimos, para que los números se puedan comparar.

### Límite que esperamos encontrar
Creemos que el primer límite va a ser el **MySQL de Inventario**: las dos instancias usan la misma base, y las reservas sobre un mismo producto se hacen de a una por el `FOR UPDATE`. Si es así, agregar instancias de Inventario no va a subir mucho la capacidad. Lo confirmamos o lo corregimos con los resultados.

### Cómo escalaríamos
Según qué se sature:

1. **Si es la CPU de Inventario:** más instancias detrás de NGINX. Ya están preparadas, porque no guardan estado propio.
2. **Si son las consultas al MySQL de Inventario:** una réplica de lectura de MySQL para las consultas, dejando la base principal para reservas y ajustes.
3. **Si son las reservas sobre un mismo producto:** no se resuelve con más máquinas, porque es un bloqueo sobre la misma fila. Habría que revisar cómo se reserva, y lo dejamos documentado como límite.

### Costos
Para la estimación suponemos que todo corre con Docker Compose en una sola máquina virtual en la nube. Contando servicios, bases, RabbitMQ, Solr, Memcached, NGINX y las herramientas de observabilidad, son unos 20 contenedores. Solr y las tres bases son los que más memoria usan.

| Recurso | Qué necesitamos | Costo aproximado por mes |
|---|---|---|
| Máquina virtual | 4 vCPU y 8 GB de memoria | Entre 40 y 70 dólares, según el proveedor |
| Disco | 50 GB | Entre 4 y 8 dólares |
| Modelo de lenguaje del asistente ([ADR-018](ADR-018-asistente.md)) | Pocas consultas por día durante la demo | Entre 0 (planes gratuitos) y unos pocos dólares |
| **Total** | | **Entre 45 y 80 dólares por mes** |

Los precios son aproximados y los confirmamos con la calculadora del proveedor que elijamos. Si el sistema tuviera que crecer, lo primero que se encarece es la base: separar el MySQL de Inventario en una base administrada cuesta bastante más que una máquina virtual.

## Consecuencias
- Las pruebas corren en la misma máquina que el sistema o en una notebook, así que los números absolutos dependen de esa máquina. Lo que más nos importa son las comparaciones (con y sin caché, una y dos instancias), que sí son válidas.
- Los números de los objetivos y alertas ([ADR-011](ADR-011-observabilidad.md)) y los timeouts ([ADR-010](ADR-010-resiliencia.md)) los ajustamos con estos resultados.
- Tener todo en una sola máquina es barato, pero si esa máquina se cae, se cae todo. Para un sistema real habría que repartirlo en varias.
