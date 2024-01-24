package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_marginInterests, down_main_marginInterests)
}

func up_main_marginInterests(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `margin_interests`\n(\n    `gid`             INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`        VARCHAR(24)     NOT NULL DEFAULT '',\n    `asset`           VARCHAR(24)     NOT NULL DEFAULT '',\n    `isolated_symbol` VARCHAR(24)     NOT NULL DEFAULT '',\n    `principle`       DECIMAL(16, 8)  NOT NULL,\n    `interest`        DECIMAL(20, 16) NOT NULL,\n    `interest_rate`   DECIMAL(20, 16) NOT NULL,\n    `time`            DATETIME(3)     NOT NULL\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_marginInterests(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `margin_interests`;")
	if err != nil {
		return err
	}
	return err
}
