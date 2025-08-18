-- !txn
-- +up
-- +begin
CREATE TABLE `deposits`
(
    `gid`      INTEGER PRIMARY KEY AUTOINCREMENT,
    `exchange` VARCHAR(24)    NOT NULL,

    -- asset is the asset name (currency)
    `asset`    VARCHAR(10)    NOT NULL,

    `address`  VARCHAR(128)    NOT NULL DEFAULT '',
    `amount`   DECIMAL(16, 8) NOT NULL,
    `txn_id`   VARCHAR(256)    NOT NULL,
    `time`     DATETIME(3)    NOT NULL
);
-- +end
-- +begin
CREATE UNIQUE INDEX `deposits_txn_id` ON `deposits` (`exchange`, `txn_id`);
-- +end


-- +down

-- +begin
DROP INDEX IF EXISTS `deposits_txn_id`;
-- +end

-- +begin
DROP TABLE IF EXISTS `deposits`;
-- +end

