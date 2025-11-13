-- +up
-- +begin
DROP INDEX IF EXISTS `profits_trade_id`;
-- +end
-- +begin
CREATE UNIQUE INDEX `profits_trade_id` ON `profits` (`exchange`, `symbol`, `side`, `trade_id`);
-- +end

-- +down
-- +begin
DROP INDEX IF EXISTS `profits_trade_id`;
-- +end
-- +begin
CREATE UNIQUE INDEX `profits_trade_id` ON `profits` (`trade_id`);
-- +end