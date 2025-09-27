-- +up
-- +begin
CREATE TABLE withdraws
(
    gid              BIGSERIAL               NOT NULL,
    exchange         VARCHAR(24)             NOT NULL DEFAULT '',

    -- asset is the asset name (currency)
    asset            VARCHAR(10)             NOT NULL,

    address          VARCHAR(128)            NOT NULL,
    network          VARCHAR(32)             NOT NULL DEFAULT '',

    amount           NUMERIC(16, 8)          NOT NULL,
    txn_id           VARCHAR(256)            NOT NULL,
    txn_fee          NUMERIC(16, 8)          NOT NULL DEFAULT 0,
    txn_fee_currency VARCHAR(32)             NOT NULL DEFAULT '',
    time             TIMESTAMP(3)            NOT NULL,

    PRIMARY KEY (gid),
    UNIQUE (exchange, txn_id)
);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS withdraws;
-- +end