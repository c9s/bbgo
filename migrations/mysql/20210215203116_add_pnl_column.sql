-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `pnl` DECIMAL NULL;
-- +end

-- +begin
ALTER TABLE `trades` ADD COLUMN `strategy` VARCHAR(32) NULL;
-- +end


-- +down

-- +begin
ALTER TABLE `trades` DROP COLUMN `pnl`;
-- +end

-- +begin
ALTER TABLE `trades` DROP COLUMN `strategy`;
-- +end
