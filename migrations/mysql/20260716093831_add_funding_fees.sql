-- +up
-- +begin
CREATE TABLE `funding_fees`
(
    `gid`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    `exchange`    VARCHAR(24)     NOT NULL DEFAULT '',
    `symbol`      VARCHAR(20)     NOT NULL DEFAULT '',
    `asset`       VARCHAR(20)     NOT NULL DEFAULT '',
    `amount`      DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `txn`         BIGINT          NOT NULL DEFAULT 0,
    `time`        DATETIME(3)     NOT NULL,

    `inserted_at` DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `uidx_exchange_symbol_txn` (`exchange`, `symbol`, `txn`),
    KEY `idx_funding_fees_exchange_symbol` (`exchange`, `symbol`),
    KEY `idx_funding_fees_time` (`time`)
);
-- +end

-- +down

-- +begin
DROP TABLE IF EXISTS `funding_fees`;
-- +end
