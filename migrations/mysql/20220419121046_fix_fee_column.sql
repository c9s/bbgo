-- +up
-- +begin
ALTER TABLE trades
    CHANGE fee fee DECIMAL(16, 8) NOT NULL;
-- +end

-- +begin
ALTER TABLE profits
    CHANGE fee fee DECIMAL(16, 8) NOT NULL;
-- +end

-- +begin
ALTER TABLE profits
    CHANGE fee_in_usd fee_in_usd DECIMAL(16, 8);
-- +end

-- +down

-- +begin
SELECT 1;
-- +end
