-- +up
-- +begin
CREATE TABLE bybit_klines
(
    gid           BIGSERIAL               NOT NULL,
    exchange      VARCHAR(10)             NOT NULL,
    start_time    TIMESTAMP(3)            NOT NULL,
    end_time      TIMESTAMP(3)            NOT NULL,
    interval      VARCHAR(3)              NOT NULL,
    symbol        VARCHAR(20)             NOT NULL,
    open          NUMERIC(20, 8)          NOT NULL CHECK (open >= 0),
    high          NUMERIC(20, 8)          NOT NULL CHECK (high >= 0),
    low           NUMERIC(20, 8)          NOT NULL CHECK (low >= 0),
    close         NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (close >= 0),
    volume        NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (volume >= 0),
    closed        BOOLEAN                 NOT NULL DEFAULT TRUE,
    last_trade_id INTEGER                 NOT NULL DEFAULT 0 CHECK (last_trade_id >= 0),
    num_trades    INTEGER                 NOT NULL DEFAULT 0 CHECK (num_trades >= 0),

    PRIMARY KEY (gid)
);
-- Ensure deduplication works with ON CONFLICT DO NOTHING in inserts
CREATE UNIQUE INDEX idx_kline_bybit_unique
    ON bybit_klines (symbol, interval, start_time);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS idx_kline_bybit_unique;
DROP TABLE IF EXISTS bybit_klines;
-- +end
