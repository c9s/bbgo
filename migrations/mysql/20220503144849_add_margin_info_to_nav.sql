-- +up
-- +begin
ALTER TABLE `nav_history_details`
    ADD COLUMN `net_asset`     DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    ADD COLUMN `borrowed`      DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    ADD COLUMN `price_in_usd` DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL
;
-- +end


-- +down

-- +begin
ALTER TABLE `nav_history_details`
    DROP COLUMN `net_asset`,
    DROP COLUMN `borrowed`,
    DROP COLUMN `price_in_usd`
;
-- +end
