package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradePriceQuantityIndex, down_main_tradePriceQuantityIndex)

}

func up_main_tradePriceQuantityIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_price_quantity ON trades (order_id,price,quantity);")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradePriceQuantityIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX trades_price_quantity;")
	if err != nil {
		return err
	}
	return err
}
