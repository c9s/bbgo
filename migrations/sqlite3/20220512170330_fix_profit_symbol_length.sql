-- +up
-- +begin
-- We can not change column type in sqlite
-- However, SQLite does not enforce the length of a VARCHAR, i.e VARCHAR(8) == VARCHAR(20) == TEXT
SELECT 1;
-- +end

-- +down

-- +begin
SELECT 1;
-- +end
