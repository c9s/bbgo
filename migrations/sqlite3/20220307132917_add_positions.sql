-- +up
CREATE TABLE `positions`
(
    `gid`                  INTEGER PRIMARY KEY AUTOINCREMENT,

    `strategy`             VARCHAR(32)    NOT NULL,
    `strategy_instance_id` VARCHAR(64)    NOT NULL,

    `symbol`               VARCHAR(20)    NOT NULL,
    `quote_currency`       VARCHAR(10)    NOT NULL,
    `base_currency`        VARCHAR(10)    NOT NULL,

    -- average_cost is the position average cost
    `average_cost`         DECIMAL(16, 8) NOT NULL,
    `base`                 DECIMAL(16, 8) NOT NULL,
    `quote`                DECIMAL(16, 8) NOT NULL,
    `profit`               DECIMAL(16, 8) NULL,

    -- trade related columns
    `trade_id`             BIGINT         NOT NULL,
    `side`                 VARCHAR(4)     NOT NULL, -- side of the trade
    `exchange`             VARCHAR(12)    NOT NULL, -- exchange of the trade
    `traded_at`            DATETIME(3)    NOT NULL
);

-- +down
DROP TABLE IF EXISTS `positions`;
