-- !txn
-- +up
-- +begin
CREATE TABLE `nav_history_details`
(
    `gid`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `exchange`         VARCHAR(30)                NOT NULL DEFAULT '',
    `subaccount`       VARCHAR(30)                NOT NULL DEFAULT '',
    `time`             DATETIME(3)                NOT NULL DEFAULT (strftime('%s', 'now')),
    `currency`         VARCHAR(30)                NOT NULL,
    `net_asset_in_usd` DECIMAL DEFAULT 0.00000000 NOT NULL,
    `net_asset_in_btc` DECIMAL DEFAULT 0.00000000 NOT NULL,
    `balance`          DECIMAL DEFAULT 0.00000000 NOT NULL,
    `available`        DECIMAL DEFAULT 0.00000000 NOT NULL,
    `locked`           DECIMAL DEFAULT 0.00000000 NOT NULL
);
-- +end
-- +begin
CREATE INDEX idx_nav_history_details
    on nav_history_details (time, currency, exchange);
-- +end

-- +down

-- +begin
DROP TABLE nav_history_details;
-- +end
