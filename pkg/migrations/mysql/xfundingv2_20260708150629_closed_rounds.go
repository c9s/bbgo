package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/mysql/20260708150629_closed_rounds.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260708150629, "pkg/strategy/xfundingv2/migrations/mysql/20260708150629_closed_rounds.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `xfundingv2_closed_rounds`\n(\n    `gid`                     BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,\n    `id`                      VARCHAR(36)        NOT NULL DEFAULT '',\n    `strategy_instance_id`    VARCHAR(64)        NOT NULL,\n    `spot_symbol`             VARCHAR(32)        NOT NULL,\n    `futures_symbol`          VARCHAR(32)        NOT NULL,\n    `spot_exchange`           VARCHAR(24)        NOT NULL DEFAULT '',\n    `futures_exchange`        VARCHAR(24)        NOT NULL DEFAULT '',\n    `direction`               VARCHAR(10)        NOT NULL DEFAULT '',\n    `collateral_asset`        VARCHAR(20)        NOT NULL DEFAULT '',\n    `leverage`                INT                NOT NULL DEFAULT 0,\n    `triggered_funding_rate`  FLOAT(8)           NOT NULL DEFAULT 0,\n    `annualized_rate`         FLOAT(8)           NOT NULL DEFAULT 0,\n    `funding_income`          DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `spot_pnl`                DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `spot_net_pnl`            DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `futures_pnl`             DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `futures_net_pnl`         DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `net_pnl`                 DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `num_holding_intervals`   INT                NOT NULL DEFAULT 0,\n    `start_at`              DATETIME(3)        NOT NULL,\n    -- a round is closed before it ever became ready, `ready_at` can be NULL\n    `ready_at`              DATETIME(3)        NULL,\n    `closing_at`            DATETIME(3)        NOT NULL,\n    `closed_at`             DATETIME(3)        NOT NULL,\n    `inserted_at`           DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,\n    PRIMARY KEY (`gid`),\n    KEY `idx_xfundingv2_closed_rounds_instance` (`strategy_instance_id`),\n    KEY `idx_xfundingv2_closed_rounds_round_id` (`id`)\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `xfundingv2_funding_fees`\n(\n    `gid`        BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,\n    `round_id`   VARCHAR(36)        NOT NULL DEFAULT '',\n    `asset`      VARCHAR(20)        NOT NULL DEFAULT '',\n    `amount`     DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `txn`        BIGINT             NOT NULL DEFAULT 0,\n    `time`       DATETIME(3)        NOT NULL,\n    `inserted_at` DATETIME(3)       DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,\n    PRIMARY KEY (`gid`),\n    KEY `idx_xfundingv2_funding_fees_round_id` (`round_id`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `xfundingv2_funding_fees`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `xfundingv2_closed_rounds`;"},
		},
	)
}
