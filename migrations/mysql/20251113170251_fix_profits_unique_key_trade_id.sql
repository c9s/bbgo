-- +up
-- +begin
ALTER TABLE `profits` DROP INDEX `trade_id`;
-- +end
-- +begin
ALTER TABLE `profits` ADD UNIQUE KEY `trade_id` (`exchange`, `symbol`, `side`, `trade_id`);
-- +end

-- +down
-- +begin
ALTER TABLE `profits` DROP INDEX `trade_id`;
-- +end
-- +begin
ALTER TABLE `profits` ADD UNIQUE KEY `trade_id` (`trade_id`);
-- +end
