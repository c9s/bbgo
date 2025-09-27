-- +up
-- +begin
ALTER TABLE nav_history_details
    ALTER COLUMN net_asset TYPE NUMERIC(32, 8),
    ALTER COLUMN net_asset SET DEFAULT 0.00000000,
    ALTER COLUMN net_asset SET NOT NULL;
-- +end

-- +begin
ALTER TABLE nav_history_details
    RENAME COLUMN balance_in_usd TO net_asset_in_usd;
-- +end

-- +begin
ALTER TABLE nav_history_details
    ALTER COLUMN net_asset_in_usd TYPE NUMERIC(32, 2),
    ALTER COLUMN net_asset_in_usd SET DEFAULT 0.00000000,
    ALTER COLUMN net_asset_in_usd SET NOT NULL;
-- +end

-- +begin
ALTER TABLE nav_history_details
    RENAME COLUMN balance_in_btc TO net_asset_in_btc;
-- +end

-- +begin
ALTER TABLE nav_history_details
    ALTER COLUMN net_asset_in_btc TYPE NUMERIC(32, 20),
    ALTER COLUMN net_asset_in_btc SET DEFAULT 0.00000000,
    ALTER COLUMN net_asset_in_btc SET NOT NULL;
-- +end

-- +begin
ALTER TABLE nav_history_details
    ADD COLUMN interest NUMERIC(32, 20) DEFAULT 0.00000000 NOT NULL CHECK (interest >= 0);
-- +end

-- +down

-- +begin
ALTER TABLE nav_history_details
    DROP COLUMN IF EXISTS interest;
-- +end