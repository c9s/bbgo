-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE;
-- +end
-- +begin
ALTER TABLE `trades` ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +begin
ALTER TABLE `orders` ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +begin
ALTER TABLE `orders` ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE;
-- +end

-- +down

-- +begin
ALTER TABLE `trades` RENAME COLUMN `is_margin` TO `is_margin_deleted`;
-- +end

-- +begin
ALTER TABLE `trades` RENAME COLUMN `is_isolated` TO `is_isolated_deleted`;
-- +end

-- +begin
ALTER TABLE `orders` RENAME COLUMN `is_margin` TO `is_margin_deleted`;
-- +end

-- +begin
ALTER TABLE `orders` RENAME COLUMN `is_isolated` TO `is_isolated_deleted`;
-- +end
