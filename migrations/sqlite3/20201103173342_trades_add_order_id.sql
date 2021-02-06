-- +up
ALTER TABLE `trades` ADD COLUMN `order_id` INTEGER NOT NULL DEFATUL 0;

-- +down
ALTER TABLE `trades` RENAME COLUMN `order_id` TO `order_id_deleted`;
