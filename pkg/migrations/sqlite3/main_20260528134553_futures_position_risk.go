package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_futuresPositionRisk, down_main_futuresPositionRisk)
}

func up_main_futuresPositionRisk(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `futures_position_risks`\n(\n    `gid`                        INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`                   TEXT     NOT NULL,\n    `symbol`                     TEXT     NOT NULL,\n    `position_side`              TEXT     NOT NULL DEFAULT '',\n    `leverage`                   REAL     NOT NULL DEFAULT 0,\n    `liquidation_price`          REAL     NOT NULL DEFAULT 0,\n    `entry_price`                REAL     NOT NULL DEFAULT 0,\n    `mark_price`                 REAL     NOT NULL DEFAULT 0,\n    `break_even_price`           REAL     NOT NULL DEFAULT 0,\n    `position_amount`            REAL     NOT NULL DEFAULT 0,\n    `unrealized_pnl`             REAL     NOT NULL DEFAULT 0,\n    `notional`                   REAL     NOT NULL DEFAULT 0,\n    `initial_margin`             REAL     NOT NULL DEFAULT 0,\n    `maint_margin`               REAL     NOT NULL DEFAULT 0,\n    `position_initial_margin`    REAL     NOT NULL DEFAULT 0,\n    `open_order_initial_margin`  REAL     NOT NULL DEFAULT 0,\n    `adl`                        REAL     NOT NULL DEFAULT 0,\n    `margin_asset`               TEXT     NOT NULL DEFAULT '',\n    `updated_at`                 DATETIME(3)        NOT NULL\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_futuresPositionRisk(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `futures_position_risks`;")
	if err != nil {
		return err
	}
	return err
}
