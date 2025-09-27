-- +up
CREATE TABLE margin_loans
(
    gid             BIGSERIAL               NOT NULL,

    transaction_id  BIGINT                  NOT NULL,

    exchange        VARCHAR(24)             NOT NULL DEFAULT '',

    asset           VARCHAR(24)             NOT NULL DEFAULT '',

    isolated_symbol VARCHAR(24)             NOT NULL DEFAULT '',

    -- quantity is the quantity of the trade that makes profit
    principle       NUMERIC(16, 8)          NOT NULL CHECK (principle >= 0),

    time            TIMESTAMP(3)            NOT NULL,

    PRIMARY KEY (gid),
    UNIQUE (transaction_id)
);

-- +down
DROP TABLE IF EXISTS margin_loans;