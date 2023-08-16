-- +up
-- +begin
CREATE TABLE `bybit_klines` LIKE `binance_klines`;
-- +end

-- +down

-- +begin
DROP TABLE `bybit_klines`;
-- +end
