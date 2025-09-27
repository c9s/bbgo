-- +up
-- +begin
ALTER TABLE trades ADD COLUMN is_futures BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +begin
ALTER TABLE orders ADD COLUMN is_futures BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +down

-- +begin
ALTER TABLE trades DROP COLUMN IF EXISTS is_futures;
-- +end

-- +begin
ALTER TABLE orders DROP COLUMN IF EXISTS is_futures;
-- +end