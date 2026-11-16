-- +up
-- margin_interest_valuations values each margin_interests row in USDT at the
-- time it was charged, from the close of the 1h kline before the charge hour,
-- in the asset paid and in the asset borrowed.
-- It is derived from margin_interests and the kline tables and can be emptied
-- and recomputed at any time.
-- +begin
CREATE TABLE `margin_interest_valuations`
(
    `gid`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `interest_gid`      BIGINT UNSIGNED NOT NULL,

    `exchange`          VARCHAR(24)     NOT NULL,
    `session`           VARCHAR(64)     NOT NULL,
    `time`              DATETIME(3)     NOT NULL,
    `isolated_symbol`   VARCHAR(24)     NOT NULL DEFAULT '',

    `paid_asset`        VARCHAR(24)     NOT NULL,
    -- '' when unknown: a converted row without rawAsset
    `borrowed_asset`    VARCHAR(24)     NOT NULL DEFAULT '',
    `interest_type`     VARCHAR(32)     NOT NULL DEFAULT '',
    `converted`         BOOLEAN         NOT NULL DEFAULT FALSE,

    -- in borrowed_asset units
    `principal`         DECIMAL(32, 16) UNSIGNED NOT NULL,
    -- the exchange's rate, daily on Binance
    `interest_rate`     DECIMAL(32, 16) UNSIGNED NOT NULL,
    -- in paid_asset units
    `interest`          DECIMAL(32, 16) UNSIGNED NOT NULL,
    -- in borrowed_asset units, estimated on converted rows; NULL when unknown
    `borrowed_interest` DECIMAL(32, 16) UNSIGNED NULL,

    `price_symbol`      VARCHAR(24)     NOT NULL DEFAULT '',
    `price_kline_start` DATETIME(3)     NULL,
    -- NULL until priced
    `price_in_usdt`     DECIMAL(32, 16) UNSIGNED NULL,
    `interest_usdt`     DECIMAL(32, 16) UNSIGNED NULL,

    -- the borrowed asset priced like the paid one, at the same kline
    `borrowed_price_symbol`  VARCHAR(24)     NOT NULL DEFAULT '',
    `borrowed_price_in_usdt` DECIMAL(32, 16) UNSIGNED NULL,
    `borrowed_interest_usdt` DECIMAL(32, 16) UNSIGNED NULL,
    `principal_usdt`         DECIMAL(32, 16) UNSIGNED NULL,

    -- priced, unknown_borrowed, no_market, or kline_missing (retried)
    `price_status`      VARCHAR(16)     NOT NULL,

    `updated_at`        DATETIME(3)     NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `uidx_margin_interest_valuations_interest` (`interest_gid`),
    KEY `idx_margin_interest_valuations_session_time` (`exchange`, `session`, `time`),
    KEY `idx_margin_interest_valuations_session_borrowed_time` (`exchange`, `session`, `borrowed_asset`, `time`)
);
-- +end

-- +down

-- +begin
DROP TABLE IF EXISTS `margin_interest_valuations`;
-- +end
