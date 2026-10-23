# ADR-013 — Capacidad y costos (D13)

## Estado
Aceptado.

## Contexto
Necesitamos saber cuánta carga aguanta el sistema, qué se satura primero y cuánto cuesta tenerlo andando. También queremos comprobar con números dos decisiones que tomamos: que la caché de sucursales y usuarios sirve ([ADR-007](ADR-007-cache.md)) y que tener dos instancias de Inventario ayuda ([ADR-012](ADR-012-balanceo-de-carga.md)).

## Decisión

### Herramienta
Escribimos un generador de carga propio en Go, en [`pruebas/carga`](../../pruebas/carga/main.go). Usa un worker pool: una cantidad fija de goroutines que toman pedidos de un channel y los mandan al gateway, cada uno con un `context.WithTimeout` de 2 segundos (el mismo timeout del gateway) para que un pedido colgado no frene la prueba. Al terminar informa cuántos pedidos hizo, cuántos fallaron, los pedidos por segundo y los percentiles de duración (p50, p95, p99 y máximo).

```bash
cd pruebas/carga
TOKEN=<token> go run . -url http://localhost:8080/api/branches/1/stock -workers 50 -duracion 20s
```

El p95 del generador es lo que ve quien consulta, de punta a punta. Prometheus nos da además el p95 de cada servicio por separado (`request_duration_seconds`), y con eso vemos en qué pieza se va el tiempo.

Todas las pruebas entran por el gateway, con un token, igual que un usuario real.

### Escenarios
| Escenario | Qué simula | Para qué | Resultados |
|---|---|---|---|
| **Consultas del contrato** | El otro grupo y nuestras pantallas consultando el stock de una sucursal | Ver cuántas consultas por segundo aguanta el sistema dentro del objetivo de 500 ms de p95 ([ADR-011](ADR-011-observabilidad.md)) | Abajo |
| **Una y dos instancias** | El mismo escenario con una sola instancia de Inventario y con dos | Ver si la segunda instancia sube la capacidad | Abajo |
| **Sucursales y usuarios, con y sin caché** | Usuarios abriendo la sección de sucursales y creando transferencias (cada una pide dos sucursales), una vez con `CACHE_ENABLED=false` y otra con la caché prendida | Medir cuánto mejora la caché y cuántas consultas les saca a los MySQL de Usuarios y Sucursales | Se mide con Sucursales y la caché |
| **Reservas concurrentes** | Muchas transferencias del mismo producto y la misma sucursal al mismo tiempo, mientras siguen las consultas | Ver cuánto aguantan las reservas con `FOR UPDATE` sobre la misma fila, y comprobar que el stock nunca queda negativo | Se mide con Transferencias |
| **Búsqueda** | Búsquedas con texto y filtro de proveedor | Ver la respuesta de Solr y su atraso mientras se modifican productos | Se mide con Solr |

Cada escenario sube la carga de a poco (de 10 a 200 workers) hasta que el p95 pasa los 500 ms o aparecen errores. Ese punto es la capacidad máxima.

Rumbo no participa de las pruebas: usamos un Rumbo falso que responde enseguida, porque no queremos cargar el servicio de otro grupo y porque lo que medimos es nuestro sistema.

### Qué registramos
Por cada corrida: p95, pedidos por segundo y porcentaje de errores (del generador y de Prometheus), CPU y memoria de cada contenedor (`docker stats`) y pedidos por instancia. También anotamos la máquina en la que corrimos, para que los números se puedan comparar.

## Resultados de la primera medición (23/10/2026)

**Máquina:** notebook con AMD Ryzen 7 7730U (8 núcleos, 16 hilos) y 15 GB de RAM, Windows 11 con Docker Desktop 29.2.1 (16 CPU y 7,4 GB asignados a Docker). El sistema y el generador corren en la misma máquina, así que compiten por la CPU.

**Versión:** `main` en `2c5d0fd`, levantada desde cero con `docker compose up --build -d`.

**Ruta:** `GET /api/branches/1/stock` (stock de una sucursal, consulta del contrato), con el token de la cuenta `INTEGRACION`. Cada corrida dura 20 segundos, después de 5 segundos de calentamiento.

