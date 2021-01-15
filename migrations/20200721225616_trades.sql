-- +up
CREATE TABLE `trades`
(
    `gid`            BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `id`             BIGINT UNSIGNED,
    `exchange`       VARCHAR(24)             NOT NULL DEFAULT '',
    `symbol`         VARCHAR(8)              NOT NULL,
    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee_currency`   VARCHAR(4)              NOT NULL,
    `is_buyer`       BOOLEAN                 NOT NULL DEFAULT FALSE,
    `is_maker`       BOOLEAN                 NOT NULL DEFAULT FALSE,
    `side`           VARCHAR(4)              NOT NULL DEFAULT '',
    `traded_at`      DATETIME(3)             NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `id` (`id`)
);
-- +down
DROP TABLE `trades`;
