package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradeIndex, down_main_tradeIndex)
}

func up_main_tradeIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradeIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}
	return err
}
