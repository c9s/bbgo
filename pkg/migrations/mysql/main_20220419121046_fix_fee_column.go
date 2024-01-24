package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixFeeColumn, down_main_fixFeeColumn)
}

func up_main_fixFeeColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades\n    CHANGE fee fee DECIMAL(16, 8) NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits\n    CHANGE fee fee DECIMAL(16, 8) NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits\n    CHANGE fee_in_usd fee_in_usd DECIMAL(16, 8);")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixFeeColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}
	return err
}
