-- +goose Up
ALTER TABLE `trades`
    ADD COLUMN `order_id` BIGINT UNSIGNED NOT NULL;

-- +goose Down
ALTER TABLE `trades`
    DROP COLUMN `order_id`;
