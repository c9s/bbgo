package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/sqlite3/20260713204233_round_snapshots.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260713204233, "pkg/strategy/xfundingv2/migrations/sqlite3/20260713204233_round_snapshots.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `xfundingv2_round_snapshots`\n(\n    `gid`                          INTEGER PRIMARY KEY AUTOINCREMENT,\n    `id`                           TEXT     NOT NULL,\n    `strategy_instance_id`         TEXT     NOT NULL DEFAULT '',\n    `spot_symbol`                  TEXT     NOT NULL,\n    `futures_symbol`               TEXT     NOT NULL,\n    `spot_exchange`                TEXT     NOT NULL DEFAULT '',\n    `futures_exchange`             TEXT     NOT NULL DEFAULT '',\n    `direction`                    TEXT     NOT NULL DEFAULT '',\n    `collateral_asset`             TEXT     NOT NULL DEFAULT '',\n    `leverage`                     REAL     NOT NULL DEFAULT 0,\n    `triggered_funding_rate`       REAL     NOT NULL DEFAULT 0,\n    `annualized_rate`              REAL     NOT NULL DEFAULT 0,\n    `funding_income`               REAL     NOT NULL DEFAULT 0,\n    `spot_position`                REAL     NOT NULL DEFAULT 0,\n    `futures_position`             REAL     NOT NULL DEFAULT 0,\n    `state`                        TEXT     NOT NULL DEFAULT '',\n    `spot_price`                   REAL     NOT NULL DEFAULT 0,\n    `futures_price`                REAL     NOT NULL DEFAULT 0,\n    `spot_average_cost`            REAL     NOT NULL DEFAULT 0,\n    `futures_average_cost`         REAL     NOT NULL DEFAULT 0,\n    `spot_pnl`                     REAL     NOT NULL DEFAULT 0,\n    `spot_net_pnl`                 REAL     NOT NULL DEFAULT 0,\n    `futures_pnl`                  REAL     NOT NULL DEFAULT 0,\n    `futures_net_pnl`              REAL     NOT NULL DEFAULT 0,\n    `net_pnl`                      REAL     NOT NULL DEFAULT 0,\n    `unrealized_spot_pnl`          REAL     NOT NULL DEFAULT 0,\n    `unrealized_futures_pnl`       REAL     NOT NULL DEFAULT 0,\n    `total_spot_net_pnl`      REAL     NOT NULL DEFAULT 0,\n    `total_futures_net_pnl`   REAL     NOT NULL DEFAULT 0,\n    `total_net_pnl`           REAL     NOT NULL DEFAULT 0,\n    `inserted_at`             DATETIME(3)   DEFAULT CURRENT_TIMESTAMP NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_xfundingv2_round_snapshots_instance` ON `xfundingv2_round_snapshots` (`strategy_instance_id`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `xfundingv2_round_snapshots`;"},
		},
	)
}
