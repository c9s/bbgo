-- +up
-- +begin
ALTER TABLE trades ADD COLUMN inserted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL;
-- +end

-- +begin
UPDATE trades SET inserted_at = traded_at;
-- +end

-- +down

-- +begin
ALTER TABLE trades DROP COLUMN IF EXISTS inserted_at;
-- +end