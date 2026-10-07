# Entrega 1 — Definiciones iniciales

## 1. Límites preliminares de los servicios

Stock Volcamelli se divide inicialmente en cuatro microservicios de negocio. Cada servicio tiene una responsabilidad específica y es propietario de los datos correspondientes a su ámbito.

### Servicio de Usuarios

**Responsabilidad**

Gestiona los usuarios y sus roles dentro del sistema.

Incluye:

- Alta y gestión de usuarios.
- Consulta de usuarios.
- Asignación y consulta de roles.

**Datos propios**

- Usuario.
- Rol.

**Roles definidos**

- `ADMIN`
- `OPERATOR`

---

### Servicio de Sucursales

**Responsabilidad**

Gestiona las sucursales pertenecientes a la empresa.

Incluye:

- Alta de sucursales.
- Modificación de sucursales.
- Consulta de sucursales.

**Datos propios**

- Sucursal.

---

### Servicio de Inventario

**Responsabilidad**

Gestiona el inventario de productos en las distintas sucursales.

Incluye:

- Gestión de productos.
- Consulta y gestión del stock.
- Registro de movimientos de inventario.
- Transferencias de productos entre sucursales.
- Validación de disponibilidad de stock.
- Aplicación de las reglas de consistencia del inventario.

**Datos propios**

- Producto.
- Stock.
- Movimiento de inventario.
- Transferencia.

El Servicio de Inventario es el servicio que concentra las principales reglas de negocio del dominio.

---

### Servicio de Notificaciones

**Responsabilidad**

Gestiona las notificaciones generadas por eventos relevantes del sistema, principalmente relacionados con los niveles de stock.

Incluye:

- Generación de notificaciones.
- Consulta de notificaciones.
- Marcado de notificaciones como leídas.

**Datos propios**

- Notificación.

Tipos de notificación iniciales:

- `LOW_STOCK`
- `OUT_OF_STOCK`

---

## 2. Propiedad de los datos

La propiedad de los datos queda definida de la siguiente manera:

| Servicio | Datos de los que es propietario |
|---|---|
| Servicio de Usuarios | Usuarios y roles |
| Servicio de Sucursales | Sucursales |
| Servicio de Inventario | Productos, stock, movimientos y transferencias |
| Servicio de Notificaciones | Notificaciones |

Cada servicio administra exclusivamente su propia persistencia.

Ningún microservicio accederá directamente a la base de datos perteneciente a otro microservicio.

Cuando un servicio necesite información que pertenece a otro servicio, utilizará la comunicación definida entre ellos.

---

# 3. Capacidad propia a exponer

La capacidad seleccionada para ser expuesta a otro grupo es el:

> **Servicio de Inventario**

Se decidió exponer el servicio completo como capacidad de integración, en lugar de limitar la integración a una única consulta.

La capacidad publicada comprende las operaciones relacionadas con:

- Productos.
- Consulta de stock.
- Gestión del inventario.
- Movimientos de inventario.
- Transferencias entre sucursales.

Esta capacidad resulta representativa del dominio de Stock Volcamelli y permite que otro grupo pueda utilizar funcionalidades reales del sistema.

## Contrato

La capacidad será publicada mediante un contrato formal y versionado.

El contrato definirá como mínimo:

- Operaciones disponibles.
- Datos de entrada.
- Resultados.
- Errores.
- Autenticación.
- Idempotencia cuando corresponda.
- Versión.
- Ejemplos de utilización.

La documentación del contrato se ubicará en:

```text
docs/contracts/
```

La capacidad deberá ser utilizada posteriormente por otro grupo dentro de un flujo de negocio relevante, de acuerdo con los requisitos de integración de la asignatura.

---

# 4. Estructura inicial de los servicios

El proyecto utilizará un único repositorio con los cuatro microservicios.

```text
Stock-Volcamelli/
├── gateway/
└── services/
    ├── servicio-usuarios/
    ├── servicio-sucursales/
    ├── servicio-inventario/
    └── servicio-notificaciones/
```

La estructura interna inicial será diferente según las necesidades de cada servicio.

### Servicio de Usuarios

Arquitectura en capas:

```text
servicio-usuarios/
├── presentacion/
├── aplicacion/
└── persistencia/
```

### Servicio de Sucursales

Arquitectura en capas:

```text
servicio-sucursales/
├── presentacion/
├── aplicacion/
└── persistencia/
```

### Servicio de Inventario

Arquitectura hexagonal:

```text
servicio-inventario/
├── dominio/
├── aplicacion/
├── entrada/
└── infraestructura/
```

### Servicio de Notificaciones

Arquitectura en capas:

```text
servicio-notificaciones/
├── presentacion/
├── aplicacion/
└── persistencia/
```

Esta distribución permite contar con dos enfoques de arquitectura interna:

- Arquitectura en capas.
- Arquitectura hexagonal.

---

# 5. Dependencias iniciales

Las dependencias entre los servicios se definen inicialmente de la siguiente manera:

```text
Interfaz Web
     │
     ▼
Puerta de Enlace API
     │
     ├──────────► Servicio de Usuarios
     │
     ├──────────► Servicio de Sucursales
     │
     ├──────────► Servicio de Inventario
     │
     └──────────► Servicio de Notificaciones

Servicio de Inventario
     │
     ├──────────► Servicio de Sucursales
     │
     └──────────► Gestor de Mensajes
                         │
                         ▼
                  Servicio de Notificaciones
```

### Dependencias principales

**Interfaz Web → Puerta de Enlace API**

La interfaz utiliza la puerta de enlace como punto de entrada al sistema.

**Puerta de Enlace API → Microservicios**

La puerta de enlace enruta las solicitudes hacia el microservicio responsable de cada operación.

**Servicio de Inventario → Servicio de Sucursales**

