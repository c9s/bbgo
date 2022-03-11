-- +up
CREATE TABLE `orders`
(
    `gid`               BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `exchange`          VARCHAR(24)             NOT NULL DEFAULT '',
    -- order_id is the order id returned from the exchange
    `order_id`          BIGINT UNSIGNED         NOT NULL,
    `client_order_id`   VARCHAR(122)            NOT NULL DEFAULT '',
    `order_type`        VARCHAR(16)             NOT NULL,
    `symbol`            VARCHAR(20)             NOT NULL,
    `status`            VARCHAR(12)             NOT NULL,
    `time_in_force`     VARCHAR(4)              NOT NULL,
    `price`             DECIMAL(16, 8) UNSIGNED NOT NULL,
    `stop_price`        DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quantity`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `executed_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL DEFAULT 0.0,
    `side`              VARCHAR(4)              NOT NULL DEFAULT '',
    `is_working`        BOOL                    NOT NULL DEFAULT FALSE,
    `created_at`        DATETIME(3)             NOT NULL,
    `updated_at`        DATETIME(3)             NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),

    `is_margin`         BOOLEAN                 NOT NULL DEFAULT FALSE,
    `is_isolated`       BOOLEAN                 NOT NULL DEFAULT FALSE,

    PRIMARY KEY (`gid`)
);
CREATE INDEX orders_symbol ON orders (exchange, symbol);
CREATE UNIQUE INDEX orders_order_id ON orders (order_id, exchange);

-- +down
DROP INDEX orders_symbol ON orders;
DROP INDEX orders_order_id ON orders;
DROP TABLE `orders`;
