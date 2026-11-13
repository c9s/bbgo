package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20200819054742_trade_index.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20200819054742, "migrations/sqlite3/20200819054742_trade_index.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_symbol ON trades(symbol);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_symbol_fee_currency ON trades(symbol, fee_currency, traded_at);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_traded_at_symbol ON trades(traded_at, symbol);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_symbol;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_symbol_fee_currency;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_traded_at_symbol;"},
		},
	)
}
