-- +up
-- +begin
ALTER TABLE profits ALTER COLUMN symbol TYPE VARCHAR(32);
-- +end

-- +down

-- +begin
SELECT 1;
-- +end