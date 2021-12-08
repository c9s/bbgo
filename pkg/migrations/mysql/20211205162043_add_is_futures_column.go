package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddIsFuturesColumn, downAddIsFuturesColumn)

}

func upAddIsFuturesColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `is_futures` BOOLEAN NOT NULL DEFAULT FALSE;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders` ADD COLUMN `is_futures` BOOLEAN NOT NULL DEFAULT FALSE;")
	if err != nil {
		return err
	}

	return err
}

func downAddIsFuturesColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` DROP COLUMN `is_futures`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders` DROP COLUMN `is_futures`;")
	if err != nil {
		return err
	}

	return err
}
