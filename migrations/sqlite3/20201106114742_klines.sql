-- +up
-- +begin
CREATE TABLE `klines`
(
    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange`      VARCHAR(10)    NOT NULL,
    `start_time`    DATETIME(3)    NOT NULL,
    `end_time`      DATETIME(3)    NOT NULL,
    `interval`      VARCHAR(3)     NOT NULL,
    `symbol`        VARCHAR(7)     NOT NULL,
    `open`          DECIMAL(16, 8) NOT NULL,
    `high`          DECIMAL(16, 8) NOT NULL,
    `low`           DECIMAL(16, 8) NOT NULL,
    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,
    `last_trade_id` INT            NOT NULL DEFAULT 0,
    `num_trades`    INT            NOT NULL DEFAULT 0
);
-- +end


-- +begin
CREATE TABLE `okex_klines`
(
    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange`      VARCHAR(10)    NOT NULL,
    `start_time`    DATETIME(3)    NOT NULL,
    `end_time`      DATETIME(3)    NOT NULL,
    `interval`      VARCHAR(3)     NOT NULL,
    `symbol`        VARCHAR(7)     NOT NULL,
    `open`          DECIMAL(16, 8) NOT NULL,
    `high`          DECIMAL(16, 8) NOT NULL,
    `low`           DECIMAL(16, 8) NOT NULL,
    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,
    `last_trade_id` INT            NOT NULL DEFAULT 0,
    `num_trades`    INT            NOT NULL DEFAULT 0
);
-- +end

-- +begin
CREATE TABLE `binance_klines`
(
    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange`      VARCHAR(10)    NOT NULL,
    `start_time`    DATETIME(3)    NOT NULL,
    `end_time`      DATETIME(3)    NOT NULL,
    `interval`      VARCHAR(3)     NOT NULL,
    `symbol`        VARCHAR(7)     NOT NULL,
    `open`          DECIMAL(16, 8) NOT NULL,
    `high`          DECIMAL(16, 8) NOT NULL,
    `low`           DECIMAL(16, 8) NOT NULL,
    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,
    `last_trade_id` INT            NOT NULL DEFAULT 0,
    `num_trades`    INT            NOT NULL DEFAULT 0
);
-- +end

-- +begin
CREATE TABLE `max_klines`
(
    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange`      VARCHAR(10)    NOT NULL,
    `start_time`    DATETIME(3)    NOT NULL,
    `end_time`      DATETIME(3)    NOT NULL,
    `interval`      VARCHAR(3)     NOT NULL,
    `symbol`        VARCHAR(7)     NOT NULL,
    `open`          DECIMAL(16, 8) NOT NULL,
    `high`          DECIMAL(16, 8) NOT NULL,
    `low`           DECIMAL(16, 8) NOT NULL,
    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,
    `last_trade_id` INT            NOT NULL DEFAULT 0,
    `num_trades`    INT            NOT NULL DEFAULT 0
);
-- +end

-- +begin
CREATE INDEX `klines_end_time_symbol_interval` ON `klines` (`end_time`, `symbol`, `interval`);
CREATE INDEX `binance_klines_end_time_symbol_interval` ON `binance_klines` (`end_time`, `symbol`, `interval`);
CREATE INDEX `okex_klines_end_time_symbol_interval` ON `okex_klines` (`end_time`, `symbol`, `interval`);
CREATE INDEX `max_klines_end_time_symbol_interval` ON `max_klines` (`end_time`, `symbol`, `interval`);
-- +end


-- +down
DROP INDEX IF EXISTS `klines_end_time_symbol_interval`;
DROP TABLE IF EXISTS `binance_klines`;
DROP TABLE IF EXISTS `okex_klines`;
DROP TABLE IF EXISTS `max_klines`;
DROP TABLE IF EXISTS `klines`;

