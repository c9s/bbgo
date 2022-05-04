-- +up
ALTER TABLE `nav_history_details` ADD COLUMN `session` VARCHAR(50) NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `borrowed` DECIMAL DEFAULT 0.00000000 NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `net_asset` DECIMAL DEFAULT 0.00000000 NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `price_in_usd` DECIMAL DEFAULT 0.00000000 NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `is_margin` BOOL DEFAULT FALSE NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `is_isolated` BOOL DEFAULT FALSE NOT NULL;
ALTER TABLE `nav_history_details` ADD COLUMN `isolated_symbol` VARCHAR(30) DEFAULT '' NOT NULL;

-- +down
-- we can not rollback alter table change in sqlite
SELECT 1;
