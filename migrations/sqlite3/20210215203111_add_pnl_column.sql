-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `pnl` DECIMAL NULL;
-- +end

-- +begin
ALTER TABLE `trades` ADD COLUMN `strategy` TEXT;
-- +end

-- +down

-- +begin
ALTER TABLE `trades` RENAME COLUMN `pnl` TO `pnl_deleted`;
-- +end

-- +begin
ALTER TABLE `trades` RENAME COLUMN `strategy` TO `strategy_deleted`;
-- +end
