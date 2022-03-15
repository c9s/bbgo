-- +up
CREATE TABLE `trades`
(
    `gid`            BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `id`             BIGINT UNSIGNED,
    `order_id`       BIGINT UNSIGNED         NOT NULL,
    `exchange`       VARCHAR(24)             NOT NULL DEFAULT '',
    `symbol`         VARCHAR(20)             NOT NULL,
    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,
    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,
    `fee_currency`   VARCHAR(6)              NOT NULL,
    `is_buyer`       BOOLEAN                 NOT NULL DEFAULT FALSE,
    `is_maker`       BOOLEAN                 NOT NULL DEFAULT FALSE,
    `side`           VARCHAR(4)              NOT NULL DEFAULT '',
    `traded_at`      DATETIME(3)             NOT NULL,

    `is_margin`      BOOLEAN                 NOT NULL DEFAULT FALSE,
    `is_isolated`    BOOLEAN                 NOT NULL DEFAULT FALSE,

    `strategy`       VARCHAR(32)             NULL,
    `pnl`            DECIMAL                 NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `id` (`exchange`, `symbol`, `side`, `id`)
);

CREATE INDEX trades_symbol ON trades (exchange, symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);


-- +down
DROP TABLE IF EXISTS `trades`;

DROP INDEX trades_symbol ON trades;
DROP INDEX trades_symbol_fee_currency ON trades;
DROP INDEX trades_traded_at_symbol ON trades;

