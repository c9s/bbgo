package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addPositionIndex, down_main_addPositionIndex)
}

func up_main_addPositionIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE INDEX positions_traded_at ON positions (traded_at, profit);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addPositionIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS positions_traded_at;")
	if err != nil {
		return err
	}
	return err
}
