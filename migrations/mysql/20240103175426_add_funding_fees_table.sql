-- +up
-- +begin
CREATE TABLE `funding_fees`
(
    `gid`        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    -- transaction id
    `txn_id`     BIGINT UNSIGNED NOT NULL,

    -- for exchange
    `exchange`   VARCHAR(24)    NOT NULL DEFAULT '',

    -- asset name, BTC, MAX, USDT ... etc
    `asset`      VARCHAR(5)     NOT NULL,

    -- the amount of the funding fee
    `amount`     DECIMAL(16, 8) NOT NULL,

    `funded_at`  DATETIME       NOT NULL,

    PRIMARY KEY (`gid`),

    UNIQUE KEY `txn_id` (`txn_id`, `exchange`)
);
-- +end

-- +down

-- +begin
DROP TABLE `funding_fees`;
-- +end
