-- +up
CREATE TABLE margin_interests
(
    gid             BIGSERIAL               NOT NULL,

    exchange        VARCHAR(24)             NOT NULL DEFAULT '',

    asset           VARCHAR(24)             NOT NULL DEFAULT '',

    isolated_symbol VARCHAR(24)             NOT NULL DEFAULT '',

    principle       NUMERIC(16, 8)          NOT NULL CHECK (principle >= 0),

    interest        NUMERIC(20, 16)         NOT NULL CHECK (interest >= 0),

    interest_rate   NUMERIC(20, 16)         NOT NULL CHECK (interest_rate >= 0),

    time            TIMESTAMP(3)            NOT NULL,

    PRIMARY KEY (gid)
);

-- +down
DROP TABLE IF EXISTS margin_interests;