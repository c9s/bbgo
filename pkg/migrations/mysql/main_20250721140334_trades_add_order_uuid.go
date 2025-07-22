package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesAddOrderUuid, down_main_tradesAddOrderUuid)
}

func up_main_tradesAddOrderUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `order_uuid` VARBINARY(16) NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradesAddOrderUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` DROP COLUMN `order_uuid`;")
	if err != nil {
		return err
	}
	return err
}
