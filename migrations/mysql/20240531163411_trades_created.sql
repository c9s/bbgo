-- +up
-- +begin
ALTER TABLE `trades` ADD COLUMN `inserted_at` DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL AFTER `traded_at`;
-- +end

-- +begin
UPDATE `trades` SET `inserted_at` = `traded_at`;
-- +end

-- +down

-- +begin
ALTER TABLE `trades` DROP COLUMN `inserted_at`;
-- +end
