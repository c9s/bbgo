package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddPnlColumn, downAddPnlColumn)

}

func upAddPnlColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
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

func downAddPnlColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
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
