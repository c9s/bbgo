-- @package xfundingv2
-- +up
-- +begin
ALTER TABLE `xfundingv2_round_snapshots` ADD COLUMN `started_at` DATETIME(3) NULL AFTER `total_net_pnl`;
-- +end

-- +down
-- +begin
ALTER TABLE `xfundingv2_round_snapshots` DROP COLUMN `started_at`;
-- +end
