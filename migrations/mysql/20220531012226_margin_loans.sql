-- +up
CREATE TABLE `margin_loans`
(
    `gid`             BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,

    `transaction_id`  BIGINT UNSIGNED         NOT NULL,

    `exchange`        VARCHAR(24)             NOT NULL DEFAULT '',

    `asset`           VARCHAR(24)             NOT NULL DEFAULT '',

    `isolated_symbol` VARCHAR(24)             NOT NULL DEFAULT '',

    -- quantity is the quantity of the trade that makes profit
    `principle`       DECIMAL(16, 8) UNSIGNED NOT NULL,

    `time`            DATETIME(3)             NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY (`transaction_id`)
);

-- +down
DROP TABLE IF EXISTS `margin_loans`;
