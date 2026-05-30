-- +up
-- +begin
CREATE TABLE `futures_position_risks`
(
    `gid`                        INTEGER PRIMARY KEY AUTOINCREMENT,

    `exchange`                   TEXT     NOT NULL,
    `symbol`                     TEXT     NOT NULL,
    `position_side`              TEXT     NOT NULL DEFAULT '',

    `leverage`                   REAL     NOT NULL DEFAULT 0,
    `liquidation_price`          REAL     NOT NULL DEFAULT 0,
    `entry_price`                REAL     NOT NULL DEFAULT 0,
    `mark_price`                 REAL     NOT NULL DEFAULT 0,
    `break_even_price`           REAL     NOT NULL DEFAULT 0,
    `position_amount`            REAL     NOT NULL DEFAULT 0,
    `unrealized_pnl`             REAL     NOT NULL DEFAULT 0,
    `notional`                   REAL     NOT NULL DEFAULT 0,
    `initial_margin`             REAL     NOT NULL DEFAULT 0,
    `maint_margin`               REAL     NOT NULL DEFAULT 0,
    `position_initial_margin`    REAL     NOT NULL DEFAULT 0,
    `open_order_initial_margin`  REAL     NOT NULL DEFAULT 0,
    `adl`                        REAL     NOT NULL DEFAULT 0,
    `margin_asset`               TEXT     NOT NULL DEFAULT '',
    `updated_at`                 DATETIME(3)        NOT NULL
);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS `futures_position_risks`;
-- +end