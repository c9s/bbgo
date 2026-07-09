package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20211211034818_add_nav_history_details.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20211211034818, "migrations/sqlite3/20211211034818_add_nav_history_details.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `nav_history_details`\n(\n    `gid`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,\n    `exchange`         VARCHAR(30)                NOT NULL DEFAULT '',\n    `subaccount`       VARCHAR(30)                NOT NULL DEFAULT '',\n    `time`             DATETIME(3)                NOT NULL DEFAULT (strftime('%s', 'now')),\n    `currency`         VARCHAR(30)                NOT NULL,\n    `net_asset_in_usd` DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `net_asset_in_btc` DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `balance`          DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `available`        DECIMAL DEFAULT 0.00000000 NOT NULL,\n    `locked`           DECIMAL DEFAULT 0.00000000 NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX idx_nav_history_details\n    on nav_history_details (time, currency, exchange);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE nav_history_details;"},
		},
	)
}
