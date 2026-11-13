-- @package xfundingv2
-- +up
-- +begin
ALTER TABLE `xfundingv2_closed_rounds` ADD COLUMN `fee_symbol` TEXT NOT NULL DEFAULT '';
-- +end
-- +begin
ALTER TABLE `xfundingv2_closed_rounds` ADD COLUMN `fee_avg_cost` REAL NOT NULL DEFAULT 0;
-- +end
-- +begin
ALTER TABLE `xfundingv2_round_snapshots` ADD COLUMN `fee_symbol` TEXT NOT NULL DEFAULT '';
-- +end
-- +begin
ALTER TABLE `xfundingv2_round_snapshots` ADD COLUMN `fee_avg_cost` REAL NOT NULL DEFAULT 0;
-- +end

-- +down
-- +begin
ALTER TABLE `xfundingv2_round_snapshots` DROP COLUMN `fee_avg_cost`;
-- +end
-- +begin
ALTER TABLE `xfundingv2_round_snapshots` DROP COLUMN `fee_symbol`;
-- +end
-- +begin
ALTER TABLE `xfundingv2_closed_rounds` DROP COLUMN `fee_avg_cost`;
-- +end
-- +begin
ALTER TABLE `xfundingv2_closed_rounds` DROP COLUMN `fee_symbol`;
-- +end
