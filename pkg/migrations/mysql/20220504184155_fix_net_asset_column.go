package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upFixNetAssetColumn, downFixNetAssetColumn)

}

func upFixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details`\n    MODIFY COLUMN `net_asset` DECIMAL(32, 8) DEFAULT 0.00000000 NOT NULL,\n    CHANGE COLUMN `balance_in_usd` `net_asset_in_usd` DECIMAL(32, 2) DEFAULT 0.00000000 NOT NULL,\n    CHANGE COLUMN `balance_in_btc` `net_asset_in_btc` DECIMAL(32, 20) DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details`\n    ADD COLUMN `interest` DECIMAL(32, 20) UNSIGNED DEFAULT 0.00000000 NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downFixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details`\n    DROP COLUMN `interest`;")
	if err != nil {
		return err
	}

	return err
}
