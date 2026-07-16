-- +up
-- +begin
CREATE TABLE `funding_fees`
(
    `gid`         INTEGER PRIMARY KEY AUTOINCREMENT,

    `exchange`    TEXT        NOT NULL DEFAULT '',
    `symbol`      TEXT        NOT NULL DEFAULT '',
    `asset`       TEXT        NOT NULL DEFAULT '',
    `amount`      REAL        NOT NULL DEFAULT 0,
    `txn`         BIGINT      NOT NULL DEFAULT 0,
    `time`        DATETIME(3) NOT NULL,

    `inserted_at` DATETIME(3)    DEFAULT CURRENT_TIMESTAMP NOT NULL
);
-- +end

-- +begin
CREATE INDEX `idx_funding_fees_exchange_symbol` ON `funding_fees` (`exchange`, `symbol`);
-- +end

-- +begin
CREATE INDEX `idx_funding_fees_time` ON `funding_fees` (`time`);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS `idx_funding_fees_time`;
-- +end

-- +begin
DROP INDEX IF EXISTS `idx_funding_fees_exchange_symbol`;
-- +end

-- +begin
DROP TABLE IF EXISTS `funding_fees`;
-- +end
