-- +up
-- +begin
ALTER TABLE `nav_history_details`
    MODIFY COLUMN `net_asset` DECIMAL(32, 8) DEFAULT 0.00000000 NOT NULL;
-- +end

-- +begin
ALTER TABLE `nav_history_details`
    ADD COLUMN `interest` DECIMAL(32, 20) UNSIGNED DEFAULT 0.00000000 NOT NULL;
-- +end

-- +down

-- +begin
ALTER TABLE `nav_history_details`
    DROP COLUMN `interest`;
-- +end
