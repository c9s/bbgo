-- +up
-- +begin
CREATE TABLE `deposits`
(
    `gid`      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `exchange` VARCHAR(24)     NOT NULL,

    -- asset is the asset name (currency)
    `asset`    VARCHAR(10)     NOT NULL,

    `address`  VARCHAR(64)     NOT NULL DEFAULT '',
    `amount`   DECIMAL(16, 8)  NOT NULL,
    `txn_id`   VARCHAR(64)     NOT NULL,
    `time`     DATETIME(3)     NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY `txn_id` (`exchange`, `txn_id`)
);
-- +end


-- +down

-- +begin
DROP TABLE IF EXISTS `deposits`;
-- +end
