-- +up
-- +begin
ALTER TABLE `orders` ADD COLUMN `uuid` VARBINARY(36) NOT NULL DEFAULT '';
-- +end

-- +down
-- +begin
ALTER TABLE `orders` DROP COLUMN `uuid`;
-- +end