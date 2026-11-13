package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20260528134553_futures_position_risk.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20260528134553, "migrations/sqlite3/20260528134553_futures_position_risk.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `futures_position_risks`\n(\n    `gid`                        INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`                   TEXT     NOT NULL,\n    `symbol`                     TEXT     NOT NULL,\n    `position_side`              TEXT     NOT NULL DEFAULT '',\n    `leverage`                   REAL     NOT NULL DEFAULT 0,\n    `liquidation_price`          REAL     NOT NULL DEFAULT 0,\n    `entry_price`                REAL     NOT NULL DEFAULT 0,\n    `mark_price`                 REAL     NOT NULL DEFAULT 0,\n    `break_even_price`           REAL     NOT NULL DEFAULT 0,\n    `position_amount`            REAL     NOT NULL DEFAULT 0,\n    `unrealized_pnl`             REAL     NOT NULL DEFAULT 0,\n    `notional`                   REAL     NOT NULL DEFAULT 0,\n    `initial_margin`             REAL     NOT NULL DEFAULT 0,\n    `maint_margin`               REAL     NOT NULL DEFAULT 0,\n    `position_initial_margin`    REAL     NOT NULL DEFAULT 0,\n    `open_order_initial_margin`  REAL     NOT NULL DEFAULT 0,\n    `adl`                        REAL     NOT NULL DEFAULT 0,\n    `margin_asset`               TEXT     NOT NULL DEFAULT '',\n    `updated_at`                 DATETIME(3)        NOT NULL\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `futures_position_risks`;"},
		},
	)
}
