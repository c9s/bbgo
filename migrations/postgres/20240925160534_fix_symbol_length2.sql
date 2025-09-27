-- +up
-- +begin
ALTER TABLE profits ALTER COLUMN symbol TYPE VARCHAR(32);
-- +end

-- +begin
ALTER TABLE profits ALTER COLUMN base_currency TYPE VARCHAR(16);
-- +end

-- +begin
ALTER TABLE profits ALTER COLUMN fee_currency TYPE VARCHAR(16);
-- +end

-- +begin
ALTER TABLE positions ALTER COLUMN base_currency TYPE VARCHAR(16);
-- +end

-- +begin
ALTER TABLE positions ALTER COLUMN symbol TYPE VARCHAR(32);
-- +end

-- +down

-- +begin
SELECT 1;
-- +end