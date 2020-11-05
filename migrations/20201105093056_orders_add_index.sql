-- +goose Up
CREATE INDEX orders_symbol ON orders (exchange, symbol);
CREATE INDEX orders_id_symbol ON orders(exchange, order_id, symbol);

-- +goose Down
DROP INDEX orders_symbol ON orders;
DROP INDEX orders_id_symbol ON orders;
