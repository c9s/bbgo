-- +up
CREATE TABLE margin_liquidations
(
    gid               BIGSERIAL               NOT NULL,

    exchange          VARCHAR(24)             NOT NULL DEFAULT '',

    symbol            VARCHAR(24)             NOT NULL DEFAULT '',

    order_id          BIGINT                  NOT NULL,

    is_isolated       BOOLEAN                 NOT NULL DEFAULT false,

    average_price     NUMERIC(16, 8)          NOT NULL CHECK (average_price >= 0),

    price             NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),

    quantity          NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),

    executed_quantity NUMERIC(16, 8)          NOT NULL CHECK (executed_quantity >= 0),

    side              VARCHAR(5)              NOT NULL DEFAULT '',

    time_in_force     VARCHAR(5)              NOT NULL DEFAULT '',

    time              TIMESTAMP(3)            NOT NULL,

    PRIMARY KEY (gid),
    UNIQUE (order_id, exchange)
);

-- +down
DROP TABLE IF EXISTS margin_liquidations;