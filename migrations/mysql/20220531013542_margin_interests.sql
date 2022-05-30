-- +up
CREATE TABLE `margin_interests`
(
    `gid`             BIGINT UNSIGNED          NOT NULL AUTO_INCREMENT,

    `transaction_id`  BIGINT UNSIGNED          NOT NULL,

    `exchange`        VARCHAR(24)              NOT NULL DEFAULT '',

    `asset`           VARCHAR(24)              NOT NULL DEFAULT '',

    `isolated_symbol` VARCHAR(24)              NOT NULL DEFAULT '',

    `principle`       DECIMAL(16, 8) UNSIGNED  NOT NULL,

    `interest`        DECIMAL(20, 16) UNSIGNED NOT NULL,

    `interest_rate`   DECIMAL(20, 16) UNSIGNED NOT NULL,

    `time`            DATETIME(3)              NOT NULL,

    PRIMARY KEY (`gid`),
    UNIQUE KEY (`transaction_id`)
);

-- +down
DROP TABLE IF EXISTS `margin_interests`;
