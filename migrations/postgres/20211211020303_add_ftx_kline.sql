-- +up
-- +begin
CREATE TABLE IF NOT EXISTS ftx_klines
(
    gid                    BIGSERIAL               NOT NULL PRIMARY KEY,
    exchange               VARCHAR(10)             NOT NULL,
    start_time             TIMESTAMP(3)            NOT NULL,
    end_time               TIMESTAMP(3)            NOT NULL,
    interval               VARCHAR(3)              NOT NULL,
    symbol                 VARCHAR(20)             NOT NULL,
    open                   NUMERIC(20,8)           NOT NULL CHECK (open >= 0),
    high                   NUMERIC(20,8)           NOT NULL CHECK (high >= 0),
    low                    NUMERIC(20,8)           NOT NULL CHECK (low >= 0),
    close                  NUMERIC(20,8)           DEFAULT 0.00000000 NOT NULL CHECK (close >= 0),
    volume                 NUMERIC(20,8)           DEFAULT 0.00000000 NOT NULL CHECK (volume >= 0),
    closed                 BOOLEAN                 DEFAULT TRUE NOT NULL,
    last_trade_id          INTEGER                 DEFAULT 0 NOT NULL CHECK (last_trade_id >= 0),
    num_trades             INTEGER                 DEFAULT 0 NOT NULL CHECK (num_trades >= 0),
    quote_volume           NUMERIC(32,4)           DEFAULT 0.0000 NOT NULL,
    taker_buy_base_volume  NUMERIC(32,8)           NOT NULL,
    taker_buy_quote_volume NUMERIC(32,4)           DEFAULT 0.0000 NOT NULL
);
-- +end
-- +begin
CREATE INDEX klines_end_time_symbol_interval_ftx
    ON ftx_klines (end_time, symbol, interval);
-- +end

-- +down

-- +begin
DROP TABLE IF EXISTS ftx_klines;
-- +end