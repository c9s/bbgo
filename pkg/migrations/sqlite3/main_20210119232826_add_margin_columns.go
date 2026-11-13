package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20210119232826_add_margin_columns.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210119232826, "migrations/sqlite3/20210119232826_add_margin_columns.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `orders` ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `orders` ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` RENAME COLUMN `is_margin` TO `is_margin_deleted`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` RENAME COLUMN `is_isolated` TO `is_isolated_deleted`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `orders` RENAME COLUMN `is_margin` TO `is_margin_deleted`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `orders` RENAME COLUMN `is_isolated` TO `is_isolated_deleted`;"},
		},
	)
}
