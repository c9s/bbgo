package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_ordersAddUuid, down_main_ordersAddUuid)
}

func up_main_ordersAddUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades ADD COLUMN order_uuid TEXT NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	return err
}

func down_main_ordersAddUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades DROP COLUMN order_uuid;")
	if err != nil {
		return err
	}
	return err
}
