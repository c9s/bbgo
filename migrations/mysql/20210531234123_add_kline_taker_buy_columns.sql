-- +up
-- +begin
ALTER TABLE `binance_klines`
    ADD COLUMN `quote_volume`           DECIMAL(32, 4) NOT NULL DEFAULT 0.0,
    ADD COLUMN `taker_buy_base_volume`  DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 4) NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE `max_klines`
    ADD COLUMN `quote_volume`           DECIMAL(32, 4) NOT NULL DEFAULT 0.0,
    ADD COLUMN `taker_buy_base_volume`  DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 4) NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE `okex_klines`
    ADD COLUMN `quote_volume`           DECIMAL(32, 4) NOT NULL DEFAULT 0.0,
    ADD COLUMN `taker_buy_base_volume`  DECIMAL(16, 8) NOT NULL DEFAULT 0.0,
    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 4) NOT NULL DEFAULT 0.0;
-- +end

-- +down
-- +begin
ALTER TABLE `binance_klines`
    DROP COLUMN `quote_volume`,
    DROP COLUMN `taker_buy_base_volume`,
    DROP COLUMN `taker_buy_quote_volume`;
-- +end

-- +begin
ALTER TABLE `max_klines`
    DROP COLUMN `quote_volume`,
    DROP COLUMN `taker_buy_base_volume`,
    DROP COLUMN `taker_buy_quote_volume`;
-- +end

-- +begin
ALTER TABLE `okex_klines`
    DROP COLUMN `quote_volume`,
    DROP COLUMN `taker_buy_base_volume`,
    DROP COLUMN `taker_buy_quote_volume`;
-- +end
