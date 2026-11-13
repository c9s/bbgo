package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20240531163411_trades_created.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20240531163411, "migrations/mysql/20240531163411_trades_created.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `inserted_at` DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL AFTER `traded_at`;"},
			{Direction: rockhopper.DirectionUp, SQL: "UPDATE `trades` SET `inserted_at` = `traded_at`;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` DROP COLUMN `inserted_at`;"},
		},
	)
}
