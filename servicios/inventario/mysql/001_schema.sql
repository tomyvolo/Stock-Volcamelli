-- La imagen oficial de MySQL ejecuta este archivo cuando el volumen se crea por primera vez.
-- El archivo está en UTF-8; sin esto MySQL lo lee como latin1 y guarda mal las tildes.
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS products (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT NOT NULL,
    -- Solo informativo: a quién le compramos el producto. Puede quedar vacío.
    supplier VARCHAR(150) NULL
);

-- Un registro por producto y sucursal. El disponible es total - reserved.
-- Las sucursales son de otro servicio, así que acá solo guardamos su id.
CREATE TABLE IF NOT EXISTS stock (
    product_id BIGINT UNSIGNED NOT NULL,
    branch_id BIGINT UNSIGNED NOT NULL,
    total INT UNSIGNED NOT NULL DEFAULT 0,
    reserved INT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, branch_id),
    INDEX idx_stock_branch (branch_id),
    CONSTRAINT fk_stock_product FOREIGN KEY (product_id) REFERENCES products (id),
    CONSTRAINT chk_stock_reserved CHECK (reserved <= total)
);

-- Una reserva por transferencia, identificada por el id que le dio Transferencias.
-- Si un pedido de liberar llega antes que el de reservar, se guarda directo como LIBERADA,
-- sin producto ni sucursales, para rechazar la reserva si llega tarde.
CREATE TABLE IF NOT EXISTS reservations (
    transfer_id VARCHAR(64) NOT NULL PRIMARY KEY,
    product_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    origin_branch_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    destination_branch_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    quantity INT UNSIGNED NOT NULL DEFAULT 0,
    status ENUM('ACTIVA', 'CONFIRMADA', 'LIBERADA') NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Cada cambio en el total de stock deja un movimiento.
-- reason dice de dónde vino; description es el motivo que escribió quien hizo el ajuste.
CREATE TABLE IF NOT EXISTS stock_movements (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    product_id BIGINT UNSIGNED NOT NULL,
    branch_id BIGINT UNSIGNED NOT NULL,
    type ENUM('ENTRADA', 'SALIDA') NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    resulting_total INT UNSIGNED NOT NULL,
    reason ENUM('AJUSTE', 'TRANSFERENCIA') NOT NULL,
    description VARCHAR(200) NOT NULL DEFAULT '',
    transfer_id VARCHAR(64) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_movements_product_branch (product_id, branch_id, id)
);

-- Los eventos de Transferencias que ya procesamos. Si RabbitMQ entrega uno repetido, lo ignoramos.
CREATE TABLE IF NOT EXISTS processed_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    type VARCHAR(40) NOT NULL,
    processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Datos de prueba: 10 productos en las sucursales 1, 2 y 3.
-- Algunos quedan con 5 o menos disponibles para probar ?lowStock=true.
INSERT INTO products (id, name, description, supplier) VALUES
    (1, 'Harina 000', 'Harina de trigo 000, paquete de 1 kg', 'Molinos del Sur'),
    (2, 'Harina integral', 'Harina de trigo integral, paquete de 1 kg', 'Molinos del Sur'),
    (3, 'Azúcar', 'Azúcar blanca, paquete de 1 kg', 'Ingenio Norte'),
    (4, 'Yerba mate', 'Yerba mate con palo, paquete de 1 kg', 'Yerbatera Misionera'),
    (5, 'Aceite de girasol', 'Aceite de girasol, botella de 1,5 l', 'Aceitera Central'),
    (6, 'Arroz largo fino', 'Arroz largo fino, paquete de 1 kg', NULL),
    (7, 'Fideos tirabuzón', 'Fideos secos tirabuzón, paquete de 500 g', 'Pastas Don Luis'),
    (8, 'Leche entera', 'Leche entera larga vida, caja de 1 l', 'Tambo Serrano'),
    (9, 'Café molido', 'Café molido tostado, paquete de 250 g', NULL),
    (10, 'Sal fina', 'Sal fina de mesa, paquete de 500 g', 'Salinas Grandes');

INSERT INTO stock (product_id, branch_id, total, reserved) VALUES
    (1, 1, 46, 4),
    (1, 2, 20, 0),
    (1, 3, 5, 0),
    (2, 1, 12, 0),
    (2, 3, 3, 0),
    (3, 1, 30, 0),
    (3, 2, 8, 4),
    (4, 1, 25, 0),
    (4, 2, 2, 0),
    (4, 3, 18, 0),
    (5, 1, 10, 0),
    (5, 2, 15, 0),
    (6, 2, 40, 0),
    (6, 3, 6, 0),
    (7, 1, 22, 0),
    (7, 3, 4, 0),
    (8, 1, 0, 0),
    (8, 2, 35, 0),
    (9, 3, 9, 0),
    (10, 1, 50, 0),
    (10, 2, 12, 0),
    (10, 3, 1, 0);

-- Las 4 unidades reservadas de Harina 000 en la sucursal 1 son de esta transferencia de ejemplo a la sucursal 2.
INSERT INTO reservations (transfer_id, product_id, origin_branch_id, destination_branch_id, quantity, status) VALUES
    ('ejemplo-0001', 1, 1, 2, 4, 'ACTIVA');
