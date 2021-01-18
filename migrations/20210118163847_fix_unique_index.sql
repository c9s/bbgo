-- +up
-- +begin
ALTER TABLE `trades` DROP INDEX `id`;
-- +end
-- +begin
ALTER TABLE `trades` ADD INDEX UNIQUE `id` ON (`exchange`,`symbol`, `side`, `id`);
-- +end

-- +down
-- +begin
ALTER TABLE `trades` DROP INDEX `id`;
-- +end
-- +begin
ALTER TABLE `trades` ADD INDEX UNIQUE `id` ON (`id`);
-- +end
