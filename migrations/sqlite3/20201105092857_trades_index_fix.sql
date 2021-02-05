-- +up
DROP INDEX IF EXISTS trades_symbol;
DROP INDEX IF EXISTS trades_symbol_fee_currency;
DROP INDEX IF EXISTS trades_traded_at_symbol;

CREATE INDEX trades_symbol ON trades (exchange, symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);

-- +down
DROP INDEX IF EXISTS trades_symbol ON trades;
DROP INDEX IF EXISTS trades_symbol_fee_currency ON trades;
DROP INDEX IF EXISTS trades_traded_at_symbol ON trades;

CREATE INDEX trades_symbol ON trades (symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (traded_at, symbol);


