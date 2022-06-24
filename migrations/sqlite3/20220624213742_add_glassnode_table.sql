-- +up
-- +begin
CREATE TABLE `glassnode`
(
    `gid`       BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT,
    `category`  VARCHAR(64)         NOT NULL,
    `metric`    VARCHAR(64)         NOT NULL,
    `asset`     VARCHAR(20)         NOT NULL,
    `interval`  VARCHAR(6)          NOT NULL,
    `time`      DATETIME(3)         NOT NULL,
    `key`       VARCHAR(64)         NOT NULL,
    `value`     DECIMAL(32, 8)      NOT NULL,

    PRIMARY KEY (`gid`)
);
-- +end

-- +down

-- +begin
DROP TABLE IF EXISTS `glassnode`;
-- +end
