package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upTradePriceQuantityIndex, downTradePriceQuantityIndex)
}

func upTradePriceQuantityIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_price_quantity ON trades (order_id,price,quantity);")
	if err != nil {
		return err
	}

	return err
}

func downTradePriceQuantityIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP INDEX trades_price_quantity ON trades")
	if err != nil {
		return err
	}

	return err
}
