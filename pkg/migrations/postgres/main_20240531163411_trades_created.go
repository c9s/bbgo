package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesCreated, down_main_tradesCreated)
}

func up_main_tradesCreated(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades ADD COLUMN inserted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE trades SET inserted_at = traded_at;")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradesCreated(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades DROP COLUMN IF EXISTS inserted_at;")
	if err != nil {
		return err
	}
	return err
}
