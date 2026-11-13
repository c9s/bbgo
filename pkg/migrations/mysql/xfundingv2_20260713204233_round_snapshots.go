package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/mysql/20260713204233_round_snapshots.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260713204233, "pkg/strategy/xfundingv2/migrations/mysql/20260713204233_round_snapshots.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `xfundingv2_round_snapshots`\n(\n    `gid`                          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `id`                           VARCHAR(36)     NOT NULL,\n    `strategy_instance_id`         VARCHAR(64)     NOT NULL DEFAULT '',\n    `spot_symbol`                  VARCHAR(32)     NOT NULL,\n    `futures_symbol`               VARCHAR(32)     NOT NULL,\n    `spot_exchange`                VARCHAR(24)     NOT NULL DEFAULT '',\n    `futures_exchange`             VARCHAR(24)     NOT NULL DEFAULT '',\n    `direction`                    VARCHAR(16)     NOT NULL DEFAULT '',\n    `collateral_asset`             VARCHAR(20)     NOT NULL DEFAULT '',\n    `leverage`                     DECIMAL(16, 8)  NOT NULL DEFAULT 0,\n    `triggered_funding_rate`       DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `annualized_rate`              DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `funding_income`               DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `spot_position`                DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `futures_position`             DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `state`                        VARCHAR(16)     NOT NULL DEFAULT '',\n    `spot_price`                   DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `futures_price`                DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `spot_average_cost`            DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `futures_average_cost`         DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `spot_pnl`                     DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `spot_net_pnl`                 DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `futures_pnl`                  DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `futures_net_pnl`              DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `net_pnl`                      DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `unrealized_spot_pnl`          DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `unrealized_futures_pnl`       DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `total_spot_net_pnl`      DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `total_futures_net_pnl`   DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `total_net_pnl`           DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `inserted_at`             DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,\n    PRIMARY KEY (`gid`),\n    KEY `idx_xfundingv2_round_snapshots_instance` (`strategy_instance_id`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `xfundingv2_round_snapshots`;"},
		},
	)
}
