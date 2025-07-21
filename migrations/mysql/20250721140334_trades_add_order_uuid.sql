-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `order_uuid` VARBINARY(16) NOT NULL DEFAULT '';
-- +end

-- +down
-- +begin
ALTER TABLE `trades` DROP COLUMN `order_uuid`;
-- +end