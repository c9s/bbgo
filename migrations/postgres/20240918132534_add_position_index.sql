-- +up
-- +begin
CREATE INDEX positions_traded_at ON positions (traded_at, profit);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS positions_traded_at;
-- +end