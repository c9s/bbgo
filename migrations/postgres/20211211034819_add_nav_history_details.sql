-- +up
-- +begin
CREATE TABLE nav_history_details
(
    gid            BIGSERIAL                           NOT NULL PRIMARY KEY,
    exchange       VARCHAR(30)                         NOT NULL,
    subaccount     VARCHAR(30)                         NOT NULL,
    time           TIMESTAMP(3)                        NOT NULL,
    currency       VARCHAR(12)                         NOT NULL,
    balance_in_usd NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (balance_in_usd >= 0),
    balance_in_btc NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (balance_in_btc >= 0),
    balance        NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (balance >= 0),
    available      NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (available >= 0),
    locked         NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (locked >= 0)
);
-- +end
-- +begin
CREATE INDEX idx_nav_history_details
    ON nav_history_details (time, currency, exchange);
-- +end

-- +down

-- +begin
DROP TABLE IF EXISTS nav_history_details;
-- +end