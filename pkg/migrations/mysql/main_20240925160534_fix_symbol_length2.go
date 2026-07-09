package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20240925160534_fix_symbol_length2.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20240925160534, "migrations/mysql/20240925160534_fix_symbol_length2.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE profits MODIFY COLUMN symbol VARCHAR(32) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE profits MODIFY COLUMN base_currency VARCHAR(16) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE profits MODIFY COLUMN fee_currency VARCHAR(16) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE positions MODIFY COLUMN base_currency VARCHAR(16) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE positions MODIFY COLUMN symbol VARCHAR(32) NOT NULL;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "SELECT 1;"},
		},
	)
}
