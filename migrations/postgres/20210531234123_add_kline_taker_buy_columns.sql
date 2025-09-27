-- +up
-- +begin
ALTER TABLE binance_klines
    ADD COLUMN quote_volume           NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_base_volume  NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_quote_volume NUMERIC(32, 8) NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE max_klines
    ADD COLUMN quote_volume           NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_base_volume  NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_quote_volume NUMERIC(32, 8) NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE okex_klines
    ADD COLUMN quote_volume           NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_base_volume  NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_quote_volume NUMERIC(32, 8) NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE klines
    ADD COLUMN quote_volume           NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_base_volume  NUMERIC(32, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN taker_buy_quote_volume NUMERIC(32, 8) NOT NULL DEFAULT 0.0;
-- +end

-- +down
-- +begin
ALTER TABLE binance_klines
    DROP COLUMN IF EXISTS quote_volume,
    DROP COLUMN IF EXISTS taker_buy_base_volume,
    DROP COLUMN IF EXISTS taker_buy_quote_volume;
-- +end

-- +begin
ALTER TABLE max_klines
    DROP COLUMN IF EXISTS quote_volume,
    DROP COLUMN IF EXISTS taker_buy_base_volume,
    DROP COLUMN IF EXISTS taker_buy_quote_volume;
-- +end

-- +begin
ALTER TABLE okex_klines
    DROP COLUMN IF EXISTS quote_volume,
    DROP COLUMN IF EXISTS taker_buy_base_volume,
    DROP COLUMN IF EXISTS taker_buy_quote_volume;
-- +end