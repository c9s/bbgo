-- +up
-- raw_asset is the borrowed asset: asset itself, or the asset borrowed when the
-- interest is paid in another asset (e.g. Binance BNB deduction, asset = BNB).
-- interest_type is the exchange interest type, e.g. PERIODIC or PERIODIC_CONVERTED.
-- +begin
ALTER TABLE `margin_interests`
    ADD COLUMN `raw_asset` VARCHAR(24) NOT NULL DEFAULT '',
    ADD COLUMN `interest_type` VARCHAR(32) NOT NULL DEFAULT '';
-- +end

-- rows synced before have no type; they are counted as paid in the borrowed asset
-- +begin
UPDATE `margin_interests` SET `raw_asset` = `asset` WHERE `raw_asset` = '';
-- +end

-- +down
-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `interest_type`, DROP COLUMN `raw_asset`;
-- +end
