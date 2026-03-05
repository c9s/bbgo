package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixTradesOrderUuid, down_main_fixTradesOrderUuid)
}

func up_main_fixTradesOrderUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` MODIFY COLUMN `order_uuid` VARBINARY(36) NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixTradesOrderUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` MODIFY COLUMN `order_uuid` VARBINARY(16) NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	return err
}
