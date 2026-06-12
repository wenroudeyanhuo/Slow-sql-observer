-- Demo business database for Slow SQL Observer local testing.
-- This script creates a standalone application-like schema, seeds data,
-- and leaves a few query examples at the bottom for manual slow-log testing.

CREATE DATABASE IF NOT EXISTS sso_demo_app
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE sso_demo_app;

CREATE TABLE IF NOT EXISTS customers (
  id BIGINT NOT NULL AUTO_INCREMENT,
  customer_no VARCHAR(32) NOT NULL,
  customer_name VARCHAR(128) NOT NULL,
  email VARCHAR(255) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  city VARCHAR(64) NOT NULL,
  customer_status VARCHAR(32) NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_customers_customer_no (customer_no),
  UNIQUE KEY uk_customers_email (email)
);

CREATE TABLE IF NOT EXISTS products (
  id BIGINT NOT NULL AUTO_INCREMENT,
  sku VARCHAR(64) NOT NULL,
  product_name VARCHAR(255) NOT NULL,
  category_name VARCHAR(64) NOT NULL,
  unit_price DECIMAL(12,2) NOT NULL,
  stock_qty INT NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_products_sku (sku)
);

CREATE TABLE IF NOT EXISTS orders (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(32) NOT NULL,
  customer_id BIGINT NOT NULL,
  order_status VARCHAR(32) NOT NULL,
  payment_status VARCHAR(32) NOT NULL,
  total_amount DECIMAL(12,2) NOT NULL,
  created_at DATETIME NOT NULL,
  paid_at DATETIME NULL,
  shipped_at DATETIME NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_orders_order_no (order_no)
);

CREATE TABLE IF NOT EXISTS order_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_id BIGINT NOT NULL,
  product_id BIGINT NOT NULL,
  quantity INT NOT NULL,
  unit_price DECIMAL(12,2) NOT NULL,
  line_amount DECIMAL(12,2) NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS page_visit_logs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  biz_date DATE NOT NULL,
  user_id BIGINT NOT NULL,
  page_name VARCHAR(128) NOT NULL,
  device_type VARCHAR(32) NOT NULL,
  ip_address VARCHAR(64) NOT NULL,
  referer_url VARCHAR(255) NOT NULL,
  stay_seconds INT NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id)
);

DROP PROCEDURE IF EXISTS seed_sso_demo_app;

DELIMITER $$

