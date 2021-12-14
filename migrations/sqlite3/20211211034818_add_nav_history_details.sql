-- +up
-- +begin
CREATE TABLE `nav_history_details`
(
    gid    bigint unsigned auto_increment PRIMARY KEY,
    `exchange`          VARCHAR             NOT NULL DEFAULT '',
    `subaccount`          VARCHAR             NOT NULL DEFAULT '',
    time   DATETIME(3)     NOT NULL DEFAULT (strftime('%s','now')),
    currency     VARCHAR                                NOT NULL,
    balance_in_usd DECIMAL DEFAULT 0.00000000 NOT NULL,
    balance_in_btc DECIMAL DEFAULT 0.00000000 NOT NULL,
    balance    DECIMAL DEFAULT 0.00000000 NOT NULL,
    available  DECIMAL DEFAULT 0.00000000 NOT NULL,
    locked     DECIMAL DEFAULT 0.00000000 NOT NULL
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
