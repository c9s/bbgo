-- +up
-- +begin
ALTER TABLE trades CHANGE fee_currency fee_currency varchar(10) NOT NULL;
-- +end

-- +down
-- +begin
ALTER TABLE trades CHANGE fee_currency fee_currency varchar(4) NOT NULL;
-- +end
