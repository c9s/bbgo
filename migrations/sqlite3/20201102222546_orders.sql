-- !txn
-- +up
CREATE TABLE `orders`
(
    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,

    `exchange`          VARCHAR             NOT NULL DEFAULT '',
    -- order_id is the order id returned from the exchange
    `order_id`          INTEGER          NOT NULL,
    `client_order_id`   VARCHAR          NOT NULL DEFAULT '',
    `order_type`        VARCHAR          NOT NULL,
    `symbol`            VARCHAR          NOT NULL,
    `status`            VARCHAR          NOT NULL,
    `time_in_force`     VARCHAR          NOT NULL,
    `price`             DECIMAL(16, 8)  NOT NULL,
    `stop_price`        DECIMAL(16, 8)  NOT NULL,
    `quantity`          DECIMAL(16, 8)  NOT NULL,
    `executed_quantity` DECIMAL(16, 8)  NOT NULL DEFAULT 0.0,
    `side`              VARCHAR         NOT NULL DEFAULT '',
    `is_working`        BOOLEAN         NOT NULL DEFAULT FALSE,
    `created_at`        DATETIME(3)     NOT NULL,
    `updated_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +down
DROP TABLE IF EXISTS `orders`;
