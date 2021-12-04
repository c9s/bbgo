package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upIncreaseDecimalLength, downIncreaseDecimalLength)
}

func upIncreaseDecimalLength(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	return err
}

func downIncreaseDecimalLength(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';")
	if err != nil {
		return err
	}

	return err
}
