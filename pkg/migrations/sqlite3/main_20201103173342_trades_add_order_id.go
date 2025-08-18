package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesAddOrderId, down_main_tradesAddOrderId)
}

func up_main_tradesAddOrderId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `order_id` INTEGER;")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradesAddOrderId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` RENAME COLUMN `order_id` TO `order_id_deleted`;")
	if err != nil {
		return err
	}
	return err
}
