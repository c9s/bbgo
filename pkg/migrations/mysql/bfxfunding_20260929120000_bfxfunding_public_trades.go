package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/bfxfunding/migrations/mysql/20260929120000_bfxfunding_public_trades.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("bfxfunding", 20260929120000, "pkg/strategy/bfxfunding/migrations/mysql/20260929120000_bfxfunding_public_trades.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `bfxfunding_public_trades`\n(\n    `gid`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `symbol`      VARCHAR(16)     NOT NULL DEFAULT '',\n    `trade_id`    BIGINT          NOT NULL,\n    `amount`      DECIMAL(24, 8)  NOT NULL DEFAULT 0,\n    `rate`        DECIMAL(20, 12) NOT NULL DEFAULT 0,\n    `period`      INT             NOT NULL DEFAULT 0,\n    `time`        DATETIME(3)     NOT NULL,\n    `inserted_at` DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `uidx_bfxfunding_public_trades_symbol_trade_id` (`symbol`, `trade_id`),\n    KEY `idx_bfxfunding_public_trades_symbol_time` (`symbol`, `time`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `bfxfunding_public_trades`;"},
		},
	)
}
