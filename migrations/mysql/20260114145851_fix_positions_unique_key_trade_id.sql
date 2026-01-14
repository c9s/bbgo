-- +up
-- +begin
ALTER TABLE `positions` DROP INDEX `trade_id`;
-- +end
-- +begin
ALTER TABLE `positions` ADD UNIQUE KEY `trade_id` (`trade_id`, `side`, `symbol`, `exchange`);
-- +end

-- +down
-- +begin
ALTER TABLE `positions` DROP INDEX `trade_id`;
-- +end
-- +begin
ALTER TABLE `positions` ADD UNIQUE KEY `trade_id` (`trade_id`, `side`, `exchange`);
-- +end
