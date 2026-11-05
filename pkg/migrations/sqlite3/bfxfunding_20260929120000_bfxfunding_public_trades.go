package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/bfxfunding/migrations/sqlite3/20260929120000_bfxfunding_public_trades.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("bfxfunding", 20260929120000, "pkg/strategy/bfxfunding/migrations/sqlite3/20260929120000_bfxfunding_public_trades.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `bfxfunding_public_trades`\n(\n    `gid`         INTEGER PRIMARY KEY AUTOINCREMENT,\n    `symbol`      TEXT        NOT NULL DEFAULT '',\n    `trade_id`    BIGINT      NOT NULL,\n    `amount`      REAL        NOT NULL DEFAULT 0,\n    `rate`        REAL        NOT NULL DEFAULT 0,\n    `period`      INTEGER     NOT NULL DEFAULT 0,\n    `time`        DATETIME(3) NOT NULL,\n    `inserted_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `uidx_bfxfunding_public_trades_symbol_trade_id` ON `bfxfunding_public_trades` (`symbol`, `trade_id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_bfxfunding_public_trades_symbol_time` ON `bfxfunding_public_trades` (`symbol`, `time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_bfxfunding_public_trades_symbol_time`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `uidx_bfxfunding_public_trades_symbol_trade_id`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `bfxfunding_public_trades`;"},
		},
	)
}
