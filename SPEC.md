# Stock Volcamelli

## 1. Descripción

Stock Volcamelli es un sistema distribuido orientado a empresas con múltiples sucursales que permite administrar y consultar el inventario de sus establecimientos.

El sistema permite gestionar productos y sucursales, consultar el stock disponible por sucursal, realizar transferencias de productos entre establecimientos y generar notificaciones cuando el stock de un producto alcanza determinados niveles.

El dominio se centra principalmente en la gestión y transferencia de stock entre sucursales, donde existen reglas de negocio relacionadas con la disponibilidad del inventario y los niveles mínimos de stock.

---

## 2. Objetivo

El objetivo de Stock Volcamelli es proporcionar una plataforma que permita centralizar la gestión del inventario de una empresa con múltiples sucursales, manteniendo una visión del stock disponible en cada establecimiento y facilitando la transferencia de productos entre ellos.

El sistema busca permitir que los usuarios consulten el inventario disponible y puedan administrar las operaciones relacionadas con el movimiento de productos entre sucursales.

---

## 3. Alcance

La primera versión del sistema contempla:

- Gestión de usuarios.
- Gestión de roles de usuario.
- Gestión de sucursales.
- Gestión de productos.
- Consulta de stock por producto y sucursal.
- Gestión del stock de los productos en las sucursales.
- Transferencia de productos entre sucursales.
- Registro de movimientos de inventario.
- Generación de notificaciones relacionadas con niveles de stock.

Las operaciones y reglas específicas de las transferencias y movimientos de stock serán detalladas durante el diseño e implementación del sistema.

---

## 4. Actores

### Administrador

El administrador posee permisos para realizar tareas de administración dentro del sistema.

Entre sus responsabilidades se encuentran:

- Gestionar usuarios.
- Gestionar sucursales.
- Gestionar productos.
- Consultar información del inventario.

### Operador

El operador utiliza el sistema principalmente para trabajar con el inventario.

Entre sus responsabilidades se encuentran:

- Consultar productos.
- Consultar stock.
- Realizar transferencias de productos entre sucursales.
- Consultar sus notificaciones.

---

## 5. Microservicios

El sistema estará compuesto por cuatro microservicios de negocio.

### Servicio de Usuarios

Responsable de la gestión de usuarios y sus roles.

Es propietario de los datos relacionados con los usuarios.

Entidades principales:

- User
- Role

---

### Servicio de Sucursales

Responsable de la gestión de las sucursales de la empresa.

Es propietario de los datos relacionados con las sucursales.

Entidad principal:

- Branch

---

### Servicio de Inventario

Responsable de la gestión del inventario.

Es el microservicio encargado de administrar:

- Productos.
- Stock por sucursal.
- Movimientos de inventario.
- Transferencias de productos.

Entidades principales:

- Product
- Stock
- StockMovement
- Transfer

El Servicio de Inventario constituye una de las capacidades principales del sistema y será la capacidad publicada por el grupo para su integración con otros sistemas.

---

### Servicio de Notificaciones

Responsable de gestionar las notificaciones generadas por eventos relacionados con el inventario.

Entre las notificaciones contempladas se encuentran:

- `LOW_STOCK`: cuando el stock alcanza el nivel mínimo definido.
- `OUT_OF_STOCK`: cuando el stock llega a cero.

Entidad principal:

- Notification

---

## 6. Entidades principales

### User

Representa a un usuario del sistema.

Atributos iniciales:

- id
- name
- email
- role

Los roles definidos inicialmente son:

- ADMIN
- OPERATOR

---

### Branch

Representa una sucursal de la empresa.

Atributos iniciales:

- id
- name
- address

---

### Product

Representa un producto administrado por la empresa.

Atributos iniciales:

- id
- name
- description

---

### Notification

Representa una notificación relacionada con el estado del inventario.

Atributos iniciales:

- id
- userId
- branchId
- productId
- type
- message
- read
- createdAt

Los tipos iniciales de notificación son:

- LOW_STOCK
- OUT_OF_STOCK

---

## 7. Reglas de negocio iniciales

Las principales reglas de negocio consideradas para el sistema son:

1. Un usuario posee un rol dentro del sistema.
2. Un producto puede tener stock asociado a distintas sucursales.
3. Las operaciones de inventario deben mantener el stock válido.
4. Las transferencias de productos se realizan entre sucursales.
5. El stock no debe quedar en valores negativos.
6. Cuando el stock de un producto alcanza las 5 unidades, se genera una notificación `LOW_STOCK`.
7. Cuando el stock de un producto llega a 0 unidades, se genera una notificación `OUT_OF_STOCK`.

Estas reglas podrán ser refinadas durante el diseño e implementación, sin ampliar innecesariamente el alcance del sistema.

---

## 8. Comunicación entre servicios

Los microservicios se comunicarán mediante mecanismos síncronos y asíncronos según las necesidades de cada interacción.

Las operaciones que requieran una respuesta inmediata utilizarán comunicación síncrona.

Los cambios relevantes del inventario podrán generar eventos de dominio que serán comunicados de forma asíncrona al Servicio de Notificaciones.

El detalle de los mecanismos de comunicación, timeouts, reintentos, eventos e idempotencia será definido en la arquitectura y en los ADR correspondientes.

---

## 9. Capacidad publicada

El grupo publicará como capacidad de integración el **Servicio de Inventario**.

La capacidad permitirá que otros sistemas puedan utilizar las funcionalidades relacionadas con el inventario a través de un contrato formal y versionado.

El contrato será documentado en:

`docs/contracts/`

La definición detallada de las operaciones, entradas, resultados, errores, autenticación, idempotencia, versión y ejemplos será establecida en el contrato correspondiente.

---

## 10. Arquitectura general

El sistema contará con un API Gateway como punto de entrada para las aplicaciones cliente.

La arquitectura estará compuesta por:

- API Gateway.
- Servicio de Usuarios.
- Servicio de Sucursales.
- Servicio de Inventario.
- Servicio de Notificaciones.

Cada microservicio será responsable de sus propios datos y no accederá directamente a la base de datos de otro microservicio.

La arquitectura detallada y las dependencias entre componentes serán documentadas en `docs/ARCHITECTURE.md`.

---

## 11. Fuera de alcance

Para mantener un alcance acotado, la primera versión no contempla:

- Gestión de proveedores.
- Compras a proveedores.
- Facturación.
- Gestión de pedidos de compra.
- Logística externa.
- Gestión de envíos.
- Sistemas contables.
- Predicción automática de demanda.
- Reposición automática de productos.
- Reportes avanzados o analítica compleja.

Estas funcionalidades podrían considerarse como futuras extensiones del sistema, pero no forman parte del alcance actual.

---

## 12. Objetivo de la primera entrega

Para la primera entrega se documentará el diseño inicial del sistema, incluyendo:

- Alcance del dominio.
- Contexto del sistema.
- Límites y responsabilidades de los microservicios.
- Propiedad de los datos.
- Arquitectura inicial.
- Comunicación inicial entre servicios.
- Capacidad publicada por el grupo.
- Contrato inicial de integración.
- Estructura inicial del repositorio.
- Decisiones arquitectónicas iniciales mediante ADRs.
