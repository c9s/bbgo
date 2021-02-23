-- +up
CREATE TABLE `rewards`
(
    `gid`         BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    -- for exchange
    `exchange`    VARCHAR(24)             NOT NULL DEFAULT '',

    -- reward record id
    `uuid`        VARCHAR(32)             NOT NULL,
    `reward_type` VARCHAR(24)             NOT NULL DEFAULT '',

    -- currency symbol, BTC, MAX, USDT ... etc
    `currency`    VARCHAR(5)              NOT NULL,

    -- the quantity of the rewards
    `quantity`    DECIMAL(16, 8) UNSIGNED NOT NULL,

    `state`       VARCHAR(5)              NOT NULL,

    `created_at`  DATETIME                NOT NULL,

    `spent`       BOOLEAN                 NOT NULL DEFAULT FALSE,

    `note`        TEXT                    NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `uuid` (`exchange`, `uuid`)
);

-- +down
DROP TABLE IF EXISTS `rewards`;
