-- +up
-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `raw_asset` VARCHAR(24) NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `interest_type` VARCHAR(32) NOT NULL DEFAULT '';
-- +end

-- +begin
UPDATE `margin_interests` SET `raw_asset` = `asset` WHERE `raw_asset` = '';
-- +end

-- +down
-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `interest_type`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `raw_asset`;
-- +end
