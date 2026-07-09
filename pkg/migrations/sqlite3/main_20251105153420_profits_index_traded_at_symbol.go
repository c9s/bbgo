package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20251105153420_profits_index_traded_at_symbol.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20251105153420, "migrations/sqlite3/20251105153420_profits_index_traded_at_symbol.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX idx_profits_traded_at_symbol ON profits (traded_at, symbol);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX idx_profits_traded_at_symbol;"},
		},
	)
}
