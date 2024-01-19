-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `order_id` INTEGER;
-- +end

-- +down
-- +begin
ALTER TABLE `trades` RENAME COLUMN `order_id` TO `order_id_deleted`;
-- +end
