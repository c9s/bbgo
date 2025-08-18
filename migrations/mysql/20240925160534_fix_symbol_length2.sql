-- +up
-- +begin
ALTER TABLE profits MODIFY COLUMN symbol VARCHAR(32) NOT NULL;
-- +end

-- +begin
ALTER TABLE profits MODIFY COLUMN base_currency VARCHAR(16) NOT NULL;
-- +end

-- +begin
ALTER TABLE profits MODIFY COLUMN fee_currency VARCHAR(16) NOT NULL;
-- +end

-- +begin
ALTER TABLE positions MODIFY COLUMN base_currency VARCHAR(16) NOT NULL;
-- +end

-- +begin
ALTER TABLE positions MODIFY COLUMN symbol VARCHAR(32) NOT NULL;
-- +end

-- +down

-- +begin
SELECT 1;
-- +end
