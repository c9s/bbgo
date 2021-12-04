package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upIncreaseSymbolLength, downIncreaseSymbolLength)
}

func upIncreaseSymbolLength(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downIncreaseSymbolLength(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}
