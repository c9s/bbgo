package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addWithdrawsTable, down_main_addWithdrawsTable)
}

func up_main_addWithdrawsTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `withdraws`\n(\n    `gid`              INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`         VARCHAR(24)    NOT NULL DEFAULT '',\n    -- asset is the asset name (currency)\n    `asset`            VARCHAR(10)    NOT NULL,\n    `address`          VARCHAR(128)    NOT NULL,\n    `network`          VARCHAR(32)    NOT NULL DEFAULT '',\n    `amount`           DECIMAL(16, 8) NOT NULL,\n    `txn_id`           VARCHAR(256)    NOT NULL,\n    `txn_fee`          DECIMAL(16, 8) NOT NULL DEFAULT 0,\n    `txn_fee_currency` VARCHAR(32)    NOT NULL DEFAULT '',\n    `time`             DATETIME(3)    NOT NULL\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `withdraws_txn_id` ON `withdraws` (`exchange`, `txn_id`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addWithdrawsTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS `withdraws_txn_id`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `withdraws`;")
	if err != nil {
		return err
	}
	return err
}
