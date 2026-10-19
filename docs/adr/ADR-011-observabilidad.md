# ADR-011 — Observabilidad (D11)

## Estado
Propuesto. Es la primera versión: los umbrales los ajustamos con las pruebas de carga.

## Contexto
Un pedido puede pasar por el gateway, NGINX, Inventario y Transferencias, y una transferencia sigue viva en procesos de fondo durante días. Cuando algo falla, necesitamos poder seguir un pedido entre servicios, saber en qué servicio falló, saber si el sistema anda bien sin mirar log por log y enterarnos de un problema antes de que nos lo diga un usuario.

## Decisión

### Herramientas
| Para qué | Herramienta |
|---|---|
| Logs y trazas | Cada servicio los guarda con GORM en la tabla `app_logs` de un MySQL aparte |
| Métricas | Cada servicio expone `GET /metrics` con la librería cliente de Prometheus; Prometheus las consulta cada 5 segundos |
| Tablero y alertas | Grafana, leyendo de Prometheus y del MySQL de logs |

Todo corre en contenedores dentro del mismo Docker Compose. La configuración está en la carpeta `observabilidad/` del repositorio:

```text
observabilidad/
├── prometheus.yml                  → qué servicios consulta Prometheus y cada cuánto
├── mysql/001_schema.sql            → crea la tabla app_logs y el usuario de solo lectura de Grafana
└── grafana/
    ├── provisioning/datasources/   → las fuentes Prometheus y MySQL Logs
    ├── provisioning/dashboards/    → le dice a Grafana que cargue el tablero
    └── dashboards/stock.json       → el tablero
```

Así, con `docker compose up` Grafana arranca con las fuentes de datos y el tablero ya cargados, sin configurar nada a mano.

### TrackID
Cada pedido tiene un único id de correlación, el **TrackID**, desde que entra al gateway hasta que termina. Lo maneja solo el gateway:

- El middleware `AddTrackID` del gateway corre antes que cualquier otra cosa. Genera 16 bytes aleatorios y los escribe como 32 caracteres hexadecimales. Si no puede generarlo, responde `500`.
- Lo guarda en el contexto de Gin con `ctx.Set("track_id", ...)`, para que el resto del gateway lo lea con `ctx.GetString("track_id")`, y lo devuelve en el header `X-Track-ID` de la respuesta. Así, si el otro grupo o un usuario reporta un problema, nos puede dar el TrackID de ese pedido.
- Se lo pasa al servicio en el mismo header `X-Track-ID`. Los servicios nunca generan uno: un middleware lo lee del header y lo deja en el contexto, y lo pasan igual en cada llamada a otro servicio (incluidas las que pasan por NGINX) y en los eventos de RabbitMQ, como un header del mensaje.
- Transferencias guarda en cada transferencia el TrackID del pedido que la creó. Todo lo que pasa después con esa transferencia en segundo plano (mandarla a Rumbo, cada cambio de estado, los eventos a Inventario) se registra con ese mismo TrackID, así la vida completa de una transferencia se sigue con un solo id.

### Logs
Cada servicio guarda sus logs directo en una tabla de MySQL, y Grafana los lee de ahí.

- **El MySQL de logs** es un contenedor aparte, solo para esto. No usamos la base de ningún servicio, porque los logs se escriben todo el tiempo y le sacarían capacidad al negocio. Cuando el volumen es nuevo, la imagen de MySQL corre `001_schema.sql`, que crea la tabla y un usuario `grafana_reader` que solo puede hacer `SELECT` sobre ella. Los datos quedan en un volumen de Docker.
- **Cada servicio tiene un paquete `logs`** con dos funciones. `Connect` abre la conexión con GORM (los datos llegan por la variable de entorno `LOGS_MYSQL_DSN`) y corre `AutoMigrate` sobre `app_logs`. `Write(trackID, stage, level, message)` guarda una fila con `db.Table("app_logs").Create(...)`.
- **Si el MySQL de logs falla**, `Write` escribe un aviso por consola con `log.Printf` y el pedido sigue: el log se puede ver con `docker compose logs <servicio>`, pero no aparece en Grafana.
- **Cada servicio espera a que el MySQL de logs esté listo** antes de arrancar (`depends_on` con `condition: service_healthy` en el Compose), porque necesita la conexión para escribir.

