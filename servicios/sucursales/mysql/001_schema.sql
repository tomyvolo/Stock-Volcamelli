-- La imagen oficial de MySQL ejecuta este archivo cuando el volumen se crea por primera vez.
-- El archivo está en UTF-8; sin esto MySQL lo lee como latin1 y guarda mal las tildes.
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS branches (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    -- Rumbo pide la dirección del origen y del destino en cada traslado.
    address VARCHAR(200) NOT NULL
);

-- Datos de prueba: las sucursales 1, 2 y 3, las mismas que usan el stock de Inventario y los usuarios.
INSERT INTO branches (id, name, address) VALUES
    (1, 'Córdoba Centro', 'Av. Colón 1250, Córdoba'),
    (2, 'Nueva Córdoba', 'Bv. San Juan 480, Córdoba'),
    (3, 'Villa Carlos Paz', 'Av. Libertad 320, Villa Carlos Paz');
