-- +up
CREATE TABLE rewards
(
    gid         BIGSERIAL               NOT NULL,

    -- for exchange
    exchange    VARCHAR(24)             NOT NULL DEFAULT '',

    -- reward record id
    uuid        VARCHAR(32)             NOT NULL,
    reward_type VARCHAR(24)             NOT NULL DEFAULT '',

    -- currency symbol, BTC, MAX, USDT ... etc
    currency    VARCHAR(5)              NOT NULL,

    -- the quantity of the rewards
    quantity    NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),

    state       VARCHAR(5)              NOT NULL,

    created_at  TIMESTAMP               NOT NULL,

    spent       BOOLEAN                 NOT NULL DEFAULT FALSE,

    note        TEXT                    NULL,

    PRIMARY KEY (gid),
    UNIQUE (exchange, uuid)
);

-- +down
DROP TABLE IF EXISTS rewards;