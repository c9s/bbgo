-- !txn
-- +up
CREATE TABLE `margin_interests`
(
    `gid`             INTEGER PRIMARY KEY AUTOINCREMENT,

    `exchange`        VARCHAR(24)     NOT NULL DEFAULT '',

    `asset`           VARCHAR(24)     NOT NULL DEFAULT '',

    `isolated_symbol` VARCHAR(24)     NOT NULL DEFAULT '',

    `principle`       DECIMAL(16, 8)  NOT NULL,

    `interest`        DECIMAL(20, 16) NOT NULL,

    `interest_rate`   DECIMAL(20, 16) NOT NULL,

    `time`            DATETIME(3)     NOT NULL
);

-- +down
DROP TABLE IF EXISTS `margin_interests`;
