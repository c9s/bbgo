package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20210421091430_increase_symbol_length.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210421091430, "migrations/mysql/20210421091430_increase_symbol_length.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `okex_klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `binance_klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `max_klines`\nMODIFY COLUMN `symbol` VARCHAR(12) NOT NULL;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `okex_klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `binance_klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `max_klines`\nMODIFY COLUMN `symbol` VARCHAR(10) NOT NULL;"},
		},
	)
}
