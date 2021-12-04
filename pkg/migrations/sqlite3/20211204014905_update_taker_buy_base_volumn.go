package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upUpdateTakerBuyBaseVolumn, downUpdateTakerBuyBaseVolumn)

}

func upUpdateTakerBuyBaseVolumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "")
	if err != nil {
		return err
	}

	return err
}

func downUpdateTakerBuyBaseVolumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "")
	if err != nil {
		return err
	}

	return err
}
