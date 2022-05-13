package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upFixProfitSymbolLength, downFixProfitSymbolLength)

}

func upFixProfitSymbolLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}

func downFixProfitSymbolLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}
