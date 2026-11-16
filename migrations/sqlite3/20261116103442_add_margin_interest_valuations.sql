-- +up
-- margin_interest_valuations values each margin_interests row in USDT at the
-- time it was charged, from the close of the 1h kline before the charge hour.
-- It is derived from margin_interests and the kline tables and can be emptied
-- and recomputed at any time.
-- +begin
CREATE TABLE `margin_interest_valuations`
(
    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,
    `interest_gid`      INTEGER     NOT NULL,

    `exchange`          TEXT        NOT NULL,
    `session`           TEXT        NOT NULL,
    `time`              DATETIME(3) NOT NULL,
    `isolated_symbol`   TEXT        NOT NULL DEFAULT '',

    `paid_asset`        TEXT        NOT NULL,
    -- '' when unknown: a converted row without rawAsset
    `borrowed_asset`    TEXT        NOT NULL DEFAULT '',
    `converted`         BOOLEAN     NOT NULL DEFAULT FALSE,

    -- in paid_asset units
    `interest`          REAL        NOT NULL,
    -- in borrowed_asset units, estimated on converted rows; NULL when unknown
    `borrowed_interest` REAL        NULL,

    `price_symbol`      TEXT        NOT NULL DEFAULT '',
    `price_kline_start` DATETIME(3) NULL,
    -- NULL until priced
    `price_in_usdt`     REAL        NULL,
    `interest_usdt`     REAL        NULL,

    `updated_at`        DATETIME(3) NOT NULL
);
-- +end

-- +begin
CREATE UNIQUE INDEX `uidx_margin_interest_valuations_interest` ON `margin_interest_valuations` (`interest_gid`);
-- +end

-- +begin
CREATE INDEX `idx_margin_interest_valuations_session_time` ON `margin_interest_valuations` (`exchange`, `session`, `time`);
-- +end

-- +begin
CREATE INDEX `idx_margin_interest_valuations_session_borrowed_time` ON `margin_interest_valuations` (`exchange`, `session`, `borrowed_asset`, `time`);
-- +end

-- +down

-- +begin
DROP INDEX IF EXISTS `idx_margin_interest_valuations_session_borrowed_time`;
-- +end

-- +begin
DROP INDEX IF EXISTS `idx_margin_interest_valuations_session_time`;
-- +end

-- +begin
DROP INDEX IF EXISTS `uidx_margin_interest_valuations_interest`;
-- +end

-- +begin
DROP TABLE IF EXISTS `margin_interest_valuations`;
-- +end
