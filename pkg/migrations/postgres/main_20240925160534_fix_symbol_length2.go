package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixSymbolLength2, down_main_fixSymbolLength2)
}

func up_main_fixSymbolLength2(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits ALTER COLUMN symbol TYPE VARCHAR(32);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits ALTER COLUMN base_currency TYPE VARCHAR(16);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE profits ALTER COLUMN fee_currency TYPE VARCHAR(16);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE positions ALTER COLUMN base_currency TYPE VARCHAR(16);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE positions ALTER COLUMN symbol TYPE VARCHAR(32);")
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
