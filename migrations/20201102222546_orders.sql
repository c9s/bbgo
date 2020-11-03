-- +goose Up
CREATE TABLE `orders`
(
    `gid`               BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `order_id`          BIGINT UNSIGNED,
    `client_order_id`   VARCHAR(32)             NOT NULL DEFAULT '',
    `exchange`          VARCHAR(24)             NOT NULL DEFAULT '',
    `symbol`            VARCHAR(7)              NOT NULL,
    `time_in_force`     VARCHAR(4)              NOT NULL,
    `price`             DECIMAL(16, 8) UNSIGNED NOT NULL,
    `stop_price`        DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quantity`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `executed_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee`               DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee_currency`      VARCHAR(4)              NOT NULL,
    `side`              VARCHAR(4)              NOT NULL DEFAULT '',
    `is_working`        BOOL                    NOT NULL DEFAULT FALSE,
    `created_at`        DATETIME(6)             NOT NULL,

    PRIMARY KEY (`gid`)

) ENGINE = InnoDB;
-- +goose Down
DROP TABLE `orders`;
