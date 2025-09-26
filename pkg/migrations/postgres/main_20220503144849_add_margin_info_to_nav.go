package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addMarginInfoToNav, down_main_addMarginInfoToNav)
}

func up_main_addMarginInfoToNav(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    ADD COLUMN session      VARCHAR(30)                               NOT NULL,\n    ADD COLUMN is_margin    BOOLEAN                                   NOT NULL DEFAULT FALSE,\n    ADD COLUMN is_isolated  BOOLEAN                                   NOT NULL DEFAULT FALSE,\n    ADD COLUMN isolated_symbol VARCHAR(30)                            NOT NULL DEFAULT '',\n    ADD COLUMN net_asset    NUMERIC(32, 8) DEFAULT 0.00000000        NOT NULL CHECK (net_asset >= 0),\n    ADD COLUMN borrowed     NUMERIC(32, 8) DEFAULT 0.00000000        NOT NULL CHECK (borrowed >= 0),\n    ADD COLUMN price_in_usd NUMERIC(32, 8) DEFAULT 0.00000000        NOT NULL CHECK (price_in_usd >= 0);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addMarginInfoToNav(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE nav_history_details\n    DROP COLUMN IF EXISTS session,\n    DROP COLUMN IF EXISTS net_asset,\n    DROP COLUMN IF EXISTS borrowed,\n    DROP COLUMN IF EXISTS price_in_usd,\n    DROP COLUMN IF EXISTS is_margin,\n    DROP COLUMN IF EXISTS is_isolated,\n    DROP COLUMN IF EXISTS isolated_symbol;")
	if err != nil {
		return err
	}
	return err
}
