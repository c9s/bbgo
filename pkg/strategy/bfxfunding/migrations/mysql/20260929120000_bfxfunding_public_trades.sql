-- @package bfxfunding
-- +up
-- +begin
CREATE TABLE `bfxfunding_public_trades`
(
    `gid`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    `symbol`      VARCHAR(16)     NOT NULL DEFAULT '',
    `trade_id`    BIGINT          NOT NULL,
    `amount`      DECIMAL(24, 8)  NOT NULL DEFAULT 0,
    `rate`        DECIMAL(20, 12) NOT NULL DEFAULT 0,
    `period`      INT             NOT NULL DEFAULT 0,
    `time`        DATETIME(3)     NOT NULL,

    `inserted_at` DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `uidx_bfxfunding_public_trades_symbol_trade_id` (`symbol`, `trade_id`),
    KEY `idx_bfxfunding_public_trades_symbol_time` (`symbol`, `time`)
);
-- +end

-- +down

-- +begin
DROP TABLE IF EXISTS `bfxfunding_public_trades`;
-- +end
