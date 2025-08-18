-- !txn
-- +up
-- +begin
CREATE TABLE `withdraws`
(
    `gid`              INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange`         VARCHAR(24)    NOT NULL DEFAULT '',

    -- asset is the asset name (currency)
    `asset`            VARCHAR(10)    NOT NULL,

    `address`          VARCHAR(128)    NOT NULL,
    `network`          VARCHAR(32)    NOT NULL DEFAULT '',
    `amount`           DECIMAL(16, 8) NOT NULL,

    `txn_id`           VARCHAR(256)    NOT NULL,
    `txn_fee`          DECIMAL(16, 8) NOT NULL DEFAULT 0,
    `txn_fee_currency` VARCHAR(32)    NOT NULL DEFAULT '',
    `time`             DATETIME(3)    NOT NULL
);
-- +end

-- +begin
CREATE UNIQUE INDEX `withdraws_txn_id` ON `withdraws` (`exchange`, `txn_id`);
-- +end


-- +down

-- +begin
DROP INDEX IF EXISTS `withdraws_txn_id`;
-- +end

-- +begin
DROP TABLE IF EXISTS `withdraws`;
-- +end

