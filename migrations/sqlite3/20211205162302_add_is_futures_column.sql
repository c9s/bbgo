-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `is_futures` BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +begin
ALTER TABLE `orders` ADD COLUMN `is_futures` BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +down

-- +begin
ALTER TABLE `trades` RENAME COLUMN `is_futures` TO `is_futures_deleted`;
-- +end

-- +begin
ALTER TABLE `orders` RENAME COLUMN `is_futures` TO `is_futures_deleted`;
-- +end
