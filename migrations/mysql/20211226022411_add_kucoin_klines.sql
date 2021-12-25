-- +up
-- +begin
CREATE TABLE `kucoin_klines` LIKE `binance_klines`;
-- +end

-- +down

-- +begin
DROP TABLE `kucoin_klines`;
-- +end
