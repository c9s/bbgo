package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20261116103442_add_margin_interest_valuations.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261116103442, "migrations/sqlite3/20261116103442_add_margin_interest_valuations.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `margin_interest_valuations`\n(\n    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,\n    `interest_gid`      INTEGER     NOT NULL,\n    `exchange`          TEXT        NOT NULL,\n    `session`           TEXT        NOT NULL,\n    `time`              DATETIME(3) NOT NULL,\n    `isolated_symbol`   TEXT        NOT NULL DEFAULT '',\n    `paid_asset`        TEXT        NOT NULL,\n    -- '' when unknown: a converted row without rawAsset\n    `borrowed_asset`    TEXT        NOT NULL DEFAULT '',\n    `interest_type`     TEXT        NOT NULL DEFAULT '',\n    `converted`         BOOLEAN     NOT NULL DEFAULT FALSE,\n    -- in borrowed_asset units\n    `principal`         REAL        NOT NULL,\n    -- the exchange's rate, daily on Binance\n    `interest_rate`     REAL        NOT NULL,\n    -- in paid_asset units\n    `interest`          REAL        NOT NULL,\n    -- in borrowed_asset units, estimated on converted rows; NULL when unknown\n    `borrowed_interest` REAL        NULL,\n    `price_symbol`      TEXT        NOT NULL DEFAULT '',\n    `price_kline_start` DATETIME(3) NULL,\n    -- NULL until priced\n    `price_in_usdt`     REAL        NULL,\n    `interest_usdt`     REAL        NULL,\n    -- the borrowed asset priced like the paid one, at the same kline\n    `borrowed_price_symbol`  TEXT NOT NULL DEFAULT '',\n    `borrowed_price_in_usdt` REAL NULL,\n    `borrowed_interest_usdt` REAL NULL,\n    `principal_usdt`         REAL NULL,\n    -- priced, unknown_borrowed, no_market, or kline_missing (retried)\n    `price_status`      TEXT        NOT NULL,\n    `updated_at`        DATETIME(3) NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `uidx_margin_interest_valuations_interest` ON `margin_interest_valuations` (`interest_gid`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_margin_interest_valuations_session_time` ON `margin_interest_valuations` (`exchange`, `session`, `time`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_margin_interest_valuations_session_borrowed_time` ON `margin_interest_valuations` (`exchange`, `session`, `borrowed_asset`, `time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_margin_interest_valuations_session_borrowed_time`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_margin_interest_valuations_session_time`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `uidx_margin_interest_valuations_interest`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `margin_interest_valuations`;"},
		},
	)
}
