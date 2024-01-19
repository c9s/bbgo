-- !txn
-- +up
CREATE TABLE `profits`
(
    `gid`                  INTEGER PRIMARY KEY AUTOINCREMENT,

    `strategy`             VARCHAR(32)    NOT NULL,
    `strategy_instance_id` VARCHAR(64)    NOT NULL,

    `symbol`               VARCHAR(8)     NOT NULL,

    -- average_cost is the position average cost
    `average_cost`         DECIMAL(16, 8) NOT NULL,

    -- profit is the pnl (profit and loss)
    `profit`               DECIMAL(16, 8) NOT NULL,

    -- net_profit is the pnl (profit and loss)
    `net_profit`           DECIMAL(16, 8) NOT NULL,

    -- profit_margin is the pnl (profit and loss)
    `profit_margin`        DECIMAL(16, 8) NOT NULL,

    -- net_profit_margin is the pnl (profit and loss)
    `net_profit_margin`    DECIMAL(16, 8) NOT NULL,

    `quote_currency`       VARCHAR(10)    NOT NULL,

    `base_currency`        VARCHAR(10)    NOT NULL,

    -- -------------------------------------------------------
    -- embedded trade data --
    -- -------------------------------------------------------
    `exchange`             VARCHAR(24)    NOT NULL DEFAULT '',

    `is_futures`           BOOLEAN        NOT NULL DEFAULT FALSE,

    `is_margin`            BOOLEAN        NOT NULL DEFAULT FALSE,

    `is_isolated`          BOOLEAN        NOT NULL DEFAULT FALSE,

    `trade_id`             BIGINT         NOT NULL,

    -- side is the side of the trade that makes profit
    `side`                 VARCHAR(4)     NOT NULL DEFAULT '',

    `is_buyer`             BOOLEAN        NOT NULL DEFAULT FALSE,

    `is_maker`             BOOLEAN        NOT NULL DEFAULT FALSE,

    -- price is the price of the trade that makes profit
    `price`                DECIMAL(16, 8) NOT NULL,

    -- quantity is the quantity of the trade that makes profit
    `quantity`             DECIMAL(16, 8) NOT NULL,

    -- trade_amount is the quote quantity of the trade that makes profit
    `quote_quantity`       DECIMAL(16, 8) NOT NULL,

    `traded_at`            DATETIME(3)    NOT NULL,

    -- fee
    `fee_in_usd`           DECIMAL(16, 8),
    `fee`                  DECIMAL(16, 8) NOT NULL,
    `fee_currency`         VARCHAR(10)    NOT NULL
);

-- +down
DROP TABLE IF EXISTS `profits`;