El Servicio de Inventario podrá consultar información de las sucursales cuando una operación de inventario necesite validar los establecimientos involucrados.

Esta comunicación será inicialmente síncrona.

**Servicio de Inventario → Gestor de Mensajes**

El Servicio de Inventario publicará eventos relacionados con cambios relevantes del stock.

**Gestor de Mensajes → Servicio de Notificaciones**

El Servicio de Notificaciones consumirá los eventos publicados por el Servicio de Inventario.

Esta comunicación será asíncrona.

---

# 6. D1 — Límites de servicios y propiedad de datos

## Contexto

La división del sistema en microservicios requiere establecer límites claros de responsabilidad y de propiedad de los datos.

## Decisión

Se establecen cuatro límites preliminares:

| Servicio | Responsabilidad | Propiedad |
|---|---|---|
| Servicio de Usuarios | Gestión de usuarios y roles | Usuarios y roles |
| Servicio de Sucursales | Gestión de sucursales | Sucursales |
| Servicio de Inventario | Gestión del inventario | Productos, stock, movimientos y transferencias |
| Servicio de Notificaciones | Gestión de notificaciones | Notificaciones |

Cada servicio será responsable de la información que pertenece a su dominio.

No se permitirá el acceso directo a la persistencia de otro servicio.

Las interacciones entre servicios se realizarán mediante las interfaces y mecanismos de comunicación definidos para cada caso.

## Justificación

Esta división permite:

- Mantener responsabilidades claramente delimitadas.
- Evitar bases de datos compartidas entre servicios.
- Mantener la propiedad de los datos.
- Aislar las reglas principales del inventario dentro del Servicio de Inventario.
- Facilitar la evolución independiente de los servicios.

---

# 7. D8 — Capacidad propia a exponer

## Decisión

La capacidad propia seleccionada para ser publicada es el **Servicio de Inventario**.

La capacidad permitirá a otro grupo consumir funcionalidades relacionadas con:

- Productos.
- Stock.
- Movimientos.
- Transferencias.

La capacidad será expuesta mediante un contrato versionado.

## Motivo

El Servicio de Inventario representa la parte central del dominio de Stock Volcamelli y contiene operaciones con reglas de negocio relevantes.

Por este motivo, resulta más útil como capacidad de integración que una operación aislada de consulta.

## Contrato

El contrato deberá documentar:

- Versión.
- Operaciones.
- Entradas.
- Resultados.
- Errores.
- Autenticación.
- Idempotencia.
- Ejemplos.

El contrato se ubicará dentro de:

```text
docs/contracts/
```

---

# 8. D3 — Persistencia inicial

## Decisión inicial

Cada microservicio tendrá su propia persistencia y será responsable de los datos que posee.

No se utilizará una base de datos compartida entre los servicios.

La distribución inicial de datos será:

| Servicio | Datos |
|---|---|
| Servicio de Usuarios | Usuarios y roles |
| Servicio de Sucursales | Sucursales |
| Servicio de Inventario | Productos, stock, movimientos y transferencias |
| Servicio de Notificaciones | Notificaciones |

La tecnología concreta de cada persistencia se definirá durante la implementación.

Como decisión inicial, la arquitectura contempla la utilización de diferentes tipos de almacenamiento según las necesidades de cada servicio.

En particular, la solución final deberá contar con al menos:

- Una persistencia relacional.
- Una persistencia no relacional.

La elección concreta será documentada y justificada en una versión posterior de esta decisión.

## Motivo

La separación de persistencias permite mantener la propiedad de los datos establecida en D1 y evita que un servicio dependa de la estructura interna de almacenamiento de otro.

Esta es una versión inicial de D3 para la primera entrega; no se fija todavía una tecnología concreta que no haya sido decidida por el equipo.

---

# 9. D5 — Comunicación inicial

## Decisión inicial

Stock Volcamelli utilizará dos formas de comunicación entre microservicios:

- Comunicación síncrona.
- Comunicación asíncrona mediante eventos.

La elección dependerá de si la operación necesita una respuesta inmediata o puede continuar mediante procesamiento posterior.

## Comunicación síncrona

El caso inicial definido es:

```text
Servicio de Inventario
        │
        │ solicitud
        ▼
Servicio de Sucursales
        │
        │ respuesta
        ▼
Servicio de Inventario
```

Esta comunicación se utilizará cuando el Servicio de Inventario necesite validar información de una sucursal antes de continuar una operación.

La comunicación deberá contemplar posteriormente:

- Timeout.
- Manejo de errores.
- Reintentos cuando corresponda.
- Tratamiento de respuestas tardías.

## Comunicación asíncrona

El caso inicial definido es el envío de eventos de cambios relevantes de stock:

```text
Servicio de Inventario
        │
        │ evento
        ▼
Gestor de Mensajes
        │
        │ entrega
        ▼
Servicio de Notificaciones
```

El Servicio de Inventario será responsable de publicar el evento.

El Servicio de Notificaciones será responsable de consumirlo y generar la notificación correspondiente.

Los eventos relacionados con el stock podrán producir notificaciones como:

- `LOW_STOCK`
- `OUT_OF_STOCK`

El consumidor deberá contemplar posteriormente mecanismos de idempotencia y tratamiento de mensajes que no puedan procesarse.

## Motivo

La comunicación síncrona resulta adecuada cuando el resultado de otro servicio es necesario para continuar una operación.

La comunicación asíncrona permite desacoplar la generación de eventos de la gestión de notificaciones y evita que el Servicio de Inventario tenga que esperar a que el Servicio de Notificaciones termine su procesamiento.

Esta es una versión inicial de D5. Los timeouts, reintentos, idempotencia y tratamiento de errores serán definidos con mayor detalle durante la implementación.
