-- +up
-- +begin
ALTER TABLE nav_history_details
    ADD COLUMN session      VARCHAR(30)                               NOT NULL,
    ADD COLUMN is_margin    BOOLEAN                                   NOT NULL DEFAULT FALSE,
    ADD COLUMN is_isolated  BOOLEAN                                   NOT NULL DEFAULT FALSE,
    ADD COLUMN isolated_symbol VARCHAR(30)                            NOT NULL DEFAULT '',
    ADD COLUMN net_asset    NUMERIC(32, 8) DEFAULT 0.00000000        NOT NULL CHECK (net_asset >= 0),
    ADD COLUMN borrowed     NUMERIC(32, 8) DEFAULT 0.00000000        NOT NULL CHECK (borrowed >= 0),
    ADD COLUMN price_in_usd NUMERIC(32, 8) DEFAULT 0.00000000        NOT NULL CHECK (price_in_usd >= 0);
-- +end


-- +down

-- +begin
ALTER TABLE nav_history_details
    DROP COLUMN IF EXISTS session,
    DROP COLUMN IF EXISTS net_asset,
    DROP COLUMN IF EXISTS borrowed,
    DROP COLUMN IF EXISTS price_in_usd,
    DROP COLUMN IF EXISTS is_margin,
    DROP COLUMN IF EXISTS is_isolated,
    DROP COLUMN IF EXISTS isolated_symbol;
-- +end