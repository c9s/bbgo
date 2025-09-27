-- !txn
-- +up
CREATE TABLE orders
(
    gid               BIGSERIAL               NOT NULL,

    exchange          VARCHAR(24)             NOT NULL DEFAULT '',
    -- order_id is the order id returned from the exchange
    order_id          BIGINT                  NOT NULL,
    client_order_id   VARCHAR(122)            NOT NULL DEFAULT '',
    order_type        VARCHAR(16)             NOT NULL,
    symbol            VARCHAR(32)             NOT NULL,
    status            VARCHAR(12)             NOT NULL,
    time_in_force     VARCHAR(4)              NOT NULL,
    price             NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),
    stop_price        NUMERIC(16, 8)          NOT NULL CHECK (stop_price >= 0),
    quantity          NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),
    executed_quantity NUMERIC(16, 8)          NOT NULL DEFAULT 0.0 CHECK (executed_quantity >= 0),
    side              VARCHAR(4)              NOT NULL DEFAULT '',
    is_working        BOOLEAN                 NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMP(3)            NOT NULL,
    updated_at        TIMESTAMP(3)            NOT NULL DEFAULT CURRENT_TIMESTAMP,

    is_margin         BOOLEAN                 NOT NULL DEFAULT FALSE,
    is_isolated       BOOLEAN                 NOT NULL DEFAULT FALSE,

    PRIMARY KEY (gid)
);
CREATE INDEX orders_symbol ON orders (exchange, symbol);
CREATE UNIQUE INDEX orders_order_id ON orders (order_id, exchange);

-- +down
DROP INDEX IF EXISTS orders_symbol;
DROP INDEX IF EXISTS orders_order_id;
DROP TABLE IF EXISTS orders;