package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upTradesAddOrderId, downTradesAddOrderId)
}

func upTradesAddOrderId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades`\n    ADD COLUMN `order_id` BIGINT UNSIGNED NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downTradesAddOrderId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades`\n    DROP COLUMN `order_id`;")
	if err != nil {
		return err
	}

	return err
}
