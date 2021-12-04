package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upFixSymbolLength, downFixSymbolLength)
}

func upFixSymbolLength(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE trades MODIFY COLUMN symbol VARCHAR(9);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE orders MODIFY COLUMN symbol VARCHAR(9);")
	if err != nil {
		return err
	}

	return err
}

func downFixSymbolLength(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE trades MODIFY COLUMN symbol VARCHAR(8);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE orders MODIFY COLUMN symbol VARCHAR(8);")
	if err != nil {
		return err
	}

	return err
}
