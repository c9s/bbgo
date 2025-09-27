package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_orders, down_main_orders)
}

func up_main_orders(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE orders\n(\n    gid               BIGSERIAL               NOT NULL,\n    exchange          VARCHAR(24)             NOT NULL DEFAULT '',\n    -- order_id is the order id returned from the exchange\n    order_id          BIGINT                  NOT NULL,\n    client_order_id   VARCHAR(122)            NOT NULL DEFAULT '',\n    order_type        VARCHAR(16)             NOT NULL,\n    symbol            VARCHAR(32)             NOT NULL,\n    status            VARCHAR(12)             NOT NULL,\n    time_in_force     VARCHAR(4)              NOT NULL,\n    price             NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),\n    stop_price        NUMERIC(16, 8)          NOT NULL CHECK (stop_price >= 0),\n    quantity          NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),\n    executed_quantity NUMERIC(16, 8)          NOT NULL DEFAULT 0.0 CHECK (executed_quantity >= 0),\n    side              VARCHAR(4)              NOT NULL DEFAULT '',\n    is_working        BOOLEAN                 NOT NULL DEFAULT FALSE,\n    created_at        TIMESTAMP(3)            NOT NULL,\n    updated_at        TIMESTAMP(3)            NOT NULL DEFAULT CURRENT_TIMESTAMP,\n    is_margin         BOOLEAN                 NOT NULL DEFAULT FALSE,\n    is_isolated       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    PRIMARY KEY (gid)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX orders_symbol ON orders (exchange, symbol);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX orders_order_id ON orders (order_id, exchange);")
	if err != nil {
		return err
	}
	return err
}

func down_main_orders(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS orders_symbol;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS orders_order_id;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS orders;")
	if err != nil {
		return err
	}
	return err
}
