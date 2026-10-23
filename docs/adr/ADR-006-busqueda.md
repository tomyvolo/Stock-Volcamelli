# ADR-006 — Búsqueda del catálogo (D6)

## Estado
Aceptado.

## Contexto
Los usuarios necesitan buscar productos por texto, en el nombre y en la descripción. Con MySQL se puede hacer con `LIKE`, pero no ordena por relevancia, no encuentra palabras parecidas y cada búsqueda le suma carga a la misma base que maneja las reservas.

Usamos Solr, que resuelve búsqueda por texto, filtros y relevancia, y desde Go se usa con simples pedidos HTTP. La fuente de verdad sigue siendo el MySQL de Inventario: Solr es una copia armada para buscar, y si se pierde se vuelve a armar desde MySQL.

## Decisión

### Qué se busca
La búsqueda es solo para nuestro frontend: la pueden usar todos los roles salvo `INTEGRACION`. No forma parte del contrato que le ofrecemos al otro grupo, así que `INTEGRACION` recibe `403`.

`GET /api/products/search`

| Parámetro | Qué hace |
|---|---|
| `q` | Texto a buscar en el nombre y la descripción. Es obligatorio: si viene vacío, responde `400` |
| `supplier` | Solo productos de ese proveedor, con el nombre exacto |
| `page` | Qué página devolver, empezando en 1. Si no viene, es la 1 |
| `size` | Cuántos resultados por página, de 1 a 100. Si no viene, son 20 |

Si `page` o `size` no son números válidos, responde `400`. Son los mismos parámetros y las mismas reglas que el listado de productos.

La respuesta trae la página pedida, el total de resultados y la lista, ordenada por relevancia, con el id, nombre, proveedor y score de cada producto:

```json
{ "page": 1, "size": 20, "total": 3, "results": [ { "id": "15", "nombre": "Harina 000", "proveedor": "Molinos del Sur", "score": 3.1 } ] }
```

La búsqueda no trae stock. Para ver cuánto hay de un producto, el frontend usa las consultas de stock de Inventario, que leen MySQL. El stock bajo de una sucursal también sale de ahí: `GET /api/branches/{id}/stock?lowStock=true` devuelve solo los productos con 5 unidades disponibles o menos en esa sucursal.

### Cómo se arma la consulta
El frontend nunca le manda a Solr una consulta armada: Inventario la arma a partir de estos parámetros.

- el texto va en `q`, con `defType=edismax` y `qf=nombre^5 descripcion`, para que una coincidencia en el nombre pese más que una en la descripción;
- el proveedor va como filtro `fq` con `{!term f=proveedor}`, que compara el valor exacto y no cambia la relevancia;
- la página se traduce a `start` y `rows` de Solr: `rows` es `size` y `start` es `(page - 1) * size`. Por ejemplo, la página 3 de a 20 es `start=40&rows=20`;
- el total sale de `numFound`, que Solr devuelve en cada respuesta;
- `fl` pide solo los campos que mostramos, más el `score`.

El código sigue la misma división que el resto de Inventario: el controller lee y valida los parámetros (las páginas con la misma función que usa el listado de productos), el service llama al buscador, y el repositorio de Solr es el único que arma la URL, hace el pedido HTTP y lee el JSON. El service usa el buscador a través de una interfaz, así que no conoce Solr.

### Qué guardamos en Solr
Un índice `productos`, con un documento por producto y solo los campos que hacen falta para buscar, filtrar y mostrar:

| Campo | Tipo | Para qué |
|---|---|---|
| `id` | `string` | El id del producto |
| `nombre`, `descripcion` | `text_es` | Búsqueda por texto. `text_es` pasa el texto a minúsculas, lo separa en palabras y saca las palabras comunes del español |
| `proveedor` | `string` | Para mostrarlo y filtrar |

### Cómo se mantiene actualizado
1. Después de cada transacción de Inventario que da de alta o cambia un producto, Inventario publica en RabbitMQ `ProductoActualizado`, con el id del producto ([ADR-016](ADR-016-comunicacion-entre-servicios.md)).
2. El indexador, que corre dentro de Inventario, consume el evento, lee el producto de MySQL y reemplaza el documento entero en Solr. Después hace ACK.

Como el indexador siempre lee el estado actual de MySQL, no importa si un evento llega repetido: vuelve a escribir lo mismo. Para que dos eventos del mismo producto no se procesen a la vez y uno viejo pise al nuevo, **el indexador corre en una sola de las dos instancias de Inventario** (se prende con una variable de entorno) y consume con QoS=1, de a un mensaje por vez. Si esa instancia se cae, los eventos esperan en la cola y se indexan cuando vuelve.

Si Solr no responde, el indexador reintenta con backoff (1, 2 y 4 segundos). Si después del cuarto intento sigue sin poder, rechaza el mensaje sin reencolarlo y va a la cola de errores. Cuando Solr vuelve, corremos la reconstrucción para recuperar los cambios de ese rato.

Aceptamos hasta **5 segundos** de atraso, contados desde que el cambio se guarda en MySQL hasta que aparece en la búsqueda. En ese tiempo entran el paso por RabbitMQ, la lectura del indexador y el segundo que tarda Solr en hacer visible lo que se escribió.

### Reconstrucción
Inventario tiene un proceso que recorre todos los productos de MySQL y reescribe el índice. Lo corremos si Solr arranca vacío, si se pierde el índice o si cambiamos el esquema. También sirve si se perdió algún evento.

### Si Solr se cae
El pedido a Solr tiene un timeout de 2 segundos. Si Solr no responde o devuelve un error, la búsqueda responde `502` con el mensaje "búsqueda no disponible". No devolvemos una lista vacía, porque parecería que no hay productos. El resto sigue funcionando desde MySQL: el listado del catálogo, el detalle de un producto, las consultas de stock y el stock bajo.

## Alternativas que descartamos
- **Guardar también el disponible de cada sucursal en Solr**, para filtrar el stock bajo desde la búsqueda. Necesitaba un campo por sucursal y reindexar el producto con cada reserva, confirmación o ajuste. El stock ya está en MySQL y la consulta de stock bajo de una sucursal es simple, así que la dejamos ahí.

## Consecuencias
- Siempre hay que escribir un texto para buscar. Los resultados vienen ordenados solo por relevancia: no se puede ordenar, por ejemplo, por nombre.
- Si se modifica un producto mientras alguien pasa de página, un resultado puede aparecer en dos páginas o no aparecer. Lo aceptamos, porque el catálogo cambia poco.
- La búsqueda solo se actualiza cuando cambia un producto, no con cada movimiento de stock, así que el indexador tiene poco trabajo.
- Si un evento se pierde, el documento queda desactualizado hasta el próximo cambio de ese producto o hasta que corramos la reconstrucción.
- Hay un proceso más que mantener (el indexador) y uno de reconstrucción que hay que probar.
