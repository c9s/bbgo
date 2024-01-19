-- !txn
-- +up
CREATE TABLE `margin_liquidations`
(
    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,

    `exchange`          VARCHAR(24)    NOT NULL DEFAULT '',

    `symbol`            VARCHAR(24)    NOT NULL DEFAULT '',

    `order_id`          INTEGER        NOT NULL,

    `is_isolated`       BOOL           NOT NULL DEFAULT false,

    `average_price`     DECIMAL(16, 8) NOT NULL,

    `price`             DECIMAL(16, 8) NOT NULL,

    `quantity`          DECIMAL(16, 8) NOT NULL,

    `executed_quantity` DECIMAL(16, 8) NOT NULL,

    `side`              VARCHAR(5)     NOT NULL DEFAULT '',

    `time_in_force`     VARCHAR(5)     NOT NULL DEFAULT '',

    `time`      DATETIME(3)    NOT NULL
);

-- +down
DROP TABLE IF EXISTS `margin_liquidations`;
