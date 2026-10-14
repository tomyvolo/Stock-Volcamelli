# ADR-017 — Contrato propio (D8)

## Estado
Aceptado. Reemplaza a [ADR-008](ADR-008-contrato-de-integracion.md). Se precisa stock disponible; la API real está pendiente.

## Por qué revisamos ADR-008
En ADR-008 publicábamos toda la API de Inventario, incluidas las transferencias y los movimientos. Con [ADR-014](ADR-014-limites-de-servicios.md) las transferencias pasaron a su propio servicio, y además no queremos que otro grupo pueda mover nuestro stock. El consumidor necesita consultar catálogo y disponibilidad, así que acotamos el contrato a eso.

## Decisión
Publicamos solamente consultas de Inventario:

| Operación | Ruta |
|---|---|
| Listar productos (paginado) | `GET /api/products` |
| Ver un producto | `GET /api/products/{productId}` |
| Ver stock disponible de una sucursal | `GET /api/branches/{branchId}/stock` |
| Ver stock disponible de un producto en una sucursal | `GET /api/branches/{branchId}/products/{productId}/stock` |

Además, `POST /api/auth/login` pertenece a Usuarios y permite obtener el token. Búsqueda avanzada y operaciones de reserva/confirmación/liberación no se agregan al contrato publicado.

El contrato está en [openapi.yaml](../contracts/openapi.yaml). Mientras no haya implementación, se prueba con Prism desde ese archivo. El mock solo muestra formas y ejemplos: no implementa seguridad ni reservas. La capacidad real deberá estar en una URL pública, con gateway y login accesibles, y mantenerse operativa hasta finalizar la evaluación.

### Autenticación
Usamos JWT como para nuestros usuarios. `ADMIN` administra y opera, `OPERATOR` consulta y crea transferencias, e `INTEGRACION` solo consulta el contrato. El grupo consumidor recibe una cuenta de ese último rol.

El token dura 1 hora y se envía en `Authorization: Bearer <token>`. Gateway y servicios aplican permisos. Al vencer hay que volver a hacer login. Credenciales por medio privado, nunca en Git.

### Stock publicado
`quantity` es **cantidad total menos cantidad reservada**, siempre no negativa. La pantalla muestra “stock disponible” y el filtro interno de stock bajo usa disponible ≤ 5 por sucursal. Reservar reduce el disponible de origen, sin acreditar destino. Confirmar tras aceptación de Rumbo mueve total en ambos extremos; liberar recupera disponible en origen.

Ejemplo: total 46 y reservado 4 devuelve `quantity: 42`. Las lecturas de stock consultan MySQL; no usan Memcached ni Solr. Una lectura no reserva ni garantiza disponibilidad para una operación futura. La ficha de producto puede usar caché con hasta 60 s de desfase, pero no incluye cantidades.

### Errores
Todos los errores usan código y mensaje:

| Código | Cuándo |
|---|---|
| `400` | Parámetros inválidos |
| `401` | Token ausente/inválido/vencido o credenciales de login incorrectas |
| `403` | Identidad válida sin permiso |
| `404` | Producto inexistente |
| `429` | Límite de tráfico excedido |
| `500` | Error interno |
| `503` | Dependencia necesaria no disponible |

Se conserva el comportamiento de stock por sucursal: sin registros devuelve lista vacía, y un producto existente sin registro en ese id de sucursal devuelve cero. Estas consultas no certifican que la sucursal exista; esa validación pertenece a Sucursales y a las operaciones internas. No se cambia ese comportamiento agregando un nuevo error de sucursal inexistente.

### Compatibilidad
No manejamos versiones del contrato. El `openapi.yaml` dice `version: 1.0.0` solo porque el formato OpenAPI exige ese campo. Para no romperle nada al otro grupo, solo hacemos cambios que no afecten lo que ya usa: podemos agregar operaciones nuevas o campos nuevos en las respuestas, pero no sacamos ni renombramos campos, ni cambiamos su tipo o su significado. Si alguna vez necesitáramos un cambio así, lo acordamos antes con el grupo consumidor.

**Observación separada, sin cambiar la política:** la consigna exige contrato formal y versionado. Esta política sigue pendiente de revisión con el equipo/cátedra. Además, si el consumidor entendía `quantity` como total, precisar que es disponible cambia su interpretación: hay que comunicárselo y validar sus casos antes de publicar. No se afirma compatibilidad semántica automática ni se cambia `1.0.0`.

## Alternativas que descartamos
- **Publicar toda la API.** Permitiría modificar cantidades y productos sin necesidad del consumidor.
- **No autenticar.** La URL pública dejaría el catálogo accesible a cualquiera.
- **API key adicional.** Agrega otro mecanismo cuando ya tenemos JWT.

## Consecuencias
- El consumidor no modifica datos; las consultas GET no requieren `Idempotency-Key`. Esa clave sí se requiere en el flujo interno de creación de transferencias, fuera de este contrato.
- El stock disponible puede cambiar después de leerlo.
- El grupo consumidor renueva el token con login y debe manejar errores y límites.
- Se mantienen rutas y campos existentes, con semántica de disponible explícita y observación de compatibilidad registrada.
