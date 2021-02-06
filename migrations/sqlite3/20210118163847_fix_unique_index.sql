-- +up
-- +begin
CREATE UNIQUE INDEX `trade_unique_id` ON `trades` (`exchange`,`symbol`, `side`, `id`);
-- +end

-- +down
-- +begin
DROP INDEX IF EXISTS `trade_unique_id`;
-- +end
