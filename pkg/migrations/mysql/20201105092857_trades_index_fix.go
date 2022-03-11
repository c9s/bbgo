package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upTradesIndexFix, downTradesIndexFix)

}

func upTradesIndexFix(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}

func downTradesIndexFix(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}
