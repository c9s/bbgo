package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20220419121046_fix_fee_column.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220419121046, "migrations/mysql/20220419121046_fix_fee_column.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE trades\n    CHANGE fee fee DECIMAL(16, 8) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE profits\n    CHANGE fee fee DECIMAL(16, 8) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE profits\n    CHANGE fee_in_usd fee_in_usd DECIMAL(16, 8);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "SELECT 1;"},
		},
	)
}
