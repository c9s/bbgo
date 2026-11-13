-- @package xfundingv2
-- +up
-- +begin
ALTER TABLE `xfundingv2_closed_rounds`
    DROP KEY `idx_xfundingv2_closed_rounds_round_id`,
    ADD UNIQUE KEY `uk_xfundingv2_closed_rounds_id` (`id`);
-- +end

-- +begin
ALTER TABLE `xfundingv2_funding_fees`
    DROP KEY `idx_xfundingv2_funding_fees_round_id`,
    ADD UNIQUE KEY `uk_xfundingv2_funding_fees_round_id_txn` (`round_id`, `txn`);
-- +end

-- +down
-- +begin
ALTER TABLE `xfundingv2_funding_fees`
    DROP KEY `uk_xfundingv2_funding_fees_round_id_txn`,
    ADD KEY `idx_xfundingv2_funding_fees_round_id` (`round_id`);
-- +end

-- +begin
ALTER TABLE `xfundingv2_closed_rounds`
    DROP KEY `uk_xfundingv2_closed_rounds_id`,
    ADD KEY `idx_xfundingv2_closed_rounds_round_id` (`id`);
-- +end
