-- +up
CREATE TABLE klines
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

CREATE INDEX klines_end_time_symbol_interval ON klines (end_time, symbol, interval);

-- Create exchange-specific klines tables
CREATE TABLE okex_klines
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

CREATE TABLE binance_klines
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

CREATE TABLE max_klines
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

-- +down
DROP INDEX IF EXISTS klines_end_time_symbol_interval;
DROP TABLE IF EXISTS binance_klines;
DROP TABLE IF EXISTS okex_klines;
DROP TABLE IF EXISTS max_klines;
DROP TABLE IF EXISTS klines;