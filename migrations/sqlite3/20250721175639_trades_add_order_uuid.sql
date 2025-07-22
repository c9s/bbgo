-- +up
-- +begin
ALTER TABLE orders ADD COLUMN uuid TEXT NOT NULL DEFAULT '';
-- +end

-- +down
-- +begin
ALTER TABLE orders DROP COLUMN uuid;
-- +end