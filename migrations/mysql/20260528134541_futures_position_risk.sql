-- +up
-- +begin
CREATE TABLE `futures_position_risks`
(
    `gid`                        BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,

    `exchange`                   VARCHAR(24)        NOT NULL DEFAULT '',
    `symbol`                     VARCHAR(32)        NOT NULL,
    `position_side`              VARCHAR(10)        NOT NULL DEFAULT '',

    `leverage`                   DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `liquidation_price`          DECIMAL(16, 8)     NOT NULL DEFAULT 0,
    `entry_price`                DECIMAL(16, 8)     NOT NULL DEFAULT 0,
    `mark_price`                 DECIMAL(16, 8)     NOT NULL DEFAULT 0,
    `break_even_price`           DECIMAL(16, 8)     NOT NULL DEFAULT 0,
    `position_amount`            DECIMAL(16, 8)     NOT NULL DEFAULT 0,
    `unrealized_pnl`             DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `notional`                   DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `initial_margin`             DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `maint_margin`               DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `position_initial_margin`    DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `open_order_initial_margin`  DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `adl`                        DECIMAL(16, 2)     NOT NULL DEFAULT 0,
    `margin_asset`               VARCHAR(20)        NOT NULL DEFAULT '',
    `updated_at`                  DATETIME(3)        NOT NULL,

    PRIMARY KEY (`gid`),
    KEY `idx_position_risks_exchange_symbol` (`exchange`, `symbol`),
    UNIQUE KEY `futures_position_risks_exchange_symbol_side_time`  (`exchange`, `symbol`, `position_side`, `updated_at`)
);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS `futures_position_risks`;
-- +end