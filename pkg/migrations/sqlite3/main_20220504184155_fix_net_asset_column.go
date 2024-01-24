package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixNetAssetColumn, down_main_fixNetAssetColumn)
}

func up_main_fixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `interest` DECIMAL DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}
	return err
}