La tabla `app_logs`:

| Columna | Qué guarda |
|---|---|
| `id` | Autoincremental |
| `created_at` | Cuándo se escribió el log, con milisegundos. Lo completa MySQL |
| `track_id` | El TrackID del pedido. Vacío en los logs que no vienen de un pedido, como el arranque del servicio |
| `stage` | Qué servicio lo escribió: `gateway`, `inventario`, `usuarios`, `sucursales` o `transferencias` |
| `level` | `info`, `warn` o `error` |
| `message` | El texto del log |

Tiene índices en `created_at`, `track_id` y `stage`, que son las tres formas en que buscamos.

Qué va en el mensaje:

- Qué pasó, con los ids del negocio cuando los hay: `reserva creada para la transferencia 42`.
- Cuánto tardó, cuando el log cierra un paso: `respuesta 201 en 120 ms`.
- Nunca contraseñas, tokens ni las credenciales de Rumbo.

### Trazas
La traza de un pedido son sus logs: todas las filas de `app_logs` con su TrackID, ordenadas por `created_at`. Cada servicio escribe un log al recibir el pedido, en cada paso importante (llamar a otro servicio o a Rumbo, usar la base, Memcached o Solr) y al responder.

Por ejemplo, la traza de crear una transferencia:

```sql
SELECT created_at, stage, level, message
FROM app_logs
WHERE track_id = 'a1b2...'
ORDER BY created_at, id;
```

| created_at | stage | level | message |
|---|---|---|---|
| 14:03:12.001 | gateway | info | pedido recibido POST /api/transfers |
| 14:03:12.025 | transferencias | info | sucursales 3 y 7 consultadas en 20 ms |
| 14:03:12.096 | inventario | info | reserva creada para la transferencia 42 en 70 ms |
| 14:03:12.112 | transferencias | info | transferencia 42 guardada como RESERVADA |
| 14:03:12.121 | gateway | info | respuesta 201 en 120 ms |

Se ve por qué servicios pasó el pedido, en qué orden y cuánto tardó cada paso. Si falla, la última fila tiene `level = error` y `stage` dice en qué servicio se cortó; los servicios que venían después no aparecen. Nuestros flujos hacen las llamadas una detrás de otra, así que el orden por fecha alcanza para reconstruir el recorrido.

La primera traza que mostramos es la de una consulta del contrato (gateway → NGINX → Inventario → MySQL), que es lo primero que va a estar andando. Cuando esté Transferencias, la del flujo principal: crear una transferencia.

### Métricas
Cada servicio tiene el middleware `RecordMetrics` en su grupo de rutas. Toma la hora, deja que Gin ejecute el controller con `ctx.Next()`, y al volver registra dos métricas con las etiquetas `route` (la ruta, por ejemplo `/api/branches/:branchId/stock`) y `status` (el código HTTP):

| Métrica | Tipo | Qué guarda |
|---|---|---|
| `requests_total` | Contador | Cantidad de respuestas |
| `request_duration_seconds` | Histograma | Cuánto tardó cada respuesta. Prometheus genera `_bucket`, `_sum` y `_count`, y de ahí sacamos la duración media y el p95 |

`/metrics` y `/health` quedan fuera de ese grupo, así no se cuentan las consultas de Prometheus ni los chequeos de Docker.

Prometheus consulta cada instancia por separado y les agrega la etiqueta `instance`, así que los pedidos de las dos instancias de Inventario se pueden ver juntos o por separado, sin código extra.

