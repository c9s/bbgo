package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20210531234123_add_kline_taker_buy_columns.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210531234123, "migrations/mysql/20210531234123_add_kline_taker_buy_columns.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `binance_klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `max_klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `okex_klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `binance_klines`\n    DROP COLUMN `quote_volume`,\n    DROP COLUMN `taker_buy_base_volume`,\n    DROP COLUMN `taker_buy_quote_volume`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `max_klines`\n    DROP COLUMN `quote_volume`,\n    DROP COLUMN `taker_buy_base_volume`,\n    DROP COLUMN `taker_buy_quote_volume`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `okex_klines`\n    DROP COLUMN `quote_volume`,\n    DROP COLUMN `taker_buy_base_volume`,\n    DROP COLUMN `taker_buy_quote_volume`;"},
		},
	)
}
