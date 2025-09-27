-- +up
CREATE TABLE positions
(
    gid                  BIGSERIAL               NOT NULL,

    strategy             VARCHAR(32)             NOT NULL,
    strategy_instance_id VARCHAR(64)             NOT NULL,

    symbol               VARCHAR(32)             NOT NULL,
    quote_currency       VARCHAR(10)             NOT NULL,
    base_currency        VARCHAR(16)             NOT NULL,

    -- average_cost is the position average cost
    average_cost         NUMERIC(16, 8)          NOT NULL CHECK (average_cost >= 0),
    base                 NUMERIC(16, 8)          NOT NULL,
    quote                NUMERIC(16, 8)          NOT NULL,
    profit               NUMERIC(16, 8)          NULL,

    -- trade related columns
    trade_id             BIGINT                  NOT NULL, -- the trade id in the exchange
    side                 VARCHAR(4)              NOT NULL, -- side of the trade
    exchange             VARCHAR(20)             NOT NULL, -- exchange of the trade
    traded_at            TIMESTAMP(3)            NOT NULL, -- millisecond timestamp

    PRIMARY KEY (gid),
    UNIQUE (trade_id, side, exchange)
);

-- +down
DROP TABLE IF EXISTS positions;