-- @package xfundingv2
-- +up
-- +begin
ALTER TABLE `xfundingv2_closed_rounds` RENAME COLUMN `start_at` TO `started_at`;
-- +end

-- +down
-- +begin
ALTER TABLE `xfundingv2_closed_rounds` RENAME COLUMN `started_at` TO `start_at`;
-- +end
