package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddMarginColumns, downAddMarginColumns)

}

func upAddMarginColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}

func downAddMarginColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}
