package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_ordersAddUuid, down_main_ordersAddUuid)
}

func up_main_ordersAddUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE orders ADD COLUMN uuid VARCHAR(36) NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	return err
}

func down_main_ordersAddUuid(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE orders DROP COLUMN uuid;")
	if err != nil {
		return err
	}
	return err
}
