-- +up
-- +begin
ALTER TABLE trades ADD COLUMN inserted_at TEXT;

UPDATE trades SET inserted_at = traded_at;

CREATE TRIGGER set_inserted_at
AFTER INSERT ON trades
FOR EACH ROW
BEGIN
    UPDATE trades
    SET inserted_at = datetime('now')
    WHERE rowid = NEW.rowid;
END;

-- +end

-- +down

-- +begin
DROP TRIGGER set_inserted_at;
ALTER TABLE trades DROP COLUMN inserted_at;
-- +end
