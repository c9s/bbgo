-- +goose Up
ALTER TABLE `trades` ADD COLUMN `order_id` BIGINT UNSIGNED;

-- +goose Down
ALTER TABLE `trades` DROP COLUMN `order_id`;
