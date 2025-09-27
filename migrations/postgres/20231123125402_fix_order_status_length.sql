-- +up
-- +begin
ALTER TABLE orders
    ALTER COLUMN status TYPE VARCHAR(20);
-- +end

-- +down

-- +begin
SELECT 1;
-- +end