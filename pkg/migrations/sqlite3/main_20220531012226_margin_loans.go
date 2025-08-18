package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_marginLoans, down_main_marginLoans)
}

func up_main_marginLoans(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `margin_loans`\n(\n    `gid`             INTEGER PRIMARY KEY AUTOINCREMENT,\n    `transaction_id`  INTEGER        NOT NULL,\n    `exchange`        VARCHAR(24)    NOT NULL DEFAULT '',\n    `asset`           VARCHAR(24)    NOT NULL DEFAULT '',\n    `isolated_symbol` VARCHAR(24)    NOT NULL DEFAULT '',\n    -- quantity is the quantity of the trade that makes profit\n    `principle`       DECIMAL(16, 8) NOT NULL,\n    `time`            DATETIME(3)    NOT NULL\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_marginLoans(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `margin_loans`;")
	if err != nil {
		return err
	}
	return err
}
