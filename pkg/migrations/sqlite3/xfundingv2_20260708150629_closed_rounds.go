package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/sqlite3/20260708150629_closed_rounds.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260708150629, "pkg/strategy/xfundingv2/migrations/sqlite3/20260708150629_closed_rounds.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `xfundingv2_closed_rounds`\n(\n    `gid`                     INTEGER PRIMARY KEY AUTOINCREMENT,\n    `id`                      TEXT       NOT NULL DEFAULT '',\n    `strategy_instance_id`    TEXT       NOT NULL,\n    `spot_symbol`             TEXT       NOT NULL,\n    `futures_symbol`          TEXT       NOT NULL,\n    `spot_exchange`           TEXT       NOT NULL DEFAULT '',\n    `futures_exchange`        TEXT       NOT NULL DEFAULT '',\n    `direction`               TEXT       NOT NULL DEFAULT '',\n    `collateral_asset`        TEXT       NOT NULL DEFAULT '',\n    `leverage`                INTEGER    NOT NULL DEFAULT 0,\n    `triggered_funding_rate`  REAL       NOT NULL DEFAULT 0,\n    `annualized_rate`         REAL       NOT NULL DEFAULT 0,\n    `funding_income`          REAL       NOT NULL DEFAULT 0,\n    `spot_pnl`                REAL       NOT NULL DEFAULT 0,\n    `spot_net_pnl`            REAL       NOT NULL DEFAULT 0,\n    `futures_pnl`             REAL       NOT NULL DEFAULT 0,\n    `futures_net_pnl`         REAL       NOT NULL DEFAULT 0,\n    `net_pnl`                 REAL       NOT NULL DEFAULT 0,\n    `num_holding_intervals`   INTEGER    NOT NULL DEFAULT 0,\n    `start_at`              DATETIME(3)   NOT NULL,\n    `ready_at`              DATETIME(3)   NULL,\n    `closing_at`            DATETIME(3)   NOT NULL,\n    `closed_at`             DATETIME(3)   NOT NULL,\n    `inserted_at`           DATETIME(3)   DEFAULT CURRENT_TIMESTAMP NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_xfundingv2_closed_rounds_instance` ON `xfundingv2_closed_rounds` (`strategy_instance_id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_xfundingv2_closed_rounds_round_id` ON `xfundingv2_closed_rounds` (`id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `xfundingv2_funding_fees`\n(\n    `gid`        INTEGER PRIMARY KEY AUTOINCREMENT,\n    `round_id`   TEXT       NOT NULL DEFAULT '',\n    `asset`      TEXT       NOT NULL DEFAULT '',\n    `amount`     REAL       NOT NULL DEFAULT 0,\n    `txn`        INTEGER    NOT NULL DEFAULT 0,\n    `time`       DATETIME   NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_xfundingv2_funding_fees_round_id` ON `xfundingv2_funding_fees` (`round_id`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `xfundingv2_funding_fees`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `xfundingv2_closed_rounds`;"},
		},
	)
}
