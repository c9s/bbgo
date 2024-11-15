-- +up
-- +begin
ALTER TABLE `positions`
    ADD COLUMN `net_profit` DECIMAL(16, 8) DEFAULT 0.00000000 NOT NULL
;
-- +end


-- +down

-- +begin
ALTER TABLE `positions`
DROP COLUMN `net_profit`
;
-- +end
