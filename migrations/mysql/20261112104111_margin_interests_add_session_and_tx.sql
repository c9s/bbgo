-- +up
-- session is the bbgo session the row was synced for. tx_id, raw_asset and
-- interest_type are stored as Binance returns them: txId, rawAsset and type.
-- principle is renamed to principal, Binance's name of the field.
-- Rows synced before this migration keep session '' and tx_id NULL; a NULL
-- tx_id never collides in the unique key, so they stay as they are.
-- +begin
ALTER TABLE `margin_interests`
    ADD COLUMN `session`       VARCHAR(64)     NOT NULL DEFAULT '',
    ADD COLUMN `tx_id`         BIGINT UNSIGNED NULL,
    ADD COLUMN `raw_asset`     VARCHAR(24)     NOT NULL DEFAULT '',
    ADD COLUMN `interest_type` VARCHAR(32)     NOT NULL DEFAULT '',
    CHANGE `principle` `principal` DECIMAL(32, 8) UNSIGNED NOT NULL,
    MODIFY `interest` DECIMAL(32, 16) UNSIGNED NOT NULL,
    ADD UNIQUE KEY `uidx_margin_interests_exchange_session_tx` (`exchange`, `session`, `tx_id`);
-- +end

-- +begin
CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);
-- +end

-- +down

-- +begin
DROP INDEX `idx_margin_interests_session_time` ON `margin_interests`;
-- +end

-- the widened principal and interest columns are kept: narrowing them could
-- truncate the values stored since the upgrade
-- +begin
ALTER TABLE `margin_interests`
    DROP INDEX `uidx_margin_interests_exchange_session_tx`,
    CHANGE `principal` `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,
    DROP COLUMN `interest_type`,
    DROP COLUMN `raw_asset`,
    DROP COLUMN `tx_id`,
    DROP COLUMN `session`;
-- +end
