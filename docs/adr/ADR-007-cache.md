# ADR-007 — Caché de sucursales y usuarios (D7)

## Estado
Aceptado.

## Contexto
Para elegir qué cachear buscamos datos que se lean mucho y cambien poco. Lo que está en la caché no se actualiza cuando el dato cambia: se sigue mostrando hasta que vence. Por eso solo sirve para datos en los que mostrar algo de hace unos segundos no es un problema.

El stock no cumple eso: cambia con cada reserva, cada traslado y cada ajuste, y mostrar un disponible viejo confundiría a quien está por transferir. Lo dejamos siempre en MySQL, que además es la única fuente que puede usarse para decidir una reserva.

Los datos de las sucursales y la lista de usuarios de cada una sí lo cumplen:

- **Sucursales** (nombre y dirección). Casi nunca cambian: solo cuando se abre una sucursal o se muda. Y se leen todo el tiempo: todas las pantallas tienen un selector de sucursal, la sección de sucursales las muestra, y Transferencias pide las dos sucursales (origen y destino) en cada transferencia que crea, porque Rumbo necesita sus direcciones.
- **Usuarios de cada sucursal** (nombre, email y rol). Cambian solo cuando entra o se va alguien, o cuando cambia de rol o de sucursal. Y como cualquier usuario puede ver los usuarios de todas las sucursales, la sección se consulta seguido.

## Decisión

### Qué guardamos
Usamos Memcached, con el patrón cache-aside: el servicio busca primero en la caché; si está (hit), la devuelve; si no está (miss), la busca en su MySQL, la guarda en la caché y la devuelve. Lo usan dos servicios, cada uno con sus propias claves:

| Servicio | Consulta | Clave | Vigencia |
|---|---|---|---|
| Sucursales | Una sucursal (`GET /api/branches/{id}`), la usan Transferencias y el frontend | `SUCURSAL:{id}` | 60 segundos |
| Sucursales | Todas las sucursales (`GET /api/branches`) | `SUCURSALES` | 60 segundos |
| Usuarios | Usuarios de una sucursal (`GET /api/users?branchId={id}`) | `USUARIOS_SUCURSAL:{id}` | 60 segundos |

Las claves llevan el tipo de dato además del id, así no se pisan entre sí. El valor se guarda como JSON.

De los usuarios guardamos solo id, nombre, email y rol. La contraseña (ni siquiera su hash) nunca entra en la caché.

Cuando un dato cambia, no borramos la clave: la caché sigue mostrando el dato viejo hasta que vence. Así, una sucursal que se mudó o un vendedor nuevo tardan como mucho 60 segundos en aparecer.

### Cómo se arma en el código
La caché es un repositorio más. El repositorio con caché implementa la misma interfaz que el de MySQL y lo tiene adentro: primero busca en Memcached y, si no encuentra, le pide el dato al de MySQL. El service no sabe si hay caché o no.

Con una variable de entorno (`CACHE_ENABLED`) elegimos al arrancar qué repositorio se usa: el que tiene caché o directo el de MySQL. Así medimos el sistema con y sin caché usando el mismo código.

### La caché nunca decide
El login siempre lee el usuario de MySQL, porque necesita el hash de la contraseña. Los permisos salen del token ([ARCHITECTURE](../ARCHITECTURE.md), sección 4), no de la caché. Así, aunque la caché mostrara un dato de hace unos segundos, nadie gana ni pierde permisos por eso.

### Si Memcached se cae
La caché no es la fuente de verdad: si se cae, el sistema sigue andando. Si la lectura de Memcached falla, el repositorio no devuelve un error: lo registra en el log y va directo a MySQL, como si fuera un miss. Si después falla el guardado en Memcached, lo ignora. El que consulta nunca recibe un `500` por la caché.

### Cómo medimos que sirve
Usuarios y Sucursales exponen en `/metrics` contadores de:

- aciertos y fallos de caché (cada fallo es una consulta que fue a MySQL);
- errores de Memcached.

El porcentaje de aciertos lo calcula Grafana con esos contadores, y el tiempo de respuesta de cada consulta (promedio y p95) sale del histograma `request_duration_seconds` de esa ruta ([ADR-011](ADR-011-observabilidad.md)).

Para comparar, corremos la misma prueba de carga dos veces, una con `CACHE_ENABLED=false` y otra con la caché prendida: usuarios abriendo la sección de sucursales y creando transferencias al mismo tiempo. Esperamos que con caché baje el tiempo de esas consultas y que los MySQL de Usuarios y Sucursales reciban muchas menos. Los resultados van al ADR de D13.

## Alternativas que descartamos
- **Cachear el stock de cada sucursal.** Es la consulta más pesada, pero el stock cambia con cada reserva, traslado y ajuste, así que la caché mostraría un disponible viejo casi siempre.
- **Cachear el catálogo de productos.** Casi no cambia, pero traer un producto por id ya es muy rápido en MySQL. La caché casi no se notaría.
- **Caché local en cada servicio (ccache).** Es más rápida porque no pasa por la red, pero se pierde cada vez que se reinicia el servicio, y si algún día corremos dos instancias, cada una tendría su propia copia. Con Memcached la caché sobrevive a los reinicios y todas las instancias leen las mismas claves.

## Consecuencias
- Un cambio en una sucursal o en un usuario tarda hasta 60 segundos en verse. Para estos datos (una dirección, un usuario nuevo) lo aceptamos.
- Si un usuario cambia de sucursal, puede seguir apareciendo en la vieja hasta 60 segundos, y sus permisos cambian recién cuando vuelve a hacer login, porque van en el token.
- Usuarios y Sucursales tienen una dependencia más y dos caminos (con y sin caché), y hay que probar los dos.
- Memcached no guarda nada en disco: si se reinicia, arranca vacía y se vuelve a llenar con las consultas.
