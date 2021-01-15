-- +up
CREATE INDEX orders_symbol ON orders (exchange, symbol);
CREATE UNIQUE INDEX orders_order_id ON orders (order_id, exchange);

-- +down
DROP INDEX orders_symbol ON orders;
DROP INDEX orders_order_id ON orders;
