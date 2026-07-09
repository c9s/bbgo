package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20201102222546_orders.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20201102222546, "migrations/sqlite3/20201102222546_orders.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `orders`\n(\n    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`          VARCHAR             NOT NULL DEFAULT '',\n    -- order_id is the order id returned from the exchange\n    `order_id`          INTEGER          NOT NULL,\n    `client_order_id`   VARCHAR          NOT NULL DEFAULT '',\n    `order_type`        VARCHAR          NOT NULL,\n    `symbol`            VARCHAR          NOT NULL,\n    `status`            VARCHAR          NOT NULL,\n    `time_in_force`     VARCHAR          NOT NULL,\n    `price`             DECIMAL(16, 8)  NOT NULL,\n    `stop_price`        DECIMAL(16, 8)  NOT NULL,\n    `quantity`          DECIMAL(16, 8)  NOT NULL,\n    `executed_quantity` DECIMAL(16, 8)  NOT NULL DEFAULT 0.0,\n    `side`              VARCHAR         NOT NULL DEFAULT '',\n    `is_working`        BOOLEAN         NOT NULL DEFAULT FALSE,\n    `created_at`        DATETIME(3)     NOT NULL,\n    `updated_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `orders`;"},
		},
	)
}
