-- La imagen oficial de MySQL ejecuta este archivo cuando el volumen se crea por primera vez.
CREATE TABLE IF NOT EXISTS usuarios (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    email VARCHAR(150) NOT NULL UNIQUE,
    password_hash VARCHAR(100) NOT NULL,
    rol ENUM('DIRECTOR', 'ADMIN_SUCURSAL', 'VENDEDOR', 'INTEGRACION') NOT NULL,
    -- Obligatoria para ADMIN_SUCURSAL y VENDEDOR; vacía para DIRECTOR e INTEGRACION.
    sucursal_id BIGINT UNSIGNED NULL,
    INDEX idx_usuarios_sucursal (sucursal_id)
);

-- TODO(Juan): datos de prueba, un usuario de cada rol (el admin y el vendedor en la sucursal 1).
-- password_hash es el hash bcrypt de la contraseña (costo 12, como utils/hash.go de DSW).
-- Las contraseñas no van en el repositorio: se generan los hashes y se pegan acá.
