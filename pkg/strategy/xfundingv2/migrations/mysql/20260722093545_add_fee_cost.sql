-- @package xfundingv2
-- +up
-- +begin
ALTER TABLE `xfundingv2_closed_rounds`
    ADD COLUMN `fee_symbol`   VARCHAR(32)    NOT NULL DEFAULT '' AFTER `funding_income`,
    ADD COLUMN `fee_avg_cost` DECIMAL(20, 8) NOT NULL DEFAULT 0  AFTER `fee_symbol`;
-- +end

-- +begin
ALTER TABLE `xfundingv2_round_snapshots`
    ADD COLUMN `fee_symbol`   VARCHAR(32)    NOT NULL DEFAULT '' AFTER `funding_income`,
    ADD COLUMN `fee_avg_cost` DECIMAL(20, 8) NOT NULL DEFAULT 0  AFTER `fee_symbol`;
-- +end

-- +down
-- +begin
ALTER TABLE `xfundingv2_round_snapshots`
    DROP COLUMN `fee_avg_cost`,
    DROP COLUMN `fee_symbol`;
-- +end

-- +begin
ALTER TABLE `xfundingv2_closed_rounds`
    DROP COLUMN `fee_avg_cost`,
    DROP COLUMN `fee_symbol`;
-- +end
