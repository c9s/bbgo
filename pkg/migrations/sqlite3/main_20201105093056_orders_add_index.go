package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_ordersAddIndex, down_main_ordersAddIndex)
}

func up_main_ordersAddIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
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

func down_main_ordersAddIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS orders_symbol;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS orders_order_id;")
	if err != nil {
		return err
	}
	return err
}
