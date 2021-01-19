package migrations

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upAddMarginColumns, downAddMarginColumns)
}

func upAddMarginColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades`\n    ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE,\n    ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE\n    ;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders`\n    ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE,\n    ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE\n    ;")
	if err != nil {
		return err
	}

	return err
}

func downAddMarginColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades`\n    DROP COLUMN `is_margin`,\n    DROP COLUMN `is_isolated`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders`\n    DROP COLUMN `is_margin`,\n    DROP COLUMN `is_isolated`;")
	if err != nil {
		return err
	}

	return err
}
