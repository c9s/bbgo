-- +up
-- +begin
CREATE UNIQUE INDEX `positions_trade_id` ON `positions` (`trade_id`, `side`, `symbol`, `exchange`);
-- +end

-- +down
-- +begin
DROP INDEX IF EXISTS `positions_trade_id`;
-- +end
