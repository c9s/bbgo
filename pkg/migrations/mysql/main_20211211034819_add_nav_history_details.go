package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20211211034819_add_nav_history_details.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20211211034819, "migrations/mysql/20211211034819_add_nav_history_details.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE nav_history_details\n(\n    gid            bigint unsigned auto_increment PRIMARY KEY,\n    exchange       VARCHAR(30)                                NOT NULL,\n    subaccount     VARCHAR(30)                                NOT NULL,\n    time           DATETIME(3)                                NOT NULL,\n    currency       VARCHAR(12)                                NOT NULL,\n    balance_in_usd DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    balance_in_btc DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    balance        DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    available      DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    locked         DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX idx_nav_history_details\n    on nav_history_details (time, currency, exchange);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE nav_history_details;"},
		},
	)
}
