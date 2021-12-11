package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upUpdateFeeCurrencyLength, downUpdateFeeCurrencyLength)

}

func upUpdateFeeCurrencyLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE trades CHANGE fee_currency fee_currency varchar(10) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downUpdateFeeCurrencyLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE trades CHANGE fee_currency fee_currency varchar(4) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}
