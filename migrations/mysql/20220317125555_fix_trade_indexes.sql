-- +up
DROP INDEX trades_symbol ON trades;
DROP INDEX trades_symbol_fee_currency ON trades;
DROP INDEX trades_traded_at_symbol ON trades;

-- this index is used for general trade query
CREATE INDEX trades_traded_at ON trades (traded_at, symbol, exchange, id, fee_currency, fee);
-- this index is used for join clause by trade_id
CREATE INDEX trades_id_traded_at ON trades (id, traded_at);
-- this index is used for join clause by order id
CREATE INDEX trades_order_id_traded_at ON trades (order_id, traded_at);

-- +down
DROP INDEX trades_traded_at ON trades;
DROP INDEX trades_id_traded_at ON trades;
DROP INDEX trades_order_id_traded_at ON trades;
CREATE INDEX trades_symbol ON trades (exchange, symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);
