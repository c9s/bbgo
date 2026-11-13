package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20220317125555_fix_trade_indexes.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220317125555, "migrations/sqlite3/20220317125555_fix_trade_indexes.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "DROP INDEX IF EXISTS trades_symbol;"},
			{Direction: rockhopper.DirectionUp, SQL: "DROP INDEX IF EXISTS trades_symbol_fee_currency;"},
			{Direction: rockhopper.DirectionUp, SQL: "DROP INDEX IF EXISTS trades_traded_at_symbol;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_traded_at ON trades (traded_at, symbol, exchange, id, fee_currency, fee);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_id_traded_at ON trades (id, traded_at);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_order_id_traded_at ON trades (order_id, traded_at);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS trades_traded_at;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS trades_id_traded_at;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS trades_order_id_traded_at;"},
			{Direction: rockhopper.DirectionDown, SQL: "CREATE INDEX trades_symbol ON trades (exchange, symbol);"},
			{Direction: rockhopper.DirectionDown, SQL: "CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);"},
			{Direction: rockhopper.DirectionDown, SQL: "CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);"},
		},
	)
}
