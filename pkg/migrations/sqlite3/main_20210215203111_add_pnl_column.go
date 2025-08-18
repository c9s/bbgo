package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addPnlColumn, down_main_addPnlColumn)
}

func up_main_addPnlColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `pnl` DECIMAL NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `strategy` TEXT;")
	if err != nil {
		return err
	}
	return err
}

func down_main_addPnlColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` RENAME COLUMN `pnl` TO `pnl_deleted`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` RENAME COLUMN `strategy` TO `strategy_deleted`;")
	if err != nil {
		return err
	}
	return err
}
