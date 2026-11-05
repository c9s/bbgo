-- @package bfxfunding
-- +up
-- +begin
CREATE TABLE `bfxfunding_public_trades`
(
    `gid`         INTEGER PRIMARY KEY AUTOINCREMENT,

    `symbol`      TEXT        NOT NULL DEFAULT '',
    `trade_id`    BIGINT      NOT NULL,
    `amount`      REAL        NOT NULL DEFAULT 0,
    `rate`        REAL        NOT NULL DEFAULT 0,
    `period`      INTEGER     NOT NULL DEFAULT 0,
    `time`        DATETIME(3) NOT NULL,

    `inserted_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP NOT NULL
);
-- +end

-- +begin
CREATE UNIQUE INDEX `uidx_bfxfunding_public_trades_symbol_trade_id` ON `bfxfunding_public_trades` (`symbol`, `trade_id`);
-- +end

-- +begin
CREATE INDEX `idx_bfxfunding_public_trades_symbol_time` ON `bfxfunding_public_trades` (`symbol`, `time`);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS `idx_bfxfunding_public_trades_symbol_time`;
-- +end

-- +begin
DROP INDEX IF EXISTS `uidx_bfxfunding_public_trades_symbol_trade_id`;
-- +end

-- +begin
DROP TABLE IF EXISTS `bfxfunding_public_trades`;
-- +end
