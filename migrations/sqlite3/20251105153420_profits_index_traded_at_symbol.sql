-- +up
-- +begin
CREATE INDEX idx_profits_traded_at_symbol ON profits (traded_at, symbol);
-- +end

-- +down
-- +begin
DROP INDEX idx_profits_traded_at_symbol;
-- +end
