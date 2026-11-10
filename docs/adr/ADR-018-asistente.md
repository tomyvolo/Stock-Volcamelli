# ADR-018 — Asistente con MCP

## Estado
Aceptado.

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
El servidor MCP llama al gateway con **el token del usuario que está chateando**. El asistente no tiene permisos propios: puede hacer exactamente lo que puede hacer ese usuario, y se aplican los mismos controles que a cualquier otro pedido: el gateway controla el rol y Transferencias controla la sucursal. Por ejemplo, un vendedor no puede crear por chat una transferencia que no sale de su sucursal ni llega a ella.

Para eso, el gateway le pasa el header `Authorization` solo a la ruta `/api/assistant/*`. Los demás servicios siguen sin recibir el token, porque les alcanza con los datos del usuario que ya les manda el gateway.

### Confirmación antes de crear
`crear_transferencia` es la única herramienta que cambia algo. Cuando el modelo la pide, el Asistente no la ejecuta: `POST /api/assistant/chat` le devuelve al frontend la tarjeta con producto, cantidad, origen y destino. Solo si el usuario toca Confirmar, el frontend llama a `POST /api/assistant/confirm` con esos datos y el Asistente ejecuta la herramienta, sin volver a pasar por el modelo. Así un error del modelo nunca crea una transferencia que el usuario no vio.

El modelo pide la herramienta con ids, y con un id el usuario no puede controlar qué está confirmando. Por eso, antes de armar la tarjeta, el Asistente busca los nombres del producto y de las sucursales con las herramientas que ya existen. Si alguno no existe, no muestra la tarjeta y se lo avisa al usuario.

Si `POST /api/transfers` no responde a tiempo, puede ser que la transferencia se haya creado igual. En ese caso el chat no dice que falló: le pide al usuario que revise la lista de transferencias antes de volver a intentarlo.

### Tiempos
Un modelo puede tardar varios segundos, y cada pregunta puede dar hasta 5 vueltas. Por eso, en la ruta `/api/assistant/*` el gateway espera hasta 30 segundos, y no los 2 de las demás rutas. El servidor MCP espera hasta 6 segundos a cada llamada al gateway: un poco más que los 5 que el gateway le da a Transferencias para crear una transferencia, para recibir su respuesta en lugar de cortar antes. El Asistente, a su vez, espera 7 segundos al servidor MCP ([ADR-010](ADR-010-resiliencia.md)).

### Modelo
El Asistente habla con el modelo usando la API compatible con OpenAI, que ofrecen la mayoría de los proveedores. El proveedor, el modelo y la clave van en variables de entorno, así que cambiar de modelo no requiere tocar código:

| Variable | Qué es | Con Groq |
|---|---|---|
| `MODEL_URL` | La dirección de la API del proveedor | `https://api.groq.com/openai/v1` |
| `MODEL_NAME` | El modelo | `openai/gpt-oss-120b` |
| `MODEL_API_KEY` | La clave, que va solo en el `.env` | La que da la consola de Groq |

Usamos el plan gratis de Groq, tanto para desarrollar como para la demo, así probamos siempre lo mismo que vamos a mostrar. Si sus límites se quedan cortos, pasamos a otro proveedor compatible, como Gemini, cambiando esas variables.

### Conversación
El historial lo guarda el frontend y lo manda en cada mensaje (los últimos 20). El Asistente no guarda nada entre pedidos. Cada pregunta puede hacer como máximo 5 vueltas de herramientas; si el modelo no termina, responde que no pudo resolverlo.

### Cómo lo probamos
- Las herramientas del servidor MCP se prueban como cualquier código, sin IA: se llama a la herramienta y se compara el resultado.
- El ciclo del Asistente se prueba con un modelo falso que responde pedidos de herramientas armados de antemano, para comprobar que ejecuta lo que corresponde, que respeta el límite de vueltas y que nunca ejecuta `crear_transferencia` sin confirmación.

## Consecuencias
- Son dos servicios más (Asistente y servidor MCP), una ruta más en el gateway (`/api/assistant/*`) y una dependencia externa: el proveedor del modelo.
- El SDK de MCP pide Go 1.25, así que estos dos servicios usan esa versión y el resto sigue en 1.22. Cada servicio es un módulo aparte, así que no se afectan entre sí.
- Si el modelo no está disponible, el chat responde que el asistente no está disponible. El resto del sistema no se entera.
- El modelo puede entender mal una pregunta y dar una respuesta equivocada. Para consultas el riesgo es bajo, porque los datos salen de nuestra API; para crear, está la confirmación.
- El costo depende del proveedor y de cuánto se use. Para la demo entra en los planes gratuitos o cuesta muy poco ([ADR-013](ADR-013-capacidad-y-costos.md)).
- Nunca se le mandan al modelo contraseñas ni tokens: el token del usuario viaja del Asistente al servidor MCP, pero no forma parte de la conversación.
