-- !txn
-- +up
-- +begin
CREATE TABLE `bybit_klines`
(
    `gid`                    INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange`               VARCHAR(10)    NOT NULL,
    `start_time`             DATETIME(3)    NOT NULL,
    `end_time`               DATETIME(3)    NOT NULL,
    `interval`               VARCHAR(3)     NOT NULL,
    `symbol`                 VARCHAR(7)     NOT NULL,
    `open`                   DECIMAL(16, 8) NOT NULL,
    `high`                   DECIMAL(16, 8) NOT NULL,
    `low`                    DECIMAL(16, 8) NOT NULL,
    `close`                  DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `volume`                 DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    `closed`                 BOOLEAN        NOT NULL DEFAULT TRUE,
    `last_trade_id`          INT            NOT NULL DEFAULT 0,
    `num_trades`             INT            NOT NULL DEFAULT 0,
    `quote_volume`           DECIMAL        NOT NULL DEFAULT 0.0,
    `taker_buy_base_volume`  DECIMAL        NOT NULL DEFAULT 0.0,
    `taker_buy_quote_volume` DECIMAL        NOT NULL DEFAULT 0.0
);
-- +end

-- +down

-- +begin
DROP TABLE bybit_klines;
-- +end
