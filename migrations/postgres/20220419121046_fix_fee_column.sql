-- +up
-- +begin
ALTER TABLE trades
    ALTER COLUMN fee TYPE NUMERIC(16, 8);
-- +end

-- +begin
ALTER TABLE profits
    ALTER COLUMN fee TYPE NUMERIC(16, 8);
-- +end

-- +begin
ALTER TABLE profits
    ALTER COLUMN fee_in_usd TYPE NUMERIC(16, 8);
-- +end

-- +down

-- +begin
SELECT 1;
-- +end