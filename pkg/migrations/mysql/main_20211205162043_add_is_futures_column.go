package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20211205162043_add_is_futures_column.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20211205162043, "migrations/mysql/20211205162043_add_is_futures_column.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `is_futures` BOOLEAN NOT NULL DEFAULT FALSE;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `orders` ADD COLUMN `is_futures` BOOLEAN NOT NULL DEFAULT FALSE;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` DROP COLUMN `is_futures`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `orders` DROP COLUMN `is_futures`;"},
		},
	)
}