CREATE PROCEDURE seed_sso_demo_app()
BEGIN
  DECLARE i INT DEFAULT 1;
  DECLARE j INT DEFAULT 1;
  DECLARE order_count INT DEFAULT 5000;
  DECLARE customer_count INT DEFAULT 1200;
  DECLARE product_count INT DEFAULT 300;
  DECLARE page_log_count INT DEFAULT 40000;
  DECLARE selected_customer BIGINT;
  DECLARE selected_product BIGINT;
  DECLARE item_qty INT;
  DECLARE item_price DECIMAL(12,2);
  DECLARE order_total DECIMAL(12,2);
  DECLARE item_loop INT;

  IF (SELECT COUNT(*) FROM customers) = 0 THEN
    SET i = 1;
    WHILE i <= customer_count DO
      INSERT INTO customers (
        customer_no,
        customer_name,
        email,
        phone,
        city,
        customer_status,
        created_at,
        updated_at
      ) VALUES (
        CONCAT('C', LPAD(i, 6, '0')),
        CONCAT('Customer ', i),
        CONCAT('customer', i, '@demo.local'),
        CONCAT('138', LPAD(i, 8, '0')),
        ELT(1 + MOD(i, 8), 'Shanghai', 'Beijing', 'Shenzhen', 'Guangzhou', 'Hangzhou', 'Chengdu', 'Wuhan', 'Xian'),
        ELT(1 + MOD(i, 4), 'active', 'active', 'active', 'inactive'),
        NOW() - INTERVAL MOD(i, 720) DAY,
        NOW() - INTERVAL MOD(i, 180) DAY
      );
      SET i = i + 1;
    END WHILE;
  END IF;

  IF (SELECT COUNT(*) FROM products) = 0 THEN
    SET i = 1;
    WHILE i <= product_count DO
      INSERT INTO products (
        sku,
        product_name,
        category_name,
        unit_price,
        stock_qty,
        created_at
      ) VALUES (
        CONCAT('SKU', LPAD(i, 6, '0')),
        CONCAT('Product ', i),
        ELT(1 + MOD(i, 6), 'phone', 'laptop', 'book', 'snack', 'toy', 'home'),
        ROUND(30 + RAND() * 9000, 2),
        10 + MOD(i * 7, 300),
        NOW() - INTERVAL MOD(i, 365) DAY
      );
      SET i = i + 1;
    END WHILE;
  END IF;

  IF (SELECT COUNT(*) FROM orders) = 0 THEN
    SET i = 1;
    WHILE i <= order_count DO
      SET selected_customer = 1 + MOD(i * 13, customer_count);
      SET order_total = 0;

      INSERT INTO orders (
        order_no,
        customer_id,
        order_status,
        payment_status,
        total_amount,
        created_at,
        paid_at,
        shipped_at
      ) VALUES (
        CONCAT('O', LPAD(i, 8, '0')),
        selected_customer,
        ELT(1 + MOD(i, 5), 'created', 'paid', 'shipped', 'completed', 'closed'),
        ELT(1 + MOD(i, 4), 'pending', 'paid', 'paid', 'refunded'),
        0,
        NOW() - INTERVAL MOD(i, 180) DAY - INTERVAL MOD(i * 19, 86400) SECOND,
        NOW() - INTERVAL MOD(i, 170) DAY,
        NOW() - INTERVAL MOD(i, 160) DAY
      );

      SET item_loop = 1;
      WHILE item_loop <= 4 DO
        SET selected_product = 1 + MOD((i * 17) + (item_loop * 11), product_count);
        SELECT unit_price INTO item_price FROM products WHERE id = selected_product;
        SET item_qty = 1 + MOD(i + item_loop, 5);

        INSERT INTO order_items (
          order_id,
          product_id,
          quantity,
          unit_price,
          line_amount,
          created_at
        ) VALUES (
          i,
          selected_product,
          item_qty,
          item_price,
          item_qty * item_price,
          NOW() - INTERVAL MOD(i, 180) DAY
        );

        SET order_total = order_total + (item_qty * item_price);
        SET item_loop = item_loop + 1;
      END WHILE;

      UPDATE orders
      SET total_amount = order_total
      WHERE id = i;

      SET i = i + 1;
    END WHILE;
  END IF;

  IF (SELECT COUNT(*) FROM page_visit_logs) = 0 THEN
    SET j = 1;
    WHILE j <= page_log_count DO
      INSERT INTO page_visit_logs (
        biz_date,
        user_id,
        page_name,
        device_type,
        ip_address,
        referer_url,
        stay_seconds,
        created_at
      ) VALUES (
        CURRENT_DATE - INTERVAL MOD(j, 60) DAY,
        1 + MOD(j * 23, customer_count),
        ELT(1 + MOD(j, 7), 'home', 'search', 'detail', 'cart', 'order', 'profile', 'campaign'),
        ELT(1 + MOD(j, 3), 'ios', 'android', 'web'),
        CONCAT('10.', MOD(j, 255), '.', MOD(j * 3, 255), '.', MOD(j * 7, 255)),
        ELT(1 + MOD(j, 5), 'direct', 'seo', 'adwords', 'wechat', 'email'),
        5 + MOD(j * 11, 900),
        NOW() - INTERVAL MOD(j, 60) DAY - INTERVAL MOD(j * 29, 86400) SECOND
      );
      SET j = j + 1;
    END WHILE;
  END IF;
END$$

DELIMITER ;

CALL seed_sso_demo_app();
DROP PROCEDURE IF EXISTS seed_sso_demo_app;

-- Optional verification:
-- SELECT COUNT(*) AS customer_count FROM customers;
-- SELECT COUNT(*) AS product_count FROM products;
-- SELECT COUNT(*) AS order_count FROM orders;
-- SELECT COUNT(*) AS order_item_count FROM order_items;
-- SELECT COUNT(*) AS page_visit_log_count FROM page_visit_logs;

-- Optional slow-query candidates for manual testing after enabling slow_query_log:
-- 1) Full scan with aggregation across large tables.
-- SELECT c.city, o.order_status, SUM(oi.line_amount) AS total_amount
-- FROM customers c
-- JOIN orders o ON o.customer_id = c.id
-- JOIN order_items oi ON oi.order_id = o.id
-- WHERE c.customer_status = 'active'
-- GROUP BY c.city, o.order_status
-- ORDER BY total_amount DESC;

-- 2) Wide LIKE search without supporting indexes.
-- SELECT *
-- FROM customers
-- WHERE customer_name LIKE '%99%'
--    OR email LIKE '%demo.local%'
--    OR phone LIKE '%138%';

-- 3) Scan-heavy analytics query on access logs.
-- SELECT page_name, device_type, AVG(stay_seconds) AS avg_stay, COUNT(*) AS visit_count
-- FROM page_visit_logs
-- WHERE biz_date >= CURRENT_DATE - INTERVAL 30 DAY
-- GROUP BY page_name, device_type
-- ORDER BY avg_stay DESC, visit_count DESC;

-- 4) Multi-join order detail lookup with date sorting.
-- SELECT o.order_no, c.customer_name, p.product_name, oi.quantity, oi.line_amount, o.created_at
-- FROM orders o
-- JOIN customers c ON c.id = o.customer_id
-- JOIN order_items oi ON oi.order_id = o.id
-- JOIN products p ON p.id = oi.product_id
-- WHERE o.created_at >= NOW() - INTERVAL 90 DAY
-- ORDER BY o.created_at DESC, oi.line_amount DESC
-- LIMIT 5000;
