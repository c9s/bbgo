-- +up
-- +begin
ALTER TABLE `nav_history_details`
    ADD COLUMN `session`      VARCHAR(30)                                NOT NULL,
    ADD COLUMN `is_margin`    BOOLEAN                                    NOT NULL DEFAULT FALSE,
    ADD COLUMN `is_isolated`  BOOLEAN                                    NOT NULL DEFAULT FALSE,
    ADD COLUMN `isolated_symbol`  VARCHAR(30)                            NOT NULL DEFAULT '',
    ADD COLUMN `net_asset`    DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    ADD COLUMN `borrowed`     DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    ADD COLUMN `price_in_usd` DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL
;
-- +end


-- +down

-- +begin
ALTER TABLE `nav_history_details`
    DROP COLUMN `session`,
    DROP COLUMN `net_asset`,
    DROP COLUMN `borrowed`,
    DROP COLUMN `price_in_usd`,
    DROP COLUMN `is_margin`,
    DROP COLUMN `is_isolated`,
    DROP COLUMN `isolated_symbol`
;
-- +end
