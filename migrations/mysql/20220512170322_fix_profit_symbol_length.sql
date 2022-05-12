-- +up
-- +begin
ALTER TABLE profits
    CHANGE symbol symbol VARCHAR(20) NOT NULL;
-- +end

-- +down

-- +begin
SELECT 1;
-- +end
