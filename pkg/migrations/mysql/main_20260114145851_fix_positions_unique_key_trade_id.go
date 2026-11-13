package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20260114145851_fix_positions_unique_key_trade_id.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20260114145851, "migrations/mysql/20260114145851_fix_positions_unique_key_trade_id.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `positions` DROP INDEX `trade_id`;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `positions` ADD UNIQUE KEY `trade_id` (`trade_id`, `side`, `symbol`, `exchange`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `positions` DROP INDEX `trade_id`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `positions` ADD UNIQUE KEY `trade_id` (`trade_id`, `side`, `exchange`);"},
		},
	)
}
