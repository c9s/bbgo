-- +goose Up
DROP INDEX trades_symbol ON trades;
DROP INDEX trades_symbol_fee_currency ON trades;
DROP INDEX trades_traded_at_symbol ON trades;

CREATE INDEX trades_symbol ON trades (exchange, symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);

-- +goose Down
DROP INDEX trades_symbol ON trades;
DROP INDEX trades_symbol_fee_currency ON trades;
DROP INDEX trades_traded_at_symbol ON trades;

CREATE INDEX trades_symbol ON trades (symbol);
CREATE INDEX trades_symbol_fee_currency ON trades (symbol, fee_currency, traded_at);
CREATE INDEX trades_traded_at_symbol ON trades (traded_at, symbol);


