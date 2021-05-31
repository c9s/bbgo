-- +up
-- +begin
ALTER TABLE `binance_klines`
    ADD COLUMN `quote_volume` DECIMAL NOT NULL DEFAULT 0.0;
ALTER TABLE `binance_klines`
    ADD COLUMN `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0;
ALTER TABLE `binance_klines`
    ADD COLUMN `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE `max_klines`
    ADD COLUMN `quote_volume` DECIMAL NOT NULL DEFAULT 0.0;
ALTER TABLE `max_klines`
    ADD COLUMN `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0;
ALTER TABLE `max_klines`
    ADD COLUMN `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0;
-- +end
-- +begin
ALTER TABLE `okex_klines`
    ADD COLUMN `quote_volume` DECIMAL NOT NULL DEFAULT 0.0;
ALTER TABLE `okex_klines`
    ADD COLUMN `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0;
ALTER TABLE `okex_klines`
    ADD COLUMN `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0;
-- +end

-- +down
