-- +up
-- +begin
ALTER TABLE `nav_history_details` ADD COLUMN `interest` DECIMAL DEFAULT 0.00000000 NOT NULL;
-- +end


-- +down

-- +begin
SELECT 1;
-- +end
