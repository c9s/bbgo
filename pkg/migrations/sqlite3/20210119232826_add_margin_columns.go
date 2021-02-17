package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddMarginColumns, downAddMarginColumns)

}

func upAddMarginColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders` ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders` ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE;")
	if err != nil {
		return err
	}

	return err
}

func downAddMarginColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` RENAME COLUMN `is_margin` TO `is_margin_deleted`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` RENAME COLUMN `is_isolated` TO `is_isolated_deleted`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders` RENAME COLUMN `is_margin` TO `is_margin_deleted`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders` RENAME COLUMN `is_isolated` TO `is_isolated_deleted`;")
	if err != nil {
		return err
	}

	return err
}
