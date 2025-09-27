-- !txn
-- +up
CREATE TABLE trades
(
    gid            BIGSERIAL               NOT NULL,

    id             BIGINT,
    order_id       BIGINT                  NOT NULL,
    exchange       VARCHAR(24)             NOT NULL DEFAULT '',
    symbol         VARCHAR(32)             NOT NULL,
    price          NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),
    quantity       NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),
    quote_quantity NUMERIC(16, 8)          NOT NULL CHECK (quote_quantity >= 0),
    fee            NUMERIC(16, 8)          NOT NULL CHECK (fee >= 0),
    fee_currency   VARCHAR(16)             NOT NULL,
    is_buyer       BOOLEAN                 NOT NULL DEFAULT FALSE,
    is_maker       BOOLEAN                 NOT NULL DEFAULT FALSE,
    side           VARCHAR(4)              NOT NULL DEFAULT '',
    traded_at      TIMESTAMP(3)            NOT NULL,

    is_margin      BOOLEAN                 NOT NULL DEFAULT FALSE,
    is_isolated    BOOLEAN                 NOT NULL DEFAULT FALSE,

    strategy       VARCHAR(32)             NULL,
    pnl            NUMERIC                 NULL,

    PRIMARY KEY (gid),
    UNIQUE (exchange, symbol, side, id)
);

CREATE INDEX trades_symbol ON trades (exchange, symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);


-- +down
DROP INDEX IF EXISTS trades_symbol;
DROP INDEX IF EXISTS trades_symbol_fee_currency;
DROP INDEX IF EXISTS trades_traded_at_symbol;
DROP TABLE IF EXISTS trades;