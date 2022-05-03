-- +up
ALTER TABLE `nav_history_details` ADD COLUMN `session` VARCHAR(50) NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `borrowed` DECIMAL UNSIGNED DEFAULT 0.00000000 NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `net_asset` DECIMAL UNSIGNED DEFAULT 0.00000000 NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `price_in_usd` DECIMAL UNSIGNED DEFAULT 0.00000000 NOT NULL;

-- +down
-- we can not rollback alter table change in sqlite
SELECT 1;
