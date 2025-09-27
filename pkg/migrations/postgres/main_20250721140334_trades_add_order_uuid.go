package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesAddOrderUuid, down_main_tradesAddOrderUuid)
}

func up_main_tradesAddOrderUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades ADD COLUMN order_uuid VARCHAR(36);\nUPDATE trades SET order_uuid = '' WHERE order_uuid IS NULL;\nALTER TABLE trades ALTER COLUMN order_uuid SET DEFAULT '';\nALTER TABLE trades ALTER COLUMN order_uuid SET NOT NULL;")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradesAddOrderUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades DROP COLUMN order_uuid;")
	if err != nil {
		return err
	}
	return err
}
