-- +up
-- +begin
ALTER TABLE `orders` ADD COLUMN `uuid` VARCHAR(255) NOT NULL DEFAULT '';
-- +end

-- +down
-- +begin
ALTER TABLE `orders` DROP COLUMN `uuid`;
-- +end