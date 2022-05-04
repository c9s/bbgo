package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddMarginInfoToNav, downAddMarginInfoToNav)

}

func upAddMarginInfoToNav(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details`\n    ADD COLUMN `session`      VARCHAR(30)                                NOT NULL,\n    ADD COLUMN `is_margin`    BOOLEAN                                    NOT NULL DEFAULT FALSE,\n    ADD COLUMN `is_isolated`  BOOLEAN                                    NOT NULL DEFAULT FALSE,\n    ADD COLUMN `isolated_symbol`  VARCHAR(30)                            NOT NULL DEFAULT '',\n    ADD COLUMN `net_asset`    DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    ADD COLUMN `borrowed`     DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    ADD COLUMN `price_in_usd` DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL\n;")
	if err != nil {
		return err
	}

	return err
}

func downAddMarginInfoToNav(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `nav_history_details`\n    DROP COLUMN `session`,\n    DROP COLUMN `net_asset`,\n    DROP COLUMN `borrowed`,\n    DROP COLUMN `price_in_usd`,\n    DROP COLUMN `is_margin`,\n    DROP COLUMN `is_isolated`,\n    DROP COLUMN `isolated_symbol`\n;")
	if err != nil {
		return err
	}

	return err
}
