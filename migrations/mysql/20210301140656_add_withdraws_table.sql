-- +up
-- +begin
CREATE TABLE `withdraws`
(
    `gid`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `exchange`         VARCHAR(24)     NOT NULL DEFAULT '',

    -- asset is the asset name (currency)
    `asset`            VARCHAR(10)     NOT NULL,

    `address`          VARCHAR(64)     NOT NULL,
    `network`          VARCHAR(32)     NOT NULL DEFAULT '',

    `amount`           DECIMAL(16, 8)  NOT NULL,
    `txn_id`           VARCHAR(64)     NOT NULL,
    `txn_fee`          DECIMAL(16, 8)  NOT NULL DEFAULT 0,
    `txn_fee_currency` VARCHAR(32)     NOT NULL DEFAULT '',
    `time`             DATETIME(3)     NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `txn_id` (`exchange`, `txn_id`)
);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS `withdraws`;
-- +end
