-- +up
-- +begin
CREATE UNIQUE INDEX idx_kline_binance_unique
    ON binance_klines (symbol, interval, start_time);
-- +end

-- +begin
CREATE UNIQUE INDEX idx_kline_max_unique
    ON max_klines (symbol, interval, start_time);
-- +end

-- +begin
CREATE UNIQUE INDEX idx_kline_ftx_unique
    ON ftx_klines (symbol, interval, start_time);
-- +end

-- +begin
CREATE UNIQUE INDEX idx_kline_kucoin_unique
    ON kucoin_klines (symbol, interval, start_time);
-- +end

-- +begin
CREATE UNIQUE INDEX idx_kline_okex_unique
    ON okex_klines (symbol, interval, start_time);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS idx_kline_ftx_unique;
-- +end

-- +begin
DROP INDEX IF EXISTS idx_kline_max_unique;
-- +end

-- +begin
DROP INDEX IF EXISTS idx_kline_binance_unique;
-- +end

-- +begin
DROP INDEX IF EXISTS idx_kline_kucoin_unique;
-- +end

-- +begin
DROP INDEX IF EXISTS idx_kline_okex_unique;
-- +end