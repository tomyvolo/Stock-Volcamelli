# ADR-003 — Persistencia inicial

## Estado
Propuesto para la Entrega 1; pendiente de validación durante la implementación.

## Contexto
Los servicios necesitan persistir información con necesidades distintas. La tecnología final aún no fue elegida por el equipo y no corresponde presentar una elección como cerrada.

## Decisión inicial
- Cada microservicio tendrá su propio esquema y será el único responsable de leer y escribir sus datos.
- Para datos transaccionales de usuarios, sucursales e inventario se evaluará una base relacional, por las relaciones y restricciones de consistencia involucradas.
- Para notificaciones se evaluará persistencia documental o una tabla relacional independiente; el volumen y los patrones de consulta determinarán la elección.
- La solución final deberá incluir al menos una persistencia relacional y una no relacional, de acuerdo con el requisito del proyecto. La selección concreta, el motor y el despliegue quedan pendientes de una decisión del equipo.

## Justificación
La propiedad independiente de los datos sostiene los límites definidos en ADR-001. Aplazar el nombre del motor evita fijar una tecnología sin comparar necesidades reales ni restricciones del equipo.

## Alternativas consideradas
- Una base compartida por todos los servicios: descartada porque permite acoplamiento por esquema y acceso directo a datos ajenos.
- Fijar ahora motores concretos sin una prueba de implementación: diferida porque no hay evidencia de operación, volumen ni capacidades del equipo que justifique esa elección.

## Consecuencias y temas pendientes
- Los servicios no compartirán tablas ni credenciales de escritura.
- Las transacciones que cruzan servicios no se resolverán con una transacción distribuida; se definirán flujos de compensación cuando un caso real lo requiera.
- Antes de implementar se documentarán motores, versiones, copias de seguridad y migraciones, y se justificará cuál servicio usa persistencia no relacional.