### Dos instancias de Inventario
| Workers | Pedidos por segundo | p50 | p95 | p99 | Errores |
|---|---|---|---|---|---|
| 10 | 245 | 40 ms | 50 ms | 61 ms | 0 % |
| 25 | 444 | 53 ms | 77 ms | 91 ms | 0 % |
| 50 | 524 | 92 ms | 128 ms | 154 ms | 0 % |
| 100 | 798 | 120 ms | 177 ms | 233 ms | 0 % |
| 200 | 834 | 206 ms | **502 ms** | 825 ms | 0 % |

### Una instancia de Inventario (`inventario2` apagada)
| Workers | Pedidos por segundo | p50 | p95 | p99 | Errores |
|---|---|---|---|---|---|
| 10 | 240 | 40 ms | 52 ms | 67 ms | 0 % |
| 50 | 593 | 79 ms | 125 ms | 156 ms | 0 % |
| 100 | 426 | 162 ms | **821 ms** | 2004 ms | **1,84 %** |
| 200 | 919 | 199 ms | 468 ms | 618 ms | 0 % |

### Repetición con 100 workers
Como con 100 workers aparecieron errores con una instancia y no con dos, repetimos tres corridas de 30 segundos de cada caso:

| Caso | Corrida | Pedidos por segundo | p95 | p99 | Errores |
|---|---|---|---|---|---|
| Dos instancias | 1 | 683 | 253 ms | 353 ms | 0 % |
| Dos instancias | 2 | 592 | 298 ms | 396 ms | 0 % |
| Dos instancias | 3 | 446 | 471 ms | 2000 ms | **1,78 %** |
| Una instancia | 1 | 688 | 233 ms | 324 ms | 0 % |
| Una instancia | 2 | 417 | 494 ms | 2000 ms | **1,52 %** |
| Una instancia | 3 | 645 | 271 ms | 342 ms | 0 % |

Los errores son pedidos que pasaron los 2 segundos sin respuesta, y aparecen con una o con dos instancias. En la tercera corrida con dos instancias, Prometheus marcó un p95 de 1033 ms en el gateway y de 247 ms en Inventario: el tiempo se iba antes o después de llegar a Inventario.

### Uso de recursos con 100 workers
| Contenedor | CPU con dos instancias | CPU con una instancia |
|---|---|---|
| gateway | 466 % | 312 % |
| mysql-logs | 286 % | 162 % |
| inventario1 | 88 % | 89 % |
| inventario2 | 91 % | — |
| mysql-inventario | 68 % | 38 % |
| nginx | 23 % | 13 % |

(100 % equivale a un núcleo entero.)

### Qué encontramos
**El primer límite no es el MySQL de Inventario, sino la escritura de logs.** Cada consulta escribe tres filas en `app_logs`, cada una con un `INSERT` sincrónico: el gateway escribe al recibir y al responder, e Inventario al atender. Con 100 workers, el MySQL de logs llegó a su máximo de conexiones (`max_connections` = 151, `Max_used_connections` = 152): GORM abre conexiones sin tope en cada servicio, y entre el gateway y las dos instancias lo superan. Desde ese momento los `INSERT` fallan con `Error 1040: Too many connections`:

| Servicio | Logs que no se pudieron guardar |
|---|---|
| gateway | 27.337 |
| inventario1 | 11.905 |
| inventario2 | 2.680 |

Eso tiene tres consecuencias:
- **Latencia:** cada pedido espera sus `INSERT`, y cuando el MySQL de logs se traba, algunos pedidos pasan los 2 segundos. Son los errores de las tablas.
- **Trazas incompletas:** bajo carga se pierden logs, así que la traza de algunos pedidos queda cortada.
- **CPU del gateway:** es el contenedor que más CPU usa, porque escribe dos logs por pedido y reintenta conexiones.

**La segunda instancia de Inventario no sube la capacidad hoy.** Con una y con dos instancias los números son parecidos, y la variación entre corridas es mayor que la diferencia entre los dos casos. Inventario no está saturado (cerca de un núcleo por instancia) y su MySQL tampoco (38 % a 68 %). La segunda instancia sí sirve para la disponibilidad: con una apagada las consultas siguen respondiendo ([ADR-012](ADR-012-balanceo-de-carga.md)).

**Capacidad dentro del objetivo:** con hasta 50 workers (alrededor de 520 a 590 consultas por segundo) el p95 queda debajo de 130 ms y no hay errores. Con 100 workers el p95 queda debajo de 500 ms, pero algunas corridas tienen entre 1,5 % y 1,8 % de errores, más que el 1 % que permite el objetivo de disponibilidad. Con 200 workers el p95 ronda los 500 ms. Tomamos como capacidad **unas 500 consultas por segundo** en esta máquina, hasta resolver el límite de los logs.

