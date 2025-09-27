package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_increaseDecimalLength, down_main_increaseDecimalLength)
}

func up_main_increaseDecimalLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE klines\nALTER COLUMN volume TYPE NUMERIC(20,8);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE okex_klines\nALTER COLUMN volume TYPE NUMERIC(20,8);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE binance_klines\nALTER COLUMN volume TYPE NUMERIC(20,8);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE max_klines\nALTER COLUMN volume TYPE NUMERIC(20,8);")
	if err != nil {
		return err
	}
	return err
}

func down_main_increaseDecimalLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE klines\nALTER COLUMN volume TYPE NUMERIC(16,8);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE okex_klines\nALTER COLUMN volume TYPE NUMERIC(16,8);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE binance_klines\nALTER COLUMN volume TYPE NUMERIC(16,8);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE max_klines\nALTER COLUMN volume TYPE NUMERIC(16,8);")
	if err != nil {
		return err
	}
	return err
}
