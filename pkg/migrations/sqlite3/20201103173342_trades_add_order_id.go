package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upTradesAddOrderId, downTradesAddOrderId)
}

func upTradesAddOrderId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `order_id` INTEGER;")
	if err != nil {
		return err
	}

	return err
}

func downTradesAddOrderId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` RENAME COLUMN `order_id` TO `order_id_deleted`;")
	if err != nil {
		return err
	}

	return err
}
