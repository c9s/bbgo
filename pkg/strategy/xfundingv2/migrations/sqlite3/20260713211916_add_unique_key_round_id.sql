-- @package xfundingv2
-- +up
-- +begin
DROP INDEX IF EXISTS `idx_xfundingv2_closed_rounds_round_id`;
-- +end

-- +begin
CREATE UNIQUE INDEX `uk_xfundingv2_closed_rounds_id` ON `xfundingv2_closed_rounds` (`id`);
-- +end

-- +begin
DROP INDEX IF EXISTS `idx_xfundingv2_funding_fees_round_id`;
-- +end

-- +begin
CREATE UNIQUE INDEX `uk_xfundingv2_funding_fees_round_id_txn` ON `xfundingv2_funding_fees` (`round_id`, `txn`);
-- +end

-- +down
-- +begin
DROP INDEX IF EXISTS `uk_xfundingv2_funding_fees_round_id_txn`;
-- +end

-- +begin
CREATE INDEX `idx_xfundingv2_funding_fees_round_id` ON `xfundingv2_funding_fees` (`round_id`);
-- +end

-- +begin
DROP INDEX IF EXISTS `uk_xfundingv2_closed_rounds_id`;
-- +end

-- +begin
CREATE INDEX `idx_xfundingv2_closed_rounds_round_id` ON `xfundingv2_closed_rounds` (`id`);
-- +end
