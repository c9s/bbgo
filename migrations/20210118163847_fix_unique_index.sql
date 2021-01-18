-- +up
-- +begin
ALTER TABLE `trades` DROP INDEX `id`;
-- +end
-- +begin
ALTER TABLE `trades` ADD UNIQUE INDEX `id` (`exchange`,`symbol`, `side`, `id`);
-- +end

-- +down
-- +begin
ALTER TABLE `trades` DROP INDEX `id`;
-- +end
-- +begin
ALTER TABLE `trades` ADD UNIQUE INDEX `id` (`id`);
-- +end
