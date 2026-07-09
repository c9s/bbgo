package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20260421214100_orders_add_original_order_id.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20260421214100, "migrations/mysql/20260421214100_orders_add_original_order_id.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `orders` ADD COLUMN `actual_order_id` BIGINT UNSIGNED NOT NULL DEFAULT 0;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `orders` DROP COLUMN `actual_order_id`;"},
		},
	)
}
