-- +up
CREATE TABLE `klines`
(
    `gid`           BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,
    `exchange`      VARCHAR(10)             NOT NULL,
    `start_time`    DATETIME(3)             NOT NULL,
    `end_time`      DATETIME(3)             NOT NULL,
    `interval`      VARCHAR(3)              NOT NULL,
    `symbol`        VARCHAR(20)              NOT NULL,
    `open`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `high`          DECIMAL(16, 8) UNSIGNED NOT NULL,
    `low`           DECIMAL(16, 8) UNSIGNED NOT NULL,
    `close`         DECIMAL(16, 8) UNSIGNED NOT NULL DEFAULT 0.0,
    `volume`        DECIMAL(16, 8) UNSIGNED NOT NULL DEFAULT 0.0,
    `closed`        BOOL                    NOT NULL DEFAULT TRUE,
    `last_trade_id` INT UNSIGNED            NOT NULL DEFAULT 0,
    `num_trades`    INT UNSIGNED            NOT NULL DEFAULT 0,

    PRIMARY KEY (`gid`)

);

CREATE INDEX `klines_end_time_symbol_interval` ON klines (`end_time`, `symbol`, `interval`);
CREATE TABLE `okex_klines` LIKE `klines`;
CREATE TABLE `binance_klines` LIKE `klines`;
CREATE TABLE `max_klines` LIKE `klines`;

-- +down
DROP INDEX `klines_end_time_symbol_interval` ON `klines`;
DROP TABLE `binance_klines`;
DROP TABLE `okex_klines`;
DROP TABLE `max_klines`;
DROP TABLE `klines`;

