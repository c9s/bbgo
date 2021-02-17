package mysql

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

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `strategy` VARCHAR(32) NULL;")
	if err != nil {
		return err
	}

	return err
}

func downAddPnlColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` DROP COLUMN `pnl`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` DROP COLUMN `strategy`;")
	if err != nil {
		return err
	}

	return err
}
