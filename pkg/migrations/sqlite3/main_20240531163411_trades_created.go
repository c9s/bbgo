package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20240531163411_trades_created.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20240531163411, "migrations/sqlite3/20240531163411_trades_created.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE trades ADD COLUMN inserted_at TEXT;\nUPDATE trades SET inserted_at = traded_at;\nCREATE TRIGGER set_inserted_at\nAFTER INSERT ON trades\nFOR EACH ROW\nBEGIN\n    UPDATE trades\n    SET inserted_at = datetime('now')\n    WHERE rowid = NEW.rowid;\nEND;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TRIGGER set_inserted_at;\nALTER TABLE trades DROP COLUMN inserted_at;"},
		},
	)
}
