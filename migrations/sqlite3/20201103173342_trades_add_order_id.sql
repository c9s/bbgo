-- +up
ALTER TABLE `trades` ADD COLUMN `order_id` INTEGER;

-- +down
ALTER TABLE `trades` RENAME COLUMN `order_id` TO `order_id_deleted`;
