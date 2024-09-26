package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixSymbolLength2, down_main_fixSymbolLength2)
}

func up_main_fixSymbolLength2(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits MODIFY COLUMN symbol VARCHAR(32) NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits MODIFY COLUMN base_currency VARCHAR(16) NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits MODIFY COLUMN fee_currency VARCHAR(16) NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE positions MODIFY COLUMN base_currency VARCHAR(16) NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE positions MODIFY COLUMN symbol VARCHAR(32) NOT NULL;")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixSymbolLength2(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}
	return err
}
