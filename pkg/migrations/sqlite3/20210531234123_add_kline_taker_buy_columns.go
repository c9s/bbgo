package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddKlineTakerBuyColumns, downAddKlineTakerBuyColumns)

}

func upAddKlineTakerBuyColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "SELECT 'up SQL query';")
	if err != nil {
		return err
	}

	return err
}

func downAddKlineTakerBuyColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 'down SQL query';")
	if err != nil {
		return err
	}

	return err
}
