package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addNetProfitColumn, down_main_addNetProfitColumn)
}

func up_main_addNetProfitColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE positions\n    ADD COLUMN net_profit NUMERIC(16, 8) DEFAULT 0.00000000 NOT NULL\n;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE positions SET net_profit = profit WHERE net_profit = 0.0;")
	if err != nil {
		return err
	}
	return err
}

func down_main_addNetProfitColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE positions\nDROP COLUMN net_profit\n;")
	if err != nil {
		return err
	}
	return err
}
