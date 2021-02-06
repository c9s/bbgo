-- +up
ALTER TABLE `trades`
    ADD COLUMN `order_id` BIGINT UNSIGNED NOT NULL;

-- +down
ALTER TABLE `trades`
    DROP COLUMN `order_id`;
