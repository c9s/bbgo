-- +up
-- +begin
CREATE TABLE deposits
(
    gid      BIGSERIAL               NOT NULL,
    exchange VARCHAR(24)             NOT NULL,

    -- asset is the asset name (currency)
    asset    VARCHAR(10)             NOT NULL,

    address  VARCHAR(128)            NOT NULL DEFAULT '',
    amount   NUMERIC(16, 8)          NOT NULL,
    txn_id   VARCHAR(256)            NOT NULL,
    time     TIMESTAMP(3)            NOT NULL,

    PRIMARY KEY (gid),
    UNIQUE (exchange, txn_id)
);
-- +end


-- +down

-- +begin
DROP TABLE IF EXISTS deposits;
-- +end