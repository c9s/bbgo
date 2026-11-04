-- +up
-- +begin
ALTER TABLE `margin_loans` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_repays` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_liquidations` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `price_in_usdt` DECIMAL(32, 16) NOT NULL DEFAULT 0;
-- +end

-- +begin
CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS `idx_margin_interests_session_time`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `price_in_usdt`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `session`;
-- +end

-- +begin
ALTER TABLE `margin_liquidations` DROP COLUMN `session`;
-- +end

-- +begin
ALTER TABLE `margin_repays` DROP COLUMN `session`;
-- +end

-- +begin
ALTER TABLE `margin_loans` DROP COLUMN `session`;
-- +end
