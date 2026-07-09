package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20220520140707_kline_unique_idx.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220520140707, "migrations/sqlite3/20220520140707_kline_unique_idx.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX idx_kline_binance_unique\n    ON binance_klines (`symbol`, `interval`, `start_time`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX idx_kline_max_unique\n    ON max_klines (`symbol`, `interval`, `start_time`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `idx_kline_ftx_unique`\n    ON ftx_klines (`symbol`, `interval`, `start_time`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `idx_kline_kucoin_unique`\n    ON kucoin_klines (`symbol`, `interval`, `start_time`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `idx_kline_okex_unique`\n    ON okex_klines (`symbol`, `interval`, `start_time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_kline_ftx_unique` ON `ftx_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_kline_max_unique` ON `max_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_kline_binance_unique` ON `binance_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_kline_kucoin_unique` ON `kucoin_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_kline_okex_unique` ON `okex_klines`;"},
		},
	)
}
