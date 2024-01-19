package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addNavHistoryDetails, down_main_addNavHistoryDetails)

}

func up_main_addNavHistoryDetails(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `nav_history_details`\n(\n    `gid`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,\n    `exchange`         VARCHAR(30)                NOT NULL DEFAULT '',\n    `subaccount`       VARCHAR(30)                NOT NULL DEFAULT '',\n    `time`             DATETIME(3)                NOT NULL DEFAULT (strftime('%s', 'now')),\n    `currency`         VARCHAR(30)                NOT NULL,\n    `net_asset_in_usd` DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `net_asset_in_btc` DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `balance`          DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `available`        DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `locked`           DECIMAL DEFAULT 0.00000000 NOT NULL\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX idx_nav_history_details\n    on nav_history_details (time, currency, exchange);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addNavHistoryDetails(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE nav_history_details;")
	if err != nil {
		return err
	}
	return err
}