### Límite que esperábamos y lo que pasó
Creíamos que el primer límite iba a ser el MySQL de Inventario, porque las dos instancias usan la misma base. Con las consultas del contrato no es así: la base de Inventario no llega al 70 % de CPU. Esa hipótesis sigue abierta para las reservas con `FOR UPDATE`, que todavía no podemos medir.

### Qué hacemos con esto
1. **Limitar las conexiones de los logs** en cada servicio (`SetMaxOpenConns` y `SetMaxIdleConns` en el `*sql.DB` de GORM del paquete `logs`), para que entre todos no pasen las 151 del MySQL de logs.
2. **Escribir los logs en segundo plano:** que `Write` deje el log en un channel con buffer y una goroutine los inserte por lotes. Así el pedido no espera al MySQL de logs. Si el buffer se llena, se descarta el log y se avisa por consola, como hoy cuando falla el `INSERT`.
3. **Repetir la medición** con esos cambios y comparar.
4. **Correr el generador en otra máquina** cuando el sistema esté desplegado, para que no compita por la CPU.

### Cómo escalaríamos
Según qué se sature:

1. **Si es la escritura de logs (lo que vemos hoy):** primero las conexiones limitadas y la escritura en segundo plano. Si no alcanza, una herramienta pensada para logs en lugar de MySQL.
2. **Si es la CPU del gateway:** una segunda instancia del gateway detrás de un balanceador. No guarda estado, así que se puede duplicar.
3. **Si es la CPU de Inventario:** más instancias detrás de NGINX. Ya están preparadas, porque no guardan estado propio.
4. **Si son las consultas al MySQL de Inventario:** una réplica de lectura de MySQL para las consultas, dejando la base principal para reservas y ajustes.
5. **Si son las reservas sobre un mismo producto:** no se resuelve con más máquinas, porque es un bloqueo sobre la misma fila. Habría que revisar cómo se reserva, y lo dejamos documentado como límite.

### Costos
Para la estimación suponemos que todo corre con Docker Compose en una sola máquina virtual en la nube. Contando servicios, bases, RabbitMQ, Solr, Memcached, NGINX y las herramientas de observabilidad, son unos 20 contenedores. Solr y las tres bases son los que más memoria usan; en la medición, cada MySQL usó alrededor de 500 MB y cada servicio en Go menos de 35 MB.

| Recurso | Qué necesitamos | Costo aproximado por mes |
|---|---|---|
| Máquina virtual | 4 vCPU y 8 GB de memoria | Entre 40 y 70 dólares, según el proveedor |
| Disco | 50 GB | Entre 4 y 8 dólares |
| Modelo de lenguaje del asistente ([ADR-018](ADR-018-asistente.md)) | Pocas consultas por día durante la demo | Entre 0 (planes gratuitos) y unos pocos dólares |
| **Total** | | **Entre 45 y 80 dólares por mes** |

Los precios son aproximados y los confirmamos con la calculadora del proveedor que elijamos. Con 4 vCPU, el límite de los logs aparecería antes que en la notebook, porque el gateway y el MySQL de logs ya usaron más de 7 núcleos entre los dos con 100 workers. Si el sistema tuviera que crecer, lo primero que se encarece es la base: separar el MySQL de Inventario en una base administrada cuesta bastante más que una máquina virtual.

## Consecuencias
- Los números absolutos dependen de la notebook en la que corrimos, y el generador compitió por la CPU con el sistema. Lo que más nos importa son las comparaciones y qué pieza se satura primero.
- Hay variación entre corridas iguales (con 100 workers, entre 417 y 688 consultas por segundo). Por eso repetimos las corridas antes de sacar conclusiones.
- Bajo carga se pierden logs y algunas trazas quedan incompletas, hasta que limitemos las conexiones y escribamos los logs en segundo plano.
- Los números de los objetivos y alertas ([ADR-011](ADR-011-observabilidad.md)) y los timeouts ([ADR-010](ADR-010-resiliencia.md)) los ajustamos con estos resultados.
- Tener todo en una sola máquina es barato, pero si esa máquina se cae, se cae todo. Para un sistema real habría que repartirlo en varias.
