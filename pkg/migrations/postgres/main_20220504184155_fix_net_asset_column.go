package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixNetAssetColumn, down_main_fixNetAssetColumn)
}

func up_main_fixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    ALTER COLUMN net_asset TYPE NUMERIC(32, 8),\n    ALTER COLUMN net_asset SET DEFAULT 0.00000000,\n    ALTER COLUMN net_asset SET NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    RENAME COLUMN balance_in_usd TO net_asset_in_usd;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    ALTER COLUMN net_asset_in_usd TYPE NUMERIC(32, 2),\n    ALTER COLUMN net_asset_in_usd SET DEFAULT 0.00000000,\n    ALTER COLUMN net_asset_in_usd SET NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    RENAME COLUMN balance_in_btc TO net_asset_in_btc;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    ALTER COLUMN net_asset_in_btc TYPE NUMERIC(32, 20),\n    ALTER COLUMN net_asset_in_btc SET DEFAULT 0.00000000,\n    ALTER COLUMN net_asset_in_btc SET NOT NULL;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    ADD COLUMN interest NUMERIC(32, 20) DEFAULT 0.00000000 NOT NULL CHECK (interest >= 0);")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixNetAssetColumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    DROP COLUMN IF EXISTS interest;")
	if err != nil {
		return err
	}
	return err
}
