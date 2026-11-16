-- +up
-- session is the bbgo session the row was synced for. tx_id, raw_asset and
-- interest_type are stored as Binance returns them: txId, rawAsset and type.
-- principle is renamed to principal, Binance's name of the field.
-- Rows synced before this migration keep session '' and tx_id NULL; a NULL
-- tx_id never collides in the unique index, so they stay as they are.
-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `session` TEXT NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `tx_id` BIGINT NULL;
-- +end

-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `raw_asset` TEXT NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_interests` ADD COLUMN `interest_type` TEXT NOT NULL DEFAULT '';
-- +end

-- +begin
ALTER TABLE `margin_interests` RENAME COLUMN `principle` TO `principal`;
-- +end

-- +begin
CREATE UNIQUE INDEX `uidx_margin_interests_exchange_session_tx` ON `margin_interests` (`exchange`, `session`, `tx_id`);
-- +end

-- +begin
CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS `idx_margin_interests_session_time`;
-- +end

-- +begin
DROP INDEX IF EXISTS `uidx_margin_interests_exchange_session_tx`;
-- +end

-- +begin
ALTER TABLE `margin_interests` RENAME COLUMN `principal` TO `principle`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `interest_type`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `raw_asset`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `tx_id`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `session`;
-- +end
