-- +up
CREATE TABLE profits
(
    gid                  BIGSERIAL               NOT NULL,

    strategy             VARCHAR(32)             NOT NULL,
    strategy_instance_id VARCHAR(64)             NOT NULL,

    symbol               VARCHAR(32)             NOT NULL,

    -- average_cost is the position average cost
    average_cost         NUMERIC(16, 8)          NOT NULL CHECK (average_cost >= 0),

    -- profit is the pnl (profit and loss)
    profit               NUMERIC(16, 8)          NOT NULL,

    -- net_profit is the pnl (profit and loss)
    net_profit           NUMERIC(16, 8)          NOT NULL,

    -- profit_margin is the pnl (profit and loss)
    profit_margin        NUMERIC(16, 8)          NOT NULL,

    -- net_profit_margin is the pnl (profit and loss)
    net_profit_margin    NUMERIC(16, 8)          NOT NULL,

    quote_currency       VARCHAR(10)             NOT NULL,

    base_currency        VARCHAR(16)             NOT NULL,

    -- -------------------------------------------------------
    -- embedded trade data --
    -- -------------------------------------------------------
    exchange             VARCHAR(24)             NOT NULL DEFAULT '',

    is_futures           BOOLEAN                 NOT NULL DEFAULT FALSE,

    is_margin            BOOLEAN                 NOT NULL DEFAULT FALSE,

    is_isolated          BOOLEAN                 NOT NULL DEFAULT FALSE,

    trade_id             BIGINT                  NOT NULL,

    -- side is the side of the trade that makes profit
    side                 VARCHAR(4)              NOT NULL DEFAULT '',

    is_buyer             BOOLEAN                 NOT NULL DEFAULT FALSE,

    is_maker             BOOLEAN                 NOT NULL DEFAULT FALSE,

    -- price is the price of the trade that makes profit
    price                NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),

    -- quantity is the quantity of the trade that makes profit
    quantity             NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),

    -- quote_quantity is the quote quantity of the trade that makes profit
    quote_quantity       NUMERIC(16, 8)          NOT NULL CHECK (quote_quantity >= 0),

    traded_at            TIMESTAMP(3)            NOT NULL,

    -- fee
    fee_in_usd           NUMERIC(16, 8),
    fee                  NUMERIC(16, 8)          NOT NULL,
    fee_currency         VARCHAR(16)             NOT NULL,

    PRIMARY KEY (gid),
    UNIQUE (trade_id)
);

-- +down
DROP TABLE IF EXISTS profits;