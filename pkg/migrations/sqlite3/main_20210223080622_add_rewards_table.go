package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addRewardsTable, down_main_addRewardsTable)
}

func up_main_addRewardsTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `rewards`\n(\n    `gid`         INTEGER PRIMARY KEY AUTOINCREMENT,\n    -- for exchange\n    `exchange`    VARCHAR(24)    NOT NULL DEFAULT '',\n    -- reward record id\n    `uuid`        VARCHAR(32)    NOT NULL,\n    `reward_type` VARCHAR(24)    NOT NULL DEFAULT '',\n    -- currency symbol, BTC, MAX, USDT ... etc\n    `currency`    VARCHAR(5)     NOT NULL,\n    -- the quantity of the rewards\n    `quantity`    DECIMAL(16, 8) NOT NULL,\n    `state`       VARCHAR(5)     NOT NULL,\n    `created_at`  DATETIME       NOT NULL,\n    `spent`       BOOLEAN        NOT NULL DEFAULT FALSE,\n    `note`        TEXT           NULL\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addRewardsTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `rewards`;")
	if err != nil {
		return err
	}
	return err
}
