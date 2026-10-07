# Arquitectura de Stock Volcamelli

## 1. Descripción general

Stock Volcamelli utiliza una **arquitectura distribuida basada en microservicios**, orientada a la gestión de inventario de empresas con múltiples sucursales.

El sistema está compuesto por un **Frontend Web**, un **API Gateway** y cuatro microservicios de negocio:

- Servicio de Usuarios
- Servicio de Sucursales
- Servicio de Inventario
- Servicio de Notificaciones

El Frontend Web actúa como cliente del sistema y se comunica con los servicios a través del API Gateway.

Cada microservicio posee una responsabilidad específica dentro del dominio y es responsable de la información que administra. Los servicios no acceden directamente a la persistencia perteneciente a otros servicios.

La arquitectura busca separar las responsabilidades del dominio, permitir la evolución independiente de los servicios y facilitar la integración con otros sistemas.

## 2. Componentes principales

### 2.1 Frontend Web

El Frontend Web proporciona la interfaz mediante la cual los usuarios interactúan con Stock Volcamelli.

Desde el frontend se podrán realizar operaciones como:

- Consultar productos.
- Consultar stock.
- Consultar sucursales.
- Realizar transferencias de productos.
- Consultar notificaciones.
- Administrar usuarios y sucursales según los permisos correspondientes.

El frontend no accede directamente a los microservicios ni a sus bases de datos. Las solicitudes se realizan a través del API Gateway.

### 2.2 API Gateway

El API Gateway funciona como punto de entrada al sistema para el Frontend Web.

Sus responsabilidades principales son:

- Recibir las solicitudes del frontend.
- Enrutar cada solicitud hacia el microservicio correspondiente.
- Centralizar aspectos comunes de acceso a los servicios.
- Evitar que el frontend necesite conocer la ubicación o estructura interna de cada microservicio.

El Gateway no contiene la lógica principal del dominio. Las operaciones de negocio son responsabilidad de los microservicios correspondientes.

### 2.3 Servicio de Usuarios

El Servicio de Usuarios es responsable de la gestión de los usuarios y sus roles.

Sus principales responsabilidades son:

- Gestionar usuarios.
- Gestionar los roles de los usuarios.
- Proporcionar la información necesaria para determinar los permisos dentro del sistema.

Los roles definidos inicialmente son:

- `ADMIN`
- `OPERATOR`

El Servicio de Usuarios es propietario de los datos relacionados con los usuarios y roles.

### 2.4 Servicio de Sucursales

El Servicio de Sucursales es responsable de la gestión de las sucursales de la empresa.

Sus principales responsabilidades son:

- Crear y administrar sucursales.
- Consultar información de las sucursales.
- Proporcionar información de las sucursales a otros servicios cuando sea necesario.

El Servicio de Sucursales es propietario de los datos relacionados con las sucursales.

### 2.5 Servicio de Inventario

El Servicio de Inventario es el servicio principal del dominio de inventario.

Sus responsabilidades incluyen:

- Gestionar productos.
- Gestionar el stock de los productos por sucursal.
- Registrar movimientos de inventario.
- Realizar transferencias de productos entre sucursales.
- Validar la disponibilidad de stock.
- Mantener las reglas de consistencia relacionadas con el inventario.
- Generar eventos relacionados con cambios relevantes del stock.

El Servicio de Inventario es propietario de los datos relacionados con:

- Productos.
- Stock.
- Movimientos de inventario.
- Transferencias.

El Servicio de Inventario constituye además la **capacidad publicada por el grupo** para la integración con otros grupos.

### 2.6 Servicio de Notificaciones

El Servicio de Notificaciones es responsable de gestionar las notificaciones generadas por eventos relevantes del sistema.

Inicialmente se contemplan los siguientes tipos:

- `LOW_STOCK`
- `OUT_OF_STOCK`

Las notificaciones de stock se generan a partir de eventos producidos por el Servicio de Inventario.

El Servicio de Notificaciones es propietario de los datos relacionados con las notificaciones.

## 3. Arquitectura interna de los microservicios

Los microservicios no utilizarán necesariamente el mismo patrón de arquitectura interna. La elección depende de la complejidad y las responsabilidades de cada servicio.

### Servicio de Usuarios

Utilizará una **arquitectura en capas**.

La separación principal será:

- Capa de presentación.
- Capa de aplicación/servicio.
- Capa de persistencia.

Este patrón resulta adecuado debido a que Servicio de Usuarios posee responsabilidades relativamente simples relacionadas con la gestión de usuarios y roles.

### Servicio de Sucursales

Utilizará una **arquitectura en capas**.

La estructura seguirá una separación entre:

- Capa de presentación.
- Capa de aplicación/servicio.
- Capa de persistencia.

Servicio de Sucursales presenta una lógica de negocio relativamente sencilla, por lo que no se considera necesario introducir una estructura arquitectónica más compleja.

### Servicio de Inventario

Utilizará una **arquitectura hexagonal**.

Esta elección responde a que Servicio de Inventario concentra las principales reglas de negocio del sistema y necesita interactuar con diferentes mecanismos externos, entre ellos:

- Persistencia.
- Comunicación con otros microservicios.
- Mensajería y eventos.

La arquitectura hexagonal permitirá mantener separada la lógica de dominio de los mecanismos externos utilizados por el servicio.

### Servicio de Notificaciones

Utilizará una **arquitectura en capas**.

El servicio tendrá una separación entre:

- Capa de presentación/entrada.
- Capa de aplicación.
- Capa de persistencia.

La lógica del servicio se mantiene relativamente simple, por lo que una arquitectura en capas resulta suficiente.

De esta manera, el sistema utiliza al menos dos patrones de arquitectura interna diferentes: **arquitectura en capas y arquitectura hexagonal**.

## 4. Propiedad de los datos

Cada microservicio es responsable de sus propios datos.

| Microservicio | Datos de los que es propietario |
|---|---|
| Servicio de Usuarios | Usuarios y roles |
| Servicio de Sucursales | Sucursales |
| Servicio de Inventario | Productos, stock, movimientos y transferencias |
| Servicio de Notificaciones | Notificaciones |

Ningún microservicio accederá directamente a la base de datos de otro microservicio.

Cuando un servicio necesite información perteneciente a otro servicio, utilizará la comunicación definida entre ambos.

## 5. Comunicación entre servicios

Los diagramas de contexto y contenedores están en diagrams/. Las decisiones correspondientes se registran en adr/ y el contrato publicado en contracts/.

La arquitectura contempla comunicación tanto **síncrona como asíncrona**.

### 5.1 Comunicación síncrona

Se utilizará comunicación síncrona cuando una operación necesite una respuesta inmediata de otro servicio.

Un caso previsto es la comunicación entre Servicio de Inventario y Servicio de Sucursales para validar información de las sucursales involucradas en operaciones de inventario.

Las comunicaciones síncronas deberán contemplar posteriormente mecanismos de timeout y manejo de errores.

### 5.2 Comunicación asíncrona

Se utilizará comunicación asíncrona para eventos que no requieran que el servicio emisor espere una respuesta inmediata.

El principal flujo previsto es:

```text
Servicio de Inventario
       |
       | Evento de cambio de stock
       v
Message Broker
       |
       v
Servicio de Notificaciones
