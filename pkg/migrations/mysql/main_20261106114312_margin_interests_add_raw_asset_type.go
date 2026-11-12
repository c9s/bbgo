package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20261106114312_margin_interests_add_raw_asset_type.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261106114312, "migrations/mysql/20261106114312_margin_interests_add_raw_asset_type.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests`\n    ADD COLUMN `raw_asset` VARCHAR(24) NOT NULL DEFAULT '',\n    ADD COLUMN `interest_type` VARCHAR(32) NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "UPDATE `margin_interests` SET `raw_asset` = `asset` WHERE `raw_asset` = '';"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `interest_type`, DROP COLUMN `raw_asset`;"},
		},
	)
}
