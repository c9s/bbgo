package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addNavHistoryDetails, down_main_addNavHistoryDetails)

}

func up_main_addNavHistoryDetails(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE nav_history_details\n(\n    gid            bigint unsigned auto_increment PRIMARY KEY,\n    exchange       VARCHAR(30)                                NOT NULL,\n    subaccount     VARCHAR(30)                                NOT NULL,\n    time           DATETIME(3)                                NOT NULL,\n    currency       VARCHAR(12)                                NOT NULL,\n    balance_in_usd DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    balance_in_btc DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    balance        DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    available      DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    locked         DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL\n);")
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
