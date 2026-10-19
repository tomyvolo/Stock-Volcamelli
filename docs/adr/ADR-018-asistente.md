# ADR-018 — Asistente con MCP

## Estado
Propuesto.

## Contexto
Muchas tareas del usuario son preguntas rápidas que hoy requieren navegar varias pantallas: "¿cuánta harina queda en Córdoba Centro?", "¿qué productos tienen stock bajo en Nueva Córdoba?", "¿en qué estado está la transferencia de ayer?". Queremos que pueda hacerlas escribiendo, desde un chat en el frontend, y que el asistente también pueda crear una transferencia, siempre con su confirmación.

Para que un modelo de lenguaje opere el sistema usamos MCP (Model Context Protocol), un protocolo estándar para exponerle herramientas a un modelo. Así las operaciones del sistema quedan definidas una sola vez y cualquier cliente que hable MCP las puede usar.

## Decisión

### Piezas
| Pieza | Qué hace |
|---|---|
| **Chat en el frontend** | Una burbuja abajo a la derecha. Muestra la conversación y, cuando el asistente quiere crear una transferencia, una tarjeta con los datos y los botones Confirmar y Cancelar |
| **Servicio Asistente** (Go) | Recibe el mensaje, se lo manda al modelo junto con la lista de herramientas, ejecuta las herramientas que el modelo pide y le devuelve los resultados, hasta que el modelo responde con texto |
| **Servidor MCP** (Go, con el SDK oficial de MCP para Go) | Expone las herramientas. No tiene nada de IA: cada herramienta es una llamada a nuestra API a través del gateway |

### Herramientas
| Herramienta | Qué hace | Llama a |
|---|---|---|
| `buscar_producto` | Busca productos por texto | `GET /api/products/search` |
| `listar_sucursales` | Lista las sucursales con su id, nombre y dirección | `GET /api/branches` |
| `usuarios_sucursal` | Quiénes trabajan en una sucursal, con su email y su rol | `GET /api/users?branchId={id}` |
| `consultar_stock` | Stock disponible de un producto en una sucursal, o de toda la sucursal | `GET /api/branches/{id}/stock` y `.../products/{id}/stock` |
| `stock_bajo` | Productos con stock bajo en una sucursal | `GET /api/branches/{id}/stock?lowStock=true` |
| `crear_transferencia` | Crea una transferencia | `POST /api/transfers` |
| `estado_transferencia` | Estado y detalle de una transferencia | `GET /api/transfers/{id}` |

### Permisos
El servidor MCP llama al gateway con **el token del usuario que está chateando**. El asistente no tiene permisos propios: puede hacer exactamente lo que puede hacer ese usuario, y se aplican los mismos controles que a cualquier otro pedido: el gateway controla el rol y Transferencias controla la sucursal. Por ejemplo, un vendedor no puede crear por chat una transferencia que no sale de su sucursal ni llega a ella. La cuenta `INTEGRACION` no puede usar el asistente.

### Confirmación antes de crear
`crear_transferencia` es la única herramienta que cambia algo. Cuando el modelo la pide, el Asistente no la ejecuta: le devuelve al frontend la tarjeta con producto, cantidad, origen y destino. Solo si el usuario toca Confirmar, el Asistente ejecuta la herramienta. Así un error del modelo nunca crea una transferencia que el usuario no vio.

### Modelo
El Asistente habla con el modelo usando la API compatible con OpenAI, que ofrecen la mayoría de los proveedores. El proveedor, el modelo y la clave van en variables de entorno, así que cambiar de modelo no requiere tocar código:

- **Desarrollo:** un modelo local con Ollama, sin costo.
- **Demo:** un modelo en la nube (Groq, Gemini o Claude Haiku), que responde más rápido y elige mejor las herramientas.

### Conversación
El historial lo guarda el frontend y lo manda en cada mensaje (los últimos 20). El Asistente no guarda nada entre pedidos. Cada pregunta puede hacer como máximo 5 vueltas de herramientas; si el modelo no termina, responde que no pudo resolverlo.

### Cómo lo probamos
- Las herramientas del servidor MCP se prueban como cualquier código, sin IA: se llama a la herramienta y se compara el resultado.
- El ciclo del Asistente se prueba con un modelo falso que responde pedidos de herramientas armados de antemano, para comprobar que ejecuta lo que corresponde, que respeta el límite de vueltas y que nunca ejecuta `crear_transferencia` sin confirmación.

## Alternativas que descartamos
- **Solo el servidor MCP, para usarlo desde un cliente externo** (Claude Desktop o una terminal). Era la mitad del trabajo, pero el usuario trabaja en nuestro frontend y no va a instalar otro programa. Igual el servidor MCP queda disponible: cualquier cliente MCP lo puede usar con un token válido.

## Consecuencias
- Son dos servicios más (Asistente y servidor MCP), una ruta más en el gateway (`/api/assistant/*`) y una dependencia externa: el proveedor del modelo.
- Si el modelo no está disponible, el chat responde que el asistente no está disponible. El resto del sistema no se entera.
- El modelo puede entender mal una pregunta y dar una respuesta equivocada. Para consultas el riesgo es bajo, porque los datos salen de nuestra API; para crear, está la confirmación.
- El costo depende del proveedor y de cuánto se use. Para la demo entra en los planes gratuitos o cuesta muy poco ([ADR-013](ADR-013-capacidad-y-costos.md)).
- Nunca se le mandan al modelo contraseñas ni tokens: el token del usuario viaja del Asistente al servidor MCP, pero no forma parte de la conversación.
