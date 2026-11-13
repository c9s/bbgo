package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20250721140334_trades_add_order_uuid.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20250721140334, "migrations/mysql/20250721140334_trades_add_order_uuid.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `order_uuid` VARBINARY(16) NOT NULL DEFAULT '';"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` DROP COLUMN `order_uuid`;"},
		},
	)
}
