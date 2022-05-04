package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddMarginInfoToNav, downAddMarginInfoToNav)

}

func upAddMarginInfoToNav(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `session` VARCHAR(50) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `borrowed` DECIMAL DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `net_asset` DECIMAL DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `price_in_usd` DECIMAL DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `is_margin` BOOL DEFAULT FALSE NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `is_isolated` BOOL DEFAULT FALSE NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details` ADD COLUMN `isolated_symbol` VARCHAR(30) DEFAULT '' NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downAddMarginInfoToNav(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}
