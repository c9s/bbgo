package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upFixNetAssetColumn, downFixNetAssetColumn)

}

func upFixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `interest` DECIMAL DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downFixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}