Además, con el mismo tipo de contador:

| Servicio | Contador |
|---|---|
| Usuarios y Sucursales | Aciertos, fallos y errores de la caché ([ADR-007](ADR-007-cache.md)) |
| Inventario | Mensajes mandados a la cola de errores |
| Transferencias | Errores de Rumbo y aperturas del circuit breaker ([ADR-010](ADR-010-resiliencia.md)) |

Los ids del negocio van en los logs, nunca como etiquetas: con una etiqueta por transferencia, Prometheus guardaría una serie por cada una.

### Tablero
Un tablero en Grafana. Arriba tiene un campo de texto **TrackID**: vacío muestra todos los logs, y con un TrackID pegado filtra los paneles de logs y muestra la traza de ese pedido. Las métricas no se filtran, siguen sumando todos los pedidos.

| Panel | De dónde sale |
|---|---|
| Pedidos por segundo, por ruta y código | Prometheus, `requests_total` |
| Duración media y p95 por ruta | Prometheus, `request_duration_seconds` |
| Porcentaje de respuestas `5xx` | Prometheus, `requests_total` |
| Pedidos por instancia de Inventario | Prometheus, `requests_total` por `instance` |
| Porcentaje de aciertos de la caché | Prometheus, contadores de la caché |
| Mensajes a la cola de errores y errores de Rumbo | Prometheus, contadores de negocio |
| Logs del recorrido | MySQL Logs, `app_logs` filtrada por el TrackID |
| Errores por servicio | MySQL Logs, filas con `level = 'error'`, con su `stage` y su TrackID |

El campo de texto se usa en las consultas SQL como `${track_id:sqlstring}`, que lo pone entre comillas antes de mandarlo a MySQL.

### Objetivos de servicio
Los ponemos sobre lo que publicamos, porque es lo que usa el otro grupo:

| Objetivo | Valor | Medido en |
|---|---|---|
| Disponibilidad de las consultas del contrato | 99 % de los pedidos responden sin error `5xx` | Ventana de 1 hora |
| Velocidad de las consultas del contrato | 95 % responden en menos de 500 ms | Ventana de 1 hora |

### Alertas
Las armamos en Grafana sobre las mismas métricas del tablero:

| Alerta | Cuándo |
|---|---|
| Inventario con errores | Más del 1 % de `5xx` en las consultas del contrato durante 5 minutos |
| Inventario lento | p95 de las consultas del contrato arriba de 500 ms durante 5 minutos |
| Servicio caído | Prometheus no puede leer `/metrics` de un servicio por 1 minuto |
| Cola de errores con mensajes | Se mandó al menos un mensaje a la cola de errores |

Por ahora las alertas se ven en Grafana. A dónde se mandan (mail, Discord) lo definimos cuando el sistema esté desplegado.

## Consecuencias
- Son tres contenedores más (Prometheus, Grafana y el MySQL de logs) para levantar en cada entorno.
- Cada log es un `INSERT` en el momento, así que cada paso de un pedido suma unos milisegundos. Con nuestro volumen no se nota, y a cambio no se pierde ningún log si un servicio se cae.
- Si el MySQL de logs se cae, los logs de ese rato solo quedan en la consola de Docker y no aparecen en Grafana.
- La tabla crece sin límite. Para limpiarla borramos las filas viejas a mano o bajamos el volumen (`docker compose down -v`).
- La traza depende de que cada servicio pase el TrackID en todas sus llamadas y eventos, y escriba sus logs. Si alguno se lo olvida, el recorrido se corta ahí. Lo resolvemos con el mismo middleware y el mismo paquete `logs` en todos los servicios.
- La traza se ve como una lista en orden, no como un árbol: si algún día hiciéramos llamadas en paralelo, habría que agregar qué paso llamó a cuál.
- Los valores de los objetivos y las alertas son iniciales. Los revisamos con los resultados de D13.
