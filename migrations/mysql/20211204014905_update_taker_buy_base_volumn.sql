-- +up
-- +begin
ALTER TABLE binance_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(32,8) NOT NULL;
-- +end
-- +begin
ALTER TABLE max_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(32,8) NOT NULL;
-- +end
-- +begin
ALTER TABLE okex_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(32,8) NOT NULL;
-- +end

-- +down
-- +begin
ALTER TABLE binance_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(16,8) NOT NULL;
-- +end
-- +begin
ALTER TABLE max_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(16,8) NOT NULL;
-- +end
-- +begin
ALTER TABLE okex_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(16,8) NOT NULL;
-- +end
