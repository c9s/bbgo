-- +up
CREATE TABLE `positions`
(
    `gid`                  BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `strategy`             VARCHAR(32)             NOT NULL,
    `strategy_instance_id` VARCHAR(64)             NOT NULL,

    `symbol`               VARCHAR(20)              NOT NULL,
    `quote_currency`       VARCHAR(10)             NOT NULL,
    `base_currency`        VARCHAR(10)             NOT NULL,

    -- average_cost is the position average cost
    `average_cost`         DECIMAL(16, 8) UNSIGNED NOT NULL,
    `base`                 DECIMAL(16, 8)          NOT NULL,
    `quote`                DECIMAL(16, 8)          NOT NULL,

    `trade_id`             BIGINT UNSIGNED         NOT NULL,
    `traded_at`            DATETIME(3)             NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `trade_id` (`trade_id`)
);

-- +down
DROP TABLE IF EXISTS `positions`;
