-- +up
-- +begin
CREATE TABLE nav_history_details
(
    gid            bigint unsigned auto_increment PRIMARY KEY,
    exchange       VARCHAR(30)                                NOT NULL,
    subaccount     VARCHAR(30)                                NOT NULL,
    time           DATETIME(3)                                NOT NULL,
    currency       VARCHAR(12)                                NOT NULL,
    balance_in_usd DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    balance_in_btc DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    balance        DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    available      DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,
    locked         DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL
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
