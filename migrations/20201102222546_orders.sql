-- +goose Up
CREATE TABLE `orders`
(
    `gid`               BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `exchange`          VARCHAR(24)             NOT NULL DEFAULT '',
    -- order_id is the order id returned from the exchange
    `order_id`          BIGINT UNSIGNED         NOT NULL,
    `client_order_id`   VARCHAR(42)             NOT NULL DEFAULT '',
    `order_type`        VARCHAR(16)             NOT NULL,
    `symbol`            VARCHAR(8)              NOT NULL,
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
    PRIMARY KEY (`gid`)

) ENGINE = InnoDB;

-- +goose Down
DROP TABLE `orders`;
