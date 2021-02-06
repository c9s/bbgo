-- +up
-- +begin
CREATE INDEX trades_price_quantity ON trades (order_id,price,quantity);
-- +end

-- +down

-- +begin
DROP INDEX trades_price_quantity ON trades
-- +end
