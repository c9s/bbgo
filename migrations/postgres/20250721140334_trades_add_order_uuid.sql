-- +up
-- +begin
-- Add order_uuid column with correct length for full UUIDs (36 chars)
-- Use nullable first, then set default, then make NOT NULL to avoid lock issues
ALTER TABLE trades ADD COLUMN order_uuid VARCHAR(36);
UPDATE trades SET order_uuid = '' WHERE order_uuid IS NULL;
ALTER TABLE trades ALTER COLUMN order_uuid SET DEFAULT '';
ALTER TABLE trades ALTER COLUMN order_uuid SET NOT NULL;
-- +end

-- +down
-- +begin
ALTER TABLE trades DROP COLUMN order_uuid;
-- +end