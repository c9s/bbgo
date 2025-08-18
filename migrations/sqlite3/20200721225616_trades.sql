-- !txn
-- +up
CREATE TABLE `trades`
(
    `gid`            INTEGER PRIMARY KEY AUTOINCREMENT,
    `id`             INTEGER,
    `exchange`       TEXT           NOT NULL DEFAULT '',
    `symbol`         TEXT           NOT NULL,
    `price`          DECIMAL(16, 8) NOT NULL,
    `quantity`       DECIMAL(16, 8) NOT NULL,
    `quote_quantity` DECIMAL(16, 8) NOT NULL,
    `fee`            DECIMAL(16, 8) NOT NULL,
    `fee_currency`   VARCHAR(4)     NOT NULL,
    `is_buyer`       BOOLEAN        NOT NULL DEFAULT FALSE,
    `is_maker`       BOOLEAN        NOT NULL DEFAULT FALSE,
    `side`           VARCHAR(4)     NOT NULL DEFAULT '',
    `traded_at`      DATETIME(3)    NOT NULL
);

-- +down
DROP TABLE IF EXISTS `trades`;
