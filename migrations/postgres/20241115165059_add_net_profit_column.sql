-- +up
-- +begin
ALTER TABLE positions
    ADD COLUMN net_profit NUMERIC(16, 8) DEFAULT 0.00000000 NOT NULL
;
-- +end
-- +begin
UPDATE positions SET net_profit = profit WHERE net_profit = 0.0;
-- +end

-- +down

-- +begin
ALTER TABLE positions
DROP COLUMN net_profit
;
-- +end