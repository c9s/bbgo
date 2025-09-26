package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addNavHistoryDetails, down_main_addNavHistoryDetails)
}

func up_main_addNavHistoryDetails(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE nav_history_details\n(\n    gid            BIGSERIAL                           NOT NULL PRIMARY KEY,\n    exchange       VARCHAR(30)                         NOT NULL,\n    subaccount     VARCHAR(30)                         NOT NULL,\n    time           TIMESTAMP(3)                        NOT NULL,\n    currency       VARCHAR(12)                         NOT NULL,\n    balance_in_usd NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (balance_in_usd >= 0),\n    balance_in_btc NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (balance_in_btc >= 0),\n    balance        NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (balance >= 0),\n    available      NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (available >= 0),\n    locked         NUMERIC(32, 8)      DEFAULT 0.00000000 NOT NULL CHECK (locked >= 0)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX idx_nav_history_details\n    ON nav_history_details (time, currency, exchange);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addNavHistoryDetails(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS nav_history_details;")
	if err != nil {
		return err
	}
	return err
}
