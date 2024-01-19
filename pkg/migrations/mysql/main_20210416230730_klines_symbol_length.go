package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_klinesSymbolLength, down_main_klinesSymbolLength)

}

func up_main_klinesSymbolLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
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

func down_main_klinesSymbolLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
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
