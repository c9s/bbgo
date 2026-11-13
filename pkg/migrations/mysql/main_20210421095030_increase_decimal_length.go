package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20210421095030_increase_decimal_length.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210421095030, "migrations/mysql/20210421095030_increase_decimal_length.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `okex_klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `binance_klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `max_klines`\nMODIFY COLUMN `volume` decimal(20,8) unsigned NOT NULL DEFAULT '0.00000000';"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `okex_klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `binance_klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `max_klines`\nMODIFY COLUMN `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000';"},
		},
	)
}
