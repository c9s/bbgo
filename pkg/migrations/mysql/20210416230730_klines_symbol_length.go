package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upKlinesSymbolLength, downKlinesSymbolLength)

}

func upKlinesSymbolLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

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

func downKlinesSymbolLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `klines`\nMODIFY COLUMN `symbol` VARCHAR(7) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\nMODIFY COLUMN `symbol` VARCHAR(7) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\nMODIFY COLUMN `symbol` VARCHAR(7) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\nMODIFY COLUMN `symbol` VARCHAR(7) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}
