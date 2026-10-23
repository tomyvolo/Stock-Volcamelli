-- La imagen oficial de MySQL ejecuta este archivo cuando el volumen se crea por primera vez.
-- El archivo está en UTF-8; sin esto MySQL lo lee como latin1 y guarda mal las tildes.
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(150) NOT NULL UNIQUE,
    password_hash VARCHAR(100) NOT NULL,
    role ENUM('DIRECTOR', 'ADMIN_SUCURSAL', 'VENDEDOR', 'INTEGRACION') NOT NULL,
    -- Obligatoria para ADMIN_SUCURSAL y VENDEDOR; vacía para DIRECTOR e INTEGRACION.
    branch_id BIGINT UNSIGNED NULL,
    INDEX idx_users_branch (branch_id)
);

-- Datos de prueba: un usuario de cada rol. El admin y el vendedor son de la sucursal 1.
-- password_hash es el hash bcrypt (costo 12) de la contraseña. Las contraseñas no están
-- en el repositorio: las tiene el equipo.
INSERT INTO users (id, name, email, password_hash, role, branch_id) VALUES
    (1, 'Director General', 'director@volcamelli.com', '$2a$12$4RXLqJMPti3z60y0bLDNM.gqW8m03Ni.6CFX9gAFrcAgFrNDGmU8O', 'DIRECTOR', NULL),
    (2, 'Admin Sucursal 1', 'admin1@volcamelli.com', '$2a$12$FLdEODe/tNs3e0p2g07HyuOpISGEQ4IBeQGVjbgopJIs8DIaNJ3Ka', 'ADMIN_SUCURSAL', 1),
    (3, 'Vendedor Sucursal 1', 'vendedor1@volcamelli.com', '$2a$12$G0M28RxbOwlI1GLeXFWmteRk7cD8qUYPuXGTyl37iZwWNq2NFcW06', 'VENDEDOR', 1),
    (4, 'Integración', 'integracion@volcamelli.com', '$2a$12$z7bDaswJQ1PRKSNjupuTtOirXKTlTwGCmmj11ApTdeZPSQ6b.JoCC', 'INTEGRACION', NULL);
